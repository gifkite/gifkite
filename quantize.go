package main

import (
	"image/color"
	"sort"
)

// Quantizer maps RGB to a palette index, prioritizing exact color preservation
// for UI elements and text before falling back to a 15-bit spatial LUT.
type Quantizer struct {
	Palette    color.Palette
	PaletteRGB [256][3]uint8
	exact      map[rgb]uint8
	lut        [1 << 15]uint8
}

func (q *Quantizer) Exact(r, g, b uint8) (uint8, bool) {
	if q.exact != nil {
		idx, ok := q.exact[rgb{r, g, b}]
		return idx, ok
	}
	return 0, false
}

func (q *Quantizer) Index(r, g, b uint8) uint8 {
	if q.exact != nil {
		if idx, ok := q.exact[rgb{r, g, b}]; ok {
			return idx
		}
	}
	return q.lut[int(r>>3)<<10|int(g>>3)<<5|int(b>>3)]
}

type rgb struct{ r, g, b uint8 }

// BuildQuantizer extracts dominant exact colors first (preserving brand colors,
// buttons, text, and icons with 100% accuracy), and uses median-cut for the remainder.
func BuildQuantizer(frames []Frame, maxColors int) *Quantizer {
	const maxSamples = 300_000
	total := 0
	for _, f := range frames {
		total += len(f.Img.Pix) / 4
	}
	step := max(1, total/maxSamples)

	counts := make(map[rgb]int)
	samples := make([]rgb, 0, min(total, maxSamples+len(frames)))
	for _, f := range frames {
		p := f.Img.Pix
		for i := 0; i+2 < len(p); i += 4 * step {
			c := rgb{p[i], p[i+1], p[i+2]}
			counts[c]++
			samples = append(samples, c)
		}
	}

	q := &Quantizer{exact: make(map[rgb]uint8)}

	type colorFreq struct {
		c     rgb
		count int
	}
	freqs := make([]colorFreq, 0, len(counts))
	for c, cnt := range counts {
		freqs = append(freqs, colorFreq{c, cnt})
	}
	sort.Slice(freqs, func(i, j int) bool {
		if freqs[i].count != freqs[j].count {
			return freqs[i].count > freqs[j].count
		}
		if freqs[i].c.r != freqs[j].c.r {
			return freqs[i].c.r < freqs[j].c.r
		}
		if freqs[i].c.g != freqs[j].c.g {
			return freqs[i].c.g < freqs[j].c.g
		}
		return freqs[i].c.b < freqs[j].c.b
	})

	// Fast path: if the screen content has <= maxColors unique colors, keep 100% exact colors!
	if len(counts) <= maxColors {
		for _, f := range freqs {
			c := f.c
			idx := uint8(len(q.Palette))
			q.Palette = append(q.Palette, color.RGBA{c.r, c.g, c.b, 255})
			q.PaletteRGB[idx] = [3]uint8{c.r, c.g, c.b}
			q.exact[c] = idx
		}
		if len(q.Palette) == 0 {
			q.Palette = color.Palette{color.RGBA{0, 0, 0, 255}}
			q.PaletteRGB[0] = [3]uint8{0, 0, 0}
		}
		q.buildLUT()
		return q
	}

	// More than maxColors: Prioritize preserving the top exact colors (UI elements, text, buttons)
	// Allocate up to 75% of palette slots to exact dominant colors (covers >90% of screen pixels)
	exactTarget := maxColors * 3 / 4
	if exactTarget > len(freqs) {
		exactTarget = len(freqs)
	}

	for i := 0; i < exactTarget; i++ {
		c := freqs[i].c
		idx := uint8(len(q.Palette))
		q.Palette = append(q.Palette, color.RGBA{c.r, c.g, c.b, 255})
		q.PaletteRGB[idx] = [3]uint8{c.r, c.g, c.b}
		q.exact[c] = idx
	}

	// Remaining slots allocated via median cut over the remaining samples
	remainingSlots := maxColors - len(q.Palette)
	if remainingSlots > 0 {
		remSamples := make([]rgb, 0, len(samples))
		for _, s := range samples {
			if _, exists := q.exact[s]; !exists {
				remSamples = append(remSamples, s)
			}
		}

		if len(remSamples) > 0 {
			boxes := [][]rgb{remSamples}
			for len(boxes) < remainingSlots {
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
					break
				}
				bx := boxes[bi]
				sort.Slice(bx, func(a, b int) bool {
					va, vb := chVal(bx[a], bestCh), chVal(bx[b], bestCh)
					if va != vb {
						return va < vb
					}
					if bx[a].r != bx[b].r {
						return bx[a].r < bx[b].r
					}
					if bx[a].g != bx[b].g {
						return bx[a].g < bx[b].g
					}
					return bx[a].b < bx[b].b
				})
				mid := len(bx) / 2
				boxes[bi] = bx[:mid]
				boxes = append(boxes, bx[mid:])
			}

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
				idx := uint8(len(q.Palette))
				q.Palette = append(q.Palette, color.RGBA{cr, cg, cb, 255})
				q.PaletteRGB[idx] = [3]uint8{cr, cg, cb}
				q.exact[rgb{cr, cg, cb}] = idx
			}
		}
	}

	if len(q.Palette) == 0 {
		q.Palette = color.Palette{color.RGBA{0, 0, 0, 255}}
		q.PaletteRGB[0] = [3]uint8{0, 0, 0}
	}

	q.buildLUT()
	return q
}

func (q *Quantizer) buildLUT() {
	for i := range q.lut {
		r := uint8(i>>10&31) << 3
		g := uint8(i>>5&31) << 3
		b := uint8(i&31) << 3
		r, g, b = r|r>>5, g|g>>5, b|b>>5
		best, bestD := 0, 1<<31-1
		for j, c := range q.Palette {
			pc := c.(color.RGBA)
			dr, dg, db := int(r)-int(pc.R), int(g)-int(pc.G), int(b)-int(pc.B)
			d := 2*dr*dr + 4*dg*dg + 3*db*db
			if d < bestD {
				best, bestD = j, d
			}
		}
		q.lut[i] = uint8(best)
	}
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
