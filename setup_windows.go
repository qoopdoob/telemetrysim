//go:build windows

package main

import (
	"fmt"
	"strconv"
	"syscall"
	"unsafe"
)

// syscallCallback wraps a window procedure so a Go panic is logged and the
// message is handled by DefWindowProc instead of killing the program.
func syscallCallback(f func(hwnd, msg, wp, lp uintptr) uintptr) uintptr {
	return syscall.NewCallback(func(hwnd, msg, wp, lp uintptr) (r uintptr) {
		defer func() {
			if e := recover(); e != nil {
				logPanic(fmt.Sprintf("window message 0x%x", msg), e)
				r = call(pDefWindowProcW, hwnd, msg, wp, lp)
			}
		}()
		return f(hwnd, msg, wp, lp)
	})
}

const (
	ctlName     = 10
	ctlRhythm   = 11
	ctlRate     = 12
	ctlEctopy   = 13
	ctlPattern  = 14
	ctlFreq     = 15
	ctlApply    = 3
	ctlStandby  = 16
	ctlRatio    = 17
	ctlSpO2     = 18
	ctlSpO2Val  = 19
	ctlPace     = 20
	ctlIntr     = 21
	ctlArt      = 22
	ctlArtLvl   = 23
	ctlResp     = 24
	ctlRR       = 25
	ctlCardiac  = 26
	ctlRespWave = 27
	ctlNBPMode  = 30
	ctlNBPInt   = 31
	ctlNBPSys   = 32
	ctlNBPDia   = 33
	ctlNBPNow   = 34
	ctlABP      = 35
	ctlABPSys   = 36
	ctlABPDia   = 37
	ctlCO2      = 38
	ctlEtCO2    = 39
)

var (
	setupHwnd                                    uintptr
	setupBed                                     int
	hName, hRhythm, hRate, hRange                uintptr
	hEctopy, hPattern, hFreq, hNote, hFreqLabel  uintptr
	hStandby, hRatio, hRateLabel                 uintptr
	hSpO2, hSpO2Val                              uintptr
	hPace, hIntr, hRow3Label, hIntrLabel         uintptr
	ctlParent                                    uintptr
	hArt, hArtLvl                                uintptr
	hResp, hRR, hCardiac, hRespWave              uintptr
	hNBPMode, hNBPInt, hNBPSys, hNBPDia, hNBPNow uintptr
	hABP, hABPSys, hABPDia, hCO2, hEtCO2         uintptr
	uiFont                                       uintptr
)

func openSetup(bed int) {
	if setupHwnd != 0 {
		call(pSetForegroundWin, setupHwnd)
		return
	}
	setupBed = bed
	uiFont = font(S(15), false, "Segoe UI")
	w, h := S(850), S(598)
	var mr RECT
	call(pGetWindowRect, mainHwnd, uintptr(unsafe.Pointer(&mr)))
	x := mr.Left + (mr.Right-mr.Left-w)/2
	y := mr.Top + (mr.Bottom-mr.Top-h)/2
	title := fmt.Sprintf("%s - Patient Setup", bedLabel(bed))
	activating := standby[bed]
	if activating {
		title = fmt.Sprintf("Activate %s", bedLabel(bed))
	}
	setupHwnd = call(pCreateWindowExW, WS_EX_DLGMODALFRAME, uintptr(unsafe.Pointer(wstr("TeleSimSetup"))), uintptr(unsafe.Pointer(wstr(title))),
		WS_CAPTION|WS_SYSMENU|WS_POPUP, uintptr(x), uintptr(y), uintptr(w), uintptr(h), mainHwnd, 0, hInst, 0)

	ctlParent = setupHwnd
	lx, cx := S(16), S(170)
	row := func(i int) int32 { return S(18) + S(38)*int32(i) }
	label := func(s string, i int) uintptr {
		return ctl("STATIC", s, 0, lx, row(i)+S(3), cx-lx-S(6), S(22), 0)
	}
	label("Patient name:", 0)
	hName = ctl("EDIT", patients[bed].Cfg.Name, WS_TABSTOP|ES_AUTOHSCROLL, cx, row(0), S(270), S(26), ctlName)
	call(pSendMessageW, hName, 0x00C5, 40, 0) // EM_LIMITTEXT

	label("Rhythm:", 1)
	hRhythm = ctl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, cx, row(1), S(270), S(420), ctlRhythm)
	for _, r := range Rhythms {
		call(pSendMessageW, hRhythm, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(wstr(r.Name))))
	}

	hRateLabel = label("Heart rate (bpm):", 2)
	hRate = ctl("EDIT", "", WS_TABSTOP|ES_NUMBER, cx, row(2), S(70), S(26), ctlRate)
	hRange = ctl("STATIC", "", 0, cx+S(80), row(2)+S(3), S(190), S(22), 0)

	// row 3 is shared: flutter conduction ratio, or pacing mode for paced rhythms
	hRow3Label = label("Flutter conduction:", 3)
	hRatio = ctl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, cx, row(3), S(270), S(220), ctlRatio)
	for _, s := range FlutterRatioNames {
		call(pSendMessageW, hRatio, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(wstr(s))))
	}
	hPace = ctl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, cx, row(3), S(270), S(160), ctlPace)
	for _, s := range PaceModeNames {
		call(pSendMessageW, hPace, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(wstr(s))))
	}
	// row 4: intrinsic rate under a demand pacemaker
	hIntrLabel = label("Intrinsic rate (bpm):", 4)
	hIntr = ctl("EDIT", "", WS_TABSTOP|ES_NUMBER, cx, row(4), S(70), S(26), ctlIntr)
	ctl("STATIC", "patient's own rate (drifts +/-10%)", 0, cx+S(80), row(4)+S(3), S(220), S(22), 0)

	label("Ectopic beats:", 5)
	hEctopy = ctl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, cx, row(5), S(270), S(220), ctlEctopy)
	for _, s := range EctopyNames {
		call(pSendMessageW, hEctopy, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(wstr(s))))
	}

	label("Ectopy pattern:", 6)
	hPattern = ctl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, cx, row(6), S(270), S(240), ctlPattern)
	for _, s := range PatternNames {
		call(pSendMessageW, hPattern, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(wstr(s))))
	}

	hFreqLabel = label("Ectopy frequency:", 7)
	hFreq = ctl("EDIT", "", WS_TABSTOP|ES_NUMBER, cx, row(7), S(70), S(26), ctlFreq)
	ctl("STATIC", "per minute", 0, cx+S(80), row(7)+S(3), S(190), S(22), 0)

	label("SpO2:", 8)
	hSpO2 = ctl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, cx, row(8), S(160), S(160), ctlSpO2)
	for _, m := range SpO2ModeNames {
		call(pSendMessageW, hSpO2, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(wstr(m))))
	}
	hSpO2Val = ctl("EDIT", "", WS_TABSTOP|ES_NUMBER, cx+S(170), row(8), S(55), S(26), ctlSpO2Val)
	ctl("STATIC", "%", 0, cx+S(232), row(8)+S(3), S(30), S(22), 0)

	label("Artifact:", 9)
	hArt = ctl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, cx, row(9), S(185), S(220), ctlArt)
	for _, a := range ArtifactNames {
		call(pSendMessageW, hArt, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(wstr(a))))
	}
	hArtLvl = ctl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, cx+S(190), row(9), S(80), S(140), ctlArtLvl)
	for _, a := range ArtLevelNames {
		call(pSendMessageW, hArtLvl, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(wstr(a))))
	}

	label("Respiration:", 10)
	hRR = ctl("EDIT", "", WS_TABSTOP|ES_NUMBER, cx, row(10), S(42), S(26), ctlRR)
	ctl("STATIC", "/min (0 = apnea)", 0, cx+S(48), row(10)+S(3), S(105), S(22), 0)
	hResp = ctl("BUTTON", "Rate", WS_TABSTOP|BS_AUTOCHECKBOX, cx+S(155), row(10), S(55), S(26), ctlResp)
	hRespWave = ctl("BUTTON", "Wave", WS_TABSTOP|BS_AUTOCHECKBOX, cx+S(212), row(10), S(60), S(26), ctlRespWave)

	hCardiac = ctl("BUTTON", "Cardiac (ECG) monitoring", WS_TABSTOP|BS_AUTOCHECKBOX, lx, row(11), S(220), S(26), ctlCardiac)
	hStandby = ctl("BUTTON", "Standby (monitoring paused)", WS_TABSTOP|BS_AUTOCHECKBOX, lx+S(230), row(11), S(220), S(26), ctlStandby)

	hNote = ctl("STATIC", "", 0, lx, row(12)+S(2), S(430), S(40), 0)

	// ---- right column: blood pressure and capnography ----
	rx, rcx := S(480), S(610)
	rlabel := func(s string, i int) uintptr {
		return ctl("STATIC", s, 0, rx, row(i)+S(3), rcx-rx-S(6), S(22), 0)
	}
	ctl("STATIC", "Blood pressure and CO\u2082", 0, rx, row(0)+S(3), S(340), S(22), 0)
	rlabel("NBP (cuff):", 1)
	hNBPMode = ctl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, rcx, row(1), S(100), S(140), ctlNBPMode)
	for _, m := range NBPModeNames {
		call(pSendMessageW, hNBPMode, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(wstr(m))))
	}
	hNBPInt = ctl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, rcx+S(106), row(1), S(110), S(180), ctlNBPInt)
	for _, m := range NBPIntervals {
		call(pSendMessageW, hNBPInt, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(wstr("Auto "+NBPIntervalLabel(m)))))
	}
	rlabel("Patient's BP:", 2)
	hNBPSys = ctl("EDIT", "", WS_TABSTOP|ES_NUMBER, rcx, row(2), S(48), S(26), ctlNBPSys)
	ctl("STATIC", "/", 0, rcx+S(52), row(2)+S(3), S(10), S(22), 0)
	hNBPDia = ctl("EDIT", "", WS_TABSTOP|ES_NUMBER, rcx+S(64), row(2), S(48), S(26), ctlNBPDia)
	ctl("STATIC", "mmHg", 0, rcx+S(118), row(2)+S(3), S(50), S(22), 0)
	hNBPNow = ctl("BUTTON", "Measure NBP now", WS_TABSTOP, rcx, row(3), S(150), S(28), ctlNBPNow)

	hABP = ctl("BUTTON", "ABP (arterial line)", WS_TABSTOP|BS_AUTOCHECKBOX, rx, row(5), S(250), S(26), ctlABP)
	rlabel("Arterial sys/dia:", 6)
	hABPSys = ctl("EDIT", "", WS_TABSTOP|ES_NUMBER, rcx, row(6), S(48), S(26), ctlABPSys)
	ctl("STATIC", "/", 0, rcx+S(52), row(6)+S(3), S(10), S(22), 0)
	hABPDia = ctl("EDIT", "", WS_TABSTOP|ES_NUMBER, rcx+S(64), row(6), S(48), S(26), ctlABPDia)
	ctl("STATIC", "mmHg", 0, rcx+S(118), row(6)+S(3), S(50), S(22), 0)

	hCO2 = ctl("BUTTON", "EtCO\u2082 (capnography)", WS_TABSTOP|BS_AUTOCHECKBOX, rx, row(8), S(250), S(26), ctlCO2)
	rlabel("End-tidal CO\u2082:", 9)
	hEtCO2 = ctl("EDIT", "", WS_TABSTOP|ES_NUMBER, rcx, row(9), S(48), S(26), ctlEtCO2)
	ctl("STATIC", "mmHg  (breaths follow the resp rate)", 0, rcx+S(54), row(9)+S(3), S(190), S(22), 0)

	by := row(13) + S(8)
	okText := "OK"
	if activating {
		okText = "Activate"
	}
	ctl("BUTTON", okText, WS_TABSTOP|BS_DEFPUSHBUTTON, w-S(330), by, S(90), S(30), 1)
	ctl("BUTTON", "Apply", WS_TABSTOP, w-S(230), by, S(90), S(30), ctlApply)
	ctl("BUTTON", "Cancel", WS_TABSTOP, w-S(130), by, S(90), S(30), 2)

	c := patients[bed].Cfg
	call(pSendMessageW, hRhythm, CB_SETCURSEL, uintptr(c.Rhythm), 0)
	call(pSendMessageW, hEctopy, CB_SETCURSEL, uintptr(c.Ectopy), 0)
	call(pSendMessageW, hPattern, CB_SETCURSEL, uintptr(c.Pattern), 0)
	call(pSendMessageW, hRatio, CB_SETCURSEL, uintptr(c.FlRatio), 0)
	call(pSendMessageW, hPace, CB_SETCURSEL, uintptr(c.PaceMode), 0)
	call(pSendMessageW, hArt, CB_SETCURSEL, uintptr(c.Artifact), 0)
	if !c.RespRateOff {
		call(pSendMessageW, hResp, BM_SETCHECK, 1, 0)
	}
	if c.RespWaveOn {
		call(pSendMessageW, hRespWave, BM_SETCHECK, 1, 0)
	}
	rr := c.RR
	if c.Apnea {
		rr = 0
	} else if rr == 0 {
		rr = 16
	}
	call(pSetWindowTextW, hRR, uintptr(unsafe.Pointer(wstr(strconv.Itoa(rr)))))
	if !c.CardiacOff {
		call(pSendMessageW, hCardiac, BM_SETCHECK, 1, 0)
	}
	call(pSendMessageW, hArtLvl, CB_SETCURSEL, uintptr(c.ArtLevel), 0)
	setNum := func(h uintptr, v, def int) {
		if v == 0 {
			v = def
		}
		call(pSetWindowTextW, h, uintptr(unsafe.Pointer(wstr(strconv.Itoa(v)))))
	}
	call(pSendMessageW, hNBPMode, CB_SETCURSEL, uintptr(c.NBPMode), 0)
	ivSel := 4 // 15 min
	for i, m := range NBPIntervals {
		if m == nbpMinutes(c) {
			ivSel = i
		}
	}
	call(pSendMessageW, hNBPInt, CB_SETCURSEL, uintptr(ivSel), 0)
	setNum(hNBPSys, c.NBPSys, 120)
	setNum(hNBPDia, c.NBPDia, 75)
	if c.ABPOn {
		call(pSendMessageW, hABP, BM_SETCHECK, 1, 0)
	}
	setNum(hABPSys, c.ABPSys, 120)
	setNum(hABPDia, c.ABPDia, 70)
	if c.CO2On {
		call(pSendMessageW, hCO2, BM_SETCHECK, 1, 0)
	}
	setNum(hEtCO2, c.EtCO2, 38)
	ir := c.IntrRate
	if ir == 0 {
		ir = 55
	}
	call(pSetWindowTextW, hIntr, uintptr(unsafe.Pointer(wstr(strconv.Itoa(ir)))))
	call(pSendMessageW, hSpO2, CB_SETCURSEL, uintptr(c.SpO2On), 0)
	sp := c.SpO2
	if sp == 0 {
		sp = 98
	}
	call(pSetWindowTextW, hSpO2Val, uintptr(unsafe.Pointer(wstr(strconv.Itoa(sp)))))
	rate := c.Rate
	if c.Rhythm == RLeadsOff && rate == 0 {
		rate = 78
	}
	if !RateAdjustable(c.Rhythm) {
		rate = 0
	}
	call(pSetWindowTextW, hRate, uintptr(unsafe.Pointer(wstr(strconv.Itoa(rate)))))
	freq := c.Freq
	if freq == 0 {
		freq = 6
	}
	call(pSetWindowTextW, hFreq, uintptr(unsafe.Pointer(wstr(strconv.Itoa(freq)))))
	// Opening a bed that is in standby defaults to activating it.
	call(pSendMessageW, hStandby, BM_SETCHECK, 0, 0)
	refreshSetup()

	call(pEnableWindow, mainHwnd, 0)
	call(pShowWindow, setupHwnd, SW_SHOW)
	call(pSetFocus, hRhythm)
}

func ctl(class, txt string, style uint32, x, y, w, h int32, id int) uintptr {
	ex := uintptr(0)
	if class == "EDIT" {
		ex = WS_EX_CLIENTEDGE
	}
	hw := call(pCreateWindowExW, ex, uintptr(unsafe.Pointer(wstr(class))), uintptr(unsafe.Pointer(wstr(txt))), uintptr(WS_CHILD|WS_VISIBLE|style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), ctlParent, uintptr(id), hInst, 0)
	call(pSendMessageW, hw, WM_SETFONT, uiFont, 1)
	return hw
}

func cbSel(h uintptr) int {
	v := int(int32(call(pSendMessageW, h, CB_GETCURSEL, 0, 0)))
	if v < 0 {
		return 0
	}
	return v
}

func enable(h uintptr, on bool) {
	v := uintptr(0)
	if on {
		v = 1
	}
	call(pEnableWindow, h, v)
}

// refreshSetup greys out fields that do not apply to the chosen rhythm/ectopy.
func refreshSetup() {
	r := Rhythm(cbSel(hRhythm))
	info := Rhythms[r]
	cardiacOn := call(pSendMessageW, hCardiac, BM_GETCHECK, 0, 0) == 1
	pulseOnly := r == RLeadsOff || !cardiacOn // rate is only seen as SpO2 pulse
	enable(hRate, RateAdjustable(r))
	ratio := cbSel(hRatio)
	enable(hRatio, r == RAflutter)
	enable(hArtLvl, cbSel(hArt) != ArtNone)
	rateOn := call(pSendMessageW, hResp, BM_GETCHECK, 0, 0) == 1
	if !rateOn { // the waveform depends on the rate number
		call(pSendMessageW, hRespWave, BM_SETCHECK, 0, 0)
	}
	enable(hRespWave, rateOn)
	enable(hRR, rateOn)
	nbpOn := cbSel(hNBPMode) != NBPOff
	enable(hNBPInt, cbSel(hNBPMode) == NBPAuto)
	enable(hNBPSys, nbpOn)
	enable(hNBPDia, nbpOn)
	enable(hNBPNow, nbpOn)
	abpOn := call(pSendMessageW, hABP, BM_GETCHECK, 0, 0) == 1
	enable(hABPSys, abpOn)
	enable(hABPDia, abpOn)
	enable(hEtCO2, call(pSendMessageW, hCO2, BM_GETCHECK, 0, 0) == 1)
	paced := IsPaced(r)
	show := func(h uintptr, on bool) {
		v := uintptr(0)
		if on {
			v = SW_SHOW
		}
		call(pShowWindow, h, v)
	}
	show(hRatio, !paced)
	show(hPace, paced)
	if paced {
		call(pSetWindowTextW, hRow3Label, uintptr(unsafe.Pointer(wstr("Pacing mode:"))))
	} else {
		call(pSetWindowTextW, hRow3Label, uintptr(unsafe.Pointer(wstr("Flutter conduction:"))))
	}
	demand := paced && cbSel(hPace) == PaceDemand
	enable(hIntr, demand)
	enable(hIntrLabel, demand)
	enable(hSpO2Val, cbSel(hSpO2) == SpO2Monitored)
	if r == RAflutter && ratio != FlAuto {
		call(pSetWindowTextW, hRateLabel, uintptr(unsafe.Pointer(wstr("Flutter wave rate (/min):"))))
	} else if demand {
		call(pSetWindowTextW, hRateLabel, uintptr(unsafe.Pointer(wstr("Pacer lower rate (bpm):"))))
	} else if paced {
		call(pSetWindowTextW, hRateLabel, uintptr(unsafe.Pointer(wstr("Pacer rate (bpm):"))))
	} else if pulseOnly {
		call(pSetWindowTextW, hRateLabel, uintptr(unsafe.Pointer(wstr("Pulse rate (bpm):"))))
	} else {
		call(pSetWindowTextW, hRateLabel, uintptr(unsafe.Pointer(wstr("Heart rate (bpm):"))))
	}
	if RateAdjustable(r) {
		lo, hi, _ := RateRange(r, ratio)
		call(pSetWindowTextW, hRange, uintptr(unsafe.Pointer(wstr(fmt.Sprintf("(range %d - %d)", lo, hi)))))
	} else {
		call(pSetWindowTextW, hRange, uintptr(unsafe.Pointer(wstr("(not applicable)"))))
	}
	enable(hEctopy, info.Ectopy)
	ect := cbSel(hEctopy)
	on := info.Ectopy && ect != EctNone
	enable(hPattern, on)
	enable(hFreq, on && PatternUsesFreq(cbSel(hPattern)))

	note := ""
	switch {
	case !info.Ectopy:
		note = "Ectopic beats are not used with this rhythm."
	case (ect == EctPAC || ect == EctPJC) && !info.AtrialEctopy:
		note = "PACs/PJCs don't apply to this rhythm - choose PVCs instead."
	case on && PatternUsesFreq(cbSel(hPattern)):
		note = "Frequency = ectopic events per minute (pairs or runs count as one event)."
	case on:
		note = "Fixed pattern - frequency not used."
	}
	call(pSetWindowTextW, hNote, uintptr(unsafe.Pointer(wstr(note))))
}

func applySetup() {
	r := Rhythm(cbSel(hRhythm))
	info := Rhythms[r]
	rate, _ := strconv.Atoi(getText(hRate))
	ratio := cbSel(hRatio)
	if r != RAflutter {
		ratio = FlAuto
	}
	if RateAdjustable(r) {
		lo, hi, _ := RateRange(r, ratio)
		if rate < lo {
			rate = lo
		}
		if rate > hi {
			rate = hi
		}
		call(pSetWindowTextW, hRate, uintptr(unsafe.Pointer(wstr(strconv.Itoa(rate)))))
	} else {
		rate = 0
	}
	pace := PaceFixed
	if IsPaced(r) {
		pace = cbSel(hPace)
	}
	intr, _ := strconv.Atoi(getText(hIntr))
	if intr < 20 {
		intr = 20
	}
	if intr > 150 {
		intr = 150
	}
	call(pSetWindowTextW, hIntr, uintptr(unsafe.Pointer(wstr(strconv.Itoa(intr)))))
	spo2, _ := strconv.Atoi(getText(hSpO2Val))
	if spo2 < 50 {
		spo2 = 50
	}
	if spo2 > 100 {
		spo2 = 100
	}
	call(pSetWindowTextW, hSpO2Val, uintptr(unsafe.Pointer(wstr(strconv.Itoa(spo2)))))
	readBP := func(hs, hd uintptr) (int, int) {
		sy, _ := strconv.Atoi(getText(hs))
		di, _ := strconv.Atoi(getText(hd))
		sy = clampInt(sy, 40, 280)
		di = clampInt(di, 15, 180)
		if di > sy-10 {
			di = sy - 10
		}
		call(pSetWindowTextW, hs, uintptr(unsafe.Pointer(wstr(strconv.Itoa(sy)))))
		call(pSetWindowTextW, hd, uintptr(unsafe.Pointer(wstr(strconv.Itoa(di)))))
		return sy, di
	}
	nbpS, nbpD := readBP(hNBPSys, hNBPDia)
	abpS, abpD := readBP(hABPSys, hABPDia)
	et, _ := strconv.Atoi(getText(hEtCO2))
	et = clampInt(et, 5, 99)
	call(pSetWindowTextW, hEtCO2, uintptr(unsafe.Pointer(wstr(strconv.Itoa(et)))))
	rr, _ := strconv.Atoi(getText(hRR))
	if rr < 0 {
		rr = 0
	}
	if rr > 60 {
		rr = 60
	}
	call(pSetWindowTextW, hRR, uintptr(unsafe.Pointer(wstr(strconv.Itoa(rr)))))
	freq, _ := strconv.Atoi(getText(hFreq))
	if freq < 0 {
		freq = 0
	}
	if freq > 40 {
		freq = 40
		call(pSetWindowTextW, hFreq, uintptr(unsafe.Pointer(wstr("40"))))
	}
	cfg := Config{
		Name:        getText(hName),
		Rhythm:      r,
		Rate:        rate,
		Ectopy:      cbSel(hEctopy),
		Pattern:     cbSel(hPattern),
		Freq:        freq,
		FlRatio:     ratio,
		SpO2On:      cbSel(hSpO2),
		PaceMode:    pace,
		IntrRate:    intr,
		Artifact:    cbSel(hArt),
		ArtLevel:    cbSel(hArtLvl),
		Tune:        patients[setupBed].Cfg.Tune, // keep fine-tuning
		Lead:        patients[setupBed].Cfg.Lead, // keep the chosen lead
		RespRateOff: call(pSendMessageW, hResp, BM_GETCHECK, 0, 0) != 1,
		RespWaveOn: call(pSendMessageW, hResp, BM_GETCHECK, 0, 0) == 1 &&
			call(pSendMessageW, hRespWave, BM_GETCHECK, 0, 0) == 1,
		RR:          rr,
		Apnea:       rr == 0,
		CardiacOff:  call(pSendMessageW, hCardiac, BM_GETCHECK, 0, 0) != 1,
		NBPMode:     cbSel(hNBPMode),
		NBPInterval: NBPIntervals[clampInt(cbSel(hNBPInt), 0, len(NBPIntervals)-1)],
		NBPSys:      nbpS,
		NBPDia:      nbpD,
		ABPOn:       call(pSendMessageW, hABP, BM_GETCHECK, 0, 0) == 1,
		ABPSys:      abpS,
		ABPDia:      abpD,
		CO2On:       call(pSendMessageW, hCO2, BM_GETCHECK, 0, 0) == 1,
		EtCO2:       et,
		SpO2:        spo2,
	}
	if !info.Ectopy {
		cfg.Ectopy = EctNone
	}
	patients[setupBed].Apply(cfg, now())
	standby[setupBed] = call(pSendMessageW, hStandby, BM_GETCHECK, 0, 0) == 1
}

func closeSetup() {
	if setupHwnd == 0 {
		return
	}
	call(pEnableWindow, mainHwnd, 1)
	call(pSetForegroundWin, mainHwnd)
	h := setupHwnd
	setupHwnd = 0
	call(pDestroyWindow, h)
}

func setupProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch uint32(msg) {
	case WM_COMMAND:
		id, code := loword(wp), hiword(wp)
		switch id {
		case 1: // OK / Enter
			applySetup()
			closeSetup()
		case 2: // Cancel / Esc
			closeSetup()
		case ctlApply:
			applySetup()
		case ctlRhythm:
			if code == CBN_SELCHANGE {
				r := Rhythm(cbSel(hRhythm))
				if r != RLeadsOff && call(pSendMessageW, hResp, BM_GETCHECK, 0, 0) != 1 &&
					call(pSendMessageW, hRespWave, BM_GETCHECK, 0, 0) != 1 {
					// a heart rhythm is connected: respiration comes with the ECG leads
					call(pSendMessageW, hResp, BM_SETCHECK, 1, 0)
				}
				if RateAdjustable(r) {
					_, _, def := RateRange(r, cbSel(hRatio))
					call(pSetWindowTextW, hRate, uintptr(unsafe.Pointer(wstr(strconv.Itoa(def)))))
				} else {
					call(pSetWindowTextW, hRate, uintptr(unsafe.Pointer(wstr("0"))))
				}
				refreshSetup()
			}
		case ctlRatio:
			if code == CBN_SELCHANGE {
				_, _, def := RateRange(RAflutter, cbSel(hRatio))
				call(pSetWindowTextW, hRate, uintptr(unsafe.Pointer(wstr(strconv.Itoa(def)))))
				refreshSetup()
			}
		case ctlResp, ctlRespWave, ctlCardiac, ctlABP, ctlCO2:
			refreshSetup()
		case ctlNBPMode, ctlNBPInt:
			if code == CBN_SELCHANGE {
				refreshSetup()
			}
		case ctlNBPNow:
			applySetup()
			patients[setupBed].StartNBP(now())
		case ctlEctopy, ctlPattern, ctlSpO2, ctlPace, ctlArt:
			if code == CBN_SELCHANGE {
				refreshSetup()
			}
		}
		return 0
	case WM_CLOSE:
		closeSetup()
		return 0
	}
	return call(pDefWindowProcW, hwnd, msg, wp, lp)
}
