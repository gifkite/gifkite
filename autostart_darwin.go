package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const launchAgentPlistName = "com.gifkite.app.plist"

func launchAgentPlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentPlistName)
}

func isLaunchAtLogin() bool {
	p := launchAgentPlistPath()
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

func setLaunchAtLogin(enabled bool) error {
	p := launchAgentPlistPath()
	if p == "" {
		return fmt.Errorf("unable to determine user home directory")
	}

	if !enabled {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	// Resolve symlinks if any
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	// Check if running inside a .app bundle
	appIdx := strings.Index(exe, ".app/Contents/MacOS/")
	var programArgsXML string
	if appIdx != -1 {
		appPath := exe[:appIdx+4]
		programArgsXML = fmt.Sprintf(`        <string>/usr/bin/open</string>
        <string>-a</string>
        <string>%s</string>`, appPath)
	} else {
		programArgsXML = fmt.Sprintf(`        <string>%s</string>`, exe)
	}

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.gifkite.app</string>
    <key>ProgramArguments</key>
    <array>
%s
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>ProcessType</key>
    <string>Interactive</string>
</dict>
</plist>
`, programArgsXML)

	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(plistContent), 0o644)
}
