package layercake

import (
	"os/exec"

	"fmt"

	"github.com/cloudfoundry/gunk/command_runner"
)

type AufsCake struct {
	Cake
	Runner command_runner.CommandRunner
}

func (a *AufsCake) Create(childID, parentID ID) error {
	if _, ok := childID.(NamespacedLayerID); !ok {
		return a.Cake.Create(childID, parentID)
	}

	if err := a.Cake.Create(childID, DockerImageID("")); err != nil {
		return err
	}

	_, err := a.Cake.Get(childID)
	if err != nil {
		return err
	}

	sourcePath, err := a.Cake.Path(parentID)
	if err != nil {
		return err
	}

	destinationPath, err := a.Cake.Path(childID)
	if err != nil {
		return err
	}

	copyCmd := fmt.Sprintf("cp -a %s/. %s", sourcePath, destinationPath)
	if err := a.Runner.Run(exec.Command("sh", "-c", copyCmd)); err != nil {
		return err
	}

	return nil
}
