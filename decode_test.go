package main

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func makeTestFrames(count int, w, h int) []Frame {
	frames := make([]Frame, count)
	colors := []color.RGBA{
		{255, 0, 0, 255},
		{0, 255, 0, 255},
		{0, 0, 255, 255},
		{255, 255, 0, 255},
		{255, 0, 255, 255},
	}
	start := time.Now()
	for i := 0; i < count; i++ {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		c := colors[i%len(colors)]
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.SetRGBA(x, y, c)
			}
		}
		frames[i] = Frame{
			Img: img,
			At:  start.Add(time.Duration(i*100) * time.Millisecond),
		}
	}
	return frames
}

func TestDecodeGIF(t *testing.T) {
	tmpDir := t.TempDir()
	gifPath := filepath.Join(tmpDir, "test.gif")

	origFrames := makeTestFrames(4, 64, 48)
	end := origFrames[len(origFrames)-1].At.Add(100 * time.Millisecond)

	f, err := os.Create(gifPath)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	_, err = EncodeGIF(f, origFrames, end, 10, "none", 256, nil)
	f.Close()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, decodedEnd, err := DecodeMediaFile(gifPath)
	if err != nil {
		t.Fatalf("DecodeMediaFile failed: %v", err)
	}
	if len(decoded) != len(origFrames) {
		t.Fatalf("expected %d frames, got %d", len(origFrames), len(decoded))
	}
	if decoded[0].Img.Bounds().Dx() != 64 || decoded[0].Img.Bounds().Dy() != 48 {
		t.Fatalf("bounds mismatch: %v", decoded[0].Img.Bounds())
	}
	if !decodedEnd.After(decoded[0].At) {
		t.Fatalf("invalid end timestamp: %v vs %v", decodedEnd, decoded[0].At)
	}
}

func TestExportAndPostSaveTrim(t *testing.T) {
	tmpDir := t.TempDir()
	srcGif := filepath.Join(tmpDir, "gifkite-2026-10-09-100000.gif")

	frames := makeTestFrames(6, 64, 48)
	end := frames[len(frames)-1].At.Add(100 * time.Millisecond)

	err := EncodeExport(srcGif, "gif", frames, end, 10, "none", 256, nil)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	s := newGifService()
	s.settings.OutputDir = tmpDir
	s.settings.FPS = 10

	// 1. Test OpenInEditor loads the recording
	if err := s.OpenInEditor("gifkite-2026-10-09-100000.gif"); err != nil {
		t.Fatalf("OpenInEditor failed: %v", err)
	}
	info := s.GetReviewInfo()
	if info.NumFrames != 6 {
		t.Fatalf("expected 6 review frames, got %d", info.NumFrames)
	}
	if info.FileName != "gifkite-2026-10-09-100000.gif" {
		t.Fatalf("expected filename to match, got %s", info.FileName)
	}

	// 2. Test trimming and ConfirmReview with saveAsCopy = true
	err = s.ConfirmReview(1, 3, "gif", nil, true, nil, 256)
	if err != nil {
		t.Fatalf("ConfirmReview failed: %v", err)
	}

	// 3. Test Direct Export to MP4
	if findBinary("ffmpeg") != "" {
		outMp4, err := s.ExportRecording("gifkite-2026-10-09-100000.gif", "mp4")
		if err != nil {
			t.Fatalf("ExportRecording to mp4 failed: %v", err)
		}
		if filepath.Ext(outMp4) != ".mp4" {
			t.Fatalf("expected .mp4 extension, got %s", outMp4)
		}
		if _, err := os.Stat(filepath.Join(tmpDir, outMp4)); err != nil {
			t.Fatalf("exported mp4 file not found: %v", err)
		}

		// Verify exported MP4 can also be opened in editor!
		if err := s.OpenInEditor(outMp4); err != nil {
			t.Fatalf("OpenInEditor on exported mp4 failed: %v", err)
		}
		mp4Info := s.GetReviewInfo()
		if mp4Info.NumFrames == 0 {
			t.Fatalf("expected >0 frames from exported mp4, got %d", mp4Info.NumFrames)
		}
		s.DiscardReview()
	}

	// 4. Test Direct Export to WebP
	if findBinary("gif2webp") != "" || findBinary("ffmpeg") != "" {
		outWebp, err := s.ExportRecording("gifkite-2026-10-09-100000.gif", "webp")
		if err != nil {
			t.Fatalf("ExportRecording to webp failed: %v", err)
		}
		if filepath.Ext(outWebp) != ".webp" {
			t.Fatalf("expected .webp extension, got %s", outWebp)
		}
		if _, err := os.Stat(filepath.Join(tmpDir, outWebp)); err != nil {
			t.Fatalf("exported webp file not found: %v", err)
		}
	} else {
		t.Logf("Skipping webp export test (neither gif2webp nor ffmpeg is installed)")
	}
}

func TestCropAndColorQuantization(t *testing.T) {
	tmpDir := t.TempDir()
	srcGif := filepath.Join(tmpDir, "gifkite-crop.gif")

	// Create 100x100 frames
	frames := makeTestFrames(4, 100, 100)
	end := frames[len(frames)-1].At.Add(100 * time.Millisecond)

	err := EncodeExport(srcGif, "gif", frames, end, 10, "none", 256, nil)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	s := newGifService()
	s.settings.OutputDir = tmpDir
	s.settings.FPS = 10

	if err := s.OpenInEditor("gifkite-crop.gif"); err != nil {
		t.Fatalf("OpenInEditor failed: %v", err)
	}

	// Crop to (10, 10, 40, 30) with 64 colors
	crop := &CropRect{X: 10, Y: 10, W: 40, H: 30}
	if err := s.ConfirmReview(0, 3, "gif", nil, true, crop, 64); err != nil {
		t.Fatalf("ConfirmReview with crop failed: %v", err)
	}

	// Wait for async encoding to finish
	deadline := time.Now().Add(5 * time.Second)
	var croppedPath string
	for time.Now().Before(deadline) {
		files, err := os.ReadDir(tmpDir)
		if err == nil {
			for _, f := range files {
				if strings.HasSuffix(f.Name(), ".gif") && f.Name() != "gifkite-crop.gif" {
					croppedPath = filepath.Join(tmpDir, f.Name())
					break
				}
			}
		}
		if croppedPath != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if croppedPath == "" {
		t.Fatalf("cropped file not found in %v within timeout", tmpDir)
	}

	decoded, _, err := DecodeMediaFile(croppedPath)
	if err != nil {
		t.Fatalf("decode cropped file failed: %v", err)
	}
	if len(decoded) == 0 {
		t.Fatalf("no frames decoded")
	}
	bounds := decoded[0].Img.Bounds()
	if bounds.Dx() != 40 || bounds.Dy() != 30 {
		t.Fatalf("expected cropped dimensions 40x30, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}
