//go:build gui

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"fyne.io/systray"
	"github.com/kbinani/screenshot"
	"golang.design/x/hotkey"
)

const (
	trayFPS    = 15
	trayMaxDur = 60 * time.Second
)

type trayApp struct {
	mu       sync.Mutex
	rec      *Recorder
	lastRect image.Rectangle
	mScreen  *systray.MenuItem
	mRegion  *systray.MenuItem
	mRepeat  *systray.MenuItem
	mStop    *systray.MenuItem
}

func cmdTray() { systray.Run((&trayApp{}).onReady, nil) }

func (t *trayApp) onReady() {
	if runtime.GOOS == "darwin" {
		systray.SetTemplateIcon(trayIconTemplate(), trayIconTemplate())
	} else {
		systray.SetIcon(trayIcon())
	}
	systray.SetTooltip("Gifkite: Ctrl+Shift+5 to start/stop")

	t.mScreen = systray.AddMenuItem("Record screen", "")
	t.mRegion = systray.AddMenuItem("Record region…", "")
	t.mRepeat = systray.AddMenuItem("Record last region", "")
	t.mRepeat.Disable()
	t.mStop = systray.AddMenuItem("Stop & save", "")
	t.mStop.Disable()
	systray.AddSeparator()
	mOpen := systray.AddMenuItem("Open output folder", "")
	mQuit := systray.AddMenuItem("Quit", "")

	// Global hotkey toggles recording of the last region (or pick one).
	hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModShift}, hotkey.Key5)
	if err := hk.Register(); err != nil {
		fmt.Fprintln(os.Stderr, "hotkey unavailable:", err)
	} else {
		go func() {
			for range hk.Keydown() {
				t.toggle()
			}
		}()
	}

	go func() {
		for {
			select {
			case <-t.mScreen.ClickedCh:
				t.start(screenshot.GetDisplayBounds(0))
			case <-t.mRegion.ClickedCh:
				t.pickAndStart()
			case <-t.mRepeat.ClickedCh:
				t.start(t.lastRect)
			case <-t.mStop.ClickedCh:
				t.stop()
			case <-mOpen.ClickedCh:
				openPath(outputDir())
			case <-mQuit.ClickedCh:
				t.stop()
				systray.Quit()
				return
			}
		}
	}()
}

func (t *trayApp) toggle() {
	t.mu.Lock()
	recording := t.rec != nil
	last := t.lastRect
	t.mu.Unlock()
	switch {
	case recording:
		t.stop()
	case !last.Empty():
		t.start(last)
	default:
		t.pickAndStart()
	}
}

func (t *trayApp) pickAndStart() {
	r, err := pickRegionSubprocess()
	if err != nil {
		return
	}
	t.start(r)
}

func (t *trayApp) start(r image.Rectangle) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.rec != nil || r.Empty() {
		return
	}
	t.lastRect = r
	t.rec = NewRecorder(r, trayFPS, 1, trayMaxDur, RecorderOptions{ShowCursor: true, CursorHighlight: true, ClickRipples: true})
	t.rec.Start()
	rec := t.rec
	go func() { // auto-save when the length cap is hit
		<-rec.Done()
		t.stopIf(rec)
	}()
	systray.SetTitle("● REC")
	t.mScreen.Disable()
	t.mRegion.Disable()
	t.mRepeat.Disable()
	t.mStop.Enable()
}

func (t *trayApp) stop() { t.stopIf(nil) }

// stopIf stops the current recording. With a non-nil want it only acts if
// that exact recording is still running, so a late auto-stop can't end a
// newer one.
func (t *trayApp) stopIf(want *Recorder) {
	t.mu.Lock()
	rec := t.rec
	if rec == nil || (want != nil && rec != want) {
		t.mu.Unlock()
		return
	}
	t.rec = nil
	t.mu.Unlock()
	systray.SetTitle("Encoding…")
	t.mStop.Disable()

	path := filepath.Join(outputDir(), defaultName())
	if err := saveRecording(rec, path, trayFPS); err != nil {
		fmt.Fprintln(os.Stderr, "save failed:", err)
	} else {
		revealPath(path)
	}
	systray.SetTitle("GIF")
	t.mScreen.Enable()
	t.mRegion.Enable()
	t.mRepeat.Enable()
}

func outputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	if d := filepath.Join(home, "Desktop"); dirExists(d) {
		return d
	}
	return home
}

func dirExists(p string) bool { st, err := os.Stat(p); return err == nil && st.IsDir() }

func openPath(p string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", p).Start()
	case "windows":
		exec.Command("explorer", p).Start()
	default:
		exec.Command("xdg-open", p).Start()
	}
}

func revealPath(p string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", "-R", p).Start()
	case "windows":
		exec.Command("explorer", "/select,", p).Start()
	default:
		exec.Command("xdg-open", filepath.Dir(p)).Start()
	}
}

// trayIcon draws a small red dot. Windows wants ICO bytes, everything else
// takes PNG; an ICO can wrap a PNG directly, so both come from one image.
func trayIcon() []byte {
	const s = 32
	img := image.NewRGBA(image.Rect(0, 0, s, s))
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			dx, dy := x-s/2, y-s/2
			if dx*dx+dy*dy <= 11*11 {
				img.Set(x, y, color.RGBA{230, 60, 60, 255})
			}
		}
	}
	var pngBuf bytes.Buffer
	png.Encode(&pngBuf, img)
	if runtime.GOOS != "windows" {
		return pngBuf.Bytes()
	}
	var ico bytes.Buffer
	binary.Write(&ico, binary.LittleEndian, []uint16{0, 1, 1})                  // header: reserved, type=icon, count
	ico.Write([]byte{s, s, 0, 0})                                               // w, h, colors, reserved
	binary.Write(&ico, binary.LittleEndian, []uint16{1, 32})                    // planes, bpp
	binary.Write(&ico, binary.LittleEndian, []uint32{uint32(pngBuf.Len()), 22}) // size, offset
	ico.Write(pngBuf.Bytes())
	return ico.Bytes()
}
