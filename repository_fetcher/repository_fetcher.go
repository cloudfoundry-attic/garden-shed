package repository_fetcher

import (
	"errors"
	"io"
	"net/url"

	"github.com/cloudfoundry-incubator/garden-shed/layercake"
	"github.com/docker/distribution"
	"github.com/docker/docker/registry"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter -o fake_registry_provider/fake_registry_provider.go . RegistryProvider
type RegistryProvider interface {
	ProvideRegistry(hostname string) (*registry.Session, *registry.Endpoint, error)
}

//go:generate counterfeiter -o fake_lock/FakeLock.go . Lock
type Lock interface {
	Acquire(key string)
	Release(key string) error
}

// apes docker's *registry.Registry
type Registry interface {
	// v1 methods
	GetRepositoryData(repoName string) (*registry.RepositoryData, error)
	GetRemoteTags(registries []string, repository string) (map[string]string, error)
	GetRemoteHistory(imageID string, registry string) ([]string, error)
	GetRemoteImageJSON(imageID string, registry string) ([]byte, int, error)
	GetRemoteImageLayer(imageID string, registry string, size int64) (io.ReadCloser, error)
}

type RemoteFetcher interface {
	Fetch(request *FetchRequest) (*Image, error)
}

//go:generate counterfeiter . RepositoryFetcher
type RepositoryFetcher interface {
	Fetch(u *url.URL, diskQuota int64) (*Image, error)
	FetchID(u *url.URL) (layercake.ID, error)
}

type FetchRequest struct {
	Session    *registry.Session
	Endpoint   *registry.Endpoint
	Repository distribution.Repository
	Path       string
	RemotePath string
	Tag        string
	Logger     lager.Logger
	MaxSize    int64
}

type Image struct {
	ImageID string
	Env     []string
	Volumes []string
}

var ErrInvalidDockerURL = errors.New("invalid docker url")

// apes dockers registry.NewEndpoint
var RegistryNewEndpoint = registry.NewEndpoint

// apes dockers registry.NewSession
var RegistryNewSession = registry.NewSession
