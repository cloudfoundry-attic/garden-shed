package rootfs_provider

import (
	"net/url"

	"github.com/cloudfoundry-incubator/garden-shed/layercake"
)

//go:generate counterfeiter -o fake_rootfs_provider/fake_rootfs_provider.go . RootFSProvider
type RootFSProvider interface {
	Create(id string, rootfs *url.URL, namespaced bool, quota int64) (mountpoint string, envvar []string, err error)
	Remove(id layercake.ID) error
}

type Graph interface {
	layercake.Cake
}