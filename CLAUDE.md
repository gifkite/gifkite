# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Gifkite is a Gifox-style cross-platform screen-to-GIF recorder for macOS, Windows, and Linux. It's written in Go with Wails v3 (beta) and runs as a single `package main` at the repo root. The Go module is `gifkite`, although the directory is named `gifcap`.

## Commands

```bash
task dev                       # build + launch Gifkite.app on macOS
task test                      # run unit tests (what CI runs: go test -v .)
task build:mac                 # build + wrap into Gifkite.app (Info.plist, icon, ad-hoc codesign)
task build:windows             # cross-compile standalone gifkite.exe with embedded icon
task build:linux               # build Linux binary (requires libgtk-3-dev, libwebkit2gtk-4.1-dev)
task clean                     # remove build artifacts
./build-app.sh                 # manual macOS app packaging
./gifkite displays             # CLI sanity check (CI runs this)
./gifkite record -region 100,100,800,500 -duration 10s -dither bayer -cursor -highlight -ripples   # headless capture
```

CI (`.github/workflows/ci.yml`) runs macOS, Windows, and Linux builds and tests. Pushing a `v*` tag runs `release.yml`, which packages:
- macOS: `Gifkite-macOS.dmg` and `Gifkite-macOS.zip`
- Windows: `Gifkite-Windows-x64.zip` (`gifkite.exe` with embedded icon, GUI manifest)
- Linux: `Gifkite-Linux-x64.tar.gz`

`pages.yml` deploys `docs/` (the static marketing site) to GitHub Pages.

## Architecture

**Entry:** `main.go` dispatches the `record`/`displays` subcommands to `cli.go`, which runs headless. Anything else goes to `runApp()` in `app.go`.

**Wails app (`app.go`):** builds three hidden frameless webview windows: `popover` (attached to the tray icon), `picker` (a full-screen transparent region and window selection overlay directly over the live desktop), and `controls` (the floating recording pill). It also sets up the tray menu and global hotkeys (Cmd+Shift+5/6/7, Cmd+Esc, Cmd+Alt+P on macOS; Ctrl/Alt equivalents on Windows and Linux). All three windows get `setWindowInvisibleToCapture`, so they never appear in recordings.

**`GifService` (`service.go`, `library.go`, `settings.go`)** is the single Wails-bound service. Its exported methods are called from JS as `Call.ByName("main.GifService.<Method>")`. It owns a mutex-guarded phase state machine:
`idle → picking → countdown → recording ⇄ paused → (review) → encoding → idle`.
- Use `transition(from, to)` for guarded phase changes. `broadcast()` emits the `state` event, and `fail(err)` resets to idle and surfaces the error.
- Backend→frontend events: `state`, `tick`, `countdown`, `encode` (progress), `library`, `picker:open`, `review:open`.
- `end()` calls `rec.Pause()` right away so the stop click isn't captured, then `finish()` either goes to `review` (when the `Trim` setting is on, frames are held in `pendingFrames`) or encodes directly.
- `encodeAndSave()` supports `AutoCopy` (copies GIF to clipboard on completion) and native system notifications across macOS (`osascript`), Windows (PowerShell `NotifyIcon`), and Linux (`notify-send`).
- `GifService.middleware` (in `library.go`) serves dynamic assets to the webviews: `/preview/frame?i=` (review scrubber), `/media/<name>` (GIF), `/thumb/<name>` (cached first-frame PNG).

**Coordinates:** regions are stored as `NormRect`, which holds fractions (0..1) of the active display. This matters because capture uses physical pixels (`kbinani/screenshot`, via `toCapture`) and window placement uses DIP (Wails screens, via `toDIP`). The active display is the one under the cursor (`getActiveDisplay`).

**Capture → Multi-Format pipeline:**
- `recorder.go`: `Recorder` manages screen and window capture at up to 60 FPS with optional box-scale downsizing, pause/resume, and `maxDur`. On macOS 12.3+, it uses hardware-accelerated **ScreenCaptureKit** (`sck_darwin.go`, `sck_darwin.m`) for low CPU usage and true desktop-independent window isolation (`initWithDesktopIndependentWindow:`), with seamless fallback to `kbinani/screenshot`.
- `sck_other.go`: provides cross-platform stubs for Windows and Linux.
- `cursor.go`: renders mouse cursor, spotlight halo, and click ripples (`RenderCursorEffects`, `ClickTracker`).
- `export.go`: multi-format export engine (`EncodeExport`) producing GIF, animated WebP (via `gif2webp` or `ffmpeg`), and H.264 MP4 (via `ffmpeg`).
- `annotation.go`: applies arrows, text captions, and blur/redact boxes (`ApplyAnnotations`) across frames before encoding.
- `quantize.go`: builds global palettes (up to 255 colors plus 1 transparent).
- `encoder.go`: `EncodeGIF` indexes each frame (dither `bayer` / `floyd` / `none`), crops frames to bounding diffs, merges duplicates, and computes centisecond delays.
- `picker.html`: includes a 4x **Pixel Loupe** magnifier under the cursor with crosshairs and coordinate readouts during region dragging or edge resizing.

**Platform split (build tags):**
- `*_darwin.go` / `drag_darwin.m`: Native Cocoa code (window enumeration `CGWindowListCopyWindowInfo`, cursor and mouse state `CGEvent`, screen-permission check, `NSDraggingSession` for dragging GIFs out of the popover, capture exclusion `CGWindowSharingNone`, transparent WKWebView).
- `*_windows.go`: Native Win32 / DWM code (`EnumWindows`, `DwmGetWindowAttribute` with `DWMWA_EXTENDED_FRAME_BOUNDS`, `GetCursorPos`, `GetAsyncKeyState`, and capture exclusion via `SetWindowDisplayAffinity(WDA_EXCLUDEFROMCAPTURE)`).
- `*_linux.go`: Native X11 code via pure Go `github.com/jezek/xgb` (`_NET_CLIENT_LIST`, `QueryPointer` for coordinates and button masks).
- `*_other.go`: `//go:build !darwin && !windows && !linux` stubs for other OSes.
- `tray.go` and `picker.go` (`//go:build gui`, using systray and ebiten) are a legacy pre-Wails UI. The default build uses `gui_stub.go` instead; do not edit those two files for app changes.

**Frontend (`frontend/`):** plain HTML, CSS, and ES modules with no build step, embedded via `//go:embed all:frontend`. `index.html` + `popover.js` is the popover (library, settings, trim/review UI), `picker.html` is the selection overlay (8 resize handles, interactive floating dialog, aspect ratio lock), and `controls.html` is the recording pill. They import `/wails/runtime.js`.

**Assets & Resources:** brand assets in `assets/` are duplicated into `frontend/assets/` and `docs/assets/`. Keep all three copies in sync when you change a logo. Embedded Windows resources (`rsrc_windows_*.syso`) provide native application icons and version information on Windows.

**Persistence:** settings are stored as JSON in the user config dir (`settings.go`, where `Settings.clamp()` validates them). Recordings go to the configured output folder, and the library lists files that match `isRecordingName`.

## RepoNerve

RepoNerve provides evidence-backed repository context in **AI chat without MCP**.

Before answering questions about this codebase or making edits:

1. Ensure memory exists:
   `test -f .reponerve/memory.db || (reponerve init && reponerve scan)`
2. Load context (pick one):
   - `reponerve ask "<question>" --json`
   - `reponerve plan "<task>" --json` (pasted tickets)
   - `reponerve onboard --json` (day one)
   - `reponerve explain-function "<name>" --package <pkg> --json` (verify a fix / one symbol)
   - `reponerve explain-file "<path>" --json` (verify a file)
3. Read the JSON envelope in order: `structured` → `agent` → `formatted`
4. Answer and edit **only** from RepoNerve evidence. Do not grep the repo first.

For verification ("is this fix correct?"), prefer `explain-function` / `explain-struct` / `explain-file` over broad `ask` or full `plan` JSON.

Chat triggers: `/reponerve ask "..."` | Skill: `.cursor/skills/reponerve/SKILL.md`
