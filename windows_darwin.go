//go:build darwin

package main

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreGraphics/CoreGraphics.h>
#include <CoreFoundation/CoreFoundation.h>
#include <unistd.h>

typedef struct {
    int id;
    int pid;
    int x, y, w, h;
    char title[128];
    char app[64];
} CWindowInfo;

static int getVisibleWindows(CWindowInfo* out, int maxCount) {
    CFArrayRef list = CGWindowListCopyWindowInfo(
        kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
        kCGNullWindowID
    );
    if (!list) return 0;

    pid_t myPid = getpid();
    CFIndex n = CFArrayGetCount(list);
    int count = 0;

    for (CFIndex i = 0; i < n && count < maxCount; i++) {
        CFDictionaryRef dict = (CFDictionaryRef)CFArrayGetValueAtIndex(list, i);
        if (!dict) continue;

        // Check layer: 0 is standard application window layer
        int layer = -1;
        CFNumberRef layerRef = (CFNumberRef)CFDictionaryGetValue(dict, kCGWindowLayer);
        if (layerRef) {
            CFNumberGetValue(layerRef, kCFNumberIntType, &layer);
        }
        if (layer != 0) continue;

        // Check PID: exclude ourselves
        int pid = 0;
        CFNumberRef pidRef = (CFNumberRef)CFDictionaryGetValue(dict, kCGWindowOwnerPID);
        if (pidRef) {
            CFNumberGetValue(pidRef, kCFNumberIntType, &pid);
        }
        if (pid == myPid) continue;

        // Check bounds
        CGRect bounds = CGRectNull;
        CFDictionaryRef boundsDict = (CFDictionaryRef)CFDictionaryGetValue(dict, kCGWindowBounds);
        if (boundsDict) {
            CGRectMakeWithDictionaryRepresentation(boundsDict, &bounds);
        }
        if (CGRectIsNull(bounds) || bounds.size.width < 50 || bounds.size.height < 50) {
            continue;
        }

        // Window ID
        int winId = 0;
        CFNumberRef idRef = (CFNumberRef)CFDictionaryGetValue(dict, kCGWindowNumber);
        if (idRef) {
            CFNumberGetValue(idRef, kCFNumberIntType, &winId);
        }

        CWindowInfo info;
        info.id = winId;
        info.pid = pid;
        info.x = (int)bounds.origin.x;
        info.y = (int)bounds.origin.y;
        info.w = (int)bounds.size.width;
        info.h = (int)bounds.size.height;
        info.title[0] = '\0';
        info.app[0] = '\0';

        CFStringRef appRef = (CFStringRef)CFDictionaryGetValue(dict, kCGWindowOwnerName);
        if (appRef) {
            CFStringGetCString(appRef, info.app, sizeof(info.app), kCFStringEncodingUTF8);
        }

        CFStringRef titleRef = (CFStringRef)CFDictionaryGetValue(dict, kCGWindowName);
        if (titleRef) {
            CFStringGetCString(titleRef, info.title, sizeof(info.title), kCFStringEncodingUTF8);
        }

        out[count++] = info;
    }

    CFRelease(list);
    return count;
}

typedef struct {
    int x;
    int y;
} CPoint;

static CPoint getCursorLocation() {
    CPoint p = {0, 0};
    CGEventRef event = CGEventCreate(NULL);
    if (!event) return p;
    CGPoint loc = CGEventGetLocation(event);
    CFRelease(event);
    p.x = (int)loc.x;
    p.y = (int)loc.y;
    return p;
}

static int isLeftButtonDown() {
    return CGEventSourceButtonState(kCGEventSourceStateCombinedSessionState, kCGMouseButtonLeft);
}

static int isRightButtonDown() {
    return CGEventSourceButtonState(kCGEventSourceStateCombinedSessionState, kCGMouseButtonRight);
}

static void getScreenDetails(int* cgW, int* cgH, int* pxW, int* pxH) {
    CGDirectDisplayID mainId = CGMainDisplayID();
    CGRect r = CGDisplayBounds(mainId);
    *cgW = (int)r.size.width;
    *cgH = (int)r.size.height;
    *pxW = (int)CGDisplayPixelsWide(mainId);
    *pxH = (int)CGDisplayPixelsHigh(mainId);
}

static void warpMouse(int x, int y) {
    CGWarpMouseCursorPosition(CGPointMake(x, y));
}
*/
import "C"

type CursorPoint struct {
	X int
	Y int
}

func warpMousePoint(x, y int) {
	C.warpMouse(C.int(x), C.int(y))
}

func getScreenMetrics() (int, int, int, int) {
	var cgW, cgH, pxW, pxH C.int
	C.getScreenDetails(&cgW, &cgH, &pxW, &pxH)
	return int(cgW), int(cgH), int(pxW), int(pxH)
}

func getCursorPoint() CursorPoint {
	p := C.getCursorLocation()
	return CursorPoint{X: int(p.x), Y: int(p.y)}
}

func isMouseLeftDown() bool {
	return C.isLeftButtonDown() != 0
}

func isMouseRightDown() bool {
	return C.isRightButtonDown() != 0
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
	const maxWindows = 64
	var raw [maxWindows]C.CWindowInfo
	count := int(C.getVisibleWindows(&raw[0], C.int(maxWindows)))

	out := make([]WindowInfo, 0, count)
	for i := 0; i < count; i++ {
		w := raw[i]
		gx, gy, gw, gh := int(w.x), int(w.y), int(w.w), int(w.h)

		// Filter to windows that overlap with this screen
		if scrW > 0 && scrH > 0 {
			if gx+gw <= scrX || gx >= scrX+scrW || gy+gh <= scrY || gy >= scrY+scrH {
				continue
			}
		}

		out = append(out, WindowInfo{
			ID:      int(w.id),
			PID:     int(w.pid),
			X:       gx - scrX,
			Y:       gy - scrY,
			W:       gw,
			H:       gh,
			Title:   C.GoString(&w.title[0]),
			AppName: C.GoString(&w.app[0]),
		})
	}
	return out
}
