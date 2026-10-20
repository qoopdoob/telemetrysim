//go:build windows

package main

// Alarm Limits window. Opened from the Alarms menu it edits the station-wide
// limits; opened from a sector's "Limits" button it edits that patient's own
// limits (or sets the patient back to the station-wide ones). The "extreme"
// limits raise a red panic alarm.

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unsafe"
)

const ctlUseStation = 40

var (
	limitsHwnd uintptr
	limEdits   [18]uintptr
	limBed     = -1 // -1 = station-wide
	hUseStn    uintptr
)

var limitLabels = []string{
	"HR low (yellow) below:",
	"HR high (yellow) above:",
	"Extreme brady - PANIC below:",
	"Extreme tachy - PANIC above:",
	"SpO2 low (yellow) below:",
	"Desat - PANIC below:",
	"RR low (yellow) below:",
	"RR high (yellow) above:",
	"Apnea (red) after no breath for:",
	"ST high (yellow) above:",
	"ST low (yellow) below:",
	"QTc high (yellow) above:",
	"NBP systolic high above:",
	"NBP systolic low below:",
	"ABP systolic high above:",
	"ABP systolic low below:",
	"etCO2 high above:",
	"etCO2 low below:",
}

var limitUnits = []string{"bpm", "bpm", "bpm", "bpm", "%", "%", "rpm", "rpm", "sec",
	"mm", "mm", "ms", "mmHg", "mmHg", "mmHg", "mmHg", "mmHg", "mmHg"}

// fields 9 and 10 (ST) are decimal millimetres; the rest are integers
func limitIsFloat(i int) bool { return i == 9 || i == 10 }

func limitGet(l *AlarmLimits, i int) string {
	if limitIsFloat(i) {
		return strconv.FormatFloat([]float64{l.STHigh, l.STLow}[i-9], 'f', 1, 64)
	}
	return strconv.Itoa(*limitFields(l)[i])
}

// limitFields returns the integer fields by index (nil for the ST floats).
func limitFields(l *AlarmLimits) []*int {
	return []*int{&l.HRLow, &l.HRHigh, &l.HRExtLow, &l.HRExtHigh, &l.SpO2Low, &l.SpO2ExtLow,
		&l.RRLow, &l.RRHigh, &l.ApneaSec, nil, nil, &l.QTcHigh,
		&l.NBPsHigh, &l.NBPsLow, &l.ABPsHigh, &l.ABPsLow, &l.EtCO2High, &l.EtCO2Low}
}

// openLimits edits the limits for bed (or the station-wide limits if bed < 0).
func openLimits(bed int) {
	if limitsHwnd != 0 {
		call(pSetForegroundWin, limitsHwnd)
		return
	}
	if setupHwnd != 0 || tuneHwnd != 0 {
		return
	}
	limBed = bed
	uiFont = font(S(15), false, "Segoe UI")
	w, h := S(800), S(510)
	var mr RECT
	call(pGetWindowRect, mainHwnd, uintptr(unsafe.Pointer(&mr)))
	x := mr.Left + (mr.Right-mr.Left-w)/2
	y := mr.Top + (mr.Bottom-mr.Top-h)/2
	title := "Alarm Limits (station-wide defaults)"
	cur := Limits
	if bed >= 0 {
		title = fmt.Sprintf("%s - Alarm Limits", bedLabel(bed))
		cur = patients[bed].lim()
	}
	limitsHwnd = call(pCreateWindowExW, WS_EX_DLGMODALFRAME, uintptr(unsafe.Pointer(wstr("TeleSimLimits"))), uintptr(unsafe.Pointer(wstr(title))),
		WS_CAPTION|WS_SYSMENU|WS_POPUP, uintptr(x), uintptr(y), uintptr(w), uintptr(h), mainHwnd, 0, hInst, 0)
	ctlParent = limitsHwnd

	top := S(16)
	if bed >= 0 {
		hUseStn = ctl("BUTTON", "Use station-wide limits for this patient", WS_TABSTOP|BS_AUTOCHECKBOX, S(16), top, S(400), S(26), ctlUseStation)
		if patients[bed].Lim == nil {
			call(pSendMessageW, hUseStn, BM_SETCHECK, 1, 0)
		}
		top += S(38)
	}
	for i, l := range limitLabels {
		col, rowi := i/9, i%9
		x := S(16) + S(390)*int32(col)
		y := top + S(36)*int32(rowi)
		ctl("STATIC", l, 0, x, y+S(3), S(240), S(22), 0)
		style := uint32(WS_TABSTOP | ES_NUMBER)
		if limitIsFloat(i) {
			style = WS_TABSTOP | ES_AUTOHSCROLL // allows "-" and "."
		}
		limEdits[i] = ctl("EDIT", limitGet(&cur, i), style, x+S(246), y, S(60), S(26), 50+i)
		ctl("STATIC", limitUnits[i], 0, x+S(314), y+S(3), S(50), S(22), 0)
	}
	note := "PANIC = red alarm tone, flashing red sector."
	if bed < 0 {
		note += " Beds with their own limits keep them."
	}
	ctl("STATIC", note, 0, S(16), top+S(36)*9+S(2), S(560), S(40), 0)
	by := top + S(36)*9 + S(46)
	ctl("BUTTON", "OK", WS_TABSTOP|BS_DEFPUSHBUTTON, w-S(230), by, S(90), S(30), 1)
	ctl("BUTTON", "Cancel", WS_TABSTOP, w-S(130), by, S(90), S(30), 2)
	refreshLimits()
	call(pEnableWindow, mainHwnd, 0)
	call(pShowWindow, limitsHwnd, SW_SHOW)
	call(pSetFocus, limEdits[0])
}

// refreshLimits greys the fields while a bed follows the station-wide limits.
func refreshLimits() {
	if limBed < 0 {
		return
	}
	useStn := call(pSendMessageW, hUseStn, BM_GETCHECK, 0, 0) == 1
	for i, e := range limEdits {
		enable(e, !useStn)
		if useStn { // show the station values that will apply
			call(pSetWindowTextW, e, uintptr(unsafe.Pointer(wstr(limitGet(&Limits, i)))))
		}
	}
}

func closeLimits() {
	if limitsHwnd == 0 {
		return
	}
	call(pEnableWindow, mainHwnd, 1)
	call(pSetForegroundWin, mainHwnd)
	h := limitsHwnd
	limitsHwnd = 0
	call(pDestroyWindow, h)
}

func applyLimits() bool {
	if limBed >= 0 && call(pSendMessageW, hUseStn, BM_GETCHECK, 0, 0) == 1 {
		patients[limBed].Lim = nil
		patients[limBed].computeHR(now())
		return true
	}
	var v [18]int
	var stHi, stLo float64
	for i := range v {
		if limitIsFloat(i) {
			f, err := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(getText(limEdits[i]), ",", ".")), 64)
			if err != nil {
				messageBox(limitsHwnd, "ST limits must be numbers in mm, e.g. 2.0 and -2.0", "Alarm Limits", 0x30)
				return false
			}
			if i == 9 {
				stHi = f
			} else {
				stLo = f
			}
			continue
		}
		v[i], _ = strconv.Atoi(getText(limEdits[i]))
	}
	if !(stLo < stHi) || stHi > 10 || stLo < -10 {
		messageBox(limitsHwnd, "ST limits: ST low must be below ST high (within -10 to +10 mm)", "Alarm Limits", 0x30)
		return false
	}
	if v[11] < 300 || v[11] > 800 {
		messageBox(limitsHwnd, "QTc high must be between 300 and 800 ms", "Alarm Limits", 0x30)
		return false
	}
	if !(v[13] < v[12] && v[15] < v[14] && v[17] < v[16]) {
		messageBox(limitsHwnd, "For NBP, ABP and etCO2 the low limit must be below the high limit", "Alarm Limits", 0x30)
		return false
	}
	if !(v[2] < v[0] && v[0] < v[1] && v[1] < v[3]) {
		messageBox(limitsHwnd, "Heart-rate limits must be in order:\nExtreme brady < HR low < HR high < Extreme tachy", "Alarm Limits", 0x30)
		return false
	}
	if !(v[5] < v[4] && v[4] <= 100 && v[5] >= 50) {
		messageBox(limitsHwnd, "SpO2 limits must satisfy: 50 <= Desat < SpO2 low <= 100", "Alarm Limits", 0x30)
		return false
	}
	if !(v[6] < v[7] && v[6] >= 0 && v[8] >= 5 && v[8] <= 120) {
		messageBox(limitsHwnd, "Respiratory limits: RR low must be below RR high, and apnea time 5-120 s", "Alarm Limits", 0x30)
		return false
	}
	var l AlarmLimits
	for i, f := range limitFields(&l) {
		if f != nil {
			*f = v[i]
		}
	}
	l.STHigh, l.STLow = math.Round(stHi*10)/10, math.Round(stLo*10)/10
	if limBed >= 0 {
		patients[limBed].Lim = &l
	} else {
		Limits = l
	}
	// re-evaluate alarms immediately
	t := now()
	for _, p := range patients {
		p.computeHR(t)
	}
	return true
}

func limitsProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch uint32(msg) {
	case WM_COMMAND:
		switch loword(wp) {
		case 1:
			if applyLimits() {
				closeLimits()
			}
		case 2:
			closeLimits()
		case ctlUseStation:
			refreshLimits()
		}
		return 0
	case WM_CLOSE:
		closeLimits()
		return 0
	}
	return call(pDefWindowProcW, hwnd, msg, wp, lp)
}
