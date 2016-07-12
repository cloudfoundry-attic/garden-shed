package rootfs_provider

import (
	"net/url"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-shed/layercake"
	"github.com/cloudfoundry-incubator/garden-shed/repository_fetcher"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . LayerCreator
type LayerCreator interface {
	Create(log lager.Logger, id string, parentImage *repository_fetcher.Image, spec Spec) (string, []string, error)
}

//go:generate counterfeiter . RepositoryFetcher
type RepositoryFetcher interface {
	Fetch(*url.URL, int64) (*repository_fetcher.Image, error)
}

//go:generate counterfeiter . GCer
type GCer interface {
	GC(log lager.Logger, cake layercake.Cake) error
}

//go:generate counterfeiter . Metricser
type Metricser interface {
	Metrics(logger lager.Logger, id layercake.ID) (garden.ContainerDiskStat, error)
}

// CakeOrdinator manages a cake, fetching layers as neccesary
type CakeOrdinator struct {
	mu sync.RWMutex

	cake         layercake.Cake
	fetcher      RepositoryFetcher
	layerCreator LayerCreator
	metrics      Metricser
	gc           GCer
}

// New creates a new cake-ordinator, there should only be one CakeOrdinator
// for a particular cake.
func NewCakeOrdinator(cake layercake.Cake, fetcher RepositoryFetcher, layerCreator LayerCreator, metrics Metricser, gc GCer) *CakeOrdinator {
	return &CakeOrdinator{
		cake:         cake,
		fetcher:      fetcher,
		layerCreator: layerCreator,
		metrics:      metrics,
		gc:           gc,
	}
}

func (c *CakeOrdinator) Create(logger lager.Logger, id string, spec Spec) (string, []string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fetcherDiskQuota := spec.QuotaSize
	if spec.QuotaScope == garden.DiskLimitScopeExclusive {
		fetcherDiskQuota = 0
	}

	image, err := c.fetcher.Fetch(spec.RootFS, fetcherDiskQuota)
	if err != nil {
		return "", nil, err
	}

	return c.layerCreator.Create(logger, id, image, spec)
}

func (c *CakeOrdinator) Metrics(logger lager.Logger, id string) (garden.ContainerDiskStat, error) {
	cid := layercake.ContainerID(id)
	return c.metrics.Metrics(logger, cid)
}

func (c *CakeOrdinator) Destroy(logger lager.Logger, id string) error {
	cid := layercake.ContainerID(id)
	if _, err := c.cake.Get(cid); err != nil {
		logger.Info("layer-already-deleted-skipping", lager.Data{"id": id, "graphID": cid, "error": err.Error()})
		return nil
	}

	return c.cake.Remove(cid)
}

func (c *CakeOrdinator) GC(logger lager.Logger) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.gc.GC(logger, c.cake)
}
