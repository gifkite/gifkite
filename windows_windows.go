//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

type CursorPoint struct {
	X int
	Y int
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

type POINT struct {
	X int32
	Y int32
}

type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

var (
	user32Win                    = syscall.NewLazyDLL("user32.dll")
	procGetCursorPos             = user32Win.NewProc("GetCursorPos")
	procGetAsyncKeyState          = user32Win.NewProc("GetAsyncKeyState")
	procEnumWindows              = user32Win.NewProc("EnumWindows")
	procIsWindowVisible          = user32Win.NewProc("IsWindowVisible")
	procIsIconic                 = user32Win.NewProc("IsIconic")
	procGetWindowRect            = user32Win.NewProc("GetWindowRect")
	procGetWindowTextW           = user32Win.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId = user32Win.NewProc("GetWindowThreadProcessId")
	procGetClassNameW            = user32Win.NewProc("GetClassNameW")

	dwmapiWin                 = syscall.NewLazyDLL("dwmapi.dll")
	procDwmGetWindowAttribute = dwmapiWin.NewProc("DwmGetWindowAttribute")

	kernel32Win                    = syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentProcessId        = kernel32Win.NewProc("GetCurrentProcessId")
	procOpenProcess                = kernel32Win.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = kernel32Win.NewProc("QueryFullProcessImageNameW")
	procCloseHandle                = kernel32Win.NewProc("CloseHandle")
)

const (
	vkLButton = 0x01
	vkRButton = 0x02

	dwmwaExtendedFrameBounds = 9
	dwmwaCloaked             = 14

	processQueryLimitedInfo = 0x1000
)

func getCursorPoint() CursorPoint {
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	return CursorPoint{X: int(pt.X), Y: int(pt.Y)}
}

func isMouseLeftDown() bool {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vkLButton))
	return (ret & 0x8000) != 0
}

func isMouseRightDown() bool {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vkRButton))
	return (ret & 0x8000) != 0
}

type enumCtx struct {
	windows []WindowInfo
	scrX    int
	scrY    int
	scrW    int
	scrH    int
	myPid   uint32
}

func enumWindowsProc(hwnd uintptr, lparam uintptr) uintptr {
	ctx := (*enumCtx)(unsafe.Pointer(lparam))
	if ctx == nil {
		return 0
	}

	if len(ctx.windows) >= 64 {
		return 0
	}

	vis, _, _ := procIsWindowVisible.Call(hwnd)
	if vis == 0 {
		return 1
	}

	iconic, _, _ := procIsIconic.Call(hwnd)
	if iconic != 0 {
		return 1
	}

	var cloaked uint32
	res, _, _ := procDwmGetWindowAttribute.Call(
		hwnd,
		uintptr(dwmwaCloaked),
		uintptr(unsafe.Pointer(&cloaked)),
		uintptr(unsafe.Sizeof(cloaked)),
	)
	if res == 0 && cloaked != 0 {
		return 1
	}

	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == ctx.myPid || pid == 0 {
		return 1
	}

	var rect RECT
	res, _, _ = procDwmGetWindowAttribute.Call(
		hwnd,
		uintptr(dwmwaExtendedFrameBounds),
		uintptr(unsafe.Pointer(&rect)),
		uintptr(unsafe.Sizeof(rect)),
	)
	if res != 0 {
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	}

	w := rect.Right - rect.Left
	h := rect.Bottom - rect.Top
	if w < 50 || h < 50 {
		return 1
	}

	gx, gy, gw, gh := int(rect.Left), int(rect.Top), int(w), int(h)

	if ctx.scrW > 0 && ctx.scrH > 0 {
		if gx+gw <= ctx.scrX || gx >= ctx.scrX+ctx.scrW || gy+gh <= ctx.scrY || gy >= ctx.scrY+ctx.scrH {
			return 1
		}
	}

	var titleBuf [256]uint16
	n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&titleBuf[0])), uintptr(len(titleBuf)))
	title := syscall.UTF16ToString(titleBuf[:n])

	var classBuf [128]uint16
	cn, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&classBuf[0])), uintptr(len(classBuf)))
	className := syscall.UTF16ToString(classBuf[:cn])
	if className == "Progman" || className == "Shell_TrayWnd" || (className == "Windows.UI.Core.CoreWindow" && title == "") {
		return 1
	}

	appName := ""
	hProc, _, _ := procOpenProcess.Call(uintptr(processQueryLimitedInfo), 0, uintptr(pid))
	if hProc != 0 {
		var exeBuf [1024]uint16
		exeLen := uint32(len(exeBuf))
		r, _, _ := procQueryFullProcessImageNameW.Call(
			hProc,
			0,
			uintptr(unsafe.Pointer(&exeBuf[0])),
			uintptr(unsafe.Pointer(&exeLen)),
		)
		if r != 0 {
			exePath := syscall.UTF16ToString(exeBuf[:exeLen])
			base := filepath.Base(exePath)
			appName = strings.TrimSuffix(base, filepath.Ext(base))
		}
		procCloseHandle.Call(hProc)
	}
	if appName == "" {
		appName = title
	}

	ctx.windows = append(ctx.windows, WindowInfo{
		ID:      int(hwnd),
		PID:     int(pid),
		X:       gx - ctx.scrX,
		Y:       gy - ctx.scrY,
		W:       gw,
		H:       gh,
		Title:   title,
		AppName: appName,
	})

	return 1
}

var enumProcCallback = syscall.NewCallback(enumWindowsProc)

func listWindows(scrX, scrY, scrW, scrH int) []WindowInfo {
	myPid, _, _ := procGetCurrentProcessId.Call()
	ctx := enumCtx{
		windows: make([]WindowInfo, 0, 32),
		scrX:    scrX,
		scrY:    scrY,
		scrW:    scrW,
		scrH:    scrH,
		myPid:   uint32(myPid),
	}

	procEnumWindows.Call(enumProcCallback, uintptr(unsafe.Pointer(&ctx)))
	return ctx.windows
}
