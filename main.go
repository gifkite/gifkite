// Gifkite: a Gifox-style screen-to-GIF recorder.
//
//	gifkite                  menu bar app (Wails v3)
//	gifkite record [flags]   headless recording from the terminal
//	gifkite displays         list displays and their bounds
package main

import "os"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "record":
			cmdRecord(os.Args[2:])
			return
		case "displays":
			cmdDisplays()
			return
		case "-h", "--help", "help":
			usage()
			return
		}
	}
	runApp()
}
