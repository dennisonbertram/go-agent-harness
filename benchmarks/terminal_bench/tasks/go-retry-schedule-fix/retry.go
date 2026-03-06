package retry

import "time"

func Schedule(base time.Duration, attempts int) []time.Duration {
	if base <= 0 || attempts <= 0 {
		return nil
	}

	delays := make([]time.Duration, 0, attempts)
	for i := 0; i < attempts; i++ {
		delays = append(delays, time.Duration(i)*base)
	}
	return delays
}
