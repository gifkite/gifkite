package main

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

type Annotation struct {
	Type  string  `json:"type"`            // "arrow", "text", "blur"
	X     float64 `json:"x"`               // 0..1 normalized coords
	Y     float64 `json:"y"`
	X1    float64 `json:"x1,omitempty"`   // alias for x
	Y1    float64 `json:"y1,omitempty"`   // alias for y
	X2    float64 `json:"x2"`              // for arrow
	Y2    float64 `json:"y2"`
	W     float64 `json:"w"`               // for blur
	H     float64 `json:"h"`
	Text  string  `json:"text"`            // for text
	Color string  `json:"color"`           // hex e.g. "#ff3b30"
}

func parseHex(hex string, def color.RGBA) color.RGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 6 {
		r, err1 := strconv.ParseUint(hex[0:2], 16, 8)
		g, err2 := strconv.ParseUint(hex[2:4], 16, 8)
		b, err3 := strconv.ParseUint(hex[4:6], 16, 8)
		if err1 == nil && err2 == nil && err3 == nil {
			return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
		}
	}
	return def
}

func ApplyAnnotations(img *image.RGBA, annotations []Annotation) {
	if len(annotations) == 0 {
		return
	}
	b := img.Bounds()
	bw, bh := float64(b.Dx()), float64(b.Dy())

	for _, a := range annotations {
		switch a.Type {
		case "blur":
			x := int(a.X * bw)
			y := int(a.Y * bh)
			w := int(a.W * bw)
			h := int(a.H * bh)
			pixelateRect(img, x, y, w, h, 10)
		case "arrow":
			ax1 := a.X
			if ax1 == 0 && a.X1 != 0 {
				ax1 = a.X1
			}
			ay1 := a.Y
			if ay1 == 0 && a.Y1 != 0 {
				ay1 = a.Y1
			}
			x1 := int(ax1 * bw)
			y1 := int(ay1 * bh)
			x2 := int(a.X2 * bw)
			y2 := int(a.Y2 * bh)
			col := parseHex(a.Color, color.RGBA{R: 255, G: 59, B: 48, A: 255}) // Apple red/coral
			drawThickLine(img, x1, y1, x2, y2, 4, col)
			drawArrowHead(img, x1, y1, x2, y2, 18, col)
		case "text":
			if a.Text == "" {
				continue
			}
			x := int(a.X * bw)
			y := int(a.Y * bh)
			textColor := parseHex(a.Color, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			drawCaption(img, x, y, a.Text, textColor)
		}
	}
}

func pixelateRect(img *image.RGBA, x, y, w, h, blockSize int) {
	b := img.Bounds()
	x0 := max(b.Min.X, x)
	y0 := max(b.Min.Y, y)
	x1 := min(b.Max.X, x+w)
	y1 := min(b.Max.Y, y+h)
	if x1 <= x0 || y1 <= y0 {
		return
	}
	if blockSize < 4 {
		blockSize = 8
	}

	for by := y0; by < y1; by += blockSize {
		for bx := x0; bx < x1; bx += blockSize {
			bw := min(blockSize, x1-bx)
			bh := min(blockSize, y1-by)

			var rSum, gSum, bSum, count int
			for py := by; py < by+bh; py++ {
				for px := bx; px < bx+bw; px++ {
					c := img.RGBAAt(px, py)
					rSum += int(c.R)
					gSum += int(c.G)
					bSum += int(c.B)
					count++
				}
			}
			if count == 0 {
				continue
			}
			avg := color.RGBA{R: uint8(rSum / count), G: uint8(gSum / count), B: uint8(bSum / count), A: 255}
			for py := by; py < by+bh; py++ {
				for px := bx; px < bx+bw; px++ {
					img.SetRGBA(px, py, avg)
				}
			}
		}
	}
}

func drawThickLine(img *image.RGBA, x1, y1, x2, y2 int, thickness int, col color.RGBA) {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	dist := math.Hypot(dx, dy)
	if dist == 0 {
		return
	}
	steps := int(dist * 2)
	r := thickness / 2
	b := img.Bounds()

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		cx := int(float64(x1) + t*dx)
		cy := int(float64(y1) + t*dy)
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx*dx+dy*dy <= r*r+1 {
					px, py := cx+dx, cy+dy
					if px >= b.Min.X && px < b.Max.X && py >= b.Min.Y && py < b.Max.Y {
						img.SetRGBA(px, py, col)
					}
				}
			}
		}
	}
}

func drawArrowHead(img *image.RGBA, x1, y1, x2, y2 int, headLen float64, col color.RGBA) {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	angle := math.Atan2(dy, dx)
	arrowAngle := math.Pi / 5.5 // ~32 degrees

	xLeft := int(float64(x2) - headLen*math.Cos(angle-arrowAngle))
	yLeft := int(float64(y2) - headLen*math.Sin(angle-arrowAngle))

	xRight := int(float64(x2) - headLen*math.Cos(angle+arrowAngle))
	yRight := int(float64(y2) - headLen*math.Sin(angle+arrowAngle))

	drawThickLine(img, x2, y2, xLeft, yLeft, 4, col)
	drawThickLine(img, x2, y2, xRight, yRight, 4, col)
}

func drawCaption(img *image.RGBA, x, y int, text string, textColor color.RGBA) {
	face := basicfont.Face7x13
	textW := len(text) * 7
	textH := 13
	padX := 8
	padY := 4

	b := img.Bounds()
	boxX0 := max(b.Min.X, x-padX)
	boxY0 := max(b.Min.Y, y-textH-padY)
	boxX1 := min(b.Max.X, x+textW+padX)
	boxY1 := min(b.Max.Y, y+padY)

	// Draw dark background pill
	bgCol := color.RGBA{R: 20, G: 20, B: 24, A: 220}
	for py := boxY0; py < boxY1; py++ {
		for px := boxX0; px < boxX1; px++ {
			c := img.RGBAAt(px, py)
			// alpha blend
			alpha := float64(bgCol.A) / 255.0
			outR := uint8(float64(bgCol.R)*alpha + float64(c.R)*(1.0-alpha))
			outG := uint8(float64(bgCol.G)*alpha + float64(c.G)*(1.0-alpha))
			outB := uint8(float64(bgCol.B)*alpha + float64(c.B)*(1.0-alpha))
			img.SetRGBA(px, py, color.RGBA{R: outR, G: outG, B: outB, A: 255})
		}
	}

	// Draw text
	d := &font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{textColor},
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)},
	}
	d.DrawString(text)
}
