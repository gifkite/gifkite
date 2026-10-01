package main

import (
	"errors"
	"image"
	"image/color"
	"image/gif"
	"io"
	"time"
)

type EncodeStats struct{ Captured, Written int }

// EncodeGIF writes frames as a looping GIF.
//
// Size tricks, the same ones Gifox relies on:
//   - one global palette (see BuildQuantizer)
//   - each frame after the first stores only the bounding box of pixels
//     that changed, with unchanged pixels inside that box set transparent
//     so they compress to long LZW runs
//   - frames that come out identical after quantization are merged into
//     the previous frame's delay
func EncodeGIF(w io.Writer, frames []Frame, end time.Time, fps int, dither string, progress func(i, n int)) (EncodeStats, error) {
	stats := EncodeStats{Captured: len(frames)}
	if len(frames) == 0 {
		return stats, errors.New("nothing was captured")
	}

	q := BuildQuantizer(frames, 255)
	pal := append(color.Palette{}, q.Palette...)
	trans := uint8(len(pal))
	pal = append(pal, color.RGBA{}) // alpha 0: image/gif marks it transparent

	bounds := frames[0].Img.Bounds()
	W, H := bounds.Dx(), bounds.Dy()
	prev := make([]uint8, W*H)
	cur := make([]uint8, W*H)

	g := &gif.GIF{
		Config:    image.Config{ColorModel: pal, Width: W, Height: H},
		LoopCount: 0,
	}

	start := frames[0].At
	lastFrameLen := time.Second / time.Duration(max(fps, 1))
	elapsedCS := 0 // centiseconds already handed out as delays

	for i, f := range frames {
		if progress != nil {
			progress(i+1, len(frames))
		}
		indexFrame(f.Img, q, cur, W, H, dither)

		next := end
		if i+1 < len(frames) {
			next = frames[i+1].At
		} else if next.Sub(f.At) < lastFrameLen {
			next = f.At.Add(lastFrameLen)
		}
		// Delays come from a running total, so rounding to 1/100 s never
		// accumulates into drift over a long recording.
		delay := int(next.Sub(start)/(10*time.Millisecond)) - elapsedCS

		var frame *image.Paletted
		if i == 0 {
			frame = image.NewPaletted(image.Rect(0, 0, W, H), pal)
			copy(frame.Pix, cur)
		} else {
			box, changed := diffBox(prev, cur, W, H)
			if !changed {
				g.Delay[len(g.Delay)-1] += delay
				elapsedCS += delay
				continue
			}
			frame = image.NewPaletted(box, pal)
			bw := box.Dx()
			for y := box.Min.Y; y < box.Max.Y; y++ {
				row := y * W
				out := frame.Pix[(y-box.Min.Y)*bw:]
				for x := box.Min.X; x < box.Max.X; x++ {
					if c := cur[row+x]; c != prev[row+x] {
						out[x-box.Min.X] = c
					} else {
						out[x-box.Min.X] = trans
					}
				}
			}
		}

		if delay < 2 {
			delay = 2 // most viewers treat 0 or 1 as 10
		}
		elapsedCS += delay
		g.Image = append(g.Image, frame)
		g.Delay = append(g.Delay, delay)
		g.Disposal = append(g.Disposal, gif.DisposalNone)
		prev, cur = cur, prev
	}

	stats.Written = len(g.Image)
	return stats, gif.EncodeAll(w, g)
}

var bayer4x4 = [4][4]int{
	{0, 8, 2, 10},
	{12, 4, 14, 6},
	{3, 11, 1, 9},
	{15, 7, 13, 5},
}

func clamp8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func indexFrame(img *image.RGBA, q *Quantizer, dst []uint8, W, H int, dither string) {
	switch dither {
	case "floyd":
		indexFrameFloyd(img, q, dst, W, H)
	case "none":
		indexFrameNone(img, q, dst, W, H)
	default: // "bayer"
		indexFrameBayer(img, q, dst, W, H)
	}
}

func indexFrameBayer(img *image.RGBA, q *Quantizer, dst []uint8, W, H int) {
	for y := 0; y < H; y++ {
		src := img.Pix[y*img.Stride:]
		row := dst[y*W : (y+1)*W]
		by := y & 3
		for x := 0; x < W; x++ {
			bias := (bayer4x4[by][x&3] - 8) * 2
			p := src[x*4:]
			r := clamp8(int(p[0]) + bias)
			g := clamp8(int(p[1]) + bias)
			b := clamp8(int(p[2]) + bias)
			row[x] = q.Index(r, g, b)
		}
	}
}

func indexFrameFloyd(img *image.RGBA, q *Quantizer, dst []uint8, W, H int) {
	errRow0 := make([][3]int, W+2)
	errRow1 := make([][3]int, W+2)

	for y := 0; y < H; y++ {
		src := img.Pix[y*img.Stride:]
		row := dst[y*W : (y+1)*W]

		for x := 0; x < W; x++ {
			p := src[x*4:]
			e := errRow0[x+1]
			r := clamp8(int(p[0]) + e[0]/16)
			g := clamp8(int(p[1]) + e[1]/16)
			b := clamp8(int(p[2]) + e[2]/16)

			idx := q.Index(r, g, b)
			row[x] = idx

			actual := q.PaletteRGB[idx]
			er := int(r) - int(actual[0])
			eg := int(g) - int(actual[1])
			eb := int(b) - int(actual[2])

			errRow0[x+2][0] += er * 7
			errRow0[x+2][1] += eg * 7
			errRow0[x+2][2] += eb * 7

			errRow1[x][0] += er * 3
			errRow1[x][1] += eg * 3
			errRow1[x][2] += eb * 3

			errRow1[x+1][0] += er * 5
			errRow1[x+1][1] += eg * 5
			errRow1[x+1][2] += eb * 5

			errRow1[x+2][0] += er * 1
			errRow1[x+2][1] += eg * 1
			errRow1[x+2][2] += eb * 1
		}

		errRow0, errRow1 = errRow1, errRow0
		for i := range errRow1 {
			errRow1[i] = [3]int{}
		}
	}
}

func indexFrameNone(img *image.RGBA, q *Quantizer, dst []uint8, W, H int) {
	for y := 0; y < H; y++ {
		src := img.Pix[y*img.Stride:]
		row := dst[y*W : (y+1)*W]
		for x := range row {
			p := src[x*4:]
			row[x] = q.Index(p[0], p[1], p[2])
		}
	}
}

func diffBox(a, b []uint8, W, H int) (image.Rectangle, bool) {
	minX, minY, maxX, maxY := W, H, -1, -1
	for y := 0; y < H; y++ {
		ra, rb := a[y*W:(y+1)*W], b[y*W:(y+1)*W]
		first := -1
		for x := 0; x < W; x++ {
			if ra[x] != rb[x] {
				first = x
				break
			}
		}
		if first < 0 {
			continue
		}
		last := first
		for x := W - 1; x > first; x-- {
			if ra[x] != rb[x] {
				last = x
				break
			}
		}
		minX, maxX = min(minX, first), max(maxX, last)
		if minY == H {
			minY = y
		}
		maxY = y
	}
	if maxY < 0 {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX+1, maxY+1), true
}
