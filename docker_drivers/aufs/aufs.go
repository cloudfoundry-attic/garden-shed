package aufs

import (
	"path/filepath"

	"github.com/docker/docker/daemon/graphdriver"
)

//go:generate counterfeiter . GraphDriver
type GraphDriver interface {
	graphdriver.Driver
}

//go:generate counterfeiter . LoopMounter
type LoopMounter interface {
	MountFile(filePath, destPath string) error
	Unmount(path string) error
}

//go:generate counterfeiter . BackingStoreMgr
type BackingStoreMgr interface {
	Create(id string, quota int64) (string, error)
	Delete(id string) error
}

type QuotaedDriver struct {
	GraphDriver
	BackingStoreMgr BackingStoreMgr
	LoopMounter     LoopMounter
	RootPath        string
}

func (a *QuotaedDriver) GetQuotaed(id, mountlabel string, quota int64) (string, error) {
	path := filepath.Join(a.RootPath, "aufs", "diff", id)

	bsPath, err := a.BackingStoreMgr.Create(id, quota)
	if err != nil {
		return "", err
	}

	if err := a.LoopMounter.MountFile(bsPath, path); err != nil {
		return "", err
	}

	return a.GraphDriver.Get(id, mountlabel)
}

func (a *QuotaedDriver) Remove(id string) error {
	path := filepath.Join(a.RootPath, "aufs", "diff", id)

	a.GraphDriver.Put(id)
	a.GraphDriver.Remove(id)

	a.LoopMounter.Unmount(path)
	a.BackingStoreMgr.Delete(id)

	a.GraphDriver.Put(id)
	a.GraphDriver.Remove(id)

	return nil
}
