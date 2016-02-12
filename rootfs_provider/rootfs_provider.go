package rootfs_provider

import (
	"net/url"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-shed/layercake"
)

type Spec struct {
	RootFS     *url.URL
	Namespaced bool
	QuotaSize  int64
	QuotaScope garden.DiskLimitScope
}

type Graph interface {
	layercake.Cake
}
