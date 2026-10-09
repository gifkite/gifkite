package main

import (
	"bytes"
	"image"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
)

// Frame is one captured image and the moment it was grabbed.
type Frame struct {
	Img        *image.RGBA
	At         time.Time
	ScreenRect image.Rectangle
}

// Recorder grabs a screen rectangle on a ticker until stopped.
// Consecutive identical frames are dropped at capture time, so a static
type RecorderOptions struct {
	ShowCursor      bool
	CursorHighlight bool
	ClickRipples    bool
	WindowID        int
}

// Recorder grabs a screen rectangle on a ticker until stopped.
// Consecutive identical frames are dropped at capture time, so a static
// screen costs almost no memory: the previous frame simply lasts longer.
type Recorder struct {
	rect   image.Rectangle
	fps    int
	scale  float64
	maxDur time.Duration
	opts   RecorderOptions

	clickTracker *ClickTracker

	frames []Frame
	end    time.Time
	err    error

	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once

	mu          sync.Mutex
	paused      bool
	pauseStart  time.Time
	pausedTotal time.Duration
}

func NewRecorder(rect image.Rectangle, fps int, scale float64, maxDur time.Duration, opts RecorderOptions) *Recorder {
	if fps < 1 {
		fps = 1
	}
	if fps > 60 {
		fps = 60
	}
	if scale <= 0 || scale > 1 {
		scale = 1
	}
	return &Recorder{rect: rect, fps: fps, scale: scale, maxDur: maxDur, opts: opts,
		stop: make(chan struct{}), done: make(chan struct{})}
}

func (r *Recorder) Start() { go r.loop() }

// Done closes when capture ends, whether stopped, capped, or failed.
func (r *Recorder) Done() <-chan struct{} { return r.done }

func (r *Recorder) Pause() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.paused {
		r.paused = true
		r.pauseStart = time.Now()
	}
}

func (r *Recorder) Resume() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.paused {
		r.paused = false
		r.pausedTotal += time.Since(r.pauseStart)
	}
}

func (r *Recorder) TogglePause() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.paused {
		r.paused = false
		r.pausedTotal += time.Since(r.pauseStart)
		return false
	}
	r.paused = true
	r.pauseStart = time.Now()
	return true
}

func (r *Recorder) IsPaused() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.paused
}

func (r *Recorder) PausedDuration() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	tot := r.pausedTotal
	if r.paused {
		tot += time.Since(r.pauseStart)
	}
	return tot
}

// Stop ends capture and returns the frames plus the time capture ended,
// which the encoder needs to size the last frame's delay.
func (r *Recorder) Stop() ([]Frame, time.Time, error) {
	r.stopOnce.Do(func() { close(r.stop) })
	<-r.done
	adjustedEnd := r.end.Add(-r.PausedDuration())
	return r.frames, adjustedEnd, r.err
}

func (r *Recorder) loopSCK() bool {
	if !isSCKAvailable() {
		return false
	}

	frameCh := make(chan Frame, 30)
	sckShowCursor := r.opts.ShowCursor

	stream, err := startSCK(r.rect, r.opts.WindowID, r.fps, sckShowCursor, func(img *image.RGBA, at time.Time, screenRect image.Rectangle) {
		select {
		case frameCh <- Frame{Img: img, At: at, ScreenRect: screenRect}:
		default:
		}
	})
	if err != nil || stream == nil {
		return false
	}
	defer stream.Stop()

	if r.opts.ClickRipples {
		r.clickTracker = NewClickTracker()
		r.clickTracker.Start()
		defer r.clickTracker.Stop()
	}

	start := time.Now()
	var last *image.RGBA

	for {
		select {
		case <-r.stop:
			return true
		case frame := <-frameCh:
			if r.IsPaused() {
				continue
			}
			now := frame.At
			img := frame.Img

			if r.opts.CursorHighlight || r.opts.ClickRipples || (r.opts.ShowCursor && !sckShowCursor) {
				cursor := getCursorPoint()
				var clicks []ClickEvent
				if r.clickTracker != nil {
					clicks = r.clickTracker.ActiveClicks(now)
				}
				captureRect := frame.ScreenRect
				if captureRect.Empty() {
					captureRect = r.rect
				}
				RenderCursorEffects(img, captureRect, cursor, clicks, now, r.opts.ShowCursor && !sckShowCursor, r.opts.CursorHighlight, r.opts.ClickRipples)
			}

			img = r.resize(img)
			adjustedAt := now.Add(-r.PausedDuration())
			if last == nil || !bytes.Equal(last.Pix, img.Pix) {
				r.frames = append(r.frames, Frame{Img: img, At: adjustedAt})
				last = img
			}
			if r.maxDur > 0 && now.Sub(start)-r.PausedDuration() >= r.maxDur {
				return true
			}
		}
	}
}

func (r *Recorder) loop() {
	defer close(r.done)
	defer func() { r.end = time.Now() }()

	if isSCKAvailable() {
		if ok := r.loopSCK(); ok {
			return
		}
	}

	if r.opts.ClickRipples {
		r.clickTracker = NewClickTracker()
		r.clickTracker.Start()
		defer r.clickTracker.Stop()
	}

	tick := time.NewTicker(time.Second / time.Duration(r.fps))
	defer tick.Stop()
	start := time.Now()
	var last *image.RGBA

	for {
		select {
		case <-r.stop:
			return
		case <-tick.C:
		}

		if r.IsPaused() {
			continue
		}

		now := time.Now()
		img, err := screenshot.CaptureRect(r.rect)
		if err != nil {
			r.err = err
			return
		}

		if r.opts.ShowCursor || r.opts.CursorHighlight || r.opts.ClickRipples {
			cursor := getCursorPoint()
			var clicks []ClickEvent
			if r.clickTracker != nil {
				clicks = r.clickTracker.ActiveClicks(now)
			}
			RenderCursorEffects(img, r.rect, cursor, clicks, now, r.opts.ShowCursor, r.opts.CursorHighlight, r.opts.ClickRipples)
		}

		img = r.resize(img)
		adjustedAt := now.Add(-r.PausedDuration())
		if last == nil || !bytes.Equal(last.Pix, img.Pix) {
			r.frames = append(r.frames, Frame{Img: img, At: adjustedAt})
			last = img
		}
		if r.maxDur > 0 && now.Sub(start)-r.PausedDuration() >= r.maxDur {
			return
		}
	}
}

func (r *Recorder) resize(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	if r.scale == 1 && b.Min == (image.Point{}) {
		return src
	}
	w := max(1, int(float64(b.Dx())*r.scale))
	h := max(1, int(float64(b.Dy())*r.scale))
	return boxScale(src, w, h)
}

// boxScale downsizes by averaging every source pixel that falls inside each
// destination pixel. For shrinking screen content (text, UI edges) this
// looks cleaner than bilinear sampling, which skips pixels.
func boxScale(src *image.RGBA, w, h int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for dy := 0; dy < h; dy++ {
		y0, y1 := dy*sh/h, max((dy+1)*sh/h, dy*sh/h+1)
		for dx := 0; dx < w; dx++ {
			x0, x1 := dx*sw/w, max((dx+1)*sw/w, dx*sw/w+1)
			var rs, gs, bs, n int
			for y := y0; y < y1; y++ {
				row := src.Pix[y*src.Stride:]
				for x := x0; x < x1; x++ {
					p := row[x*4:]
					rs += int(p[0])
					gs += int(p[1])
					bs += int(p[2])
					n++
				}
			}
			o := dst.Pix[dy*dst.Stride+dx*4:]
			o[0], o[1], o[2], o[3] = uint8(rs/n), uint8(gs/n), uint8(bs/n), 255
		}
	}
	return dst
}
