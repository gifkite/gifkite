package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func findBinary(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	candidates := []string{
		filepath.Join("/opt/homebrew/bin", name),
		filepath.Join("/usr/local/bin", name),
		filepath.Join("/usr/bin", name),
		filepath.Join("/bin", name),
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			filepath.Join("C:\\ProgramData\\chocolatey\\bin", name+".exe"),
			filepath.Join("C:\\ffmpeg\\bin", name+".exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft\\WinGet\\Links", name+".exe"),
		)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// EncodeExport writes the frames in the requested format (gif, webp, mp4).
func EncodeExport(path string, format string, frames []Frame, end time.Time, fps int, dither string, maxColors int, progress func(i, n int)) error {
	if len(frames) == 0 {
		return errors.New("nothing was captured")
	}

	format = strings.ToLower(format)
	switch format {
	case "webp":
		return encodeWebP(path, frames, end, fps, dither, maxColors, progress)
	case "mp4":
		return encodeMP4(path, frames, end, fps, progress)
	default:
		// Default to GIF
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = EncodeGIF(f, frames, end, fps, dither, maxColors, progress)
		return err
	}
}

func encodeWebP(path string, frames []Frame, end time.Time, fps int, dither string, maxColors int, progress func(i, n int)) error {
	gif2webp := findBinary("gif2webp")
	if gif2webp != "" {
		if progress != nil {
			progress(5, 100)
		}
		// Encode to a temporary GIF first, then convert with gif2webp for optimal delta-frame compression
		tmpGif := path + ".tmp.gif"
		f, err := os.Create(tmpGif)
		if err != nil {
			return err
		}
		// Scale GIF progress to 0%..85%
		gifProgress := func(i, n int) {
			if progress != nil && n > 0 {
				pct := i * 85 / n
				progress(pct, 100)
			}
		}
		_, err = EncodeGIF(f, frames, end, fps, dither, maxColors, gifProgress)
		f.Close()
		if err != nil {
			os.Remove(tmpGif)
			return err
		}
		defer os.Remove(tmpGif)

		if progress != nil {
			progress(88, 100)
		}
		cmd := exec.Command(gif2webp, "-q", "80", "-m", "4", tmpGif, "-o", path)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("gif2webp failed: %v (%s)", err, strings.TrimSpace(string(out)))
		}
		if progress != nil {
			progress(100, 100)
		}
		return nil
	}

	ffmpeg := findBinary("ffmpeg")
	if ffmpeg != "" {
		if progress != nil {
			progress(5, 100)
		}
		// Use ffmpeg with animated webp
		bounds := frames[0].Img.Bounds()
		W, H := bounds.Dx() &^ 1, bounds.Dy() &^ 1
		if W < 2 {
			W = 2
		}
		if H < 2 {
			H = 2
		}

		cmd := exec.Command(ffmpeg,
			"-y",
			"-f", "rawvideo",
			"-pix_fmt", "rgba",
			"-s", fmt.Sprintf("%dx%d", W, H),
			"-r", strconv.Itoa(max(fps, 1)),
			"-i", "-",
			"-vcodec", "libwebp",
			"-loop", "0",
			"-qscale", "75",
			path,
		)

		var stderrBuf bytes.Buffer
		cmd.Stderr = &stderrBuf

		stdin, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("ffmpeg webp start failed: %w (%s)", err, strings.TrimSpace(stderrBuf.String()))
		}

		go func() {
			defer stdin.Close()
			needsCrop := (W != bounds.Dx() || H != bounds.Dy())
			var cropBuf *image.RGBA
			if needsCrop {
				cropBuf = image.NewRGBA(image.Rect(0, 0, W, H))
			}

			nFrames := len(frames)
			for i, f := range frames {
				if progress != nil {
					pct := 5 + ((i + 1) * 90 / max(1, nFrames))
					progress(pct, 100)
				}
				if needsCrop {
					for y := 0; y < H; y++ {
						copy(cropBuf.Pix[y*cropBuf.Stride:y*cropBuf.Stride+W*4],
							f.Img.Pix[y*f.Img.Stride:y*f.Img.Stride+W*4])
					}
					if _, err := stdin.Write(cropBuf.Pix); err != nil {
						return
					}
				} else {
					if _, err := stdin.Write(f.Img.Pix); err != nil {
						return
					}
				}
			}
		}()

		if err := cmd.Wait(); err != nil {
			errMsg := strings.TrimSpace(stderrBuf.String())
			if errMsg != "" {
				return fmt.Errorf("ffmpeg webp failed: %v: %s", err, errMsg)
			}
			return fmt.Errorf("ffmpeg webp failed: %v", err)
		}
		if progress != nil {
			progress(100, 100)
		}
		return nil
	}

	return errors.New("install webp tools (brew install webp) or ffmpeg to export to WebP")
}

func encodeMP4(path string, frames []Frame, end time.Time, fps int, progress func(i, n int)) error {
	ffmpeg := findBinary("ffmpeg")
	if ffmpeg == "" {
		return errors.New("install ffmpeg (brew install ffmpeg) to export to MP4")
	}

	if progress != nil {
		progress(5, 100)
	}

	bounds := frames[0].Img.Bounds()
	// H.264 requires even dimensions
	w := bounds.Dx() &^ 1
	h := bounds.Dy() &^ 1
	if w < 2 {
		w = 2
	}
	if h < 2 {
		h = 2
	}

	cmd := exec.Command(ffmpeg,
		"-y",
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", w, h),
		"-r", strconv.Itoa(max(fps, 1)),
		"-i", "-",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		"-crf", "22",
		path,
	)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg mp4 start failed: %w (%s)", err, strings.TrimSpace(stderrBuf.String()))
	}

	go func() {
		defer stdin.Close()
		needsCrop := (w != bounds.Dx() || h != bounds.Dy())
		var cropBuf *image.RGBA
		if needsCrop {
			cropBuf = image.NewRGBA(image.Rect(0, 0, w, h))
		}

		nFrames := len(frames)
		for i, f := range frames {
			if progress != nil {
				pct := 5 + ((i + 1) * 90 / max(1, nFrames))
				progress(pct, 100)
			}
			if needsCrop {
				// Copy cropped area
				for y := 0; y < h; y++ {
					copy(cropBuf.Pix[y*cropBuf.Stride:y*cropBuf.Stride+w*4],
						f.Img.Pix[y*f.Img.Stride:y*f.Img.Stride+w*4])
				}
				if _, err := stdin.Write(cropBuf.Pix); err != nil {
					return
				}
			} else {
				if _, err := stdin.Write(f.Img.Pix); err != nil {
					return
				}
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		errMsg := strings.TrimSpace(stderrBuf.String())
		if errMsg != "" {
			return fmt.Errorf("ffmpeg mp4 failed: %v: %s", err, errMsg)
		}
		return fmt.Errorf("ffmpeg mp4 failed: %v", err)
	}
	if progress != nil {
		progress(100, 100)
	}
	return nil
}
