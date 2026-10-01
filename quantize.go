package main

import (
	"image/color"
	"sort"
)

// Quantizer maps RGB to a palette index through a 15-bit lookup table,
// so per-pixel mapping is one array read instead of a palette search.
type Quantizer struct {
	Palette    color.Palette
	PaletteRGB [256][3]uint8
	lut        [1 << 15]uint8
}

func (q *Quantizer) Index(r, g, b uint8) uint8 {
	return q.lut[int(r>>3)<<10|int(g>>3)<<5|int(b>>3)]
}

type rgb struct{ r, g, b uint8 }

// BuildQuantizer runs median cut over pixels sampled from every frame,
// giving one global palette for the whole GIF. A global palette means no
// per-frame color tables and no palette flicker between frames.
func BuildQuantizer(frames []Frame, maxColors int) *Quantizer {
	const maxSamples = 300_000
	total := 0
	for _, f := range frames {
		total += len(f.Img.Pix) / 4
	}
	step := max(1, total/maxSamples)

	samples := make([]rgb, 0, min(total, maxSamples+len(frames)))
	for _, f := range frames {
		p := f.Img.Pix
		for i := 0; i+2 < len(p); i += 4 * step {
			samples = append(samples, rgb{p[i], p[i+1], p[i+2]})
		}
	}

	boxes := [][]rgb{samples}
	for len(boxes) < maxColors {
		// Split the box with the widest single-channel spread.
		bi, bestRange, bestCh := -1, 0, 0
		for i, bx := range boxes {
			if len(bx) < 2 {
				continue
			}
			rng, ch := channelRange(bx)
			if rng > bestRange {
				bi, bestRange, bestCh = i, rng, ch
			}
		}
		if bi < 0 {
			break // every box is a single color
		}
		bx := boxes[bi]
		sort.Slice(bx, func(a, b int) bool { return chVal(bx[a], bestCh) < chVal(bx[b], bestCh) })
		mid := len(bx) / 2
		boxes[bi] = bx[:mid]
		boxes = append(boxes, bx[mid:])
	}

	q := &Quantizer{}
	for _, bx := range boxes {
		if len(bx) == 0 {
			continue
		}
		var r, g, b int
		for _, c := range bx {
			r += int(c.r)
			g += int(c.g)
			b += int(c.b)
		}
		n := len(bx)
		cr, cg, cb := uint8(r/n), uint8(g/n), uint8(b/n)
		q.Palette = append(q.Palette, color.RGBA{cr, cg, cb, 255})
		q.PaletteRGB[len(q.Palette)-1] = [3]uint8{cr, cg, cb}
	}
	if len(q.Palette) == 0 {
		q.Palette = color.Palette{color.RGBA{0, 0, 0, 255}}
		q.PaletteRGB[0] = [3]uint8{0, 0, 0}
	}

	for i := range q.lut {
		r := uint8(i>>10&31) << 3
		g := uint8(i>>5&31) << 3
		b := uint8(i&31) << 3
		r, g, b = r|r>>5, g|g>>5, b|b>>5
		best, bestD := 0, 1<<31-1
		for j, c := range q.Palette {
			pc := c.(color.RGBA)
			dr, dg, db := int(r)-int(pc.R), int(g)-int(pc.G), int(b)-int(pc.B)
			// Weighted distance: the eye is most sensitive to green.
			d := 2*dr*dr + 4*dg*dg + 3*db*db
			if d < bestD {
				best, bestD = j, d
			}
		}
		q.lut[i] = uint8(best)
	}
	return q
}

func channelRange(bx []rgb) (int, int) {
	lo := [3]uint8{255, 255, 255}
	var hi [3]uint8
	for _, c := range bx {
		v := [3]uint8{c.r, c.g, c.b}
		for k := 0; k < 3; k++ {
			lo[k] = min(lo[k], v[k])
			hi[k] = max(hi[k], v[k])
		}
	}
	best, ch := 0, 0
	for k := 0; k < 3; k++ {
		if r := int(hi[k]) - int(lo[k]); r > best {
			best, ch = r, k
		}
	}
	return best, ch
}

func chVal(c rgb, ch int) uint8 {
	switch ch {
	case 0:
		return c.r
	case 1:
		return c.g
	}
	return c.b
}
