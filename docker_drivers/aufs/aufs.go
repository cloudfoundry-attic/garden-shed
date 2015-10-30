package aufs

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/pivotal-golang/lager"
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
	Logger          lager.Logger
}

func (a *QuotaedDriver) GetQuotaed(id, mountlabel string, quota int64) (string, error) {
	path := filepath.Join(a.RootPath, "aufs", "diff", id)
	log := a.Logger.Session("get-quotaed", lager.Data{"id": id, "mountlabel": mountlabel, "quota": quota, "path": path})

	bsPath, err := a.BackingStoreMgr.Create(id, quota)
	if err != nil {
		log.Error("bs-create", err)
		return "", err
	}

	if err := a.LoopMounter.MountFile(bsPath, path); err != nil {
		log.Error("loop-mount", err)
		if err2 := a.BackingStoreMgr.Delete(id); err2 != nil {
			log.Error("bs-delete", err)
			return "", fmt.Errorf("cleaning backing store beacause of %s: %s", err, err2)
		}

		return "", err
	}

	mntPath, err := a.GraphDriver.Get(id, mountlabel)
	if err != nil {
		log.Error("driver-get", err)
		if err2 := a.LoopMounter.Unmount(path); err2 != nil {
			log.Error("loop-unmount", err)
			return "", fmt.Errorf("unmounting the loop device because of %s: %s", err, err2)
		}
		if err2 := a.BackingStoreMgr.Delete(id); err2 != nil {
			log.Error("bs-delete", err)
			return "", fmt.Errorf("cleaning backing store beacause of %s: %s", err, err2)
		}

		return "", err
	}

	return mntPath, nil
}

func (a *QuotaedDriver) Remove(id string) error {
	path := filepath.Join(a.RootPath, "aufs", "diff", id)
	log := a.Logger.Session("get-quotaed", lager.Data{"id": id, "path": path})

	var err error
	for i := 0; i < aufsRetryCount; i++ {
		err = a.GraphDriver.Put(id)
		if err == nil {
			log.Info("driver-put-succeed")
			break
		}
		log.Error("driver-put-failed-in-loop", err)

		time.Sleep(time.Millisecond * 50)
	}

	if err != nil {
		log.Error("driver-put-failed", err)
		return err
	}

	if err := a.LoopMounter.Unmount(path); err != nil {
		log.Error("loop-unmount", err)
		return err
	}

	if err := a.BackingStoreMgr.Delete(id); err != nil {
		log.Error("bs-delete", err)
		return err
	}

	if err := a.GraphDriver.Remove(id); err != nil {
		log.Error("driver-remove", err)
		return err
	}

	return nil
}