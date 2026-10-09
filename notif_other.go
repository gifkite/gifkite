//go:build !darwin

package main

func showDarwinNotification(title, message string) {}

func copyDarwinFile(path string) error {
	return nil
}
