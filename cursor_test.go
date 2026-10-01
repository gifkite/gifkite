package main

import (
	"image"
	"testing"
	"time"
)

func TestCursorEffectsRender(t *testing.T) {
	rect := image.Rect(0, 0, 400, 300)
	img := image.NewRGBA(rect)

	now := time.Now()
	cursor := CursorPoint{X: 200, Y: 150}
	clicks := []ClickEvent{
		{Point: CursorPoint{X: 200, Y: 150}, At: now.Add(-100 * time.Millisecond), Button: 0},
	}

	// Render with all effects enabled
	RenderCursorEffects(img, rect, cursor, clicks, now, true, true, true)

	// Verify that pixels around (200, 150) were modified and alpha is preserved
	centerIdx := (150*400 + 200) * 4
	if img.Pix[centerIdx+3] == 0 {
		t.Errorf("expected non-zero alpha at cursor location, got 0")
	}

	// Check that cursor tip is rendered (white body or black border)
	hasCursorPixels := false
	for y := 150; y < 165; y++ {
		for x := 200; x < 215; x++ {
			idx := (y*400 + x) * 4
			if img.Pix[idx+3] > 200 && (img.Pix[idx] == 255 || img.Pix[idx] == 0) {
				hasCursorPixels = true
				break
			}
		}
	}
	if !hasCursorPixels {
		t.Errorf("expected cursor pixels around cursor location")
	}
}

func TestClickTrackerPrune(t *testing.T) {
	ct := NewClickTracker()
	now := time.Now()
	ct.clicks = []ClickEvent{
		{Point: CursorPoint{X: 10, Y: 10}, At: now.Add(-800 * time.Millisecond), Button: 0},
		{Point: CursorPoint{X: 20, Y: 20}, At: now.Add(-200 * time.Millisecond), Button: 1},
	}

	active := ct.ActiveClicks(now)
	if len(active) != 1 {
		t.Fatalf("expected 1 active click, got %d", len(active))
	}
	if active[0].Point.X != 20 {
		t.Errorf("expected active click at X=20, got %d", active[0].Point.X)
	}
}
