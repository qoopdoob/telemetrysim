//go:build windows

package main

// Alarm Limits window. Opened from the Alarms menu it edits the station-wide
// limits; opened from a sector's "Limits" button it edits that patient's own
// limits (or sets the patient back to the station-wide ones). It also holds
// the arrhythmia alarm limits (V-Tach HR/run, SVT, PVCs/min, thresholds).

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unsafe"
)

const (
	ctlUseStation = 40
	ctlAllYelOff  = 41
	ctlAllYelOn   = 42
)

// limField describes one editable limit: an int or a float64 (one decimal).
type limField struct {
	label, unit string
	i           func(l *AlarmLimits) *int
	f           func(l *AlarmLimits) *float64
}

var limFields = []limField{
	// column 1: heart rate and arrhythmia
	{"HR low limit:", "bpm", func(l *AlarmLimits) *int { return &l.HRLow }, nil},
	{"HR high limit:", "bpm", func(l *AlarmLimits) *int { return &l.HRHigh }, nil},
	{"\u2206ExtrBrady (below HR low):", "bpm", func(l *AlarmLimits) *int { return &l.ExtBradyD }, nil},
	{"\u2206ExtrTachy (above HR high):", "bpm", func(l *AlarmLimits) *int { return &l.ExtTachyD }, nil},
	{"V-Tach HR:", "bpm", func(l *AlarmLimits) *int { return &l.VTachHR }, nil},
	{"V-Tach run:", "PVCs", func(l *AlarmLimits) *int { return &l.VTachRun }, nil},
	{"Vent rhythm run:", "PVCs", func(l *AlarmLimits) *int { return &l.VentRhythmRun }, nil},
	{"SVT HR:", "bpm", func(l *AlarmLimits) *int { return &l.SVTHR }, nil},
	{"SVT run:", "SV beats", func(l *AlarmLimits) *int { return &l.SVTRun }, nil},
	{"PVCs/min:", "/min", func(l *AlarmLimits) *int { return &l.PVCsMin }, nil},
	{"Asystole threshold:", "sec", nil, func(l *AlarmLimits) *float64 { return &l.AsystoleSec }},
	{"Pause threshold:", "sec", nil, func(l *AlarmLimits) *float64 { return &l.PauseSec }},
	{"AFib/IHR end delay:", "sec", func(l *AlarmLimits) *int { return &l.AfEndDelay }, nil},
	{"ST high limit:", "mm", nil, func(l *AlarmLimits) *float64 { return &l.STHigh }},
	{"ST low limit:", "mm", nil, func(l *AlarmLimits) *float64 { return &l.STLow }},
	// column 2: other measurements
	{"QTc high limit:", "ms", func(l *AlarmLimits) *int { return &l.QTcHigh }, nil},
	{"SpO\u2082 low limit:", "%", func(l *AlarmLimits) *int { return &l.SpO2Low }, nil},
	{"SpO\u2082 desat limit:", "%", func(l *AlarmLimits) *int { return &l.Desat }, nil},
	{"RR low limit:", "rpm", func(l *AlarmLimits) *int { return &l.RRLow }, nil},
	{"RR high limit:", "rpm", func(l *AlarmLimits) *int { return &l.RRHigh }, nil},
	{"Apnea time:", "sec", func(l *AlarmLimits) *int { return &l.ApneaSec }, nil},
	{"NBPs high limit:", "mmHg", func(l *AlarmLimits) *int { return &l.NBPsHigh }, nil},
	{"NBPs low limit:", "mmHg", func(l *AlarmLimits) *int { return &l.NBPsLow }, nil},
	{"ABPs high limit:", "mmHg", func(l *AlarmLimits) *int { return &l.ABPsHigh }, nil},
	{"ABPs low limit:", "mmHg", func(l *AlarmLimits) *int { return &l.ABPsLow }, nil},
	{"ABPs extreme high (red):", "mmHg", func(l *AlarmLimits) *int { return &l.ABPsXHigh }, nil},
	{"ABPs extreme low (red):", "mmHg", func(l *AlarmLimits) *int { return &l.ABPsXLow }, nil},
	{"etCO\u2082 high limit:", "mmHg", func(l *AlarmLimits) *int { return &l.EtCO2High }, nil},
	{"etCO\u2082 low limit:", "mmHg", func(l *AlarmLimits) *int { return &l.EtCO2Low }, nil},
}

const limRows = 15

var (
	limitsHwnd uintptr
	limEdits   [29]uintptr
	limArrBox  [16]uintptr
	limBed     = -1 // -1 = station-wide
	hUseStn    uintptr
	hLimNote   uintptr
)

func limitGet(l *AlarmLimits, i int) string {
	if f := limFields[i]; f.f != nil {
		return strconv.FormatFloat(*f.f(l), 'f', 1, 64)
	}
	return strconv.Itoa(*limFields[i].i(l))
}

func extremeNote(l AlarmLimits) string {
	return fmt.Sprintf("Extreme Brady < %d bpm, Extreme Tachy > %d bpm (clamped at %d / %d bpm). Red alarms latch until silenced.",
		l.XBrady(), l.XTachy(), BradyClamp, TachyClamp)
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
	w, h := S(1110), S(708)
	if bed >= 0 {
		h += S(38) // room for "Use station-wide limits"
	}
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
	for i, fd := range limFields {
		col, rowi := i/limRows, i%limRows
		x := S(16) + S(400)*int32(col)
		y := top + S(34)*int32(rowi)
		ctl("STATIC", fd.label, 0, x, y+S(3), S(220), S(22), 0)
		style := uint32(WS_TABSTOP | ES_NUMBER)
		if fd.f != nil {
			style = WS_TABSTOP | ES_AUTOHSCROLL // allows "-" and "."
		}
		limEdits[i] = ctl("EDIT", limitGet(&cur, i), style, x+S(226), y, S(60), S(26), 50+i)
		ctl("STATIC", fd.unit, 0, x+S(294), y+S(3), S(70), S(22), 0)
	}
	// column 3: arrhythmia alarms switched on/off individually
	ax := S(16) + S(400)*2
	ctl("STATIC", "Arrhythmia alarms (on/off):", 0, ax, top+S(3), S(260), S(22), 0)
	for i, sw := range ArrSwitches {
		col, rowi := i/8, i%8
		limArrBox[i] = ctl("BUTTON", sw.Name, WS_TABSTOP|BS_AUTOCHECKBOX, ax+S(140)*int32(col), top+S(30)+S(32)*int32(rowi), S(135), S(26), 200+i)
		if cur.ArrOff&(1<<uint(i)) == 0 {
			call(pSendMessageW, limArrBox[i], BM_SETCHECK, 1, 0)
		}
	}
	ctl("BUTTON", "All Yellow Off", WS_TABSTOP, ax, top+S(30)+S(32)*8+S(6), S(130), S(28), ctlAllYelOff)
	ctl("BUTTON", "All Yellow On", WS_TABSTOP, ax+S(140), top+S(30)+S(32)*8+S(6), S(130), S(28), ctlAllYelOn)
	ctl("STATIC", "Alarms switched off are not announced\n(INOP: Some ECG AlarmsOff).", 0, ax, top+S(30)+S(32)*9+S(10), S(280), S(44), 0)
	hLimNote = ctl("STATIC", extremeNote(cur), 0, S(16), top+S(34)*limRows+S(4), S(1060), S(40), 0)
	by := top + S(34)*limRows + S(48)
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
	for i, b := range limArrBox {
		enable(b, !useStn)
		if useStn {
			v := uintptr(1)
			if Limits.ArrOff&(1<<uint(i)) != 0 {
				v = 0
			}
			call(pSendMessageW, b, BM_SETCHECK, v, 0)
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
	var l AlarmLimits
	for i, fd := range limFields {
		txt := strings.TrimSpace(strings.ReplaceAll(getText(limEdits[i]), ",", "."))
		if fd.f != nil {
			v, err := strconv.ParseFloat(txt, 64)
			if err != nil {
				messageBox(limitsHwnd, fd.label+" must be a number", "Alarm Limits", 0x30)
				return false
			}
			*fd.f(&l) = math.Round(v*10) / 10
		} else {
			*fd.i(&l), _ = strconv.Atoi(txt)
		}
	}
	for i, b := range limArrBox {
		if call(pSendMessageW, b, BM_GETCHECK, 0, 0) != 1 {
			l.ArrOff |= 1 << uint(i)
		}
	}
	bad := func(msg string) bool { messageBox(limitsHwnd, msg, "Alarm Limits", 0x30); return false }
	switch {
	case !(l.HRLow >= 15 && l.HRLow < l.HRHigh && l.HRHigh <= 300):
		return bad("HR limits: 15 <= HR low < HR high <= 300 bpm")
	case l.ExtBradyD < 0 || l.ExtBradyD > 50 || l.ExtTachyD < 0 || l.ExtTachyD > 50:
		return bad("\u2206ExtrBrady and \u2206ExtrTachy must be 0 - 50 bpm")
	case l.VTachHR < 20 || l.VTachHR > 300:
		return bad("V-Tach HR must be 20 - 300 bpm")
	case l.VTachRun < 3 || l.VTachRun > 99:
		return bad("V-Tach run must be 3 - 99 PVCs")
	case l.VentRhythmRun < 2 || l.VentRhythmRun > 99:
		return bad("Vent rhythm run must be 2 - 99 PVCs")
	case l.SVTHR < 120 || l.SVTHR > 300:
		return bad("SVT HR must be 120 - 300 bpm")
	case l.SVTRun < 3 || l.SVTRun > 99:
		return bad("SVT run must be 3 - 99 SV beats")
	case l.PVCsMin < 1 || l.PVCsMin > 99:
		return bad("PVCs/min must be 1 - 99")
	case l.AsystoleSec < 2.5 || l.AsystoleSec > 10:
		return bad("Asystole threshold must be 2.5 - 10 seconds")
	case l.PauseSec < 1.5 || l.PauseSec > 5 || l.PauseSec >= l.AsystoleSec:
		return bad("Pause threshold must be 1.5 - 5 seconds and below the asystole threshold")
	case l.AfEndDelay < 10 || l.AfEndDelay > 600:
		return bad("AFib/IHR end delay must be 10 - 600 seconds")
	case l.QTcHigh < 200 || l.QTcHigh > 800:
		return bad("QTc high must be 200 - 800 ms")
	case !(l.Desat >= 50 && l.Desat <= l.SpO2Low && l.SpO2Low <= 100):
		return bad("SpO2 limits: 50 <= desat <= SpO2 low <= 100")
	case !(l.RRLow >= 0 && l.RRLow < l.RRHigh && l.ApneaSec >= 5 && l.ApneaSec <= 120):
		return bad("Respiratory limits: RR low must be below RR high, and apnea time 5 - 120 s")
	case !(l.STLow < l.STHigh && l.STHigh <= 20 && l.STLow >= -20):
		return bad("ST limits: ST low must be below ST high (within -20 to +20 mm)")
	case !(l.NBPsLow < l.NBPsHigh && l.ABPsLow < l.ABPsHigh && l.EtCO2Low < l.EtCO2High):
		return bad("For NBP, ABP and etCO2 the low limit must be below the high limit")
	case !(l.ABPsXLow <= l.ABPsLow && l.ABPsHigh <= l.ABPsXHigh):
		return bad("ABP extreme limits must lie outside the ABP limits:\nextreme low <= low < high <= extreme high")
	}
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
		case ctlAllYelOff, ctlAllYelOn:
			v := uintptr(0)
			if loword(wp) == ctlAllYelOn {
				v = 1
			}
			for _, b := range limArrBox {
				call(pSendMessageW, b, BM_SETCHECK, v, 0)
			}
		}
		return 0
	case WM_CLOSE:
		closeLimits()
		return 0
	}
	return call(pDefWindowProcW, hwnd, msg, wp, lp)
}
