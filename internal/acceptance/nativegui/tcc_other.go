//go:build !darwin || !cgo

package nativegui

import "context"

func PlatformPermissionState(context.Context) (PermissionReport, error) {
	return PermissionReport{State: PermissionUnavailable, Source: "unsupported_platform"}, nil
}
