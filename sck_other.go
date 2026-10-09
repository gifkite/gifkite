//go:build !darwin

package main

import (
	"errors"
	"image"
	"time"
)

type SCKStream struct{}

func isSCKAvailable() bool {
	return false
}

func startSCK(rect image.Rectangle, windowID int, fps int, showCursor bool, onFrame func(img *image.RGBA, at time.Time, screenRect image.Rectangle)) (*SCKStream, error) {
	return nil, errors.New("ScreenCaptureKit is only supported on macOS 12.3+")
}

func (s *SCKStream) Stop() {}
