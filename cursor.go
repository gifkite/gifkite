package main

import (
	"image"
	"image/color"
	"math"
	"sync"
	"time"
)

type ClickEvent struct {
	Point  CursorPoint
	At     time.Time
	Button int // 0 = left, 1 = right
}

// ClickTracker polls mouse button state at 100Hz and records click events.
type ClickTracker struct {
	mu     sync.Mutex
	clicks []ClickEvent
	stop   chan struct{}
	done   chan struct{}
}

func NewClickTracker() *ClickTracker {
	return &ClickTracker{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (ct *ClickTracker) Start() {
	go ct.poll()
}

func (ct *ClickTracker) Stop() {
	close(ct.stop)
	<-ct.done
}

func (ct *ClickTracker) poll() {
	defer close(ct.done)
	ticker := time.NewTicker(10 * time.Millisecond) // 100Hz
	defer ticker.Stop()

	var wasLeft, wasRight bool

	for {
		select {
		case <-ct.stop:
			return
		case <-ticker.C:
			left := isMouseLeftDown()
			right := isMouseRightDown()
			now := time.Now()

			if left && !wasLeft {
				pt := getCursorPoint()
				ct.mu.Lock()
				ct.clicks = append(ct.clicks, ClickEvent{Point: pt, At: now, Button: 0})
				ct.mu.Unlock()
			}
			if right && !wasRight {
				pt := getCursorPoint()
				ct.mu.Lock()
				ct.clicks = append(ct.clicks, ClickEvent{Point: pt, At: now, Button: 1})
				ct.mu.Unlock()
			}
			wasLeft = left
			wasRight = right

			// Prune clicks older than 600ms
			ct.mu.Lock()
			cutoff := now.Add(-600 * time.Millisecond)
			n := 0
			for _, c := range ct.clicks {
				if c.At.After(cutoff) {
					ct.clicks[n] = c
					n++
				}
			}
			ct.clicks = ct.clicks[:n]
			ct.mu.Unlock()
		}
	}
}

func (ct *ClickTracker) ActiveClicks(now time.Time) []ClickEvent {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	cutoff := now.Add(-400 * time.Millisecond)
	var active []ClickEvent
	for _, c := range ct.clicks {
		if c.At.After(cutoff) && !c.At.After(now) {
			active = append(active, c)
		}
	}
	return active
}

// blendPixel blends src RGBA over dst RGBA in-place with alpha compositing.
func blendPixel(dst *image.RGBA, x, y int, r, g, b, a uint8) {
	if a == 0 || x < dst.Rect.Min.X || x >= dst.Rect.Max.X || y < dst.Rect.Min.Y || y >= dst.Rect.Max.Y {
		return
	}
	idx := (y-dst.Rect.Min.Y)*dst.Stride + (x-dst.Rect.Min.X)*4
	if a == 255 {
		dst.Pix[idx] = r
		dst.Pix[idx+1] = g
		dst.Pix[idx+2] = b
		dst.Pix[idx+3] = 255
		return
	}
	sa := uint32(a)
	dr := uint32(dst.Pix[idx])
	dg := uint32(dst.Pix[idx+1])
	db := uint32(dst.Pix[idx+2])
	da := uint32(dst.Pix[idx+3])

	invA := 255 - sa
	dst.Pix[idx] = uint8((uint32(r)*sa + dr*invA) / 255)
	dst.Pix[idx+1] = uint8((uint32(g)*sa + dg*invA) / 255)
	dst.Pix[idx+2] = uint8((uint32(b)*sa + db*invA) / 255)
	dst.Pix[idx+3] = uint8(sa + (da*invA)/255)
}

// drawHalo draws a soft translucent circular spotlight.
func drawHalo(img *image.RGBA, cx, cy int, radius float64, clr color.RGBA) {
	rInt := int(math.Ceil(radius))
	for dy := -rInt; dy <= rInt; dy++ {
		for dx := -rInt; dx <= rInt; dx++ {
			dist := math.Hypot(float64(dx), float64(dy))
			if dist <= radius {
				factor := 1.0 - (dist/radius)*(dist/radius)
				a := uint8(float64(clr.A) * factor)
				blendPixel(img, cx+dx, cy+dy, clr.R, clr.G, clr.B, a)
			}
		}
	}
}

// drawRippleRing draws an expanding circular ring with anti-aliasing.
func drawRippleRing(img *image.RGBA, cx, cy int, radius, thickness float64, clr color.RGBA) {
	halfThick := thickness / 2.0
	rMax := int(math.Ceil(radius + halfThick + 1.0))
	for dy := -rMax; dy <= rMax; dy++ {
		for dx := -rMax; dx <= rMax; dx++ {
			dist := math.Hypot(float64(dx), float64(dy))
			diff := math.Abs(dist - radius)
			if diff <= halfThick+0.8 {
				var factor float64
				if diff <= halfThick {
					factor = 1.0
				} else {
					factor = 1.0 - (diff-halfThick)/0.8
				}
				a := uint8(float64(clr.A) * factor)
				blendPixel(img, cx+dx, cy+dy, clr.R, clr.G, clr.B, a)
			}
		}
	}
}

// Standard macOS cursor bitmap (15x18 pixels)
// 0 = transparent, 1 = black border, 2 = white body
var cursorBitmap = []string{
	"1              ",
	"11             ",
	"121            ",
	"1221           ",
	"12221          ",
	"122221         ",
	"1222221        ",
	"12222221       ",
	"122222221      ",
	"1222222221     ",
	"12222211111    ",
	"1221221        ",
	"121 1221       ",
	"11   1221      ",
	"1    1221      ",
	"      1221     ",
	"      1221     ",
	"       11      ",
}

func drawCursor(img *image.RGBA, cx, cy int, scale int) {
	if scale < 1 {
		scale = 1
	}
	for row, line := range cursorBitmap {
		for col, ch := range line {
			var r, g, b, a uint8
			if ch == '1' {
				r, g, b, a = 0, 0, 0, 235
			} else if ch == '2' {
				r, g, b, a = 255, 255, 255, 255
			} else {
				continue
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					blendPixel(img, cx+col*scale+dx, cy+row*scale+dy, r, g, b, a)
				}
			}
		}
	}
}

// RenderCursorEffects overlays click ripples, cursor spotlight halo, and cursor arrow onto img.
func RenderCursorEffects(img *image.RGBA, captureRect image.Rectangle, cursor CursorPoint, clicks []ClickEvent, now time.Time, showCursor, highlight, ripples bool) {
	b := img.Bounds()
	if captureRect.Dx() == 0 || captureRect.Dy() == 0 {
		return
	}
	scaleX := float64(b.Dx()) / float64(captureRect.Dx())
	scaleY := float64(b.Dy()) / float64(captureRect.Dy())

	// 1. Render active click ripples
	if ripples {
		for _, c := range clicks {
			age := now.Sub(c.At).Seconds()
			if age < 0 || age > 0.40 {
				continue
			}
			progress := age / 0.40 // 0.0 to 1.0
			clickImgX := b.Min.X + int(float64(c.Point.X-captureRect.Min.X)*scaleX)
			clickImgY := b.Min.Y + int(float64(c.Point.Y-captureRect.Min.Y)*scaleY)

			// Expanding radius from 5px to 26px (scaled)
			rad := (5.0 + 22.0*progress) * scaleX
			thick := (2.8 - 0.8*progress) * scaleX
			alpha := uint8(220.0 * (1.0 - progress))

			var ringClr color.RGBA
			if c.Button == 1 { // right click: vibrant violet
				ringClr = color.RGBA{R: 139, G: 92, B: 246, A: alpha}
			} else { // left click: warm amber
				ringClr = color.RGBA{R: 245, G: 158, B: 11, A: alpha}
			}

			drawRippleRing(img, clickImgX, clickImgY, rad, thick, ringClr)

			// Center dot for first 150ms
			if age < 0.15 {
				dotA := uint8(180.0 * (1.0 - age/0.15))
				drawHalo(img, clickImgX, clickImgY, 3.5*(1.0-age/0.15)*scaleX, color.RGBA{R: ringClr.R, G: ringClr.G, B: ringClr.B, A: dotA})
			}
		}
	}

	// 2. Render cursor halo & arrow
	cursorImgX := b.Min.X + int(float64(cursor.X-captureRect.Min.X)*scaleX)
	cursorImgY := b.Min.Y + int(float64(cursor.Y-captureRect.Min.Y)*scaleY)

	margin := int(35.0 * scaleX)
	if cursorImgX >= b.Min.X-margin && cursorImgX <= b.Max.X+margin &&
		cursorImgY >= b.Min.Y-margin && cursorImgY <= b.Max.Y+margin {
		if highlight {
			// Amber translucent spotlight halo
			drawHalo(img, cursorImgX, cursorImgY, 20.0*scaleX, color.RGBA{R: 245, G: 158, B: 11, A: 85})
		}
		if showCursor {
			cScale := 1
			if scaleX >= 1.5 {
				cScale = 2
			}
			drawCursor(img, cursorImgX, cursorImgY, cScale)
		}
	}
}
