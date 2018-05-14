package goci

import "os/exec"

// The DefaultRuncBinary, i.e. 'runc'.
var DefaultRuncBinary = RuncBinary{Path: "runc"}

// RuncBinary is the path to a runc binary.
type RuncBinary struct {
	Path string
}

// StartCommand creates a start command using the default runc binary name.
func StartCommand(path, id string, detach bool, log string) *exec.Cmd {
	return DefaultRuncBinary.StartCommand(path, id, detach, log)
}

// ExecCommand creates an exec command using the default runc binary name.
func ExecCommand(id, processJSONPath, pidFilePath string) *exec.Cmd {
	return DefaultRuncBinary.ExecCommand(id, processJSONPath, pidFilePath)
}

// KillCommand creates a kill command using the default runc binary name.
func KillCommand(id, signal, logFile string) *exec.Cmd {
	return DefaultRuncBinary.KillCommand(id, signal, logFile)
}

func StateCommand(id, logFile string) *exec.Cmd {
	return DefaultRuncBinary.StateCommand(id, logFile)
}

// StatsCommands creates a command that gets the stats of a container using the default runc binary name.
func StatsCommand(id, logFile string) *exec.Cmd {
	return DefaultRuncBinary.StatsCommand(id, logFile)
}

// DeleteCommand creates a command that deletes a container using the default runc binary name.
func DeleteCommand(id string, force bool, logFile string) *exec.Cmd {
	return DefaultRuncBinary.DeleteCommand(id, force, logFile)
}

func EventsCommand(id string) *exec.Cmd {
	return DefaultRuncBinary.EventsCommand(id)
}

// StartCommand returns an *exec.Cmd that, when run, will execute a given bundle.
func (runc RuncBinary) StartCommand(path, id string, detach bool, log string) *exec.Cmd {
	args := []string{"--debug", "--log", log, "--log-format", "json", "start"}
	if detach {
		args = append(args, "-d")
	}

	args = append(args, id)

	cmd := exec.Command(runc.Path, args...)
	cmd.Dir = path
	return cmd
}

// ExecCommand returns an *exec.Cmd that, when run, will execute a process spec
// in a running container.
func (runc RuncBinary) ExecCommand(id, processJSONPath, pidFilePath string) *exec.Cmd {
	return exec.Command(
		runc.Path, []string{"exec", id, "--pid-file", pidFilePath, "-p", processJSONPath}...,
	)
}

// EventsCommand returns an *exec.Cmd that, when run, will retrieve events for the container
func (runc RuncBinary) EventsCommand(id string) *exec.Cmd {
	return exec.Command(runc.Path, []string{"events", id}...)
}

// KillCommand returns an *exec.Cmd that, when run, will signal the running
// container.
func (runc RuncBinary) KillCommand(id, signal, logFile string) *exec.Cmd {
	return exec.Command(
		runc.Path, []string{"--debug", "--log", logFile, "--log-format", "json", "kill", id, signal}...,
	)
}

// StateCommand returns an *exec.Cmd that, when run, will get the state of the
// container.
func (runc RuncBinary) StateCommand(id, logFile string) *exec.Cmd {
	return exec.Command(runc.Path, []string{"--debug", "--log", logFile, "--log-format", "json", "state", id}...)
}

// StatsCommand returns an *exec.Cmd that, when run, will get the stats of the
// container.
func (runc RuncBinary) StatsCommand(id, logFile string) *exec.Cmd {
	return exec.Command(runc.Path, []string{"--debug", "--log", logFile, "--log-format", "json", "events", "--stats", id}...)
}

// DeleteCommand returns an *exec.Cmd that, when run, will signal the running
// container.
func (runc RuncBinary) DeleteCommand(id string, force bool, logFile string) *exec.Cmd {
	deleteArgs := []string{"--debug", "--log", logFile, "--log-format", "json", "delete"}
	if force {
		deleteArgs = append(deleteArgs, "--force")
	}
	return exec.Command(runc.Path, append(deleteArgs, id)...)
}
