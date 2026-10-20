//go:build windows

package main

// Fine-tune window: per-patient PR / QRS / QT intervals, P / QRS / T
// amplitudes and ST elevation/depression. Changes apply live.

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unsafe"
)

const (
	ctlTuneApply = 3
	ctlTuneReset = 4
)

var (
	tuneHwnd  uintptr
	tuneBed   int
	tuneEdits [7]uintptr
	tuneOrig  Tune // to restore on Cancel
)

type tuneField struct {
	label, unit string
	lo, hi      float64
	zeroAuto    bool // 0 means "automatic"
}

var tuneFields = []tuneField{
	{"PR interval:", "ms", 80, 400, true},
	{"QRS duration:", "ms", 40, 240, true},
	{"QT interval:", "ms", 200, 700, true},
	{"P wave amplitude:", "%", 0, 300, false},
	{"QRS amplitude:", "%", 0, 300, false},
	{"T wave amplitude:", "%", -300, 300, false},
	{"ST segment:", "mm", -10, 10, false},
}

func tuneValues(t Tune) []float64 {
	if !t.Set {
		t = DefaultTune()
	}
	return []float64{float64(t.PRms), float64(t.QRSms), float64(t.QTms),
		float64(t.PAmp), float64(t.QRSAmp), float64(t.TAmp), t.STmm}
}

func fmtNum(v float64) string {
	if v == math.Trunc(v) {
		return strconv.Itoa(int(v))
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// tuneHints describes what "auto" means for this patient right now.
func tuneHints(p *Patient) []string {
	pr := "160"
	switch p.Cfg.Rhythm {
	case RFirstDegree:
		pr = "300"
	case RMobitzI, RMobitzII:
		pr = "180 (base)"
	}
	rr := 60 / p.ventRate()
	qt := int(math.Round(1000 * clamp(0.40*math.Sqrt(rr), 0.24, 0.46)))
	return []string{
		"0 = auto (" + pr + ")",
		"0 = auto (80)",
		fmt.Sprintf("0 = auto (%d at this rate)", qt),
		"100 = normal",
		"100 = normal",
		"100 = normal, negative = inverted",
		"+ elevation / - depression",
	}
}

func openTune(bed int) {
	if tuneHwnd != 0 {
		call(pSetForegroundWin, tuneHwnd)
		return
	}
	if setupHwnd != 0 || limitsHwnd != 0 {
		return
	}
	tuneBed = bed
	p := patients[bed]
	tuneOrig = p.Cfg.Tune
	uiFont = font(S(15), false, "Segoe UI")
	w, h := S(470), S(420)
	var mr RECT
	call(pGetWindowRect, mainHwnd, uintptr(unsafe.Pointer(&mr)))
	x := mr.Left + (mr.Right-mr.Left-w)/2
	y := mr.Top + (mr.Bottom-mr.Top-h)/2
	title := fmt.Sprintf("%s - Fine-tune waveform", bedLabel(bed))
	tuneHwnd = call(pCreateWindowExW, WS_EX_DLGMODALFRAME, uintptr(unsafe.Pointer(wstr("TeleSimTune"))), uintptr(unsafe.Pointer(wstr(title))),
		WS_CAPTION|WS_SYSMENU|WS_POPUP, uintptr(x), uintptr(y), uintptr(w), uintptr(h), mainHwnd, 0, hInst, 0)
	ctlParent = tuneHwnd
	vals := tuneValues(p.Cfg.Tune)
	hints := tuneHints(p)
	for i, f := range tuneFields {
		ry := S(16) + S(36)*int32(i)
		ctl("STATIC", f.label, 0, S(16), ry+S(3), S(140), S(22), 0)
		tuneEdits[i] = ctl("EDIT", fmtNum(vals[i]), WS_TABSTOP|ES_AUTOHSCROLL, S(160), ry, S(64), S(26), 60+i)
		ctl("STATIC", f.unit+"   "+hints[i], 0, S(232), ry+S(3), S(225), S(22), 0)
	}
	ctl("STATIC", "PR applies to sinus, AV-block and atrial-paced rhythms. QRS width, QT and ST shape the patient's own (narrow) beats.",
		0, S(16), S(16)+S(36)*7, w-S(40), S(40), 0)
	by := S(16) + S(36)*7 + S(48)
	ctl("BUTTON", "OK", WS_TABSTOP|BS_DEFPUSHBUTTON, S(30), by, S(95), S(30), 1)
	ctl("BUTTON", "Apply", WS_TABSTOP, S(135), by, S(95), S(30), ctlTuneApply)
	ctl("BUTTON", "Reset", WS_TABSTOP, S(240), by, S(95), S(30), ctlTuneReset)
	ctl("BUTTON", "Cancel", WS_TABSTOP, S(345), by, S(95), S(30), 2)
	call(pEnableWindow, mainHwnd, 0)
	call(pShowWindow, tuneHwnd, SW_SHOW)
	call(pSetFocus, tuneEdits[0])
}

func closeTune() {
	if tuneHwnd == 0 {
		return
	}
	call(pEnableWindow, mainHwnd, 1)
	call(pSetForegroundWin, mainHwnd)
	h := tuneHwnd
	tuneHwnd = 0
	call(pDestroyWindow, h)
}

// readTune parses and clamps the fields, writing back any corrected values.
func readTune() Tune {
	v := make([]float64, len(tuneFields))
	for i, f := range tuneFields {
		s := strings.TrimSpace(strings.ReplaceAll(getText(tuneEdits[i]), ",", "."))
		x, err := strconv.ParseFloat(s, 64)
		if err != nil {
			x = tuneValues(DefaultTune())[i]
		}
		if !(f.zeroAuto && x == 0) {
			x = clamp(x, f.lo, f.hi)
		}
		if i < 6 {
			x = math.Round(x)
		} else {
			x = math.Round(x*2) / 2 // ST in 0.5 mm steps
		}
		v[i] = x
		call(pSetWindowTextW, tuneEdits[i], uintptr(unsafe.Pointer(wstr(fmtNum(x)))))
	}
	return Tune{Set: true, UserSet: true, Natural: patients[tuneBed].Cfg.Tune.Natural,
		PRms: int(v[0]), QRSms: int(v[1]), QTms: int(v[2]),
		PAmp: int(v[3]), QRSAmp: int(v[4]), TAmp: int(v[5]), STmm: v[6]}
}

func tuneProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch uint32(msg) {
	case WM_COMMAND:
		switch loword(wp) {
		case 1: // OK
			patients[tuneBed].SetTune(readTune())
			closeTune()
		case ctlTuneApply:
			patients[tuneBed].SetTune(readTune())
		case ctlTuneReset:
			for i, x := range tuneValues(DefaultTune()) {
				call(pSetWindowTextW, tuneEdits[i], uintptr(unsafe.Pointer(wstr(fmtNum(x)))))
			}
			patients[tuneBed].SetTune(DefaultTune())
		case 2: // Cancel: undo anything applied
			patients[tuneBed].SetTune(tuneOrig)
			closeTune()
		}
		return 0
	case WM_CLOSE:
		patients[tuneBed].SetTune(tuneOrig)
		closeTune()
		return 0
	}
	return call(pDefWindowProcW, hwnd, msg, wp, lp)
}
