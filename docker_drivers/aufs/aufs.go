package aufs

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/docker/docker/daemon/graphdriver"
)

const aufsRetryCount = 100

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
		if err2 := a.BackingStoreMgr.Delete(id); err2 != nil {
			return "", fmt.Errorf("cleaning backing store beacause of %s: %s", err, err2)
		}

		return "", err
	}

	mntPath, err := a.GraphDriver.Get(id, mountlabel)
	if err != nil {
		if err2 := a.LoopMounter.Unmount(path); err2 != nil {
			return "", fmt.Errorf("unmounting the loop device because of %s: %s", err, err2)
		}
		if err2 := a.BackingStoreMgr.Delete(id); err2 != nil {
			return "", fmt.Errorf("cleaning backing store beacause of %s: %s", err, err2)
		}

		return "", err
	}

	return mntPath, nil
}

func (a *QuotaedDriver) Remove(id string) error {
	path := filepath.Join(a.RootPath, "aufs", "diff", id)

	var err error
	for i := 0; i < aufsRetryCount; i++ {
		err = a.GraphDriver.Put(id)
		if err == nil {
			break
		}

		time.Sleep(time.Millisecond * 50)
	}

	if err != nil {
		return err
	}

	if err := a.LoopMounter.Unmount(path); err != nil {
		return err
	}

	if err := a.BackingStoreMgr.Delete(id); err != nil {
		return err
	}

	return a.GraphDriver.Remove(id)
}
