package aufs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type BackingStore struct {
	RootPath string
}

func (bm *BackingStore) Create(id string, quota int64) (string, error) {
	path := bm.backingStorePath(id)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("creating the backing store file: %s", err)
	}
	f.Close()

	if quota == 0 {
		return "", errors.New("cannot have zero sized quota")
	}

	if err := os.Truncate(path, quota); err != nil {
		return "", fmt.Errorf("truncating the file returned error: %s", err)
	}

	output, err := exec.Command("mkfs.ext4", "-F", path).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("formatting filesystem (%s): %s", err, output)
	}

	return path, nil
}

func (bm *BackingStore) Delete(id string) error {
	if err := os.Remove(bm.backingStorePath(id)); err != nil {
		return fmt.Errorf("deleting backing store file: %s", id, err)
	}

	return nil
}

func (bm *BackingStore) backingStorePath(id string) string {
	return filepath.Join(bm.RootPath, id)
}
