package rootfs_provider

import (
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-shed/layercake"
)

type GetUsageFunc func(logger lager.Logger, rootfsPath string) (garden.ContainerDiskStat, error)

// MetricsAdapter implements cakeordinator.Metricser using existing quota_manager.GetUsage func
type MetricsAdapter struct {
	fn      GetUsageFunc
	id2path func(layercake.ID) string
}

func NewMetricsAdapter(fn GetUsageFunc, id2path func(layercake.ID) string) MetricsAdapter {
	return MetricsAdapter{fn, id2path}
}

func (m MetricsAdapter) Metrics(logger lager.Logger, id layercake.ID) (garden.ContainerDiskStat, error) {
	return m.fn(logger, m.id2path(id))
}
