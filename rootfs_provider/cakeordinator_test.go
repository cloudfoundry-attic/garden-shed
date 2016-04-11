package rootfs_provider_test

import (
	"errors"
	"net/url"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-shed/layercake"
	"github.com/cloudfoundry-incubator/garden-shed/layercake/fake_cake"
	"github.com/cloudfoundry-incubator/garden-shed/repository_fetcher"
	"github.com/cloudfoundry-incubator/garden-shed/rootfs_provider"
	"github.com/cloudfoundry-incubator/garden-shed/rootfs_provider/fakes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("The Cake Co-ordinator", func() {
	var (
		fakeFetcher      *fakes.FakeRepositoryFetcher
		fakeLayerCreator *fakes.FakeLayerCreator
		fakeCake         *fake_cake.FakeCake
		fakeGCer         *fakes.FakeGCer
		fakeMetrics      *fakes.FakeMetricser
		logger           *lagertest.TestLogger

		cakeOrdinator *rootfs_provider.CakeOrdinator
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeFetcher = new(fakes.FakeRepositoryFetcher)

		fakeLayerCreator = new(fakes.FakeLayerCreator)
		fakeCake = new(fake_cake.FakeCake)
		fakeGCer = new(fakes.FakeGCer)
		fakeMetrics = new(fakes.FakeMetricser)
		cakeOrdinator = rootfs_provider.NewCakeOrdinator(fakeCake, fakeFetcher, fakeLayerCreator, fakeMetrics, fakeGCer)
	})

	Describe("creating container layers", func() {
		Context("When the image is succesfully fetched", func() {
			It("creates a container layer on top of the fetched layer", func() {
				image := &repository_fetcher.Image{ImageID: "my cool image"}
				fakeFetcher.FetchReturns(image, nil)
				fakeLayerCreator.CreateReturns("potato", []string{"foo=bar"}, errors.New("cake"))

				spec := rootfs_provider.Spec{
					RootFS:     &url.URL{Path: "parent"},
					Namespaced: true,
					QuotaSize:  55,
				}
				rootfsPath, envs, err := cakeOrdinator.Create(logger, "container-id", spec)
				Expect(rootfsPath).To(Equal("potato"))
				Expect(envs).To(Equal([]string{"foo=bar"}))
				Expect(err).To(MatchError("cake"))

				Expect(fakeLayerCreator.CreateCallCount()).To(Equal(1))
				containerID, parentImage, layerCreatorSpec := fakeLayerCreator.CreateArgsForCall(0)
				Expect(containerID).To(Equal("container-id"))
				Expect(parentImage).To(Equal(image))
				Expect(layerCreatorSpec).To(Equal(spec))
			})
		})

		Context("when fetching fails", func() {
			It("returns an error", func() {
				fakeFetcher.FetchReturns(nil, errors.New("amadeus"))
				_, _, err := cakeOrdinator.Create(logger, "", rootfs_provider.Spec{
					RootFS:     nil,
					Namespaced: true,
					QuotaSize:  12,
				})
				Expect(err).To(MatchError("amadeus"))
			})
		})

		Context("when the quota scope is exclusive", func() {
			It("disables quota for the fetcher", func() {
				_, _, err := cakeOrdinator.Create(logger, "", rootfs_provider.Spec{
					RootFS:     &url.URL{},
					Namespaced: false,
					QuotaSize:  33,
					QuotaScope: garden.DiskLimitScopeExclusive,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeFetcher.FetchCallCount()).To(Equal(1))
				_, diskQuota := fakeFetcher.FetchArgsForCall(0)
				Expect(diskQuota).To(BeNumerically("==", 0))
			})
		})

		Context("when the quota scope is total", func() {
			It("passes down the same quota number to the fetcher", func() {
				_, _, err := cakeOrdinator.Create(logger, "", rootfs_provider.Spec{
					RootFS:     &url.URL{},
					Namespaced: false,
					QuotaSize:  33,
					QuotaScope: garden.DiskLimitScopeTotal,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeFetcher.FetchCallCount()).To(Equal(1))
				_, diskQuota := fakeFetcher.FetchArgsForCall(0)
				Expect(diskQuota).To(BeNumerically("==", 33))
			})
		})
	})

	Describe("Metrics", func() {
		It("delegates metrics retrieval to the metricser", func() {
			fakeMetrics.MetricsReturns(garden.ContainerDiskStat{
				TotalBytesUsed: 12,
			}, nil)

			metrics, err := cakeOrdinator.Metrics(logger, "something")
			Expect(err).NotTo(HaveOccurred())

			_, path := fakeMetrics.MetricsArgsForCall(0)
			Expect(path).To(Equal(layercake.ContainerID("something")))

			Expect(metrics).To(Equal(garden.ContainerDiskStat{
				TotalBytesUsed: 12,
			}))
		})

		Context("when it fails to get the metrics", func() {
			BeforeEach(func() {
				fakeMetrics.MetricsReturns(garden.ContainerDiskStat{}, errors.New("rotten banana"))
			})

			It("should return an error", func() {
				_, err := cakeOrdinator.Metrics(logger, "something")
				Expect(err).To(MatchError("rotten banana"))
			})
		})
	})

	Describe("Destroy", func() {
		It("delegates removal", func() {
			Expect(cakeOrdinator.Destroy(logger, "something")).To(Succeed())
			Expect(fakeCake.RemoveCallCount()).To(Equal(1))
			Expect(fakeCake.RemoveArgsForCall(0)).To(Equal(layercake.ContainerID("something")))
		})

		Context("when the layer is already destroyed", func() {
			It("does not destroy again", func() {
				fakeCake.GetReturns(nil, errors.New("cannae find it"))

				Expect(cakeOrdinator.Destroy(logger, "something")).To(Succeed())
				Expect(fakeCake.RemoveCallCount()).To(Equal(0))
			})
		})

		Context("when the graph can't retrieve information about the layer", func() {
			BeforeEach(func() {
				fakeCake.GetReturns(nil, errors.New("failed tae find it"))
			})

			It("skips destroy and logs the error instead", func() {
				Expect(cakeOrdinator.Destroy(logger, "something")).To(Succeed())
				Expect(logger.LogMessages()).To(ContainElement("test.layer-already-deleted-skipping"))
			})
		})
	})

	Describe("GC", func() {
		It("delegates GC", func() {
			Expect(cakeOrdinator.GC(logger)).To(Succeed())
			Expect(fakeGCer.GCCallCount()).To(Equal(1))

			_, cake := fakeGCer.GCArgsForCall(0)
			Expect(cake).To(Equal(fakeCake))
		})

		It("prevents concurrent garbage collection and creation", func() {
			gcStarted := make(chan struct{})
			gcReturns := make(chan struct{})
			fakeGCer.GCStub = func(_ lager.Logger, _ layercake.Cake) error {
				close(gcStarted)
				<-gcReturns
				return nil
			}

			go cakeOrdinator.GC(logger)
			<-gcStarted

			go cakeOrdinator.Create(logger, "", rootfs_provider.Spec{
				RootFS:     &url.URL{},
				Namespaced: false,
				QuotaSize:  33,
			})

			Consistently(fakeFetcher.FetchCallCount).Should(Equal(0))
			close(gcReturns)
			Eventually(fakeFetcher.FetchCallCount).Should(Equal(1))
		})
	})

	It("allows concurrent creation as long as deletion is not ongoing", func() {
		fakeBlocks := make(chan struct{})
		fakeFetcher.FetchStub = func(*url.URL, int64) (*repository_fetcher.Image, error) {
			<-fakeBlocks
			return nil, nil
		}

		go cakeOrdinator.Create(logger, "", rootfs_provider.Spec{
			RootFS:     &url.URL{},
			Namespaced: false,
			QuotaSize:  33,
		})
		go cakeOrdinator.Create(logger, "", rootfs_provider.Spec{
			RootFS:     &url.URL{},
			Namespaced: false,
			QuotaSize:  33,
		})

		Eventually(fakeFetcher.FetchCallCount).Should(Equal(2))
		close(fakeBlocks)
	})
})
