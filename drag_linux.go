//go:build linux

package main

import (
	"unsafe"
)

func setWindowInvisibleToCapture(window unsafe.Pointer) {
	// Standard X11/Wayland protocols do not have a universal capture exclusion window attribute.
}

func setWindowTransparent(window unsafe.Pointer) {
	// Handled by Wails v3 WebKitGTK with RGBA visual and system compositor.
}

func startNativeFileDrag(window unsafe.Pointer, path string) {
	if defaultGifService != nil {
		defaultGifService.onDragFinished(0)
	}
}
