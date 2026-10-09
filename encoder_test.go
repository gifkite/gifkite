package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"testing"
	"time"
)

// synthFrames makes a gradient background with a square sliding across it,
// plus a run of identical frames in the middle (a "pause").
func synthFrames(n int) ([]Frame, time.Time) {
	const W, H = 320, 200
	bg := image.NewRGBA(image.Rect(0, 0, W, H))
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			bg.Set(x, y, color.RGBA{uint8(x * 255 / W), uint8(y * 255 / H), 128, 255})
		}
	}
	start := time.Unix(0, 0)
	var fs []Frame
	for i := 0; i < n; i++ {
		img := image.NewRGBA(bg.Rect)
		copy(img.Pix, bg.Pix)
		pos := i
		if i > n/3 && i < 2*n/3 {
			pos = n / 3 // pause: identical content
		}
		draw.Draw(img, image.Rect(pos*4, 80, pos*4+30, 110), image.NewUniform(color.RGBA{250, 250, 250, 255}), image.Point{}, draw.Src)
		fs = append(fs, Frame{Img: img, At: start.Add(time.Duration(i) * 66 * time.Millisecond)})
	}
	return fs, start.Add(time.Duration(n) * 66 * time.Millisecond)
}

func TestEncodeRoundTrip(t *testing.T) {
	frames, end := synthFrames(60)
	var buf bytes.Buffer
	st, err := EncodeGIF(&buf, frames, end, 15, "none", 256, nil)
	if err != nil {
		t.Fatal(err)
	}
	g, err := gif.DecodeAll(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if st.Written >= st.Captured {
		t.Errorf("expected identical frames merged: captured %d written %d", st.Captured, st.Written)
	}
	total := 0
	for _, d := range g.Delay {
		total += d
	}
	if want := int(end.Sub(frames[0].At) / (10 * time.Millisecond)); total < want-2 || total > want+2 {
		t.Errorf("total delay %dcs, want ~%dcs", total, want)
	}

	// Composite the delta frames and compare the final image to the source.
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	for _, f := range g.Image {
		draw.Draw(canvas, f.Bounds(), f, f.Bounds().Min, draw.Over)
	}
	// The composited result must equal the last frame quantized directly:
	// delta frames and transparency may not lose or corrupt any pixel.
	q := BuildQuantizer(frames, 255)
	last := frames[len(frames)-1].Img
	mismatches := 0
	for i := 0; i < len(last.Pix); i += 4 {
		want := q.Palette[q.Index(last.Pix[i], last.Pix[i+1], last.Pix[i+2])].(color.RGBA)
		if canvas.Pix[i] != want.R || canvas.Pix[i+1] != want.G || canvas.Pix[i+2] != want.B {
			mismatches++
		}
	}
	if mismatches > 0 {
		t.Errorf("%d pixels differ from direct quantization", mismatches)
	}
	t.Logf("captured=%d written=%d size=%dB mismatches=%d totalDelay=%dcs", st.Captured, st.Written, buf.Len(), mismatches, total)
}

func TestEncodeDithering(t *testing.T) {
	frames, end := synthFrames(20)
	for _, mode := range []string{"bayer", "floyd", "none"} {
		var buf bytes.Buffer
		st, err := EncodeGIF(&buf, frames, end, 15, mode, 256, nil)
		if err != nil {
			t.Fatalf("mode %s failed: %v", mode, err)
		}
		g, err := gif.DecodeAll(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("mode %s failed to decode: %v", mode, err)
		}
		if len(g.Image) == 0 {
			t.Fatalf("mode %s produced 0 frames", mode)
		}
		t.Logf("mode=%s captured=%d written=%d size=%dB", mode, st.Captured, st.Written, buf.Len())
	}
}

