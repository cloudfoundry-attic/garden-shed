package aufs

import (
	"fmt"
	"os/exec"
	"syscall"
)

type Loop struct{}

func (lm *Loop) MountFile(filePath, destPath string) error {
	output, err := exec.Command("mount", "-t", "ext4", "-o", "loop",
		filePath, destPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("failed to mount file (%s): %s", err, output))
	}

	return nil
}

func (lm *Loop) Unmount(path string) error {
	if err := syscall.Unmount(path, 0); err != nil {
		return err
	}

	return nil
}
