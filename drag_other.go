//go:build !darwin

package main

import "unsafe"

func startNativeFileDrag(window unsafe.Pointer, path string) {}

func setWindowInvisibleToCapture(window unsafe.Pointer) {}
