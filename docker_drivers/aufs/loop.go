package aufs

import (
	"fmt"
	"os/exec"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Retrier
type Retrier interface {
	Retry(cb func() error) error
}

type Loop struct {
	Retrier Retrier
	Logger  lager.Logger
}

func (lm *Loop) MountFile(filePath, destPath string) error {
	log := lm.Logger.Session("mount-file", lager.Data{"filePath": filePath, "destPath": destPath})

	output, err := exec.Command("mount", "-t", "ext4", "-o", "loop",
		filePath, destPath).CombinedOutput()
	if err != nil {
		log.Error("mounting", err, lager.Data{"output": string(output)})
		return fmt.Errorf("mounting file: %s", err)
	}

	return nil
}

func (lm *Loop) Unmount(path string) error {
	log := lm.Logger.Session("unmount", lager.Data{"path": path})

	var output []byte
	err := lm.Retrier.Retry(func() error {
		var err error
		output, err = exec.Command("umount", "-d", path).CombinedOutput()
		if err != nil {
			if err2 := exec.Command("mountpoint", path).Run(); err2 != nil {
				// if it's not a mountpoint then this is fine
				return nil
			}
			return err
		}
		return nil
	})

	if err != nil {
		log.Error("unmounting", err, lager.Data{"output": string(output)})
		return fmt.Errorf("unmounting file: %s", err)
	}

	return nil
}
