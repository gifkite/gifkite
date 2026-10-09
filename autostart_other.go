//go:build !darwin

package main

func isLaunchAtLogin() bool {
	return false
}

func setLaunchAtLogin(enabled bool) error {
	return nil
}
