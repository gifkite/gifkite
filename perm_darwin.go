//go:build darwin

package main

/*
#cgo LDFLAGS: -framework CoreGraphics
#include <CoreGraphics/CoreGraphics.h>
*/
import "C"

func hasScreenPermission() bool {
	return bool(C.CGPreflightScreenCaptureAccess())
}

func requestScreenPermission() {
	C.CGRequestScreenCaptureAccess()
}
