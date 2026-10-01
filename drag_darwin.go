//go:build darwin

package main

/*
#cgo LDFLAGS: -framework Cocoa
#include "drag_darwin.h"
#include <stdlib.h>
*/
import "C"
import "unsafe"

//export onNativeDragEnded
func onNativeDragEnded(op C.int) {
	if defaultGifService != nil {
		defaultGifService.onDragFinished(int(op))
	}
}

func startNativeFileDrag(window unsafe.Pointer, path string) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	C.performNativeDrag(window, cpath)
}
