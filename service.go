package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	controlsW = 278
	controlsH = 52
)

// Phases, in order. The frontend renders from these.
const (
	phaseIdle      = "idle"
	phasePicking   = "picking"
	phaseCountdown = "countdown"
	phaseRecording = "recording"
	phasePaused    = "paused"
	phaseReview    = "review"
	phaseEncoding  = "encoding"
)

// NormRect is a region as fractions of the primary display (0..1).
// Screen capture and window placement use different coordinate spaces
// (physical pixels vs. DIP, depending on the OS), so the region is stored
// in a space both can be derived from.
type NormRect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type State struct {
	Phase    string   `json:"phase"`
	Elapsed  float64  `json:"elapsed"`
	HasLast  bool     `json:"hasLast"`
	Hotkey   string   `json:"hotkey"`
	Settings Settings `json:"settings"`
	Error    string   `json:"error,omitempty"`
	HasPerm  bool     `json:"hasPerm"`
	Paused   bool     `json:"paused"`
}

type ReviewInfo struct {
	NumFrames int     `json:"numFrames"`
	Duration  float64 `json:"duration"` // total seconds
	FPS       int     `json:"fps"`
}

// GifService is bound to the frontend: its exported methods are callable
// from JS as main.GifService.<Method>.
type GifService struct {
	app                       *application.App
	tray                      *application.SystemTray
	popover, picker, controls *application.WebviewWindow

	mu            sync.Mutex
	phase         string
	rec           *Recorder
	started       time.Time
	last          *NormRect
	frozen        []byte
	abort         chan struct{}
	settings      Settings
	lastErr       string
	thumbs        *thumbCache
	pendingFrames []Frame
	pendingEnd    time.Time
	isDragging    bool

	activeDisplay int
	activeScreen  *application.Screen
}

var defaultGifService *GifService

func newGifService() *GifService {
	s := &GifService{phase: phaseIdle, settings: loadSettings(), thumbs: newThumbCache()}
	defaultGifService = s
	return s
}

func (s *GifService) onDragFinished(op int) {
	s.mu.Lock()
	s.isDragging = false
	s.mu.Unlock()
	if op != 0 && s.popover != nil {
		s.popover.Hide()
	}
}

func (s *GifService) getActiveDisplay() (int, *application.Screen) {
	pt := getCursorPoint()
	numDisplays := screenshot.NumActiveDisplays()
	activeIdx := 0

	for i := 0; i < numDisplays; i++ {
		b := screenshot.GetDisplayBounds(i)
		if pt.X >= b.Min.X && pt.X < b.Max.X && pt.Y >= b.Min.Y && pt.Y < b.Max.Y {
			activeIdx = i
			break
		}
	}

	scr := s.app.Screen.ScreenNearestDipPoint(application.Point{X: pt.X, Y: pt.Y})
	if scr == nil {
		scr = s.app.Screen.GetPrimary()
	}
	return activeIdx, scr
}

// ---- state ----

func (s *GifService) checkPermission() bool {
	return hasScreenPermission()
}

func (s *GifService) OpenPermissionSettings() {
	requestScreenPermission()
	if runtime.GOOS == "darwin" {
		exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture").Start()
	}
}

func playSound(name string) {
	if runtime.GOOS == "darwin" {
		p := filepath.Join("/System/Library/Sounds", name+".aiff")
		if _, err := os.Stat(p); err == nil {
			go exec.Command("afplay", p).Run()
		}
	}
}

func (s *GifService) GetState() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := State{
		Phase:    s.phase,
		HasLast:  s.last != nil,
		Hotkey:   hotkey,
		Settings: s.settings,
		Error:    s.lastErr,
		HasPerm:  s.checkPermission(),
		Paused:   s.phase == phasePaused,
	}
	if s.rec != nil {
		net := time.Since(s.started) - s.rec.PausedDuration()
		if net < 0 {
			net = 0
		}
		st.Elapsed = net.Seconds()
	}
	return st
}

func (s *GifService) broadcast() { s.app.Event.Emit("state", s.GetState()) }

// transition moves from one phase to another only if we're in `from`.
func (s *GifService) transition(from, to string) bool {
	s.mu.Lock()
	ok := s.phase == from
	if ok {
		s.phase = to
		s.lastErr = ""
	}
	s.mu.Unlock()
	if ok {
		s.broadcast()
	}
	return ok
}

func (s *GifService) fail(err error) {
	s.mu.Lock()
	s.phase = phaseIdle
	s.lastErr = err.Error()
	s.mu.Unlock()
	s.controls.Hide()
	s.picker.Hide()
	s.tray.SetLabel("")
	s.broadcast()
	s.tray.ShowWindow()
}

// Toggle is what the global hotkey does: start, or stop whatever is running.
func (s *GifService) Toggle() {
	switch s.GetState().Phase {
	case phaseIdle:
		s.StartRegion()
	case phasePicking:
		s.PickerCancel()
	case phaseCountdown, phaseRecording, phasePaused:
		s.Stop()
	}
}

func (s *GifService) Pause() {
	s.mu.Lock()
	if s.phase == phaseRecording && s.rec != nil {
		s.rec.Pause()
		s.phase = phasePaused
		net := time.Since(s.started) - s.rec.PausedDuration()
		if net < 0 {
			net = 0
		}
		s.tray.SetLabel(fmt.Sprintf("❚❚ %d:%02d", int(net.Minutes()), int(net.Seconds())%60))
		s.mu.Unlock()
		s.broadcast()
		return
	}
	s.mu.Unlock()
}

func (s *GifService) Resume() {
	s.mu.Lock()
	if s.phase == phasePaused && s.rec != nil {
		s.rec.Resume()
		s.phase = phaseRecording
		s.mu.Unlock()
		s.broadcast()
		return
	}
	s.mu.Unlock()
}

func (s *GifService) TogglePause() {
	s.mu.Lock()
	phase := s.phase
	s.mu.Unlock()
	switch phase {
	case phaseRecording:
		s.Pause()
	case phasePaused:
		s.Resume()
	}
}

// ---- starting ----

// StartRegion freezes the screen and opens the region/window picker.
func (s *GifService) StartRegion() {
	if !s.transition(phaseIdle, phasePicking) {
		return
	}
	s.popover.Hide()
	time.Sleep(180 * time.Millisecond) // let the popover fade before we grab the screen

	dispIdx, scr := s.getActiveDisplay()
	s.mu.Lock()
	s.activeDisplay = dispIdx
	s.activeScreen = scr
	s.mu.Unlock()

	img, err := screenshot.CaptureRect(screenshot.GetDisplayBounds(dispIdx))
	if err != nil {
		s.fail(fmt.Errorf("couldn't capture screen %d: %w (check screen recording permission)", dispIdx, err))
		return
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 88})
	s.mu.Lock()
	s.frozen = buf.Bytes()
	s.mu.Unlock()

	wins := listWindows(scr.Bounds.X, scr.Bounds.Y, scr.Bounds.Width, scr.Bounds.Height)
	s.picker.SetBounds(scr.Bounds)
	s.app.Event.Emit("picker:open", map[string]any{
		"time":    time.Now().UnixNano(),
		"windows": wins,
	})
	s.picker.Show().Focus()
}

func (s *GifService) StartWindow() {
	s.StartRegion()
}

// PickerDone is called by the picker with the dragged region.
func (s *GifService) PickerDone(r NormRect) {
	s.picker.Hide()
	if r.W <= 0 || r.H <= 0 || !s.transition(phasePicking, phaseCountdown) {
		s.PickerCancel()
		return
	}
	s.mu.Lock()
	s.last = &r
	s.mu.Unlock()
	go s.begin(r, false)
}

func (s *GifService) PickerCancel() {
	s.picker.Hide()
	s.transition(phasePicking, phaseIdle)
}

func (s *GifService) StartScreen() {
	if !s.transition(phaseIdle, phaseCountdown) {
		return
	}
	s.popover.Hide()
	dispIdx, scr := s.getActiveDisplay()
	s.mu.Lock()
	s.activeDisplay = dispIdx
	s.activeScreen = scr
	s.mu.Unlock()
	go s.begin(NormRect{0, 0, 1, 1}, true)
}

func (s *GifService) StartLast() {
	s.mu.Lock()
	last := s.last
	s.mu.Unlock()
	if last == nil || !s.transition(phaseIdle, phaseCountdown) {
		return
	}
	s.popover.Hide()
	go s.begin(*last, false)
}

func (s *GifService) begin(r NormRect, full bool) {
	abort := make(chan struct{})
	s.mu.Lock()
	s.abort = abort
	cfg := s.settings
	s.mu.Unlock()

	if cfg.Countdown {
		for n := 3; n > 0; n-- {
			s.app.Event.Emit("countdown", n)
			s.tray.SetLabel(fmt.Sprintf("%d…", n))
			select {
			case <-abort:
				s.tray.SetLabel("")
				s.transition(phaseCountdown, phaseIdle)
				return
			case <-time.After(time.Second):
			}
		}
		s.app.Event.Emit("countdown", 0)
	} else if full {
		time.Sleep(150 * time.Millisecond) // popover fade
	}

	rec := NewRecorder(s.toCapture(r), cfg.FPS, cfg.Scale, time.Duration(cfg.MaxSeconds)*time.Second, RecorderOptions{
		ShowCursor:      cfg.ShowCursor,
		CursorHighlight: cfg.CursorHighlight,
		ClickRipples:    cfg.ClickRipples,
	})
	s.mu.Lock()
	if s.phase != phaseCountdown { // stopped during the countdown
		s.mu.Unlock()
		return
	}
	s.rec = rec
	s.started = time.Now()
	s.phase = phaseRecording
	s.mu.Unlock()
	rec.Start()
	playSound("Tink")
	s.broadcast()

	s.placeControls(s.toDIP(r))
	s.controls.Show()

	go func() { // timer for the tray and the controls pill
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-rec.Done():
				return
			case <-t.C:
				s.mu.Lock()
				ph := s.phase
				s.mu.Unlock()
				net := time.Since(s.started) - rec.PausedDuration()
				if net < 0 {
					net = 0
				}
				if ph == phasePaused {
					s.tray.SetLabel(fmt.Sprintf("❚❚ %d:%02d", int(net.Minutes()), int(net.Seconds())%60))
				} else {
					s.tray.SetLabel(fmt.Sprintf("● %d:%02d", int(net.Minutes()), int(net.Seconds())%60))
				}
				s.app.Event.Emit("tick", net.Seconds())
			}
		}
	}()
	go func() { // hit the length cap: save automatically
		<-rec.Done()
		s.finish(rec, true)
	}()
}

// ---- stopping ----

// Stop ends the recording (or countdown) and saves the GIF.
func (s *GifService) Stop() { s.end(true) }

// Discard ends the recording without saving.
func (s *GifService) Discard() { s.end(false) }

func (s *GifService) end(save bool) {
	s.mu.Lock()
	phase, rec, abort := s.phase, s.rec, s.abort
	s.mu.Unlock()
	switch phase {
	case phaseCountdown:
		if abort != nil {
			select {
			case <-abort:
			default:
				close(abort)
			}
		}
	case phaseRecording, phasePaused:
		go s.finish(rec, save)
	}
}

func (s *GifService) finish(rec *Recorder, save bool) {
	s.mu.Lock()
	if s.rec != rec {
		s.mu.Unlock()
		return // already handled
	}
	s.rec = nil
	s.mu.Unlock()

	s.controls.Hide()
	frames, end, err := rec.Stop()
	if !save {
		s.mu.Lock()
		s.phase = phaseIdle
		s.mu.Unlock()
		s.tray.SetLabel("")
		s.broadcast()
		return
	}
	if err != nil {
		s.fail(err)
		return
	}
	if len(frames) == 0 {
		s.fail(errors.New("no frames captured"))
		return
	}

	s.mu.Lock()
	shouldTrim := s.settings.Trim
	if shouldTrim {
		s.pendingFrames = frames
		s.pendingEnd = end
		s.phase = phaseReview
		info := s.reviewInfoLocked()
		s.mu.Unlock()

		s.tray.SetLabel("Review")
		s.broadcast()
		s.app.Event.Emit("review:open", info)
		s.tray.ShowWindow()
		return
	}
	s.phase = phaseEncoding
	s.mu.Unlock()

	s.tray.SetLabel("Saving…")
	s.broadcast()
	go s.encodeAndSave(frames, end)
}

func (s *GifService) reviewInfoLocked() ReviewInfo {
	n := len(s.pendingFrames)
	if n == 0 {
		return ReviewInfo{NumFrames: 0, Duration: 0, FPS: s.settings.FPS}
	}
	dur := s.pendingEnd.Sub(s.pendingFrames[0].At).Seconds()
	if dur <= 0 {
		dur = float64(n) / float64(max(s.settings.FPS, 1))
	}
	return ReviewInfo{
		NumFrames: n,
		Duration:  dur,
		FPS:       s.settings.FPS,
	}
}

// GetReviewInfo returns summary info for current pending recording in review.
func (s *GifService) GetReviewInfo() ReviewInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reviewInfoLocked()
}

// GetPreviewFrame returns the JPEG-encoded image of the requested pending frame index.
func (s *GifService) GetPreviewFrame(i int) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 0 || i >= len(s.pendingFrames) {
		return nil
	}
	img := s.pendingFrames[i].Img
	if img == nil {
		return nil
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		return nil
	}
	return buf.Bytes()
}

// ConfirmReview trims the pending frames to [startIdx, endIdx] inclusive and encodes the GIF.
func (s *GifService) ConfirmReview(startIdx, endIdx int) error {
	s.mu.Lock()
	if s.phase != phaseReview || len(s.pendingFrames) == 0 {
		s.mu.Unlock()
		return errors.New("not in review mode")
	}
	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx >= len(s.pendingFrames) {
		endIdx = len(s.pendingFrames) - 1
	}
	if startIdx > endIdx {
		startIdx, endIdx = endIdx, startIdx
	}

	trimmed := make([]Frame, endIdx-startIdx+1)
	copy(trimmed, s.pendingFrames[startIdx:endIdx+1])

	var end time.Time
	if endIdx+1 < len(s.pendingFrames) {
		end = s.pendingFrames[endIdx+1].At
	} else {
		end = s.pendingEnd
	}

	s.pendingFrames = nil
	s.phase = phaseEncoding
	s.mu.Unlock()

	s.tray.SetLabel("Saving…")
	s.broadcast()
	go s.encodeAndSave(trimmed, end)
	return nil
}

// DiscardReview discards the pending recording without saving.
func (s *GifService) DiscardReview() {
	s.mu.Lock()
	if s.phase != phaseReview {
		s.mu.Unlock()
		return
	}
	s.pendingFrames = nil
	s.phase = phaseIdle
	s.mu.Unlock()

	s.tray.SetLabel("")
	s.broadcast()
}

func (s *GifService) encodeAndSave(frames []Frame, end time.Time) {
	s.mu.Lock()
	fps, dir, dither := s.settings.FPS, s.settings.OutputDir, s.settings.Dither
	s.mu.Unlock()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.fail(err)
		return
	}
	path := filepath.Join(dir, defaultName())
	f, err := os.Create(path)
	if err != nil {
		s.fail(err)
		return
	}
	lastPct := -1
	_, err = EncodeGIF(f, frames, end, fps, dither, func(i, n int) {
		if pct := i * 100 / n; pct != lastPct {
			lastPct = pct
			s.app.Event.Emit("encode", pct)
		}
	})
	f.Close()
	if err != nil {
		os.Remove(path)
		s.fail(err)
		return
	}

	s.mu.Lock()
	s.phase = phaseIdle
	s.mu.Unlock()
	s.tray.SetLabel("")
	s.broadcast()
	s.app.Event.Emit("library", filepath.Base(path))
	playSound("Pop")
	s.tray.ShowWindow() // like Gifox: pop the new recording up
}

// ---- geometry ----

func (s *GifService) toCapture(r NormRect) image.Rectangle {
	s.mu.Lock()
	disp := s.activeDisplay
	s.mu.Unlock()

	b := screenshot.GetDisplayBounds(disp)
	x0 := b.Min.X + int(r.X*float64(b.Dx()))
	y0 := b.Min.Y + int(r.Y*float64(b.Dy()))
	x1 := b.Min.X + int((r.X+r.W)*float64(b.Dx()))
	y1 := b.Min.Y + int((r.Y+r.H)*float64(b.Dy()))
	return image.Rect(x0, y0, x1, y1).Intersect(b)
}

func (s *GifService) toDIP(r NormRect) application.Rect {
	s.mu.Lock()
	scr := s.activeScreen
	s.mu.Unlock()
	if scr == nil {
		scr = s.app.Screen.GetPrimary()
	}
	b := scr.Bounds
	return application.Rect{
		X:      b.X + int(r.X*float64(b.Width)),
		Y:      b.Y + int(r.Y*float64(b.Height)),
		Width:  int(r.W * float64(b.Width)),
		Height: int(r.H * float64(b.Height)),
	}
}

// placeControls puts the pill centred under the region, or above it, or
// (for regions that fill the screen) inside the bottom edge.
func (s *GifService) placeControls(d application.Rect) {
	s.mu.Lock()
	scr := s.activeScreen
	s.mu.Unlock()
	if scr == nil {
		scr = s.app.Screen.GetPrimary()
	}
	wa := scr.WorkArea
	x := d.X + d.Width/2 - controlsW/2
	x = max(wa.X+8, min(x, wa.X+wa.Width-controlsW-8))
	gap := 12
	var y int
	switch {
	case d.Y+d.Height+gap+controlsH <= wa.Y+wa.Height:
		y = d.Y + d.Height + gap
	case d.Y-gap-controlsH >= wa.Y:
		y = d.Y - gap - controlsH
	default:
		y = wa.Y + wa.Height - controlsH - 16
	}
	s.controls.SetBounds(application.Rect{X: x, Y: y, Width: controlsW, Height: controlsH})
}

func (s *GifService) Quit() { s.app.Quit() }
