package rootfs_spec

import (
	"net/url"

	"code.cloudfoundry.org/garden"
)

type Spec struct {
	RootFS     *url.URL
	Username   string `json:"-"`
	Password   string `json:"-"`
	Namespaced bool
	QuotaSize  int64
	QuotaScope garden.DiskLimitScope
}
