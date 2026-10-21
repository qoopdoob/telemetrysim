//go:build windows

package main

// Help → About TeleSim: program name, author and licence, with a small
// hand-drawn smiley face centred at the bottom of the window.

import (
	"math"
	"unsafe"
)

var (
	aboutHwnd uintptr
	pDrawIcon = user32.NewProc("DrawIconEx")

	pPolygon           = gdi32.NewProc("Polygon")
	pStretchBlt        = gdi32.NewProc("StretchBlt")
	pSetStretchBltMode = gdi32.NewProc("SetStretchBltMode")
	pSetBrushOrgEx     = gdi32.NewProc("SetBrushOrgEx")

	pDestroyIcon         = user32.NewProc("DestroyIcon")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

const idAbout = 317

func openAbout() {
	if aboutHwnd != 0 {
		call(pSetForegroundWindow, aboutHwnd)
		return
	}
	w, h := S(440), S(380)
	var mr RECT
	call(pGetWindowRect, mainHwnd, uintptr(unsafe.Pointer(&mr)))
	x := mr.Left + (mr.Right-mr.Left-w)/2
	y := mr.Top + (mr.Bottom-mr.Top-h)/2
	aboutHwnd = call(pCreateWindowExW, WS_EX_DLGMODALFRAME, uintptr(unsafe.Pointer(wstr("TeleSimAbout"))), uintptr(unsafe.Pointer(wstr("About TeleSim"))),
		WS_CAPTION|WS_SYSMENU|WS_POPUP, uintptr(x), uintptr(y), uintptr(w), uintptr(h), mainHwnd, 0, hInst, 0)
	call(pShowWindow, aboutHwnd, SW_SHOW)
}

func drawAbout(dc uintptr, w, h int32) {
	fill(dc, 0, 0, w, h, rgb(255, 255, 255))
	y := S(18)
	if ic := call(pLoadImageW, hInst, 1, 1, uintptr(S(48)), uintptr(S(48)), 0); ic != 0 {
		call(pDrawIcon, dc, uintptr((w-S(48))/2), uintptr(y), ic, uintptr(S(48)), uintptr(S(48)), 0, 0, 3) // DI_NORMAL
		call(pDestroyIcon, ic)
	}
	y += S(56)
	centre := func(s string, hgt int32, c uint32, f uintptr) {
		text(dc, s, 0, y, w, y+hgt, c, f, DT_CENTER|DT_SINGLELINE)
		y += hgt
	}
	centre("TeleSim", S(36), rgb(0, 0, 0), font(S(28), true, "Segoe UI"))
	centre("Central Telemetry Station Simulator", S(30), rgb(70, 70, 70), font(S(15), false, "Segoe UI"))
	y += S(8)
	centre("Vibe-coded by Jeffrey Olvesen", S(26), rgb(0, 0, 0), font(S(16), false, "Segoe UI"))
	centre("Licensed under the GNU GPL v3", S(26), rgb(0, 0, 0), font(S(16), false, "Segoe UI"))
	y += S(4)
	centre("For education and training only. Not a medical device.", S(22), rgb(120, 120, 120), font(S(13), false, "Segoe UI"))

	smiley(dc, w/2, h-S(16))
}

// smiley draws the traced smiley face (smileyShapes, smiley.go) with its
// bottom centre at (cx, bottom). GDI fills polygons without anti-aliasing,
// so the face is filled at 4x size in a memory bitmap and shrunk onto the
// window with halftone filtering, which gives smooth edges on the white
// background.
func smiley(dc uintptr, cx, bottom int32) {
	const ss = 4
	h := S(50)
	w := int32(math.Round(float64(h) * smileyW / smileyH))
	scale := float64(h*ss) / smileyH
	mdc := call(pCreateCompatibleDC, dc)
	bmp := call(pCreateCompatibleBitmap, dc, uintptr(w*ss), uintptr(h*ss))
	old := call(pSelectObject, mdc, bmp)
	fill(mdc, 0, 0, w*ss, h*ss, rgb(255, 255, 255))
	ink := rgb(20, 20, 20)
	call(pSelectObject, mdc, brush(ink))
	call(pSelectObject, mdc, pen(ink, 1))
	for _, sh := range smileyShapes {
		pts := make([]POINT, 0, len(sh)/2)
		for i := 0; i+1 < len(sh); i += 2 {
			pts = append(pts, POINT{int32(math.Round(float64(sh[i]) * scale)), int32(math.Round(float64(sh[i+1]) * scale))})
		}
		call(pPolygon, mdc, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
	}
	call(pSetStretchBltMode, dc, 4) // HALFTONE
	call(pSetBrushOrgEx, dc, 0, 0, 0)
	call(pStretchBlt, dc, uintptr(cx-w/2), uintptr(bottom-h), uintptr(w), uintptr(h), mdc, 0, 0, uintptr(w*ss), uintptr(h*ss), SRCCOPY)
	call(pSelectObject, mdc, old)
	call(pDeleteObject, bmp)
	call(pDeleteDC, mdc)
}

func closeAbout() {
	if aboutHwnd == 0 {
		return
	}
	h := aboutHwnd
	aboutHwnd = 0
	call(pDestroyWindow, h)
}

func aboutProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch uint32(msg) {
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc := call(pBeginPaint, hwnd, uintptr(unsafe.Pointer(&ps)))
		var rc RECT
		call(pGetClientRect, hwnd, uintptr(unsafe.Pointer(&rc)))
		drawAbout(hdc, rc.Right, rc.Bottom)
		call(pEndPaint, hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_KEYDOWN:
		if wp == VK_ESCAPE || wp == 0x0D { // Esc or Enter
			closeAbout()
		}
		return 0
	case WM_CLOSE:
		closeAbout()
		return 0
	}
	return call(pDefWindowProcW, hwnd, msg, wp, lp)
}
