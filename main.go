// Gifkite: a Gifox-style screen-to-GIF recorder.
//
//	gifkite                  menu bar app (Wails v3)
//	gifkite record [flags]   headless recording from the terminal
//	gifkite displays         list displays and their bounds
package main

import (
	"fmt"
	"os"
)

// Version is populated at compile time via -ldflags "-X main.Version=vX.Y.Z"
var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "record":
			cmdRecord(os.Args[2:])
			return
		case "displays":
			cmdDisplays()
			return
		case "version", "-v", "--version":
			fmt.Printf("gifkite %s\n", Version)
			return
		case "-h", "--help", "help":
			usage()
			return
		}
	}
	runApp()
}
