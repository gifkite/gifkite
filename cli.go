package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/kbinani/screenshot"
)

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  gifkite                   start the menu bar app
  gifkite record [-o out.gif] [-fps 15] [-scale 1] [-display 0 | -region x,y,w,h] [-duration 0] [-max 60s]
  gifkite displays`)
}

func cmdDisplays() {
	for i := 0; i < screenshot.NumActiveDisplays(); i++ {
		b := screenshot.GetDisplayBounds(i)
		fmt.Printf("Display %d bounds: Min=(%d,%d) Size=(%d x %d)\n", i, b.Min.X, b.Min.Y, b.Dx(), b.Dy())
	}
	pt := getCursorPoint()
	fmt.Printf("Current cursor: X=%d Y=%d\n", pt.X, pt.Y)
}

func cmdRecord(args []string) {
	fs := flag.NewFlagSet("record", flag.ExitOnError)
	out := fs.String("o", "", "output file (default gifkite-<timestamp>.gif)")
	fps := fs.Int("fps", 15, "frames per second")
	scale := fs.Float64("scale", 1, "downscale factor, e.g. 0.5 halves width and height")
	display := fs.Int("display", 0, "display index (see `gifkite displays`)")
	region := fs.String("region", "", "region x,y,w,h in screen coordinates")
	duration := fs.Duration("duration", 0, "stop after this long (0 = wait for Ctrl+C)")
	maxDur := fs.Duration("max", 60*time.Second, "hard cap on recording length")
	dither := fs.String("dither", "bayer", "dithering algorithm: bayer, floyd, or none")
	showCursor := fs.Bool("cursor", true, "render mouse cursor in recording")
	highlight := fs.Bool("highlight", true, "highlight cursor with spotlight halo")
	ripples := fs.Bool("ripples", true, "render animated click ripple rings")
	fs.Parse(args)

	rect := screenshot.GetDisplayBounds(*display)
	var err error
	if *region != "" {
		rect, err = parseRegion(*region)
	}
	if err != nil {
		fatal(err)
	}
	if *out == "" {
		*out = defaultName()
	}

	rec := NewRecorder(rect, *fps, *scale, *maxDur, RecorderOptions{
		ShowCursor:      *showCursor,
		CursorHighlight: *highlight,
		ClickRipples:    *ripples,
	})
	rec.Start()
	fmt.Fprintf(os.Stderr, "recording %dx%d at %d,%d, Ctrl+C to stop\n", rect.Dx(), rect.Dy(), rect.Min.X, rect.Min.Y)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	var timeout <-chan time.Time
	if *duration > 0 {
		timeout = time.After(*duration)
	}
	select {
	case <-sig:
	case <-timeout:
	case <-rec.Done():
	}
	signal.Stop(sig)

	if err := saveRecording(rec, *out, *fps, *dither); err != nil {
		fatal(err)
	}
}

// saveRecording stops the recorder and writes the GIF.
func saveRecording(rec *Recorder, path string, fps int, dither string) error {
	frames, end, err := rec.Stop()
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	start := time.Now()
	stats, err := EncodeGIF(f, frames, end, fps, dither, 256, func(i, n int) {
		fmt.Fprintf(os.Stderr, "\rencoding %d/%d", i, n)
	})
	if err != nil {
		return err
	}
	info, _ := f.Stat()
	fmt.Fprintf(os.Stderr, "\rwrote %s: %d captured, %d written, %.1f KB, %.1fs to encode\n",
		path, stats.Captured, stats.Written, float64(info.Size())/1024, time.Since(start).Seconds())
	return nil
}

func parseRegion(s string) (image.Rectangle, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return image.Rectangle{}, fmt.Errorf("region must be x,y,w,h, got %q", s)
	}
	var v [4]int
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return image.Rectangle{}, fmt.Errorf("bad region value %q", p)
		}
		v[i] = n
	}
	if v[2] <= 0 || v[3] <= 0 {
		return image.Rectangle{}, fmt.Errorf("region width and height must be positive")
	}
	return image.Rect(v[0], v[1], v[0]+v[2], v[1]+v[3]), nil
}

func defaultName() string {
	return defaultNameWithExt(".gif")
}

func defaultNameWithExt(ext string) string {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return "gifkite-" + time.Now().Format("2006-01-02-150405") + ext
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gifkite:", err)
	os.Exit(1)
}
