<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/gifkite-logo-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="docs/assets/gifkite-logo-light.svg">
    <img src="docs/assets/gifkite-logo-dark.svg" alt="Gifkite" width="380">
  </picture>
</p>

<p align="center">
  <strong>The Pro Screen-to-GIF Recorder for macOS</strong><br>
  Crafted for clarity, speed, and beautiful pixel-perfect loops.
</p>

<p align="center">
  <a href="https://github.com/gifkite/gifkite/actions/workflows/ci.yml"><img src="https://github.com/gifkite/gifkite/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://gifkite.github.io/gifkite/"><img src="https://github.com/gifkite/gifkite/actions/workflows/pages.yml/badge.svg" alt="Pages"></a>
  <a href="https://github.com/gifkite/gifkite/releases"><img src="https://img.shields.io/github/v/release/gifkite/gifkite?color=coral&label=Download%20macOS" alt="Release"></a>
</p>

---

A Gifox-style menu bar app for recording your screen to GIF, written in Go with Wails v3.

🌐 **Website & Interactive Demo**: [https://gifkite.github.io/gifkite/](https://gifkite.github.io/gifkite/)

## Build

Needs Go 1.25+.

    go mod tidy
    go build -o gifkite .

macOS needs Xcode command line tools. Linux needs GTK/WebKit headers:
`sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev`

Run `./gifkite`. A record-button icon appears in the menu bar (system tray on Windows/Linux).
On macOS, grant Screen Recording permission the first time, then quit and relaunch.
When run from a terminal the permission goes to the terminal app; wrap it in a
.app bundle (`wails3 package`) to give Gifkite its own entry.

## Use

- **Menu bar popover**: Click the menu bar icon to open the popover with Start/Stop buttons and recent GIFs.
- **Shortcuts (Gifox style)**:
  - `Cmd+Shift+5`: window capture mode (snaps to any app window).
  - `Cmd+Shift+6` (or `Ctrl+Shift+6`): pick a region to record.
  - `Cmd+Shift+7`: record full screen.
  - `Cmd+Option+P`: pause or resume recording.
  - `Cmd+Escape`: stop recording anywhere.
  - In picker: click a window to snap, drag for custom region, `Space` to record, `Cmd+A` for full screen, `Esc` to cancel.
- **Recording UI**: While recording, a floating draggable controls pill with elapsed time, Pause/Resume, Stop and Discard buttons is always available on-screen (and excluded from the captured recording).
- **Post-Recording Trim & Review**: Review your captured frames before saving. Scrub through the timeline, set in/out trim points with dual sliders, preview the trimmed loop, and save or discard. (Configurable in Settings).
- **Dithering Controls**: Choose between Bayer 4×4 spatial ordered dithering (smooth gradients with small file sizes), Floyd-Steinberg error diffusion, or none.
- **Cursor Effects & Click Ripples**: High-visibility mouse cursor tracking, soft translucent spotlight halo, and animated expanding ripple rings on left and right mouse clicks (configurable in Settings).
- **Recent library**: Click a GIF to copy the file (pastes as an attachment), hover to preview animation, double-click to open.
- **Right-click the icon** for a quick context menu.

Headless recording still works: `gifkite record -region 100,100,800,500 -duration 10s -dither bayer -cursor -highlight -ripples`.

## Files

| file | what it does |
|---|---|
| app.go | Wails setup: tray, popover, picker, border and controls windows, hotkey |
| service.go | recording flow and state machine, bound to the frontend |
| windows_darwin.go | native macOS window enumeration and snapping |
| library.go | recent recordings, thumbnails, copy/reveal/delete |
| settings.go | settings stored in the user config dir |
| drag_darwin.m, drag_darwin.go | native Cocoa NSDraggingSession for dragging files out of popover |
| cursor.go | cursor spotlight halo, animated click ripple rings, and click tracking |
| recorder.go, quantize.go, encoder.go | capture, dithering (Bayer, Floyd-Steinberg), and GIF encoding |
| frontend/ | plain HTML/CSS/JS, no build step |

## Multi-display support

Gifkite automatically detects which display your cursor is on, allowing you to select windows, drag regions, or record full-screen across any connected monitor or mixed-DPI display setup.

## Drag-and-Drop Sharing

You can drag any GIF thumbnail directly out of the popover and drop it into **Slack, Discord, Messages, Finder, Desktop, VS Code, or web browsers**. Clicking a thumbnail copies the file to the clipboard, and double-clicking opens it in Preview.
