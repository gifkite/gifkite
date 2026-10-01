package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// The tray icon is a record button: a ring with a dot. On macOS it's a
// template image (black + alpha) so the system tints it for light/dark
// menu bars; elsewhere it's drawn in colour.
func drawIcon(ring, dot color.NRGBA) []byte {
	const s = 44 // 22pt @2x
	img := image.NewNRGBA(image.Rect(0, 0, s, s))
	c := float64(s-1) / 2
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			d := math.Hypot(float64(x)-c, float64(y)-c)
			// 1px of antialiasing on each edge
			ringA := clamp01(2 - math.Abs(d-15.5)) // ring: radius 15.5, ~3px wide
			dotA := clamp01(8.5 - d)
			switch {
			case dotA > 0:
				img.SetNRGBA(x, y, color.NRGBA{dot.R, dot.G, dot.B, uint8(float64(dot.A) * dotA)})
			case ringA > 0:
				img.SetNRGBA(x, y, color.NRGBA{ring.R, ring.G, ring.B, uint8(float64(ring.A) * ringA)})
			}
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func trayIconTemplate() []byte {
	black := color.NRGBA{0, 0, 0, 255}
	return drawIcon(black, black)
}

func trayIconColor() []byte {
	return drawIcon(color.NRGBA{120, 124, 132, 255}, color.NRGBA{229, 72, 77, 255})
}
