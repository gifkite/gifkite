//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	user32Drag                   = syscall.NewLazyDLL("user32.dll")
	procSetWindowDisplayAffinity = user32Drag.NewProc("SetWindowDisplayAffinity")
)

const wdaExcludeFromCapture = 0x00000011

func setWindowInvisibleToCapture(window unsafe.Pointer) {
	if window != nil {
		procSetWindowDisplayAffinity.Call(uintptr(window), uintptr(wdaExcludeFromCapture))
	}
}

func setWindowTransparent(window unsafe.Pointer) {
	// Handled natively by Wails v3 WebView2 with BackgroundColour RGBA(0,0,0,0) on Windows.
}

func startNativeFileDrag(window unsafe.Pointer, path string) {
	if defaultGifService != nil {
		defaultGifService.onDragFinished(0)
	}
}
