package cron

import "time"

// Clock abstracts time for testing.
type Clock interface {
	Now() time.Time
}

// RealClock uses the actual system time.
type RealClock struct{}

// Now returns the current UTC time.
func (RealClock) Now() time.Time { return time.Now().UTC() }
