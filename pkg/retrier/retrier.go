package retrier

import (
	"time"

	"code.cloudfoundry.org/clock"
)

type Retrier struct {
	Timeout         time.Duration
	PollingInterval time.Duration
	Clock           clock.Clock
}

func (r *Retrier) Retry(callback func() error) error {
	count := int(r.Timeout / r.PollingInterval)
	var lastErr error

	for i := 0; i < count; i++ {
		if lastErr = callback(); lastErr == nil {
			return nil
		}

		r.Clock.Sleep(r.PollingInterval)
	}

	return lastErr
}
