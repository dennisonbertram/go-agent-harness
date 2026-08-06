//go:build !darwin || !cgo

package nativegui

import (
	"context"
	"fmt"
)

type DarwinCorePlatform struct{}

func (DarwinCorePlatform) SubmitPrompt(context.Context, int, string) error {
	return fmt.Errorf("native rendered controls are unavailable on this platform")
}
func (DarwinCorePlatform) AccessibilitySnapshot(context.Context, int) ([]byte, error) {
	return nil, fmt.Errorf("native rendered controls are unavailable on this platform")
}
func (DarwinCorePlatform) CaptureScreenshot(context.Context, int, string) error {
	return fmt.Errorf("native rendered controls are unavailable on this platform")
}
