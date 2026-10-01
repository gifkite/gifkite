package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	FPS             int     `json:"fps"`
	Scale           float64 `json:"scale"`
	MaxSeconds      int     `json:"maxSeconds"`
	Countdown       bool    `json:"countdown"`
	OutputDir       string  `json:"outputDir"`
	Dither          string  `json:"dither"`
	Trim            bool    `json:"trim"`
	ShowCursor      bool    `json:"showCursor"`
	CursorHighlight bool    `json:"cursorHighlight"`
	ClickRipples    bool    `json:"clickRipples"`
	ShowControls    bool    `json:"showControls"`
}

func defaultSettings() Settings {
	return Settings{
		FPS:             15,
		Scale:           1,
		MaxSeconds:      60,
		Countdown:       true,
		OutputDir:       defaultOutputDir(),
		Dither:          "none",
		Trim:            true,
		ShowCursor:      true,
		CursorHighlight: true,
		ClickRipples:    true,
		ShowControls:    true,
	}
}

func settingsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "gifkite", "settings.json")
}

func loadSettings() Settings {
	s := defaultSettings()
	if b, err := os.ReadFile(settingsPath()); err == nil {
		json.Unmarshal(b, &s)
	}
	return s.clamp()
}

func (s Settings) clamp() Settings {
	d := defaultSettings()
	if s.FPS < 5 || s.FPS > 50 {
		s.FPS = d.FPS
	}
	if s.Scale <= 0 || s.Scale > 1 {
		s.Scale = d.Scale
	}
	if s.MaxSeconds < 5 || s.MaxSeconds > 600 {
		s.MaxSeconds = d.MaxSeconds
	}
	if s.OutputDir == "" {
		s.OutputDir = d.OutputDir
	}
	if s.Dither != "none" && s.Dither != "bayer" && s.Dither != "floyd" {
		s.Dither = d.Dither
	}
	return s
}

// SaveSettings is called from the settings pane.
func (g *GifService) SaveSettings(s Settings) (Settings, error) {
	s = s.clamp()
	g.mu.Lock()
	g.settings = s
	g.mu.Unlock()
	p := settingsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return s, err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	g.broadcast()
	return s, os.WriteFile(p, b, 0o644)
}

// ChooseFolder opens a native folder picker and saves the choice.
func (g *GifService) ChooseFolder() (string, error) {
	cur := g.GetState().Settings
	dir, err := g.app.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		SetDirectory(cur.OutputDir).
		SetTitle("Choose where recordings are saved").
		PromptForSingleSelection()
	g.tray.ShowWindow() // the dialog stole focus and closed the popover
	if err != nil || dir == "" {
		return cur.OutputDir, err
	}
	cur.OutputDir = dir
	_, err = g.SaveSettings(cur)
	g.app.Event.Emit("library", "")
	return dir, err
}

func defaultOutputDir() string {
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
