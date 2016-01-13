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

type Graph interface {
	layercake.Cake
}
