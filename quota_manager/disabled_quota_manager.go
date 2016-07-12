package quota_manager

import (
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-incubator/garden"
)

type DisabledQuotaManager struct{}

func (DisabledQuotaManager) SetLimits(logger lager.Logger, containerRootFSPath string, limits garden.DiskLimits) error {
	return nil
}

func (DisabledQuotaManager) GetLimits(logger lager.Logger, containerRootFSPath string) (garden.DiskLimits, error) {
	return garden.DiskLimits{}, nil
}

func (DisabledQuotaManager) GetUsage(logger lager.Logger, containerRootFSPath string) (garden.ContainerDiskStat, error) {
	return garden.ContainerDiskStat{}, nil
}

func (DisabledQuotaManager) Setup() error {
	return nil
}
