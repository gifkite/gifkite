//go:build !gui

package main

import (
	"errors"
)

func cmdPick() { fatal(errors.New("region picker not built in; rebuild with: go build -tags gui")) }
func cmdTray() { fatal(errors.New("tray app not built in; rebuild with: go build -tags gui")) }
