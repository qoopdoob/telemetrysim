//go:build windows

package main

// Scenario odds window: the chance (in percent) that a generated patient on
// each care-unit preset has SpO2, auto NBP, an arterial line, capnography or
// artifact.

import (
	"strconv"
	"unsafe"
)

const ctlOddsDefaults = 5

var (
	oddsHwnd  uintptr
	oddsEdits [5][UnitCount]uintptr
)

var oddsRows = []string{"SpO₂ monitored", "NBP on auto cycle", "Arterial line (ABP)", "Capnography (EtCO₂)", "Artifact"}

var defaultOdds = Odds // captured at start-up

func oddsField(o *UnitOdds, row int) *float64 {
	return []*float64{&o.SpO2, &o.NBP, &o.ABP, &o.CO2, &o.Artifact}[row]
}

func openOdds() {
	if oddsHwnd != 0 {
		call(pSetForegroundWin, oddsHwnd)
		return
	}
	if setupHwnd != 0 || limitsHwnd != 0 || tuneHwnd != 0 {
		return
	}
	uiFont = font(S(15), false, "Segoe UI")
	w, h := S(670), S(330)
	var mr RECT
	call(pGetWindowRect, mainHwnd, uintptr(unsafe.Pointer(&mr)))
	x := mr.Left + (mr.Right-mr.Left-w)/2
	y := mr.Top + (mr.Bottom-mr.Top-h)/2
	oddsHwnd = call(pCreateWindowExW, WS_EX_DLGMODALFRAME, uintptr(unsafe.Pointer(wstr("TeleSimOdds"))), uintptr(unsafe.Pointer(wstr("Scenario odds (% of patients)"))),
		WS_CAPTION|WS_SYSMENU|WS_POPUP, uintptr(x), uintptr(y), uintptr(w), uintptr(h), mainHwnd, 0, hInst, 0)
	ctlParent = oddsHwnd
	cx := func(u int) int32 { return S(220) + S(110)*int32(u) }
	for u, n := range UnitNames {
		ctl("STATIC", n, 0, cx(u)-S(10), S(14), S(110), S(22), 0)
	}
	for row, label := range oddsRows {
		yy := S(44) + S(36)*int32(row)
		ctl("STATIC", label, 0, S(16), yy+S(3), S(200), S(22), 0)
		for u := 0; u < UnitCount; u++ {
			v := *oddsField(&Odds[u], row)
			oddsEdits[row][u] = ctl("EDIT", strconv.FormatFloat(v, 'f', -1, 64), WS_TABSTOP|ES_NUMBER, cx(u), yy, S(52), S(26), 70+row*UnitCount+u)
			ctl("STATIC", "%", 0, cx(u)+S(56), yy+S(3), S(20), S(22), 0)
		}
	}
	ctl("STATIC", "Used by Patients > Simulate ... presets (rhythm mixes are fixed per unit).", 0, S(16), S(44)+S(36)*5+S(4), S(630), S(22), 0)
	by := S(44) + S(36)*5 + S(36)
	ctl("BUTTON", "Defaults", WS_TABSTOP, S(16), by, S(100), S(30), ctlOddsDefaults)
	ctl("BUTTON", "OK", WS_TABSTOP|BS_DEFPUSHBUTTON, w-S(230), by, S(90), S(30), 1)
	ctl("BUTTON", "Cancel", WS_TABSTOP, w-S(130), by, S(90), S(30), 2)
	call(pEnableWindow, mainHwnd, 0)
	call(pShowWindow, oddsHwnd, SW_SHOW)
	call(pSetFocus, oddsEdits[0][0])
}

func closeOdds() {
	if oddsHwnd == 0 {
		return
	}
	call(pEnableWindow, mainHwnd, 1)
	call(pSetForegroundWin, mainHwnd)
	h := oddsHwnd
	oddsHwnd = 0
	call(pDestroyWindow, h)
}

func oddsProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch uint32(msg) {
	case WM_COMMAND:
		switch loword(wp) {
		case 1:
			for row := range oddsRows {
				for u := 0; u < UnitCount; u++ {
					v, _ := strconv.Atoi(getText(oddsEdits[row][u]))
					*oddsField(&Odds[u], row) = float64(clampInt(v, 0, 100))
				}
			}
			closeOdds()
		case ctlOddsDefaults:
			for row := range oddsRows {
				for u := 0; u < UnitCount; u++ {
					v := *oddsField(&defaultOdds[u], row)
					call(pSetWindowTextW, oddsEdits[row][u], uintptr(unsafe.Pointer(wstr(strconv.FormatFloat(v, 'f', -1, 64)))))
				}
			}
		case 2:
			closeOdds()
		}
		return 0
	case WM_CLOSE:
		closeOdds()
		return 0
	}
	return call(pDefWindowProcW, hwnd, msg, wp, lp)
}
