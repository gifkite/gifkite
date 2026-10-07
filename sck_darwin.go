//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework ScreenCaptureKit -framework Foundation -framework CoreMedia -framework CoreVideo
#include "sck_darwin.h"
*/
import "C"
import (
	"errors"
	"image"
	"sync"
	"time"
	"unsafe"
)

type SCKStream struct {
	id      uintptr
	session unsafe.Pointer
	onFrame func(img *image.RGBA, at time.Time)
	stopped bool
	mu      sync.Mutex
}

var (
	sckRegistry   = make(map[uintptr]*SCKStream)
	sckRegistryMu sync.Mutex
	sckNextID     uintptr = 1
)

func registerSCK(s *SCKStream) uintptr {
	sckRegistryMu.Lock()
	defer sckRegistryMu.Unlock()
	id := sckNextID
	sckNextID++
	sckRegistry[id] = s
	return id
}

func unregisterSCK(id uintptr) {
	sckRegistryMu.Lock()
	defer sckRegistryMu.Unlock()
	delete(sckRegistry, id)
}

func getSCK(id uintptr) *SCKStream {
	sckRegistryMu.Lock()
	defer sckRegistryMu.Unlock()
	return sckRegistry[id]
}

//export sckFrameCallback
func sckFrameCallback(ctx C.uintptr_t, baseAddress unsafe.Pointer, width C.int, height C.int, bytesPerRow C.int, ptsNs C.int64_t) {
	s := getSCK(uintptr(ctx))
	if s == nil || baseAddress == nil || width <= 0 || height <= 0 {
		return
	}

	w := int(width)
	h := int(height)
	stride := int(bytesPerRow)

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	srcBytes := unsafe.Slice((*byte)(baseAddress), stride*h)

	// Convert BGRA pixels to RGBA row by row
	for y := 0; y < h; y++ {
		srcRow := srcBytes[y*stride : y*stride+w*4]
		dstRow := img.Pix[y*img.Stride : y*img.Stride+w*4]
		for x := 0; x < w*4; x += 4 {
			dstRow[x] = srcRow[x+2]   // R
			dstRow[x+1] = srcRow[x+1] // G
			dstRow[x+2] = srcRow[x]   // B
			dstRow[x+3] = srcRow[x+3] // A
		}
	}

	s.mu.Lock()
	cb := s.onFrame
	stopped := s.stopped
	s.mu.Unlock()

	if !stopped && cb != nil {
		cb(img, time.Now())
	}
}

func isSCKAvailable() bool {
	return bool(C.isSCKSupported())
}

func startSCK(rect image.Rectangle, windowID int, fps int, showCursor bool, onFrame func(img *image.RGBA, at time.Time)) (*SCKStream, error) {
	if !isSCKAvailable() {
		return nil, errors.New("ScreenCaptureKit requires macOS 12.3+")
	}

	s := &SCKStream{
		onFrame: onFrame,
	}
	ctxID := registerSCK(s)
	s.id = ctxID

	sess := C.startSCKStream(
		C.int(0),
		C.int(windowID),
		C.int(rect.Min.X),
		C.int(rect.Min.Y),
		C.int(rect.Dx()),
		C.int(rect.Dy()),
		C.int(fps),
		C.bool(showCursor),
		C.uintptr_t(ctxID),
	)

	if sess == nil {
		unregisterSCK(ctxID)
		return nil, errors.New("failed to initialize ScreenCaptureKit stream")
	}

	s.session = sess
	return s, nil
}

func (s *SCKStream) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	sess := s.session
	s.session = nil
	ctxID := s.id
	s.mu.Unlock()

	unregisterSCK(ctxID)
	if sess != nil {
		C.stopSCKStream(sess)
	}
}
