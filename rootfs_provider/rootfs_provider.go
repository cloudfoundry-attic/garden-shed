package rootfs_provider

import (
	"net/url"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/layercake"
)

type Spec struct {
	RootFS     *url.URL
	Username   string
	Password   string
	Namespaced bool
	QuotaSize  int64
	QuotaScope garden.DiskLimitScope
}

type Graph interface {
	layercake.Cake
}
