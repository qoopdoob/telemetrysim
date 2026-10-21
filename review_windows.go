//go:build windows

package main

// Review window: the patient's alarm history (newest first), like the
// "Review Alarms" list at the monitor - date and time, then the message.

import (
	"strings"
	"unsafe"
)

var (
	reviewHwnd, reviewList uintptr
	reviewBed              = -1
	reviewCount            int
)

const (
	lbAddString    = 0x0180
	lbResetContent = 0x0184
	ctlReviewClose = 1
)

func openReview(bed int) {
	if reviewHwnd != 0 {
		closeReview()
	}
	reviewBed = bed
	uiFont = font(S(15), false, "Segoe UI")
	w, h := S(560), S(460)
	var mr RECT
	call(pGetWindowRect, mainHwnd, uintptr(unsafe.Pointer(&mr)))
	x := mr.Left + (mr.Right-mr.Left-w)/2
	y := mr.Top + (mr.Bottom-mr.Top-h)/2
	title := bedLabel(bed) + " - Review Alarms"
	if n := patients[bed].Cfg.Name; n != "" {
		title += " - " + n
	}
	reviewHwnd = call(pCreateWindowExW, WS_EX_DLGMODALFRAME, uintptr(unsafe.Pointer(wstr("TeleSimReview"))), uintptr(unsafe.Pointer(wstr(title))),
		WS_CAPTION|WS_SYSMENU|WS_POPUP, uintptr(x), uintptr(y), uintptr(w), uintptr(h), mainHwnd, 0, hInst, 0)
	ctlParent = reviewHwnd
	reviewList = ctl("LISTBOX", "", WS_TABSTOP|WS_VSCROLL|0x00800000, S(12), S(12), w-S(30), h-S(100), 0) // WS_BORDER
	ctl("BUTTON", "Close", WS_TABSTOP|BS_DEFPUSHBUTTON, w-S(120), h-S(80), S(90), S(30), ctlReviewClose)
	reviewCount = -1
	refreshReview()
	call(pShowWindow, reviewHwnd, SW_SHOW)
}

// refreshReview reloads the list when new entries have arrived.
func refreshReview() {
	if reviewHwnd == 0 {
		return
	}
	hist := patients[reviewBed].History
	if len(hist) == reviewCount {
		return
	}
	reviewCount = len(hist)
	call(pSendMessageW, reviewList, lbResetContent, 0, 0)
	if len(hist) == 0 {
		call(pSendMessageW, reviewList, lbAddString, 0, uintptr(unsafe.Pointer(wstr("(no alarms yet)"))))
	}
	for i := len(hist) - 1; i >= 0; i-- {
		e := hist[i]
		line := e.At.Format("2 Jan 15:04:05") + "   " + strings.TrimSpace(e.Text)
		call(pSendMessageW, reviewList, lbAddString, 0, uintptr(unsafe.Pointer(wstr(line))))
	}
}

func closeReview() {
	if reviewHwnd == 0 {
		return
	}
	h := reviewHwnd
	reviewHwnd = 0
	call(pDestroyWindow, h)
}

func reviewProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch uint32(msg) {
	case WM_COMMAND:
		if loword(wp) == ctlReviewClose || loword(wp) == 2 {
			closeReview()
		}
		return 0
	case WM_CLOSE:
		closeReview()
		return 0
	}
	return call(pDefWindowProcW, hwnd, msg, wp, lp)
}
