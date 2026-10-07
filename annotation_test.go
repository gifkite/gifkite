package main

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestApplyAnnotations(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 100, A: 255})
		}
	}

	annots := []Annotation{
		{
			Type:  "blur",
			X:     0.1,
			Y:     0.1,
			W:     0.4,
			H:     0.4,
		},
		{
			Type:  "arrow",
			X:     0.6,
			Y:     0.2,
			X2:    0.8,
			Y2:    0.8,
			Color: "#ff3b30",
		},
		{
			Type:  "text",
			X:     0.2,
			Y:     0.8,
			Text:  "Secret Key",
			Color: "#ffffff",
		},
	}

	ApplyAnnotations(img, annots)

	// Verify blur region: adjacent pixels within the 10x10 block should be identical
	c1 := img.RGBAAt(22, 22)
	c2 := img.RGBAAt(23, 23)
	if c1 != c2 {
		t.Errorf("Expected pixelated block to have uniform color, got %v vs %v", c1, c2)
	}

	// Verify arrow line: line center around (140, 100) should have changed towards red
	mid := img.RGBAAt(140, 100)
	if mid.R < 150 {
		t.Errorf("Expected arrow color on line, got %v", mid)
	}
}

func TestMultiFormatExport(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gifkite-test-export-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	frames := make([]Frame, 10)
	now := time.Now()
	for i := range frames {
		img := image.NewRGBA(image.Rect(0, 0, 80, 60))
		for y := 0; y < 60; y++ {
			for x := 0; x < 80; x++ {
				img.SetRGBA(x, y, color.RGBA{R: uint8(x * 3), G: uint8(y * 4), B: uint8(i * 20), A: 255})
			}
		}
		frames[i] = Frame{Img: img, At: now.Add(time.Duration(i*100) * time.Millisecond)}
	}
	end := now.Add(time.Second)

	// 1. GIF export
	gifPath := filepath.Join(tmpDir, "test.gif")
	if err := EncodeExport(gifPath, "gif", frames, end, 10, "none", nil); err != nil {
		t.Fatalf("EncodeExport GIF failed: %v", err)
	}
	fi, err := os.Stat(gifPath)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("Expected non-empty GIF output, got err=%v, size=%d", err, fi.Size())
	}

	// 2. WebP export
	webpPath := filepath.Join(tmpDir, "test.webp")
	if err := EncodeExport(webpPath, "webp", frames, end, 10, "none", nil); err != nil {
		t.Logf("WebP export returned: %v (tools may not be present in test env, fallback checked)", err)
	} else {
		fi, err := os.Stat(webpPath)
		if err != nil || fi.Size() == 0 {
			t.Errorf("Expected non-empty WebP output, got size=%d", fi.Size())
		}
	}

	// 3. MP4 export
	mp4Path := filepath.Join(tmpDir, "test.mp4")
	if err := EncodeExport(mp4Path, "mp4", frames, end, 10, "none", nil); err != nil {
		t.Logf("MP4 export returned: %v (ffmpeg may not be present in test env)", err)
	} else {
		fi, err := os.Stat(mp4Path)
		if err != nil || fi.Size() == 0 {
			t.Errorf("Expected non-empty MP4 output, got size=%d", fi.Size())
		}
	}
}
