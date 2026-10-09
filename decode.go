package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DecodeMediaFile loads a media file (.gif, .webp, .mp4, etc.) and returns its frames as []Frame and the end timestamp.
func DecodeMediaFile(path string) ([]Frame, time.Time, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".gif" {
		frames, end, err := decodeGIFFile(path)
		if err == nil && len(frames) > 0 {
			return frames, end, nil
		}
		// If native GIF decode failed, fall back to ffmpeg if available
	}
	return decodeWithFFmpeg(path)
}

// decodeGIFFile decodes an animated GIF into full RGBA frames respecting GIF disposal methods.
func decodeGIFFile(path string) ([]Frame, time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer f.Close()

	g, err := gif.DecodeAll(f)
	if err != nil {
		return nil, time.Time{}, err
	}
	if len(g.Image) == 0 {
		return nil, time.Time{}, errors.New("no frames in gif")
	}

	w, h := g.Config.Width, g.Config.Height
	if w <= 0 || h <= 0 {
		w = g.Image[0].Bounds().Dx()
		h = g.Image[0].Bounds().Dy()
	}

	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	var prevSnapshot *image.RGBA

	frames := make([]Frame, len(g.Image))
	startTime := time.Now()
	currentTime := startTime

	for i, palImg := range g.Image {
		var disposal byte = gif.DisposalNone
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}

		if disposal == gif.DisposalPrevious {
			// Save snapshot before drawing
			prevSnapshot = image.NewRGBA(image.Rect(0, 0, w, h))
			copy(prevSnapshot.Pix, canvas.Pix)
		}

		// Draw current paletted frame over canvas
		draw.Draw(canvas, palImg.Bounds(), palImg, palImg.Bounds().Min, draw.Over)

		// Create full RGBA frame copy
		frameImg := image.NewRGBA(image.Rect(0, 0, w, h))
		copy(frameImg.Pix, canvas.Pix)
		frames[i] = Frame{
			Img: frameImg,
			At:  currentTime,
		}

		// Advance time based on delay (in centiseconds)
		delayCS := 10 // default 100ms
		if i < len(g.Delay) && g.Delay[i] > 0 {
			delayCS = g.Delay[i]
		}
		currentTime = currentTime.Add(time.Duration(delayCS*10) * time.Millisecond)

		// Disposal for the next frame
		switch disposal {
		case gif.DisposalBackground:
			draw.Draw(canvas, palImg.Bounds(), image.Transparent, image.ZP, draw.Src)
		case gif.DisposalPrevious:
			if prevSnapshot != nil {
				copy(canvas.Pix, prevSnapshot.Pix)
			}
		}
	}

	return frames, currentTime, nil
}

// decodeWithFFmpeg uses ffmpeg to extract frames from any supported media file (e.g. mp4, webp, mov).
func decodeWithFFmpeg(path string) ([]Frame, time.Time, error) {
	ffmpeg := findBinary("ffmpeg")
	if ffmpeg == "" {
		return nil, time.Time{}, errors.New("ffmpeg is required to decode this media format")
	}

	fps := 15.0
	if ffprobe := findBinary("ffprobe"); ffprobe != "" {
		out, err := exec.Command(ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=r_frame_rate", "-of", "csv=p=0", path).Output()
		if err == nil {
			parts := strings.Split(strings.TrimSpace(string(out)), "/")
			if len(parts) == 2 {
				num, _ := strconv.ParseFloat(parts[0], 64)
				den, _ := strconv.ParseFloat(parts[1], 64)
				if den > 0 && num > 0 {
					fps = num / den
				}
			} else if len(parts) == 1 {
				if val, _ := strconv.ParseFloat(parts[0], 64); val > 0 {
					fps = val
				}
			}
		}
	}
	if fps <= 0 || fps > 120 {
		fps = 15.0
	}

	tmpDir, err := os.MkdirTemp("", "gifkite-decode-*")
	if err != nil {
		return nil, time.Time{}, err
	}
	defer os.RemoveAll(tmpDir)

	pattern := filepath.Join(tmpDir, "f_%06d.png")
	cmd := exec.Command(ffmpeg, "-y", "-i", path, "-vsync", "0", pattern)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, time.Time{}, fmt.Errorf("ffmpeg decode failed: %v (%s)", err, string(out))
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, time.Time{}, err
	}

	var pngFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".png") {
			pngFiles = append(pngFiles, filepath.Join(tmpDir, e.Name()))
		}
	}
	sort.Strings(pngFiles)
	if len(pngFiles) == 0 {
		return nil, time.Time{}, errors.New("no frames extracted from video")
	}

	frames := make([]Frame, len(pngFiles))
	startTime := time.Now()
	frameInterval := time.Duration(float64(time.Second) / fps)

	for i, pf := range pngFiles {
		f, err := os.Open(pf)
		if err != nil {
			return nil, time.Time{}, err
		}
		rawImg, err := png.Decode(f)
		f.Close()
		if err != nil {
			return nil, time.Time{}, err
		}

		rgba, ok := rawImg.(*image.RGBA)
		if !ok {
			b := rawImg.Bounds()
			rgba = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
			draw.Draw(rgba, rgba.Bounds(), rawImg, b.Min, draw.Src)
		}
		frames[i] = Frame{
			Img: rgba,
			At:  startTime.Add(time.Duration(i) * frameInterval),
		}
	}

	endTime := startTime.Add(time.Duration(len(frames)) * frameInterval)
	return frames, endTime, nil
}

// ExtractThumbnailImage returns a thumbnail image for a recording file of any supported format (GIF, WebP, MP4).
func ExtractThumbnailImage(path string) (image.Image, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".gif" {
		f, err := os.Open(path)
		if err == nil {
			defer f.Close()
			if first, err := gif.Decode(f); err == nil {
				return first, nil
			}
		}
	}

	// Try image.Decode for static or webp images
	if f, err := os.Open(path); err == nil {
		defer f.Close()
		if img, _, err := image.Decode(f); err == nil {
			return img, nil
		}
	}

	// Fallback to ffmpeg single frame extraction for video / animated webp
	ffmpeg := findBinary("ffmpeg")
	if ffmpeg != "" {
		cmd := exec.Command(ffmpeg, "-ss", "00:00:00", "-i", path, "-vframes", "1", "-f", "image2pipe", "-vcodec", "png", "-")
		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		if err := cmd.Run(); err == nil && outBuf.Len() > 0 {
			if img, err := png.Decode(&outBuf); err == nil {
				return img, nil
			}
		}
	}

	// If jpeg
	if f, err := os.Open(path); err == nil {
		defer f.Close()
		if img, err := jpeg.Decode(f); err == nil {
			return img, nil
		}
	}

	return nil, fmt.Errorf("unable to extract thumbnail for %s", path)
}
