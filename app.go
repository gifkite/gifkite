package main

import (
	"embed"
	"io/fs"
	"log"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend
var frontend embed.FS

// Cmd+Shift+5 is taken by macOS's own screenshot tool, so use 6.
const hotkey = "CmdOrCtrl+Shift+6"

func runApp() {
	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}
	svc := newGifService()

	app := application.New(application.Options{
		Name:        "Gifkite",
		Description: "Record your screen to GIF",
		Services:    []application.Service{application.NewService(svc)},
		Assets: application.AssetOptions{
			Handler:    application.BundledAssetFileServer(assets),
			Middleware: svc.middleware,
		},
		Mac: application.MacOptions{
			// Menu bar only: no Dock icon, no app switcher entry.
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})
	svc.app = app

	// The popover that drops down from the menu bar icon.
	svc.popover = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:            "popover",
		URL:             "/index.html",
		Width:           380,
		Height:          560,
		Frameless:       true,
		AlwaysOnTop:     true,
		Hidden:          true,
		DisableResize:   true,
		HideOnEscape:    true,
		HideOnFocusLost: true,
		Windows:         application.WindowsWindow{HiddenOnTaskbar: true},
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTranslucent,
		},
	})
	svc.popover.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		svc.popover.Hide()
		e.Cancel()
	})
	svc.popover.RegisterHook(events.Common.WindowLostFocus, func(e *application.WindowEvent) {
		svc.mu.Lock()
		dragging := svc.isDragging
		svc.mu.Unlock()
		if dragging {
			e.Cancel()
		}
	})

	// Full-screen overlay showing a frozen screenshot to drag a region on.
	svc.picker = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          "picker",
		URL:           "/picker.html",
		Frameless:     true,
		AlwaysOnTop:   true,
		Hidden:        true,
		DisableResize: true,
		Windows:       application.WindowsWindow{HiddenOnTaskbar: true},
		Mac: application.MacWindow{
			WindowLevel:        application.MacWindowLevelScreenSaver, // above the menu bar
			CollectionBehavior: application.MacWindowCollectionBehaviorCanJoinAllSpaces | application.MacWindowCollectionBehaviorFullScreenAuxiliary,
			DisableShadow:      true,
		},
	})

	// Floating timer + stop button shown next to the region while recording.
	svc.controls = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          "controls",
		URL:           "/controls.html",
		Width:         controlsW,
		Height:        controlsH,
		Frameless:     true,
		AlwaysOnTop:   true,
		Hidden:        true,
		DisableResize: true,
		Windows:       application.WindowsWindow{HiddenOnTaskbar: true},
		Mac: application.MacWindow{
			Backdrop:            application.MacBackdropTransparent,
			WindowLevel:        application.MacWindowLevelStatus,
			CollectionBehavior: application.MacWindowCollectionBehaviorCanJoinAllSpaces | application.MacWindowCollectionBehaviorFullScreenAuxiliary,
			DisableShadow:      true,
		},
	})

	setWindowInvisibleToCapture(svc.popover.NativeWindow())
	setWindowInvisibleToCapture(svc.picker.NativeWindow())
	setWindowInvisibleToCapture(svc.controls.NativeWindow())

	tray := app.SystemTray.New()
	svc.tray = tray
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(trayIconTemplate())
	} else {
		tray.SetIcon(trayIconColor())
	}
	tray.SetTooltip("Gifkite")

	const windowHotkey = "CmdOrCtrl+Shift+5"

	menu := application.NewMenu()
	menu.Add("Start recording (Window)").SetAccelerator(windowHotkey).OnClick(func(*application.Context) { go svc.StartWindow() })
	menu.Add("Start recording (Region)").SetAccelerator(hotkey).OnClick(func(*application.Context) { go svc.StartRegion() })
	menu.Add("Start recording (Full screen)").OnClick(func(*application.Context) { go svc.StartScreen() })
	menu.Add("Stop recording").OnClick(func(*application.Context) { go svc.Stop() })
	menu.AddSeparator()
	menu.Add("Open output folder").OnClick(func(*application.Context) { svc.OpenFolder() })
	menu.Add("Quit Gifkite").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(menu)                             // right-click
	tray.AttachWindow(svc.popover).WindowOffset(6) // left-click

	if err := app.GlobalShortcut.Register(windowHotkey, func() { go svc.StartWindow() }); err != nil {
		log.Printf("global shortcut %s unavailable: %v", windowHotkey, err)
	}

	if err := app.GlobalShortcut.Register(hotkey, func() { go svc.Toggle() }); err != nil {
		log.Printf("global shortcut %s unavailable: %v", hotkey, err)
	}

	// Gifox-style universal stop: Cmd+Esc stops recording anywhere
	const stopHotkey = "CmdOrCtrl+Escape"
	if err := app.GlobalShortcut.Register(stopHotkey, func() { go svc.Stop() }); err != nil {
		log.Printf("global shortcut %s unavailable: %v", stopHotkey, err)
	}

	// Gifox-style screen recording hotkey: Cmd+Shift+7
	const screenHotkey = "CmdOrCtrl+Shift+7"
	if err := app.GlobalShortcut.Register(screenHotkey, func() { go svc.StartScreen() }); err != nil {
		log.Printf("global shortcut %s unavailable: %v", screenHotkey, err)
	}

	// Pause/Resume recording hotkey: Cmd+Alt+P
	const pauseHotkey = "CmdOrCtrl+Alt+P"
	if err := app.GlobalShortcut.Register(pauseHotkey, func() { go svc.TogglePause() }); err != nil {
		log.Printf("global shortcut %s unavailable: %v", pauseHotkey, err)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
