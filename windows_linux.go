//go:build linux

package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"sync"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
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

var (
	xConnMu  sync.Mutex
	xConn    *xgb.Conn
	xScreen  *xproto.ScreenInfo
	xLastTry time.Time
)

func getX11() (*xgb.Conn, *xproto.ScreenInfo) {
	xConnMu.Lock()
	defer xConnMu.Unlock()

	if xConn != nil {
		return xConn, xScreen
	}

	if time.Since(xLastTry) < 2*time.Second {
		return nil, nil
	}
	xLastTry = time.Now()

	c, err := xgb.NewConn()
	if err != nil {
		return nil, nil
	}

	setup := xproto.Setup(c)
	if setup == nil || len(setup.Roots) == 0 {
		c.Close()
		return nil, nil
	}

	xConn = c
	xScreen = &setup.Roots[0]
	return xConn, xScreen
}

func closeX11() {
	xConnMu.Lock()
	defer xConnMu.Unlock()
	if xConn != nil {
		xConn.Close()
		xConn = nil
		xScreen = nil
	}
}

func queryPointer() (*xproto.QueryPointerReply, error) {
	c, screen := getX11()
	if c == nil || screen == nil {
		return nil, xgb.ErrNilConn
	}
	reply, err := xproto.QueryPointer(c, screen.Root).Reply()
	if err != nil {
		closeX11()
		return nil, err
	}
	return reply, nil
}

func getCursorPoint() CursorPoint {
	reply, err := queryPointer()
	if err != nil || reply == nil {
		return CursorPoint{X: 0, Y: 0}
	}
	return CursorPoint{X: int(reply.RootX), Y: int(reply.RootY)}
}

func isMouseLeftDown() bool {
	reply, err := queryPointer()
	if err != nil || reply == nil {
		return false
	}
	return (reply.Mask & xproto.ButtonMask1) != 0
}

func isMouseRightDown() bool {
	reply, err := queryPointer()
	if err != nil || reply == nil {
		return false
	}
	return (reply.Mask & xproto.ButtonMask3) != 0
}

func getAtom(c *xgb.Conn, name string) (xproto.Atom, error) {
	reply, err := xproto.InternAtom(c, false, uint16(len(name)), name).Reply()
	if err != nil {
		return 0, err
	}
	return reply.Atom, nil
}

func getProp(c *xgb.Conn, win xproto.Window, atom xproto.Atom, propType xproto.Atom) []byte {
	reply, err := xproto.GetProperty(c, false, win, atom, propType, 0, 1024).Reply()
	if err != nil || reply == nil {
		return nil
	}
	return reply.Value
}

func listWindows(scrX, scrY, scrW, scrH int) []WindowInfo {
	c, screen := getX11()
	if c == nil || screen == nil {
		return nil
	}

	clientListAtom, err := getAtom(c, "_NET_CLIENT_LIST")
	if err != nil {
		return nil
	}

	val := getProp(c, screen.Root, clientListAtom, xproto.GetPropertyTypeAny)
	if len(val) == 0 {
		return nil
	}

	netWmNameAtom, _ := getAtom(c, "_NET_WM_NAME")
	netWmPidAtom, _ := getAtom(c, "_NET_WM_PID")
	utf8Atom, _ := getAtom(c, "UTF8_STRING")

	myPid := os.Getpid()
	out := make([]WindowInfo, 0, 32)

	for i := 0; i+4 <= len(val) && len(out) < 64; i += 4 {
		win := xproto.Window(binary.LittleEndian.Uint32(val[i : i+4]))

		// 1. Check if window is viewable (not minimized or hidden)
		attrs, err := xproto.GetWindowAttributes(c, win).Reply()
		if err != nil || attrs.MapState != xproto.MapStateViewable {
			continue
		}

		// 2. Geometry & screen translation
		geom, err := xproto.GetGeometry(c, xproto.Drawable(win)).Reply()
		if err != nil {
			continue
		}
		trans, err := xproto.TranslateCoordinates(c, win, screen.Root, 0, 0).Reply()
		if err != nil {
			continue
		}

		gx, gy := int(trans.DstX), int(trans.DstY)
		gw, gh := int(geom.Width), int(geom.Height)

		if gw < 50 || gh < 50 {
			continue
		}

		if scrW > 0 && scrH > 0 {
			if gx+gw <= scrX || gx >= scrX+scrW || gy+gh <= scrY || gy >= scrY+scrH {
				continue
			}
		}

		// 3. PID
		var pid int
		if netWmPidAtom != 0 {
			pidBytes := getProp(c, win, netWmPidAtom, xproto.AtomCardinal)
			if len(pidBytes) >= 4 {
				pid = int(binary.LittleEndian.Uint32(pidBytes[:4]))
			}
		}
		if pid == myPid {
			continue
		}

		// 4. Window title
		title := ""
		if netWmNameAtom != 0 && utf8Atom != 0 {
			titleBytes := getProp(c, win, netWmNameAtom, utf8Atom)
			if len(titleBytes) > 0 {
				title = string(bytes.TrimRight(titleBytes, "\x00"))
			}
		}
		if title == "" {
			nameBytes := getProp(c, win, xproto.AtomWmName, xproto.AtomString)
			if len(nameBytes) > 0 {
				title = string(bytes.TrimRight(nameBytes, "\x00"))
			}
		}

		// 5. App Name from WM_CLASS
		appName := ""
		classBytes := getProp(c, win, xproto.AtomWmClass, xproto.AtomString)
		if len(classBytes) > 0 {
			parts := bytes.Split(classBytes, []byte{0})
			if len(parts) > 1 && len(parts[1]) > 0 {
				appName = string(parts[1])
			} else if len(parts) > 0 && len(parts[0]) > 0 {
				appName = string(parts[0])
			}
		}
		if appName == "" {
			appName = title
		}

		out = append(out, WindowInfo{
			ID:      int(win),
			PID:     pid,
			X:       gx - scrX,
			Y:       gy - scrY,
			W:       gw,
			H:       gh,
			Title:   title,
			AppName: appName,
		})
	}

	return out
}
