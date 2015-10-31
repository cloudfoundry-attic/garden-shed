package aufs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pivotal-golang/lager"
)

type BackingStore struct {
	Logger   lager.Logger
	RootPath string
}

func (bm *BackingStore) Create(id string, quota int64) (string, error) {
	log := bm.Logger.Session("create", lager.Data{"id": id, "quota": quota})

	path := bm.backingStorePath(id)
	f, err := os.Create(path)
	if err != nil {
		log.Error("create-file", err, lager.Data{"path": path})
		return "", fmt.Errorf("creating the backing store file: %s", err)
	}
	f.Close()

	if quota == 0 {
		log.Info("quota-is-zero")
		return "", errors.New("cannot have zero sized quota")
	}

	if err := os.Truncate(path, quota); err != nil {
		log.Error("truncating-file", err, lager.Data{"path": path})
		return "", fmt.Errorf("truncating the file returned error: %s", err)
	}

	output, err := exec.Command("mkfs.ext4", "-F", path).CombinedOutput()
	if err != nil {
		log.Error("formatting-file", err, lager.Data{"path": path, "output": output})
		return "", fmt.Errorf("formatting filesystem (%s): %s", err, output)
	}

	return path, nil
}

func (bm *BackingStore) Delete(id string) error {
	log := bm.Logger.Session("delete", lager.Data{"id": id})

	if err := os.RemoveAll(bm.backingStorePath(id)); err != nil {
		log.Error("deleteing-file", err, lager.Data{"path": bm.backingStorePath(id)})
		return fmt.Errorf("deleting backing store file: %s", id, err)
	}

	return nil
}

func (bm *BackingStore) backingStorePath(id string) string {
	return filepath.Join(bm.RootPath, id)
}
