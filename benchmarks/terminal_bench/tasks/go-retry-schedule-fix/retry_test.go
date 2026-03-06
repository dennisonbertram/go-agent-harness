package retry

import (
	"reflect"
	"testing"
	"time"
)

func TestScheduleReturnsMonotonicRetryDelays(t *testing.T) {
	want := []time.Duration{
		5 * time.Second,
		10 * time.Second,
		15 * time.Second,
	}

	if got := Schedule(5*time.Second, 3); !reflect.DeepEqual(got, want) {
		t.Fatalf("Schedule(5s, 3) = %v, want %v", got, want)
	}
}

func TestScheduleRejectsNonPositiveInputs(t *testing.T) {
	if got := Schedule(0, 3); got != nil {
		t.Fatalf("Schedule(0, 3) = %v, want nil", got)
	}
	if got := Schedule(5*time.Second, 0); got != nil {
		t.Fatalf("Schedule(5s, 0) = %v, want nil", got)
	}
}

func TestScheduleCapsDelaysAtThirtySeconds(t *testing.T) {
	want := []time.Duration{
		12 * time.Second,
		24 * time.Second,
		30 * time.Second,
		30 * time.Second,
	}

	if got := Schedule(12*time.Second, 4); !reflect.DeepEqual(got, want) {
		t.Fatalf("Schedule(12s, 4) = %v, want %v", got, want)
	}
}
