//go:build darwin

package main

/*
#cgo LDFLAGS: -framework Cocoa
#include "notif_darwin.h"
#include <stdlib.h>
*/
import "C"
import "unsafe"

func showDarwinNotification(title, message string) {
	cTitle := C.CString(title)
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cMessage))
	C.showNativeDarwinNotification(cTitle, cMessage)
}

func copyDarwinFile(path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	C.copyNativeFileToClipboard(cPath)
	return nil
}
