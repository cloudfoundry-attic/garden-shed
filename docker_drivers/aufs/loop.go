package aufs

import (
	"fmt"
	"os/exec"

	"github.com/pivotal-golang/lager"
)

type Loop struct {
	Logger lager.Logger
}

func (lm *Loop) MountFile(filePath, destPath string) error {
	log := lm.Logger.Session("mount-file", lager.Data{"filePath": filePath, "destPath": destPath})

	output, err := exec.Command("mount", "-t", "ext4", "-o", "loop",
		filePath, destPath).CombinedOutput()
	if err != nil {
		log.Error("failed-to-mount", err, lager.Data{"output": output})
		return fmt.Errorf(fmt.Sprintf("failed to mount file (%s): %s", err, output))
	}

	return nil
}

func (lm *Loop) Unmount(path string) error {
	log := lm.Logger.Session("unmount", lager.Data{"path": path})

	if err := exec.Command("umount", "-d", path).Run(); err != nil {
		if err2 := exec.Command("mountpoint", path).Run(); err2 != nil {
			// if it's not a mountpoint then this is fine
			log.Error("failed-to-run-mountpoint", err)
			return nil
		}
		log.Error("failed-to-unmount", err)

		return err
	}

	return nil
}
