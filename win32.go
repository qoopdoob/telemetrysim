//go:build windows

package main

// Minimal Win32 bindings using only the Go standard library.

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	pRegisterClassExW   = user32.NewProc("RegisterClassExW")
	pCreateWindowExW    = user32.NewProc("CreateWindowExW")
	pDefWindowProcW     = user32.NewProc("DefWindowProcW")
	pGetMessageW        = user32.NewProc("GetMessageW")
	pTranslateMessage   = user32.NewProc("TranslateMessage")
	pDispatchMessageW   = user32.NewProc("DispatchMessageW")
	pPostQuitMessage    = user32.NewProc("PostQuitMessage")
	pShowWindow         = user32.NewProc("ShowWindow")
	pUpdateWindow       = user32.NewProc("UpdateWindow")
	pSetTimer           = user32.NewProc("SetTimer")
	pInvalidateRect     = user32.NewProc("InvalidateRect")
	pBeginPaint         = user32.NewProc("BeginPaint")
	pEndPaint           = user32.NewProc("EndPaint")
	pGetClientRect      = user32.NewProc("GetClientRect")
	pGetWindowRect      = user32.NewProc("GetWindowRect")
	pLoadCursorW        = user32.NewProc("LoadCursorW")
	pLoadIconW          = user32.NewProc("LoadIconW")
	pSetCursor          = user32.NewProc("SetCursor")
	pGetCursorPos       = user32.NewProc("GetCursorPos")
	pScreenToClient     = user32.NewProc("ScreenToClient")
	pDestroyWindow      = user32.NewProc("DestroyWindow")
	pSendMessageW       = user32.NewProc("SendMessageW")
	pGetWindowTextW     = user32.NewProc("GetWindowTextW")
	pSetWindowTextW     = user32.NewProc("SetWindowTextW")
	pEnableWindow       = user32.NewProc("EnableWindow")
	pMessageBoxW        = user32.NewProc("MessageBoxW")
	pCreateMenu         = user32.NewProc("CreateMenu")
	pCreatePopupMenu    = user32.NewProc("CreatePopupMenu")
	pAppendMenuW        = user32.NewProc("AppendMenuW")
	pSetMenu            = user32.NewProc("SetMenu")
	pCheckMenuItem      = user32.NewProc("CheckMenuItem")
	pIsDialogMessageW   = user32.NewProc("IsDialogMessageW")
	pFillRect           = user32.NewProc("FillRect")
	pDrawTextW          = user32.NewProc("DrawTextW")
	pSetProcessDPIAware = user32.NewProc("SetProcessDPIAware")
	pGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
	pSetForegroundWin   = user32.NewProc("SetForegroundWindow")
	pGetDC              = user32.NewProc("GetDC")
	pReleaseDC          = user32.NewProc("ReleaseDC")
	pSetFocus           = user32.NewProc("SetFocus")
	pIsWindow           = user32.NewProc("IsWindow")
	pTrackPopupMenu     = user32.NewProc("TrackPopupMenu")
	pClientToScreen     = user32.NewProc("ClientToScreen")
	pDestroyMenu        = user32.NewProc("DestroyMenu")
	pMonitorFromWindow  = user32.NewProc("MonitorFromWindow")
	pWindowFromPoint    = user32.NewProc("WindowFromPoint")
	pIsWindowEnabled    = user32.NewProc("IsWindowEnabled")
	shcore              = syscall.NewLazyDLL("shcore.dll")
	pGetDpiForMonitor   = shcore.NewProc("GetDpiForMonitor")

	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pDeleteDC               = gdi32.NewProc("DeleteDC")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pCreatePen              = gdi32.NewProc("CreatePen")
	pCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	pPolyline               = gdi32.NewProc("Polyline")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pCreateFontW            = gdi32.NewProc("CreateFontW")
	pGetDeviceCaps          = gdi32.NewProc("GetDeviceCaps")
	pMoveToEx               = gdi32.NewProc("MoveToEx")
	pLineTo                 = gdi32.NewProc("LineTo")
	pRoundRect              = gdi32.NewProc("RoundRect")
	pSetViewportOrgEx       = gdi32.NewProc("SetViewportOrgEx")
	pGetTextExtentPoint32W  = gdi32.NewProc("GetTextExtentPoint32W")
	pGetStockObject         = gdi32.NewProc("GetStockObject")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

const (
	WM_CREATE        = 0x0001
	WM_DESTROY       = 0x0002
	WM_SIZE          = 0x0005
	WM_PAINT         = 0x000F
	WM_CLOSE         = 0x0010
	WM_ERASEBKGND    = 0x0014
	WM_SETCURSOR     = 0x0020
	WM_SETFONT       = 0x0030
	WM_KEYDOWN       = 0x0100
	WM_COMMAND       = 0x0111
	WM_TIMER         = 0x0113
	WM_MOUSEMOVE     = 0x0200
	WM_LBUTTONDOWN   = 0x0201
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONDOWN   = 0x0204

	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_TABSTOP          = 0x00010000
	WS_CAPTION          = 0x00C00000
	WS_SYSMENU          = 0x00080000
	WS_POPUP            = 0x80000000
	WS_VSCROLL          = 0x00200000
	WS_EX_CLIENTEDGE    = 0x00000200
	WS_EX_DLGMODALFRAME = 0x00000001

	CBS_DROPDOWNLIST = 0x0003
	ES_NUMBER        = 0x2000
	ES_AUTOHSCROLL   = 0x0080
	BS_DEFPUSHBUTTON = 0x0001
	BS_AUTOCHECKBOX  = 0x0003
	BM_GETCHECK      = 0x00F0
	BM_SETCHECK      = 0x00F1
	CB_ADDSTRING     = 0x0143
	CB_SETCURSEL     = 0x014E
	CB_GETCURSEL     = 0x0147
	CBN_SELCHANGE    = 1

	CW_USEDEFAULT = 0x80000000
	SW_SHOW       = 5
	SW_MAXIMIZE   = 3
	CS_HREDRAW    = 2
	CS_VREDRAW    = 1

	MF_STRING    = 0
	MF_POPUP     = 0x10
	MF_SEPARATOR = 0x800
	MF_CHECKED   = 8

	IDC_ARROW = 32512
	IDC_HAND  = 32649
	IDI_APP   = 32512
	HTCLIENT  = 1

	DT_CENTER       = 0x1
	DT_RIGHT        = 0x2
	DT_VCENTER      = 0x4
	DT_SINGLELINE   = 0x20
	DT_NOPREFIX     = 0x800
	DT_END_ELLIPSIS = 0x8000
	DT_WORDBREAK    = 0x10
	DT_BOTTOM       = 0x8

	SRCCOPY   = 0x00CC0020
	VK_ESCAPE = 0x1B
)

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
	Private uint32
}
type WNDCLASSEXW struct {
	CbSize     uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}
type PAINTSTRUCT struct {
	Hdc         uintptr
	Erase       int32
	RcPaint     RECT
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

// call invokes a Win32 function. The uintptrescapes directive makes Go keep
// any pointer converted to uintptr in the argument list alive and on the heap
// for the whole call (without it, the GC or stack growth can move/free the
// memory while Windows is using it, corrupting memory minutes later).
//
//go:uintptrescapes
func call(p *syscall.LazyProc, a ...uintptr) uintptr {
	r, _, _ := p.Call(a...)
	return r
}

func wstr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("?")
	}
	return p
}

func rgb(r, g, b uint8) uint32 { return uint32(r) | uint32(g)<<8 | uint32(b)<<16 }

func loword(v uintptr) int { return int(v & 0xffff) }
func hiword(v uintptr) int { return int((v >> 16) & 0xffff) }
func lpX(v uintptr) int32  { return int32(int16(v & 0xffff)) }
func lpY(v uintptr) int32  { return int32(int16((v >> 16) & 0xffff)) }

// ---- GDI helpers with caches ----

var brushCache = map[uint32]uintptr{}
var penCache = map[[2]uint32]uintptr{}
var fontCache = map[string]uintptr{}

func brush(c uint32) uintptr {
	if b, ok := brushCache[c]; ok {
		return b
	}
	b := call(pCreateSolidBrush, uintptr(c))
	brushCache[c] = b
	return b
}

// pen returns a cached solid pen; width 0 is a 1-pixel cosmetic pen (fastest).
func pen(c uint32, w int32) uintptr {
	if w < 0 {
		w = 0
	}
	k := [2]uint32{c, uint32(w)}
	if p, ok := penCache[k]; ok {
		return p
	}
	p := call(pCreatePen, 0, uintptr(w), uintptr(c))
	penCache[k] = p
	return p
}

func font(h int32, bold bool, face string) uintptr {
	if h < 6 {
		h = 6
	}
	w := 400
	if bold {
		w = 700
	}
	key := face + string(rune(h)) + string(rune(w))
	if f, ok := fontCache[key]; ok {
		return f
	}
	f := call(pCreateFontW, uintptr(uint32(-h)), 0, 0, 0, uintptr(w), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(wstr(face))))
	fontCache[key] = f
	return f
}

func fill(dc uintptr, l, t, r, b int32, c uint32) {
	rc := RECT{l, t, r, b}
	call(pFillRect, dc, uintptr(unsafe.Pointer(&rc)), brush(c))
}

func text(dc uintptr, s string, l, t, r, b int32, c uint32, f uintptr, flags uint32) {
	u, _ := syscall.UTF16FromString(s)
	if len(u) <= 1 {
		return
	}
	rc := RECT{l, t, r, b}
	call(pSelectObject, dc, f)
	call(pSetTextColor, dc, uintptr(c))
	call(pSetBkMode, dc, 1)
	call(pDrawTextW, dc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1),
		uintptr(unsafe.Pointer(&rc)), uintptr(flags|DT_NOPREFIX))
}

func line(dc uintptr, x1, y1, x2, y2 int32, p uintptr) {
	call(pSelectObject, dc, p)
	call(pMoveToEx, dc, uintptr(x1), uintptr(y1), 0)
	call(pLineTo, dc, uintptr(x2), uintptr(y2))
}

// thickPolyline draws a w-pixel-thick line using the fast 1-pixel (cosmetic)
// pen several times with small offsets. Wide GDI pens are drawn by outlining
// every segment, which gets very slow for long steep strokes such as the QRS
// complexes of tachycardic rhythms; cosmetic pens stay fast at any rate.
func thickPolyline(dc uintptr, pts []POINT, c uint32, w int32) {
	if len(pts) < 2 {
		return
	}
	pn := pen(c, 0)
	offs := [][2]int32{{0, 0}, {1, 0}, {0, 1}, {1, 1}, {2, 0}, {0, 2}, {2, 1}, {1, 2}, {2, 2}}
	n := 1
	switch {
	case w == 2:
		n = 3 // (0,0) (1,0) (0,1): 2 px wide both horizontally and vertically
	case w == 3:
		n = 4
	case w >= 4:
		n = 9
	}
	for i := 0; i < n; i++ {
		call(pSetViewportOrgEx, dc, uintptr(offs[i][0]), uintptr(offs[i][1]), 0)
		polyline(dc, pts, pn)
	}
	call(pSetViewportOrgEx, dc, 0, 0, 0)
}

func polyline(dc uintptr, pts []POINT, p uintptr) {
	if len(pts) < 2 {
		return
	}
	call(pSelectObject, dc, p)
	call(pPolyline, dc, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
}

func roundRect(dc uintptr, l, t, r, b, rad int32, fillC, edgeC uint32) {
	call(pSelectObject, dc, brush(fillC))
	call(pSelectObject, dc, pen(edgeC, 1))
	call(pRoundRect, dc, uintptr(l), uintptr(t), uintptr(r), uintptr(b), uintptr(rad), uintptr(rad))
}

func (r RECT) contains(x, y int32) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

func messageBox(h uintptr, msg, title string, flags uintptr) {
	call(pMessageBoxW, h, uintptr(unsafe.Pointer(wstr(msg))), uintptr(unsafe.Pointer(wstr(title))), flags)
}

func getText(h uintptr) string {
	var b [512]uint16
	call(pGetWindowTextW, h, uintptr(unsafe.Pointer(&b[0])), 512)
	return syscall.UTF16ToString(b[:])
}

func registerClass(name string, proc uintptr, bg uintptr, style uint32) {
	wc := WNDCLASSEXW{
		Style:      style,
		WndProc:    proc,
		Instance:   hInst,
		Icon:       call(pLoadIconW, 0, IDI_APP),
		IconSm:     call(pLoadIconW, 0, IDI_APP),
		Cursor:     call(pLoadCursorW, 0, IDC_ARROW),
		Background: bg,
		ClassName:  wstr(name),
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	call(pRegisterClassExW, uintptr(unsafe.Pointer(&wc)))
}

// Back buffer for flicker-free painting.
type backBuf struct {
	dc, bmp, old uintptr
	w, h         int32
}

func (bb *backBuf) ensure(hdc uintptr, w, h int32) {
	if bb.dc != 0 && bb.w == w && bb.h == h {
		return
	}
	bb.free()
	bb.dc = call(pCreateCompatibleDC, hdc)
	bb.bmp = call(pCreateCompatibleBitmap, hdc, uintptr(w), uintptr(h))
	bb.old = call(pSelectObject, bb.dc, bb.bmp)
	bb.w, bb.h = w, h
}

func (bb *backBuf) free() {
	if bb.dc != 0 {
		call(pSelectObject, bb.dc, bb.old)
		call(pDeleteObject, bb.bmp)
		call(pDeleteDC, bb.dc)
		bb.dc = 0
	}
}
