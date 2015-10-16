package repository_fetcher

import (
	"fmt"
	"net/url"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-shed/layercake"
)

type CompositeFetcher struct {
	// fetcher used for requests without a scheme
	LocalFetcher RepositoryFetcher

	// fetchers used for docker:// urls, depending on the version
	RemoteFetcher RepositoryFetcher
}

func (f *CompositeFetcher) Fetch(repoURL *url.URL, diskQuota int64) (*Image, error) {
	if repoURL.Scheme == "" {
		return f.LocalFetcher.Fetch(repoURL, diskQuota)
	}

	return f.RemoteFetcher.Fetch(repoURL, diskQuota)
}

func (f *CompositeFetcher) FetchID(repoURL *url.URL) (layercake.ID, error) {
	if repoURL.Scheme == "" {
		return f.LocalFetcher.FetchID(repoURL)
	}

	return f.RemoteFetcher.FetchID(repoURL)
}

type dockerImage struct {
	layers []*dockerLayer
}

func (d dockerImage) Env() []string {
	var envs []string
	for _, l := range d.layers {
		envs = append(envs, l.env...)
	}

	return envs
}

func (d dockerImage) Vols() []string {
	var vols []string
	for _, l := range d.layers {
		vols = append(vols, l.vols...)
	}

	return vols
}

type dockerLayer struct {
	env  []string
	vols []string
	size int64
}

func FetchError(context, registry, reponame string, err error) error {
	return garden.NewServiceUnavailableError(fmt.Sprintf("repository_fetcher: %s: could not fetch image %s from registry %s: %s", context, reponame, registry, err))
}
