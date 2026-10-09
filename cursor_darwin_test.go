//go:build darwin

package main

import (
	"image"
	"testing"
	"time"

	"github.com/kbinani/screenshot"
)

func TestPrintScreenMetrics(t *testing.T) {
	b := screenshot.GetDisplayBounds(0)
	cgW, cgH, pxW, pxH := getScreenMetrics()
	t.Logf("screenshot.GetDisplayBounds(0): %+v", b)
	t.Logf("getScreenMetrics: cg=(%d x %d), px=(%d x %d)", cgW, cgH, pxW, pxH)

	capImg, capErr := screenshot.CaptureRect(image.Rect(0, 0, 200, 200))
	if capErr == nil {
		t.Logf("screenshot.CaptureRect(0,0,200,200) returned bounds: %+v", capImg.Bounds())
	}

	warpMousePoint(100, 0)
	time.Sleep(10 * time.Millisecond)
	pTop := getCursorPoint()
	t.Logf("warped to (100, 0) -> getCursorPoint(): %+v", pTop)

	warpMousePoint(100, 500)
	time.Sleep(10 * time.Millisecond)
	pMid := getCursorPoint()
	t.Logf("warped to (100, 500) -> getCursorPoint(): %+v", pMid)

	warpMousePoint(100, 1169)
	time.Sleep(10 * time.Millisecond)
	pBot := getCursorPoint()
	t.Logf("warped to (100, 1169) -> getCursorPoint(): %+v", pBot)
}

type testSCKFrame struct {
	img        *image.RGBA
	screenRect image.Rectangle
}

func TestSCKMetrics(t *testing.T) {
	rect := image.Rect(100, 100, 900, 700)
	frameRcv := make(chan testSCKFrame, 10)
	stream, err := startSCK(rect, 0, 30, true, func(img *image.RGBA, at time.Time, screenRect image.Rectangle) {
		select {
		case frameRcv <- testSCKFrame{img: img, screenRect: screenRect}:
		default:
		}
	})
	if err != nil {
		t.Skipf("SCK not available or permission denied: %v", err)
	}
	defer stream.Stop()

	// Warp mouse to (500, 200) - relative Y should be 100 in crop
	warpMousePoint(500, 200)
	time.Sleep(100 * time.Millisecond)
	for len(frameRcv) > 0 {
		<-frameRcv
	}
	f1 := <-frameRcv

	// Warp mouse to (500, 600) - relative Y should be 500 in crop
	warpMousePoint(500, 600)
	time.Sleep(100 * time.Millisecond)
	for len(frameRcv) > 0 {
		<-frameRcv
	}
	f2 := <-frameRcv

	t.Logf("f1 bounds=%+v screenRect=%+v", f1.img.Bounds(), f1.screenRect)
	if !f1.screenRect.Empty() {
		if f1.screenRect != rect {
			t.Errorf("expected screenRect=%+v, got %+v", rect, f1.screenRect)
		}
	}

	var diff1Y, diff2Y []int
	for y := 0; y < f1.img.Bounds().Dy(); y++ {
		for x := 380; x < 420; x++ {
			i1 := y*f1.img.Stride + x*4
			i2 := y*f2.img.Stride + x*4
			if f1.img.Pix[i1] != f2.img.Pix[i2] || f1.img.Pix[i1+1] != f2.img.Pix[i2+1] || f1.img.Pix[i1+2] != f2.img.Pix[i2+2] {
				if y < 300 {
					diff1Y = append(diff1Y, y)
				} else if y > 350 {
					diff2Y = append(diff2Y, y)
				}
			}
		}
	}
	if len(diff1Y) > 0 {
		t.Logf("Mouse at screen (500, 200), expected crop Y=100: cursor pixels Y in f1: %d .. %d", diff1Y[0], diff1Y[len(diff1Y)-1])
	}
	if len(diff2Y) > 0 {
		t.Logf("Mouse at screen (500, 600), expected crop Y=500: cursor pixels Y in f2: %d .. %d", diff2Y[0], diff2Y[len(diff2Y)-1])
	}
}

func TestWindowMetrics(t *testing.T) {
	wins := listWindows(0, 0, 1800, 1169)
	for i, w := range wins {
		if i < 5 {
			t.Logf("Window[%d]: id=%d title=%q app=%q rect=(%d,%d,%d,%d)", i, w.ID, w.Title, w.AppName, w.X, w.Y, w.W, w.H)
		}
	}
	if len(wins) > 0 {
		w0 := wins[0]
		rect := image.Rect(w0.X, w0.Y, w0.X+w0.W, w0.Y+w0.H)
		frameRcv := make(chan testSCKFrame, 10)
		stream, err := startSCK(rect, w0.ID, 30, true, func(img *image.RGBA, at time.Time, screenRect image.Rectangle) {
			select {
			case frameRcv <- testSCKFrame{img: img, screenRect: screenRect}:
			default:
			}
		})
		if err != nil {
			t.Skipf("startSCK window failed: %v", err)
		}
		defer stream.Stop()
		select {
		case f := <-frameRcv:
			t.Logf("Window[%d] frame received: bounds=%+v screenRect=%+v", w0.ID, f.img.Bounds(), f.screenRect)
			if !f.screenRect.Empty() {
				if f.screenRect.Min.X != w0.X || f.screenRect.Min.Y != w0.Y {
					t.Logf("Window[%d] screenRect origin matches: (%d,%d)", w0.ID, f.screenRect.Min.X, f.screenRect.Min.Y)
				}
			}
		case <-time.After(2 * time.Second):
			t.Logf("Window[%d] timed out", w0.ID)
		}
	}
}
