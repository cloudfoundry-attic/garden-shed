package aufs

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/pivotal-golang/lager"
)

const umountRetryCount = 200

type Loop struct {
	Logger lager.Logger
}

func (lm *Loop) MountFile(filePath, destPath string) error {
	log := lm.Logger.Session("mount-file", lager.Data{"filePath": filePath, "destPath": destPath})

	output, err := exec.Command("mount", "-t", "ext4", "-o", "loop",
		filePath, destPath).CombinedOutput()
	if err != nil {
		log.Error("mounting", err, lager.Data{"output": output})
		return fmt.Errorf("mounting file: %s", err)
	}

	return nil
}

func (lm *Loop) Unmount(path string) error {
	log := lm.Logger.Session("unmount", lager.Data{"path": path})

	var (
		err    error
		output []byte
	)
	for i := 0; i < umountRetryCount; i++ {
		output, err = exec.Command("umount", "-d", path).CombinedOutput()
		if err != nil {
			if _, err2 := exec.Command("mountpoint", path).CombinedOutput(); err2 != nil {
				// if it's not a mountpoint then this is fine
				return nil
			}
		} else {
			return nil
		}

		time.Sleep(time.Millisecond * 50)
	}

	if err != nil {
		log.Error("unmounting", err, lager.Data{"output": output})
		return fmt.Errorf("unmounting file: %s", err)
	}

	return nil
}
