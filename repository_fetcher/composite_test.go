package repository_fetcher_test

import (
	"net/url"

	. "github.com/cloudfoundry-incubator/garden-shed/repository_fetcher"
	"github.com/cloudfoundry-incubator/garden-shed/repository_fetcher/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FetcherFactory", func() {
	var (
		fakeLocalFetcher *fakes.FakeRepositoryFetcher
		factory          *CompositeFetcher
	)

	BeforeEach(func() {
		fakeLocalFetcher = new(fakes.FakeRepositoryFetcher)

		factory = &CompositeFetcher{
			LocalFetcher: fakeLocalFetcher,
		}
	})

	Context("when the URL does not contain a scheme", func() {
		It("delegates .Fetch to the local fetcher", func() {
			factory.Fetch(&url.URL{Path: "cake"}, 24)
			Expect(fakeLocalFetcher.FetchCallCount()).To(Equal(1))
		})

		It("delegates .FetchID to the local fetcher", func() {
			factory.FetchID(&url.URL{Path: "cake"})
			Expect(fakeLocalFetcher.FetchIDCallCount()).To(Equal(1))
		})
	})

	PIt("when the scheme is docker://", func() {})
})
