package symphd

import (
	"context"
	"testing"
	"time"
)

func TestNewOrchestrator(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	if o == nil {
		t.Fatal("NewOrchestrator returned nil")
	}
	if o.config != cfg {
		t.Error("config not set")
	}
	if o.startedAt.IsZero() {
		t.Error("startedAt not set")
	}
}

func TestOrchestrator_State(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	state := o.State()
	if state["version"] != "0.1.0" {
		t.Errorf("version = %v", state["version"])
	}
	if _, ok := state["running_since"]; !ok {
		t.Error("running_since missing")
	}
	if state["agent_count"] != 0 {
		t.Errorf("agent_count = %v", state["agent_count"])
	}
}

func TestOrchestrator_State_RunningTimeIncreases(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	s1 := o.State()
	time.Sleep(10 * time.Millisecond)
	s2 := o.State()
	// running_since should be the same (fixed at start)
	if s1["running_since"] != s2["running_since"] {
		t.Error("running_since changed between calls")
	}
}

func TestOrchestrator_Start(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	if err := o.Start(context.Background()); err != nil {
		t.Errorf("Start returned error: %v", err)
	}
}

func TestOrchestrator_Shutdown(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	if err := o.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}
}

func TestOrchestrator_Concurrent_State(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	// Concurrent reads should not race
	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			_ = o.State()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}
