package repository_fetcher

import (
	"net/url"

	"github.com/cloudfoundry-incubator/garden-shed/layercake"
	"github.com/pivotal-golang/lager"
)

const MAX_ATTEMPTS = 3

type Retryable struct {
	RepositoryFetcher interface {
		Fetch(*url.URL, int64) (*Image, error)
		FetchID(*url.URL) (layercake.ID, error)
	}

	Logger lager.Logger
}

func (retryable Retryable) Fetch(repoName *url.URL, diskQuota int64) (*Image, error) {
	var err error
	var response *Image
	for attempt := 1; attempt <= MAX_ATTEMPTS; attempt++ {
		response, err = retryable.RepositoryFetcher.Fetch(repoName, diskQuota)
		if err == nil {
			break
		}

		retryable.Logger.Error("failed-to-fetch", err, lager.Data{
			"attempt": attempt,
			"of":      MAX_ATTEMPTS,
		})
	}

	return response, err
}

func (retryable Retryable) FetchID(repoURL *url.URL) (layercake.ID, error) {
	var err error
	var response layercake.ID
	for attempt := 1; attempt <= MAX_ATTEMPTS; attempt++ {
		response, err = retryable.RepositoryFetcher.FetchID(repoURL)
		if err == nil {
			break
		}

		retryable.Logger.Error("failed-to-fetch-ID", err, lager.Data{
			"attempt": attempt,
			"of":      MAX_ATTEMPTS,
		})
	}

	return response, err
}
