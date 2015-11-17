package rootfs_provider_test

import (
	"errors"
	"net/url"

	"github.com/cloudfoundry-incubator/garden-shed/layercake"
	"github.com/cloudfoundry-incubator/garden-shed/layercake/fake_cake"
	"github.com/cloudfoundry-incubator/garden-shed/layercake/fake_retainer"
	"github.com/cloudfoundry-incubator/garden-shed/repository_fetcher"
	"github.com/cloudfoundry-incubator/garden-shed/rootfs_provider"
	"github.com/cloudfoundry-incubator/garden-shed/rootfs_provider/fakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("The Cake Co-ordinator", func() {
	var (
		fakeFetcher      *fakes.FakeRepositoryFetcher
		fakeLayerCreator *fakes.FakeLayerCreator
		fakeCake         *fake_cake.FakeCake
		fakeRetainer     *fake_retainer.FakeRetainer
		logger           *lagertest.TestLogger

		cakeOrdinator *rootfs_provider.CakeOrdinator
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeFetcher = new(fakes.FakeRepositoryFetcher)

		fakeRetainer = new(fake_retainer.FakeRetainer)
		fakeLayerCreator = new(fakes.FakeLayerCreator)
		fakeCake = new(fake_cake.FakeCake)
		cakeOrdinator = rootfs_provider.NewCakeOrdinator(fakeCake, fakeFetcher, fakeLayerCreator, fakeRetainer, logger)
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
				rootfsPath, envs, err := cakeOrdinator.Create("container-id", spec)
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
				_, _, err := cakeOrdinator.Create("", rootfs_provider.Spec{
					RootFS:     nil,
					Namespaced: true,
					QuotaSize:  12,
				})
				Expect(err).To(MatchError("amadeus"))
			})
		})

		Context("when the quota scope is exclusive", func() {
			It("disables quota for the fetcher", func() {
				_, _, err := cakeOrdinator.Create("", rootfs_provider.Spec{
					RootFS:     &url.URL{},
					Namespaced: false,
					QuotaSize:  33,
					QuotaScope: rootfs_provider.QuotaScopeExclusive,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeFetcher.FetchCallCount()).To(Equal(1))
				_, diskQuota := fakeFetcher.FetchArgsForCall(0)
				Expect(diskQuota).To(BeNumerically("==", 0))
			})
		})

		Context("when the quota scope is total", func() {
			It("passes down the same quota number to the fetcher", func() {
				_, _, err := cakeOrdinator.Create("", rootfs_provider.Spec{
					RootFS:     &url.URL{},
					Namespaced: false,
					QuotaSize:  33,
					QuotaScope: rootfs_provider.QuotaScopeTotal,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeFetcher.FetchCallCount()).To(Equal(1))
				_, diskQuota := fakeFetcher.FetchArgsForCall(0)
				Expect(diskQuota).To(BeNumerically("==", 33))
			})
		})
	})

	Describe("Retain", func() {
		It("can be retained by Retainer", func() {
			retainedId := layercake.ContainerID("banana")
			cakeOrdinator.Retain(retainedId)

			Expect(fakeRetainer.RetainCallCount()).To(Equal(1))
			var id layercake.ID = fakeRetainer.RetainArgsForCall(0)
			Expect(id).To(Equal(retainedId))
		})
	})

	Describe("Remove", func() {
		It("delegates removals", func() {
			fakeCake.RemoveReturns(errors.New("returned-error"))

			err := cakeOrdinator.Remove(layercake.DockerImageID("something"))
			Expect(err).To(MatchError("returned-error"))
		})

		It("prevents concurrent garbage collection and creation", func() {
			removeStarted := make(chan struct{})
			removeReturns := make(chan struct{})
			fakeCake.RemoveStub = func(id layercake.ID) error {
				close(removeStarted)
				<-removeReturns
				return nil
			}

			go cakeOrdinator.Remove(layercake.DockerImageID(""))
			<-removeStarted
			go cakeOrdinator.Create("", rootfs_provider.Spec{
				RootFS:     &url.URL{},
				Namespaced: false,
				QuotaSize:  33,
			})

			Consistently(fakeFetcher.FetchCallCount).Should(Equal(0))
			close(removeReturns)
			Eventually(fakeFetcher.FetchCallCount).Should(Equal(1))
		})
	})

	It("allows concurrent creation as long as deletion is not ongoing", func() {
		fakeBlocks := make(chan struct{})
		fakeFetcher.FetchStub = func(*url.URL, int64) (*repository_fetcher.Image, error) {
			<-fakeBlocks
			return nil, nil
		}

		go cakeOrdinator.Create("", rootfs_provider.Spec{
			RootFS:     &url.URL{},
			Namespaced: false,
			QuotaSize:  33,
		})
		go cakeOrdinator.Create("", rootfs_provider.Spec{
			RootFS:     &url.URL{},
			Namespaced: false,
			QuotaSize:  33,
		})

		Eventually(fakeFetcher.FetchCallCount).Should(Equal(2))
		close(fakeBlocks)
	})
})
