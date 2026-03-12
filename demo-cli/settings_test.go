package main

import (
	"testing"
)

// TestSettingItem_Toggle verifies that toggle flips the value both ways.
func TestSettingItem_Toggle(t *testing.T) {
	val := false
	item := settingItem{
		label: "Test setting",
		getValue: func() string {
			if val {
				return "ON"
			}
			return "OFF"
		},
		toggle: func() { val = !val },
	}

	// Initial state should be OFF
	if item.getValue() != "OFF" {
		t.Errorf("expected initial value OFF, got %q", item.getValue())
	}

	// First toggle: should be ON
	item.toggle()
	if item.getValue() != "ON" {
		t.Errorf("expected ON after first toggle, got %q", item.getValue())
	}

	// Second toggle: should be OFF again
	item.toggle()
	if item.getValue() != "OFF" {
		t.Errorf("expected OFF after second toggle, got %q", item.getValue())
	}
}

// TestRenderSettings_DoesNotPanic verifies that renderSettings with valid input does not panic.
func TestRenderSettings_DoesNotPanic(t *testing.T) {
	items := []settingItem{
		{
			label:    "Verbose mode",
			getValue: func() string { return "OFF" },
			toggle:   func() {},
		},
	}
	// renderSettings writes to stdout; we just ensure it does not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("renderSettings panicked: %v", r)
		}
	}()
	renderSettings(items, 0)
}
