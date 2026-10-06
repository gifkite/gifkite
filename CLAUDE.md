# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Gifkite is a Gifox-style macOS menu bar screen-to-GIF recorder. It's written in Go with Wails v3 (beta) and runs as a single `package main` at the repo root. The Go module is `gifkite`, although the directory is named `gifcap`.

## Commands

```bash
go build -o gifkite .          # build the binary (needs Go 1.25+, Xcode CLT for cgo)
./build-app.sh                 # build + wrap into Gifkite.app (Info.plist, icon, ad-hoc codesign)
open Gifkite.app               # run as a bundle so Screen Recording permission attaches to Gifkite, not the terminal
go test -v .                   # all tests (what CI runs)
go test -run TestEncodeRoundTrip -v .   # single test
./gifkite displays             # CLI sanity check (CI runs this)
./gifkite record -region 100,100,800,500 -duration 10s -dither bayer -cursor -highlight -ripples   # headless capture
```

CI (`.github/workflows/ci.yml`, macos-latest) runs `go test -v .`, then `./build-app.sh`, then `./gifkite displays`. Pushing a `v*` tag runs `release.yml`, which builds the DMG and ZIP and publishes a GitHub release. `pages.yml` deploys `docs/` (the static marketing site) to GitHub Pages.

## Architecture

**Entry:** `main.go` dispatches the `record`/`displays` subcommands to `cli.go`, which runs headless. Anything else goes to `runApp()` in `app.go`.

**Wails app (`app.go`):** builds three hidden frameless webview windows: `popover` (attached to the tray icon), `picker` (a full-screen region and window selection overlay at screen-saver level), and `controls` (the floating recording pill). It also sets up the tray menu and the global hotkeys (Cmd+Shift+5/6/7, Cmd+Esc, Cmd+Alt+P). All three windows get `setWindowInvisibleToCapture` (NSWindowSharingNone), so they never appear in recordings. This is set again in `begin()` when recording starts.

**`GifService` (`service.go`, `library.go`, `settings.go`)** is the single Wails-bound service. Its exported methods are called from JS as `Call.ByName("main.GifService.<Method>")`. It owns a mutex-guarded phase state machine:
`idle → picking → countdown → recording ⇄ paused → (review) → encoding → idle`.
- Use `transition(from, to)` for guarded phase changes. `broadcast()` emits the `state` event, and `fail(err)` resets to idle and surfaces the error.
- Backend→frontend events: `state`, `tick`, `countdown`, `encode` (progress), `library`, `picker:open`, `review:open`.
- `end()` calls `rec.Pause()` right away so the stop click isn't captured, then `finish()` either goes to `review` (when the `Trim` setting is on, frames are held in `pendingFrames`) or encodes directly.
- `GifService.middleware` (in `library.go`) serves dynamic assets to the webviews: `/frozen.jpg` (picker background screenshot), `/preview/frame?i=` (review scrubber), `/media/<name>` (GIF), `/thumb/<name>` (cached first-frame PNG).

**Coordinates:** regions are stored as `NormRect`, which holds fractions (0..1) of the active display. This matters because capture uses physical pixels (`kbinani/screenshot`, via `toCapture`) and window placement uses DIP (Wails screens, via `toDIP`). The active display is the one under the cursor (`getActiveDisplay`).

**Capture → GIF pipeline:**
- `recorder.go`: `Recorder` grabs frames at the target FPS with optional box-scale downsizing, supports pause/resume (tracking paused duration), and stops at `maxDur`. Cursor, halo, and click ripples are drawn into frames by `cursor.go` (`RenderCursorEffects`, `ClickTracker` polling mouse buttons).
- `quantize.go`: builds one global palette (up to 255 colors plus 1 transparent) across all frames. Colors that appear exactly are preserved via `Quantizer.Exact`, with a LUT for the rest.
- `encoder.go`: `EncodeGIF` indexes each frame (dither `bayer` / `floyd` / `none`), crops each frame to the diff box against the previous one, merges identical frames into the previous delay, and computes delays from a running centisecond total so rounding doesn't drift.

**Platform split (build tags):** `*_darwin.go` / `drag_darwin.m` hold native Cocoa code (window enumeration and snapping, cursor and mouse state, screen-permission check, NSDraggingSession for dragging GIFs out of the popover, capture exclusion). `*_other.go` holds no-op stubs. When you add a native function, add a matching stub to the `!darwin` file. `tray.go` and `picker.go` (`//go:build gui`, using systray and ebiten) are a legacy pre-Wails UI. The default build uses `gui_stub.go` instead, so don't edit those two files for app changes.

**Frontend (`frontend/`):** plain HTML, CSS, and ES modules with no build step, embedded via `//go:embed all:frontend`. `index.html` + `popover.js` is the popover (library, settings, trim/review UI), `picker.html` is the selection overlay, and `controls.html` is the recording pill. They import `/wails/runtime.js`.

**Assets:** the brand assets in `assets/` are duplicated into `frontend/assets/` and `docs/assets/`. Keep all three copies in sync when you change a logo. The tray template icon is embedded through `icon.go`.

**Persistence:** settings are stored as JSON in the user config dir (`settings.go`, where `Settings.clamp()` validates them). Recordings go to the configured output folder, and the library lists files that match `isRecordingName`.
