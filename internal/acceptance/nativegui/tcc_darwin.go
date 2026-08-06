//go:build darwin && cgo

package nativegui

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics
#include <ApplicationServices/ApplicationServices.h>
#include <CoreGraphics/CoreGraphics.h>

static int gocodeAXTrusted(void) {
    return AXIsProcessTrusted() ? 1 : 0;
}

static int gocodeScreenCaptureTrusted(void) {
    return CGPreflightScreenCaptureAccess() ? 1 : 0;
}
*/
import "C"

import "context"

func PlatformPermissionState(context.Context) (PermissionReport, error) {
	accessibility := C.gocodeAXTrusted() == 1
	screenRecording := C.gocodeScreenCaptureTrusted() == 1
	state := PermissionPromptRequired
	if accessibility && screenRecording {
		state = PermissionAvailable
	}
	return PermissionReport{
		State:           state,
		Accessibility:   accessibility,
		ScreenRecording: screenRecording,
		Source:          "darwin_non_prompting_preflight",
	}, nil
}
