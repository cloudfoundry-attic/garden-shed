package rootfs_provider

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/garden-shed/distclient"
	quotaed_aufs "code.cloudfoundry.org/garden-shed/docker_drivers/aufs"
	"code.cloudfoundry.org/garden-shed/layercake"
	"code.cloudfoundry.org/garden-shed/layercake/cleaner"
	"code.cloudfoundry.org/garden-shed/quota_manager"
	"code.cloudfoundry.org/garden-shed/repository_fetcher"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/idmapper"
	"code.cloudfoundry.org/lager"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/graph"
	"github.com/eapache/go-resiliency/retrier"
)

type Graph interface {
	layercake.Cake
}

func Wire(
	logger lager.Logger,
	runner *logging.Runner,
	graphRoot string,
	rootFS string,
	dockerRegistry string,
	insecureRegistries []string,
	persistentImages []string,
	cleanupThresholdInMegabytes int,
	uidMappings idmapper.MappingList,
	gidMappings idmapper.MappingList,
) *CakeOrdinator {
	logger = logger.Session(gardener.VolumizerSession, lager.Data{"graphRoot": graphRoot})

	if err := exec.Command("modprobe", "aufs").Run(); err != nil {
		logger.Error("unable-to-load-aufs", err)
	}

	if err := os.MkdirAll(graphRoot, 0755); err != nil {
		logger.Fatal("failed-to-create-graph-directory", err)
	}

	dockerGraphDriver, err := graphdriver.New(graphRoot, nil)
	if err != nil {
		logger.Fatal("failed-to-construct-graph-driver", err)
	}

	backingStoresPath := filepath.Join(graphRoot, "backing_stores")
	if mkdirErr := os.MkdirAll(backingStoresPath, 0660); mkdirErr != nil {
		logger.Fatal("failed-to-mkdir-backing-stores", mkdirErr)
	}

	quotaedGraphDriver := &quotaed_aufs.QuotaedDriver{
		GraphDriver: dockerGraphDriver,
		Unmount:     quotaed_aufs.Unmount,
		BackingStoreMgr: &quotaed_aufs.BackingStore{
			RootPath: backingStoresPath,
			Logger:   logger.Session("backing-store-mgr"),
		},
		LoopMounter: &quotaed_aufs.Loop{
			Retrier: retrier.New(retrier.ConstantBackoff(200, 500*time.Millisecond), nil),
			Logger:  logger.Session("loop-mounter"),
		},
		Retrier:  retrier.New(retrier.ConstantBackoff(200, 500*time.Millisecond), nil),
		RootPath: graphRoot,
		Logger:   logger.Session("quotaed-driver"),
	}

	dockerGraph, err := graph.NewGraph(graphRoot, quotaedGraphDriver)
	if err != nil {
		logger.Fatal("failed-to-construct-graph", err)
	}

	var cake layercake.Cake = &layercake.Docker{
		Graph:  dockerGraph,
		Driver: quotaedGraphDriver,
	}

	if cake.DriverName() == "aufs" {
		cake = &layercake.AufsCake{
			Cake:      cake,
			Runner:    runner,
			GraphRoot: graphRoot,
		}
	}

	repoFetcher := repository_fetcher.Retryable{
		RepositoryFetcher: &repository_fetcher.CompositeFetcher{
			LocalFetcher: &repository_fetcher.Local{
				Cake:              cake,
				DefaultRootFSPath: rootFS,
				IDProvider:        repository_fetcher.LayerIDProvider{},
			},
			RemoteFetcher: repository_fetcher.NewRemote(
				dockerRegistry,
				cake,
				distclient.NewDialer(insecureRegistries),
				repository_fetcher.VerifyFunc(repository_fetcher.Verify),
			),
		},
	}

	rootFSNamespacer := &UidNamespacer{
		Translator: NewUidTranslator(
			uidMappings,
			gidMappings,
		),
	}

	retainer := cleaner.NewRetainer()
	ovenCleaner := cleaner.NewOvenCleaner(retainer,
		cleaner.NewThreshold(int64(cleanupThresholdInMegabytes)*1024*1024),
	)

	imageRetainer := &repository_fetcher.ImageRetainer{
		GraphRetainer:             retainer,
		DirectoryRootfsIDProvider: repository_fetcher.LayerIDProvider{},
		DockerImageIDFetcher:      repoFetcher,

		NamespaceCacheKey: rootFSNamespacer.CacheKey(),
		Logger:            logger,
	}

	// spawn off in a go function to avoid blocking startup
	// worst case is if an image is immediately created and deleted faster than
	// we can retain it we'll garbage collect it when we shouldn't. This
	// is an OK trade-off for not having garden startup block on dockerhub.
	go imageRetainer.Retain(persistentImages)

	layerCreator := NewLayerCreator(cake, SimpleVolumeCreator{}, rootFSNamespacer)

	quotaManager := &quota_manager.AUFSQuotaManager{
		BaseSizer: quota_manager.NewAUFSBaseSizer(cake),
		DiffSizer: &quota_manager.AUFSDiffSizer{
			AUFSDiffPathFinder: quotaedGraphDriver,
		},
	}

	return NewCakeOrdinator(cake,
		repoFetcher,
		layerCreator,
		NewMetricsAdapter(quotaManager.GetUsage, quotaedGraphDriver.GetMntPath),
		ovenCleaner)
}
