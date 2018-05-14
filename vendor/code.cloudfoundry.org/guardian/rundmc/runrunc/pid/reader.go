package pid

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"time"

	"code.cloudfoundry.org/clock"
)

type FileReader struct {
	Clock         clock.Clock
	Timeout       time.Duration
	SleepInterval time.Duration
}

func (p *FileReader) Pid(pidFilePath string) (int, error) {
	var (
		pidContents   []byte
		err           error
		timeRemaining time.Duration
	)

	for timeRemaining = p.Timeout; timeRemaining > 0; timeRemaining -= time.Millisecond * 20 {
		pidContents, err = ioutil.ReadFile(pidFilePath)
		if err == nil && len(pidContents) > 0 {
			break
		}

		p.Clock.Sleep(p.SleepInterval)
	}

	if len(pidContents) == 0 {
		err = fmt.Errorf("file '%s' is empty", pidFilePath)
	}
	if err != nil {
		return 0, fmt.Errorf("timeout: %s", err)
	}

	pid, err := strconv.Atoi(string(pidContents))
	if err != nil {
		return 0, fmt.Errorf("parsing pid file contents: %s", err)
	}

	return pid, nil
}
