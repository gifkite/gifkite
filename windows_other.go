//go:build !darwin && !windows && !linux

package main

type CursorPoint struct {
	X int
	Y int
}

func getCursorPoint() CursorPoint {
	return CursorPoint{X: 0, Y: 0}
}

func isMouseLeftDown() bool {
	return false
}

func isMouseRightDown() bool {
	return false
}

type WindowInfo struct {
	ID      int    `json:"id"`
	PID     int    `json:"pid"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	W       int    `json:"w"`
	H       int    `json:"h"`
	Title   string `json:"title"`
	AppName string `json:"appName"`
}

func listWindows(scrX, scrY, scrW, scrH int) []WindowInfo {
	return nil
}
