package rootfs_provider

import (
	"net/url"

	"github.com/cloudfoundry-incubator/garden-shed/layercake"
)

type QuotaScope int

const (
	QuotaScopeTotal QuotaScope = iota
	QuotaScopeExclusive
)

type Spec struct {
	RootFS     *url.URL
	Namespaced bool
	QuotaSize  int64
	QuotaScope QuotaScope
}

//go:generate counterfeiter -o fake_rootfs_provider/fake_rootfs_provider.go . RootFSProvider
type RootFSProvider interface {
	Create(id string, spec Spec) (mountpoint string, envvar []string, err error)
	Remove(id layercake.ID) error
}

type Graph interface {
	layercake.Cake
}
