//go:build !darwin

package main

func hasScreenPermission() bool {
	return true
}

func requestScreenPermission() {}
