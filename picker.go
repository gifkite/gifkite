//go:build gui

package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/kbinani/screenshot"
)

// The picker freezes the screen: it grabs a screenshot, shows it
// fullscreen, and lets you drag a rectangle over it. Same trick as
// Gifox and macOS's own Cmd+Shift+5.
type picker struct {
	shot     *ebiten.Image
	sw, sh   int
	dragging bool
	a, b     image.Point
	result   image.Rectangle
}

func cmdPick() {
	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		fatal(err)
	}
	p := &picker{shot: ebiten.NewImageFromImage(img), sw: img.Bounds().Dx(), sh: img.Bounds().Dy()}

	ebiten.SetWindowTitle("Gifkite: drag to select, Esc to cancel")
	ebiten.SetFullscreen(true)
	ebiten.SetCursorShape(ebiten.CursorShapeCrosshair)
	if err := ebiten.RunGame(p); err != nil && err != ebiten.Termination {
		fatal(err)
	}
	if p.result.Empty() {
		os.Exit(1)
	}
	// The screenshot may be in physical pixels (Retina) while capture
	// bounds are in screen coordinates, so convert by ratio.
	sx := float64(bounds.Dx()) / float64(p.sw)
	sy := float64(bounds.Dy()) / float64(p.sh)
	r := p.result
	fmt.Printf("%d,%d,%d,%d\n",
		bounds.Min.X+int(float64(r.Min.X)*sx), bounds.Min.Y+int(float64(r.Min.Y)*sy),
		int(float64(r.Dx())*sx), int(float64(r.Dy())*sy))
}

func (p *picker) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	x, y := ebiten.CursorPosition()
	switch {
	case inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft):
		p.dragging, p.a, p.b = true, image.Pt(x, y), image.Pt(x, y)
	case p.dragging && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft):
		p.b = image.Pt(x, y)
	case p.dragging:
		p.dragging = false
		if r := p.sel(); r.Dx() > 4 && r.Dy() > 4 {
			p.result = r
			return ebiten.Termination
		}
	}
	return nil
}

func (p *picker) sel() image.Rectangle {
	return image.Rectangle{Min: p.a, Max: p.b}.Canon().Intersect(image.Rect(0, 0, p.sw, p.sh))
}

func (p *picker) Draw(screen *ebiten.Image) {
	screen.DrawImage(p.shot, nil)
	vector.DrawFilledRect(screen, 0, 0, float32(p.sw), float32(p.sh), color.RGBA{0, 0, 0, 110}, false)
	if !p.dragging {
		ebitenutil.DebugPrintAt(screen, "drag to select a region, Esc to cancel", 20, 20)
		return
	}
	r := p.sel()
	if r.Empty() {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
	screen.DrawImage(p.shot.SubImage(r).(*ebiten.Image), op)
	vector.StrokeRect(screen, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), 2, color.RGBA{255, 80, 80, 255}, false)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d x %d", r.Dx(), r.Dy()), r.Min.X, max(0, r.Min.Y-18))
}

func (p *picker) Layout(_, _ int) (int, int) { return p.sw, p.sh }
