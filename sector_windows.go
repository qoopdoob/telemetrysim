//go:build windows

package main

// Central-station screen layout, styled after an IntelliVue-type information
// centre: per-bed sectors with the bed label and patient name at top left,
// alarm messages boxed at top right, green ECG with lead label and 1 mV
// calibration bar, numerics (label, alarm limits and large value) on the
// right, a blue background while a new alarm is unacknowledged, and flat grey
// buttons that appear only while the mouse is over the sector.

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	cGreen    = rgb(0, 255, 0)
	cWhite    = rgb(240, 240, 240)
	cGrey     = rgb(140, 140, 140)
	cCyan     = rgb(0, 230, 255)
	cPurple   = rgb(205, 125, 255)
	cAlarmBg  = rgb(30, 34, 132) // sector background for a new (unacknowledged) alarm
	cSepLine  = rgb(96, 96, 96)
	cHoverBdr = rgb(214, 138, 46)
)

// hover buttons
const (
	bStrip = iota + 1
	bRhythm
	bTune
	bLimits
	bSilence
	bActivate
	bExpand // dock the patient in the expanded view at the bottom (or collapse it)
)

type hbtn struct {
	rc   RECT
	kind int
}

var (
	hoverSec     = -1 // index (in sectors) of the sector under the mouse
	hoverBtnSec  = -1
	hoverBtnKind = 0
	expandedBed  = -1   // bed shown in the expanded view (-1 = none)
	hoverNBPSec  = -1   // sector whose NBP title is under the mouse
	showRhythm   = true // rhythm label over the ECG
	hoverSilAll  bool
)

// updateHover finds the sector under the mouse (called every timer tick, so
// the buttons disappear when the mouse leaves the window).
func updateHover(hwnd uintptr) {
	var pt POINT
	call(pGetCursorPos, uintptr(unsafe.Pointer(&pt)))
	under := call(pWindowFromPoint, uintptr(uint32(pt.X))|uintptr(uint32(pt.Y))<<32)
	call(pScreenToClient, hwnd, uintptr(unsafe.Pointer(&pt)))
	hoverSec, hoverBtnSec, hoverBtnKind, hoverNBPSec = -1, -1, 0, -1
	if under != hwnd || call(pIsWindowEnabled, hwnd) == 0 {
		return
	}
	for i, s := range sectors {
		if s.rc.contains(pt.X, pt.Y) {
			hoverSec = i
			if s.nbp.contains(pt.X, pt.Y) {
				hoverNBPSec = i
			}
			for _, b := range s.btns {
				if b.rc.contains(pt.X, pt.Y) {
					hoverBtnSec, hoverBtnKind = i, b.kind
				}
			}
		}
	}
}

// rhythmLabel is the arrhythmia-analysis label shown over the ECG.
func rhythmLabel(p *Patient) string {
	c := p.Cfg
	if c.CardiacOff || c.Rhythm == RLeadsOff {
		return ""
	}
	if c.Ectopy == EctPVCUni || c.Ectopy == EctPVCMulti {
		switch c.Pattern {
		case PatBigeminy:
			return "Vent Bigeminy"
		case PatTrigeminy:
			return "Vent Trigeminy"
		}
	}
	hr := p.HR
	byRate := func(base string) string {
		switch {
		case hr > 100:
			return base + " Tach"
		case hr < 60:
			return base + " Brady"
		}
		return base + " Rhythm"
	}
	switch c.Rhythm {
	case RAsystole:
		return "Asystole"
	case RSinus, RSinusArrhythmia, RFirstDegree, RMobitzI, RMobitzII:
		return byRate("Sinus")
	case RAfib:
		return "Atrial Fib"
	case RAflutter, RSVT, RJunctional:
		return byRate("Supravent")
	case RThirdDegree:
		if c.Rate >= 40 {
			return byRate("Supravent")
		}
		return "Vent Brady"
	case RIdioventricular:
		return "Vent Rhythm"
	case RVTach:
		return "V-Tach"
	case RTorsades, RVFibCoarse, RVFibFine:
		return "V-Fib/V-Tach"
	case RPaced:
		return "Vent Paced"
	case RAPaced:
		return "Atrial Paced"
	case RAVPaced:
		return "A-V Paced"
	}
	return ""
}

func displayName(p *Patient) string {
	if p.Cfg.Name == "" {
		return "Not Admitted"
	}
	return p.Cfg.Name
}

// statusLabel is the text shown on the ECG: an arrhythmia status message such
// as "Irregular HR" (bold) while one is active, otherwise the rhythm label.
func statusLabel(p *Patient) (string, bool) {
	if !showAlarms || p.Cfg.CardiacOff {
		return "", false
	}
	if p.AlarmLvl == 1 && p.AlarmText != "" {
		return strings.TrimLeft(p.AlarmText, "* "), true
	}
	if showRhythm {
		return rhythmLabel(p), false
	}
	return "", false
}

// drawNumeric draws a parameter: label and its alarm limits on the left,
// large value on the right.
func drawNumeric(dc uintptr, r RECT, label, val string, c uint32, hi, lo string) {
	nh := float64(r.Bottom - r.Top)
	nw := float64(r.Right - r.Left)
	lab := int32(clamp(nh*0.20, float64(S(10)), float64(S(16))))
	text(dc, label, r.Left+S(4), r.Top+S(2), r.Right, r.Top+S(2)+lab*2, c, font(lab, true, "Segoe UI"), DT_SINGLELINE)
	lim := int32(clamp(nh*0.15, float64(S(9)), float64(S(13))))
	ly := r.Top + S(4) + lab
	if hi != "" && nh > float64(lab+2*lim+S(6)) {
		text(dc, hi, r.Left+S(6), ly, r.Left+S(60), ly+lim+S(2), c, font(lim, false, "Segoe UI"), DT_SINGLELINE)
		text(dc, lo, r.Left+S(6), ly+lim+S(1), r.Left+S(60), ly+2*lim+S(3), c, font(lim, false, "Segoe UI"), DT_SINGLELINE)
	}
	big := int32(math.Min(nh*0.72, nw*0.36))
	text(dc, val, r.Left, r.Top, r.Right-S(6), r.Bottom, c, font(big, false, "Arial"), DT_SINGLELINE|DT_VCENTER|DT_RIGHT)
}

// flatButton draws a flat grey push button in the central-station style.
func flatButton(dc uintptr, r RECT, label string, hot bool, fh int32) {
	bg, edge := rgb(98, 98, 98), rgb(170, 170, 170)
	if hot {
		bg = rgb(132, 132, 132)
	}
	fill(dc, r.Left, r.Top, r.Right, r.Bottom, edge)
	fill(dc, r.Left+1, r.Top+1, r.Right-1, r.Bottom-1, bg)
	text(dc, label, r.Left+S(2), r.Top, r.Right-S(2), r.Bottom, rgb(235, 235, 235), font(fh, false, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_CENTER|DT_END_ELLIPSIS)
}

func bgFor(hot bool) uint32 {
	if hot {
		return rgb(230, 230, 230)
	}
	return rgb(196, 196, 196)
}

// expandedNow is set while drawing buttons for the bed that is expanded.
var expandedNow bool

// iconButton draws a light-grey square button holding a glyph.
func iconButton(dc uintptr, r RECT, kind int, hot bool) {
	bg := bgFor(hot)
	fill(dc, r.Left, r.Top, r.Right, r.Bottom, rgb(120, 120, 120))
	fill(dc, r.Left+1, r.Top+1, r.Right-1, r.Bottom-1, bg)
	w, h := float64(r.Right-r.Left), float64(r.Bottom-r.Top)
	dark := pen(rgb(40, 40, 40), int32(math.Max(1, math.Round(1.5*dpiScale))))
	switch kind {
	case bStrip: // paper strip with a beat on it
		fill(dc, r.Left+int32(w*0.18), r.Top+int32(h*0.28), r.Right-int32(w*0.18), r.Bottom-int32(h*0.28), rgb(255, 255, 255))
		shape := []float64{0, 0, 0.1, 0, -0.1, 1, -0.35, 0, 0.15, 0, 0}
		pts := make([]POINT, len(shape))
		for i, v := range shape {
			pts[i] = POINT{r.Left + int32(w*0.22+w*0.56*float64(i)/float64(len(shape)-1)), r.Top + int32(h*0.60-v*h*0.25)}
		}
		polyline(dc, pts, dark)
	case bExpand: // a window with its lower part filled: "show at the bottom"
		l, t := r.Left+int32(w*0.2), r.Top+int32(h*0.25)
		rr, b := r.Right-int32(w*0.2), r.Bottom-int32(h*0.25)
		fill(dc, l, t, rr, b, rgb(40, 40, 40))
		fill(dc, l+S(2), t+S(2), rr-S(2), b-S(2), bgFor(hot))
		mid := t + (b-t)/2
		if expandedNow { // collapse: filled top half
			fill(dc, l+S(2), t+S(2), rr-S(2), mid, rgb(40, 40, 40))
		} else {
			fill(dc, l+S(2), mid, rr-S(2), b-S(2), rgb(40, 40, 40))
		}
	case bSilence: // bell with a slash
		cx := float64(r.Left) + w/2
		top := float64(r.Top) + h*0.22
		bot := float64(r.Top) + h*0.68
		pts := []POINT{
			{int32(cx - w*0.24), int32(bot)}, {int32(cx - w*0.17), int32(bot - h*0.08)},
			{int32(cx - w*0.15), int32(top + h*0.10)}, {int32(cx - w*0.07), int32(top)},
			{int32(cx + w*0.07), int32(top)}, {int32(cx + w*0.15), int32(top + h*0.10)},
			{int32(cx + w*0.17), int32(bot - h*0.08)}, {int32(cx + w*0.24), int32(bot)},
			{int32(cx - w*0.24), int32(bot)},
		}
		polyline(dc, pts, dark)
		fill(dc, int32(cx-w*0.05), int32(bot+h*0.03), int32(cx+w*0.05), int32(bot+h*0.11), rgb(40, 40, 40))
		line(dc, r.Left+int32(w*0.22), r.Bottom-int32(h*0.18), r.Right-int32(w*0.22), r.Top+int32(h*0.18), pen(rgb(200, 0, 0), S(2)))
	}
}

func drawMain(dc uintptr, w, h int32) {
	topH, botH := S(26), S(22)
	// separators between sectors
	fill(dc, 0, 0, w, h, cSepLine)

	// top status bar (light, as on the information centre)
	fill(dc, 0, 0, w, topH, rgb(58, 58, 62)) // dark grey, as on the information centre
	fill(dc, 0, topH-1, w, topH, rgb(110, 110, 115))
	dark := rgb(235, 235, 235)
	text(dc, "  Central Station", 0, 0, w/3, topH, dark, font(S(15), true, "Segoe UI"), DT_SINGLELINE|DT_VCENTER)
	text(dc, "Telemetry Unit    Status: Connected", S(150), 0, w/2-S(70), topH, rgb(180, 180, 185), font(S(13), false, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	text(dc, time.Now().Format("Monday, January 2, 2006  15:04:05")+"  ", w/2, 0, w, topH, dark, font(S(13), false, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_RIGHT)
	silenceAll = RECT{}
	if anyAudible {
		bw := S(120)
		silenceAll = RECT{w/2 - bw/2, S(2), w/2 + bw/2, topH - S(3)}
		flatButton(dc, silenceAll, "Silence All", hoverSilAll, S(13))
	}
	snd := "Sound: ON"
	if !soundOn {
		snd = "Sound: OFF"
	}
	ix := w/2 + S(70)
	text(dc, snd, ix, 0, ix+S(90), topH, rgb(180, 180, 185), font(S(12), false, "Segoe UI"), DT_SINGLELINE|DT_VCENTER)
	text(dc, fmt.Sprintf("%g mm/s", sweepSpeed), ix+S(90), 0, ix+S(170), topH, rgb(180, 180, 185), font(S(12), false, "Segoe UI"), DT_SINGLELINE|DT_VCENTER)
	if showFPS {
		text(dc, fmt.Sprintf("%.0f fps", fpsVal), ix+S(170), 0, ix+S(240), topH, rgb(255, 200, 90), font(S(12), true, "Segoe UI"), DT_SINGLELINE|DT_VCENTER)
	}

	// bottom hint bar
	fill(dc, 0, h-botH, w, h, rgb(34, 34, 34))
	hint := "  Hover over a patient for options  •  Double-click a patient to silence its alarm  •  Click the lead label to change lead  •  Training simulator - not a medical device"
	text(dc, hint, 0, h-botH, w, h, rgb(165, 165, 165), font(S(12), false, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)

	cols := colsSet
	if cols == 0 {
		cols = 2
		if nBeds <= 4 {
			cols = 1
		}
	}
	if cols > nBeds {
		cols = nBeds
	}
	rows := (nBeds + cols - 1) / cols
	gy0, gy1 := topH, h-botH
	if expandedBed >= nBeds || (expandedBed >= 0 && standby[expandedBed]) {
		expandedBed = -1
	}
	var panel RECT
	if expandedBed >= 0 { // expanded patient docked across the bottom
		ph := int32(clamp(float64(gy1-gy0)*0.44, float64(S(230)), float64(gy1-gy0)*0.62))
		panel = RECT{0, gy1 - ph, w, gy1}
		gy1 = panel.Top - S(2)
	}
	cw := float64(w) / float64(cols)
	ch := float64(gy1-gy0) / float64(rows)
	sectors = sectors[:0]
	t := now()
	for i := 0; i < nBeds; i++ {
		c, r := i/rows, i%rows
		l := int32(float64(c) * cw)
		rr := int32(float64(c+1) * cw)
		tp := gy0 + int32(float64(r)*ch)
		bt := gy0 + int32(float64(r+1)*ch)
		drawSector(dc, i, RECT{l, tp, rr, bt}, t)
	}
	if expandedBed >= 0 {
		fill(dc, 0, gy1, w, panel.Top, rgb(150, 150, 150))
		drawExpanded(dc, expandedBed, panel, t)
	}
}

// alarmBox draws an alarm message box (right-aligned, ending at right).
func alarmBox(dc uintptr, right, top, bottom, maxW int32, msg string, lvl int, silenced, blink bool, fh int32) {
	f := font(fh, true, "Segoe UI")
	tw := textWidth(dc, msg, f) + S(16)
	if tw > maxW {
		tw = maxW
	}
	if tw < S(40) {
		return
	}
	l := right - tw
	var bg, fg uint32
	switch lvl {
	case 3, AlarmPanic:
		bg, fg = rgb(225, 0, 0), rgb(255, 255, 255)
		if lvl == AlarmPanic && !silenced && !blink {
			bg = rgb(130, 0, 0)
		}
	case 2:
		bg, fg = rgb(255, 255, 0), rgb(0, 0, 0)
	case 4:
		bg, fg = rgb(0, 238, 255), rgb(0, 0, 0)
	default: // short yellow: text only
		text(dc, msg, l, top, right-S(6), bottom, rgb(255, 255, 0), f, DT_SINGLELINE|DT_VCENTER|DT_RIGHT|DT_END_ELLIPSIS)
		return
	}
	roundRect(dc, l, top+S(1), right-S(1), bottom-S(1), S(7), bg, bg) // slightly rounded, as on the monitor
	text(dc, msg, l+S(6), top, right-S(6), bottom, fg, f, DT_SINGLELINE|DT_VCENTER|DT_CENTER|DT_END_ELLIPSIS)
}

func drawSector(dc uintptr, bed int, rc RECT, t float64) {
	p := patients[bed]
	in := RECT{rc.Left + 1, rc.Top + 1, rc.Right, rc.Bottom}
	sw, sh := float64(in.Right-in.Left), float64(in.Bottom-in.Top)
	idx := len(sectors) // this sector's index once appended
	hovered := hoverSec == idx

	alarmOn := showAlarms && !standby[bed] && p.AlarmText != ""
	isSil := alarmOn && silenced[bed] != ""
	newAlarm := alarmOn && !isSil && p.AlarmLvl != 1
	blink := int(t*4)%2 == 0

	bg := rgb(0, 0, 0)
	if newAlarm {
		bg = cAlarmBg
	}
	fill(dc, in.Left, in.Top, in.Right, in.Bottom, bg)

	// header metrics
	l1 := int32(clamp(sh*0.12, float64(S(15)), float64(S(22))))
	fh := int32(float64(l1) * 0.78)
	l2 := int32(float64(l1) * 0.95)
	geom := sectorGeom{bed: bed, rc: in}

	if standby[bed] {
		drawStandbyBody(dc, bed, in, l1, fh, hovered, &geom, idx)
		drawHeader(dc, bed, in, l1, l2, fh, true)
		finishSector(dc, rc, hovered, geom)
		return
	}

	// ---- waves (left) and numerics (right), stacked by what is monitored ----
	ecgOn := !p.Cfg.CardiacOff
	spo2On := p.Cfg.SpO2On != SpO2Off
	respOn := !p.Cfg.RespRateOff
	respWave := p.Cfg.RespWaveShown()
	pulseNum := spo2On && (!ecgOn || p.Cfg.Rhythm == RLeadsOff)

	type slot struct {
		kind int // kHR / kSpO2 / kRR / kPulse / kABP / kNBP / kCO2 (waves: ECG=kHR, Pleth=kSpO2, Resp=kRR)
		wt   float64
	}
	var waves, nums []slot
	if ecgOn {
		waves = append(waves, slot{kHR, 1.7})
		nums = append(nums, slot{kHR, 1.0})
	} else if pulseNum {
		nums = append(nums, slot{kPulse, 1.0})
	}
	if p.Cfg.ABPOn {
		waves = append(waves, slot{kABP, 1.1})
		nums = append(nums, slot{kABP, 1.0})
	}
	// NBP sits below the resp rate (RR above NBP, as on the monitor)
	nbpAfterRR := true
	if p.Cfg.NBPMode != NBPOff && !nbpAfterRR {
		nums = append(nums, slot{kNBP, 1.0})
	}
	if respOn {
		nums = append(nums, slot{kRR, 0.8})
	}
	if p.Cfg.NBPMode != NBPOff && nbpAfterRR {
		nums = append(nums, slot{kNBP, 1.0})
	}
	if spo2On {
		waves = append(waves, slot{kSpO2, 1.0})
		nums = append(nums, slot{kSpO2, 0.9})
	}
	if ecgOn && pulseNum {
		nums = append(nums, slot{kPulse, 0.8})
	}
	if p.Cfg.CO2On {
		waves = append(waves, slot{kCO2, 0.9})
		nums = append(nums, slot{kCO2, 0.9})
	}
	if respWave {
		waves = append(waves, slot{kRR, 0.85})
	}
	// more than three numerics: lay them out in two columns
	ncols := 1
	if len(nums) > 3 {
		ncols = 2
	}
	numW := int32(clamp(sw*0.27, float64(S(96)), float64(S(230))))
	if ncols == 2 {
		numW = int32(clamp(sw*0.44, float64(S(160)), float64(S(400))))
	}
	waveTop := in.Top + l1 + l2/2
	wave := RECT{in.Left, waveTop, in.Right - numW, in.Bottom - S(2)}
	num := RECT{in.Right - numW, in.Top + l1 + S(2), in.Right, in.Bottom - S(2)}
	split := func(r RECT, ss []slot) []RECT {
		tot := 0.0
		for _, x := range ss {
			tot += x.wt
		}
		out := make([]RECT, len(ss))
		y := float64(r.Top)
		for i, x := range ss {
			h := float64(r.Bottom-r.Top) * x.wt / tot
			out[i] = RECT{r.Left, int32(y), r.Right, int32(y + h)}
			y += h
		}
		return out
	}
	lf := font(int32(clamp(float64(fh)*0.95, float64(S(10)), float64(S(15)))), true, "Segoe UI")
	labW := S(30) // room at the left of each wave for its label / cal bar
	wr := split(wave, waves)
	for i, w := range waves {
		r := wr[i]
		tr := RECT{r.Left + labW, r.Top, r.Right, r.Bottom} // trace area
		switch w.kind {
		case 0:
			lead := p.Cfg.LeadName()
			lt := in.Top + l1 + l2 // below the patient name
			geom.lead = RECT{r.Left + S(3), lt, r.Left + S(3) + textWidth(dc, lead, lf) + S(8), lt + int32(float64(fh)*1.25)}
			if hovered {
				fill(dc, geom.lead.Left, geom.lead.Top, geom.lead.Right, geom.lead.Bottom, rgb(0, 70, 0))
			}
			text(dc, lead, geom.lead.Left+S(3), geom.lead.Top, geom.lead.Right, geom.lead.Bottom, cGreen, lf, DT_SINGLELINE|DT_VCENTER)
			// 1 mV calibration bar
			wh := float64(tr.Bottom - tr.Top)
			yb := float64(tr.Top) + wh*0.60
			mv := wh * 0.34
			if yb-mv > float64(geom.lead.Bottom+S(2)) {
				bx := r.Left + S(14)
				fill(dc, bx, int32(yb-mv), bx+S(3), int32(yb), cGreen)
				if wh > float64(S(60)) { // "1 mV" under the bar, clear of the trace
					text(dc, "1 mV", r.Left+S(2), int32(yb)+S(2), tr.Left, int32(yb)+S(15), cGreen, font(S(10), false, "Segoe UI"), DT_SINGLELINE|DT_CENTER)
				}
			}
			drawSweep(dc, p, tr)
			if lbl, bold := statusLabel(p); lbl != "" {
				// on the ECG, a little right of centre (as on the monitor), clear of the name
				rf := font(int32(float64(fh)*0.9), bold, "Segoe UI")
				nameEnd := in.Left + S(4) + textWidth(dc, displayName(p), font(int32(float64(fh)*0.95), true, "Segoe UI")) + S(10)
				lw := textWidth(dc, lbl, rf)
				x := tr.Left + int32(float64(tr.Right-tr.Left)*0.60) - lw/2
				if x+lw > tr.Right-S(4) {
					x = tr.Right - S(4) - lw
				}
				if x < nameEnd {
					x = nameEnd
				}
				ly := in.Top + l1
				text(dc, lbl, x, ly, tr.Right, ly+l2, cWhite, rf, DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
			}
		case 1:
			text(dc, "Pleth", r.Left+S(3), r.Top+S(1), r.Left+S(80), r.Top+S(1)+fh+S(4), cCyan, lf, DT_SINGLELINE)
			drawPleth(dc, p, tr)
		case 2:
			text(dc, "Resp", r.Left+S(3), r.Top+S(1), r.Left+S(80), r.Top+S(1)+fh+S(4), cPurple, lf, DT_SINGLELINE)
			drawResp(dc, p, tr)
		case kABP:
			text(dc, "ABP", r.Left+S(3), r.Top+S(1), r.Left+S(80), r.Top+S(1)+fh+S(4), cRed, lf, DT_SINGLELINE)
			drawABP(dc, p, r, tr)
		case kCO2:
			text(dc, "CO\u2082", r.Left+S(3), r.Top+S(1), r.Left+S(80), r.Top+S(1)+fh+S(4), cYellow, lf, DT_SINGLELINE)
			drawCO2(dc, p, r, tr)
		}
	}
	if len(waves) == 0 {
		text(dc, "No waveforms displayed", wave.Left, wave.Top, wave.Right, wave.Bottom, rgb(90, 90, 100), font(S(13), false, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_CENTER)
	}

	lastNBPRect, nbpHot = RECT{}, hoverNBPSec == idx
	if ncols == 1 {
		nr := split(num, nums)
		for i, n := range nums {
			drawParamNumeric(dc, nr[i], n.kind, p)
		}
	} else {
		// Column 1: each numeric that has a waveform, level with that wave
		// (HR beside the ECG, SpO2 beside the Pleth, ...). Column 2: the rest.
		mid := num.Left + (num.Right-num.Left)/2
		used := map[int]bool{}
		var col1 []RECT
		var col1k []int
		has := map[int]bool{}
		for _, n := range nums {
			has[n.kind] = true
		}
		for i, w := range waves {
			if !has[w.kind] || used[w.kind] {
				continue
			}
			r := wr[i]
			top := r.Top
			if len(col1) == 0 {
				top = num.Top
			}
			col1 = append(col1, RECT{num.Left, top, mid, r.Bottom})
			col1k = append(col1k, w.kind)
			used[w.kind] = true
		}
		var rest []slot
		for _, n := range nums {
			if !used[n.kind] {
				rest = append(rest, n)
			}
		}
		if len(col1) == 0 { // nothing to align: plain two columns
			half := (len(nums) + 1) / 2
			c1 := split(RECT{num.Left, num.Top, mid, num.Bottom}, nums[:half])
			for i, n := range nums[:half] {
				drawParamNumeric(dc, c1[i], n.kind, p)
			}
			rest = nums[half:]
		} else {
			col1[len(col1)-1].Bottom = num.Bottom
			for i, k := range col1k {
				drawParamNumeric(dc, col1[i], k, p)
			}
		}
		c2 := split(RECT{mid, num.Top, num.Right, num.Bottom}, rest)
		for i, n := range rest {
			drawParamNumeric(dc, c2[i], n.kind, p)
		}
	}
	geom.nbp = lastNBPRect
	if len(nums) == 0 {
		text(dc, "Not monitored", num.Left, num.Top, num.Right, num.Bottom, rgb(90, 90, 100), font(S(12), false, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_CENTER)
	}

	// header (drawn over the top of the waves, as on the real screen)
	drawHeader(dc, bed, in, l1, l2, fh, false)
	if alarmOn && p.AlarmLvl != 1 { // short-yellow messages (Irregular HR) sit on the ECG instead
		msg := p.AlarmText
		if isSil {
			msg = "✓ " + msg // acknowledged
		}
		alarmBox(dc, in.Right, in.Top, in.Top+l1, int32(sw*0.62), msg, p.AlarmLvl, isSil, blink, fh)
	}

	// hover buttons
	if hovered {
		var specs []int
		if ecgOn {
			specs = append(specs, bStrip)
		}
		specs = append(specs, bRhythm)
		if ecgOn {
			specs = append(specs, bTune)
		}
		specs = append(specs, bLimits)
		if alarmOn && !isSil {
			specs = append(specs, bSilence)
		}
		specs = append(specs, bExpand)
		expandedNow = expandedBed == bed
		geom.btns = layoutButtons(dc, in, specs, p, idx)
	}
	finishSector(dc, rc, hovered, geom)
}

func drawHeader(dc uintptr, bed int, in RECT, l1, l2, fh int32, isStandby bool) {
	p := patients[bed]
	text(dc, strings.ToUpper(bedLabel(bed)), in.Left+S(4), in.Top, in.Left+S(120), in.Top+l1, cWhite, font(fh, true, "Segoe UI"), DT_SINGLELINE|DT_VCENTER)
	name := displayName(p)
	nameW := int32(float64(in.Right-in.Left) * 0.45)
	text(dc, name, in.Left+S(4), in.Top+l1, in.Left+S(4)+nameW, in.Top+l1+l2, cWhite, font(int32(float64(fh)*0.95), true, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
}

func drawStandbyBody(dc uintptr, bed int, in RECT, l1, fh int32, hovered bool, geom *sectorGeom, idx int) {
	body := RECT{in.Left, in.Top + 2*l1, in.Right, in.Bottom}
	bh := float64(body.Bottom - body.Top)
	big := int32(clamp(bh*0.22, float64(S(13)), float64(S(30))))
	text(dc, "Standby", body.Left, body.Top, body.Right, body.Top+int32(bh*0.55), rgb(150, 150, 160), font(big, false, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_CENTER)
	if hovered {
		geom.btns = layoutButtons(dc, in, []int{bActivate}, patients[bed], idx)
	}
}

// layoutButtons draws the hover buttons centred across the sector, sized to fit.
func layoutButtons(dc uintptr, in RECT, specs []int, p *Patient, sec int) []hbtn {
	sw, sh := float64(in.Right-in.Left), float64(in.Bottom-in.Top)
	bh := int32(clamp(sh*0.18, float64(S(20)), float64(S(34))))
	gap := S(10)
	textW := int32(clamp(sw*0.17, float64(S(70)), float64(S(122))))
	total := int32(0)
	for _, k := range specs {
		if k == bStrip || k == bSilence || k == bExpand {
			total += bh
		} else {
			total += textW
		}
	}
	total += gap * int32(len(specs)-1)
	avail := int32(sw) - S(16)
	if total > avail { // shrink text buttons to fit narrow sectors
		over := total - avail
		ntext := int32(0)
		for _, k := range specs {
			if k != bStrip && k != bSilence && k != bExpand {
				ntext++
			}
		}
		if ntext > 0 {
			textW -= (over + ntext - 1) / ntext
		}
		if textW < S(44) {
			textW = S(44)
		}
		total = 0
		for _, k := range specs {
			if k == bStrip || k == bSilence || k == bExpand {
				total += bh
			} else {
				total += textW
			}
		}
		total += gap * int32(len(specs)-1)
	}
	x := in.Left + (int32(sw)-total)/2 // centred on the bottom edge
	y := in.Bottom - S(5) - bh
	fh := int32(clamp(float64(bh)*0.46, float64(S(10)), float64(S(15))))
	bf := font(fh, false, "Segoe UI")
	fit := func(long, short string) string { // short label if the long one won't fit
		if textWidth(dc, long, bf)+S(10) <= textW {
			return long
		}
		return short
	}
	var out []hbtn
	for _, k := range specs {
		w := textW
		if k == bStrip || k == bSilence || k == bExpand {
			w = bh
		}
		r := RECT{x, y, x + w, y + bh}
		hot := hoverBtnSec == sec && hoverBtnKind == k
		switch k {
		case bStrip, bSilence, bExpand:
			iconButton(dc, r, k, hot)
		case bRhythm:
			flatButton(dc, r, fit("Adjust Rhythm", "Rhythm"), hot, fh)
		case bTune:
			flatButton(dc, r, fit("Fine Tune", "Tune"), hot, fh)
			if p.Cfg.Tune.UserSet {
				fill(dc, r.Right-S(7), r.Top+S(3), r.Right-S(3), r.Top+S(7), cCyan)
			}
		case bLimits:
			flatButton(dc, r, fit("Alarm Limits", "Limits"), hot, fh)
			if p.Lim != nil {
				fill(dc, r.Right-S(7), r.Top+S(3), r.Right-S(3), r.Top+S(7), rgb(255, 200, 0))
			}
		case bActivate:
			flatButton(dc, r, "Activate Bed", hot, fh)
		}
		out = append(out, hbtn{r, k})
		x += w + gap
	}
	return out
}

func finishSector(dc uintptr, rc RECT, hovered bool, geom sectorGeom) {
	if geom.bed == expandedBed && !hovered && rc.Bottom-rc.Top < S(400) { // grid sector of the docked patient
		c := rgb(200, 200, 200)
		fill(dc, rc.Left+1, rc.Top+1, rc.Right, rc.Top+2, c)
		fill(dc, rc.Left+1, rc.Bottom-1, rc.Right, rc.Bottom, c)
		fill(dc, rc.Left+1, rc.Top+1, rc.Left+2, rc.Bottom, c)
		fill(dc, rc.Right-1, rc.Top+1, rc.Right, rc.Bottom, c)
	}
	if hovered { // highlight frame on the sector under the mouse
		b := S(2)
		fill(dc, rc.Left+1, rc.Top+1, rc.Right, rc.Top+1+b, cHoverBdr)
		fill(dc, rc.Left+1, rc.Bottom-b, rc.Right, rc.Bottom, cHoverBdr)
		fill(dc, rc.Left+1, rc.Top+1, rc.Left+1+b, rc.Bottom, cHoverBdr)
		fill(dc, rc.Right-b, rc.Top+1, rc.Right, rc.Bottom, cHoverBdr)
	}
	sectors = append(sectors, geom)
}

// chooseNBP pops up the NBP interval / mode menu under the NBP title.
func chooseNBP(hwnd uintptr, bed int, r RECT) {
	p := patients[bed]
	m := call(pCreatePopupMenu)
	auto := p.Cfg.NBPMode == NBPAuto
	for i, mins := range NBPIntervals {
		f := uintptr(MF_STRING)
		if auto && mins == nbpMinutes(p.Cfg) {
			f |= MF_CHECKED
		}
		call(pAppendMenuW, m, f, uintptr(2000+i), uintptr(unsafe.Pointer(wstr("Auto "+NBPIntervalLabel(mins)))))
	}
	call(pAppendMenuW, m, MF_SEPARATOR, 0, 0)
	f := uintptr(MF_STRING)
	if p.Cfg.NBPMode == NBPManual {
		f |= MF_CHECKED
	}
	call(pAppendMenuW, m, f, 2100, uintptr(unsafe.Pointer(wstr("Manual"))))
	call(pAppendMenuW, m, MF_SEPARATOR, 0, 0)
	call(pAppendMenuW, m, MF_STRING, 2101, uintptr(unsafe.Pointer(wstr("Measure NBP now"))))
	call(pAppendMenuW, m, MF_STRING, 2102, uintptr(unsafe.Pointer(wstr("Turn NBP off"))))
	pt := POINT{r.Left, r.Bottom}
	call(pClientToScreen, hwnd, uintptr(unsafe.Pointer(&pt)))
	id := int(call(pTrackPopupMenu, m, 0x0100|0x0080, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0))
	call(pDestroyMenu, m)
	t := now()
	switch {
	case id >= 2000 && id < 2000+len(NBPIntervals):
		p.SetNBP(NBPAuto, NBPIntervals[id-2000], t)
	case id == 2100:
		p.SetNBP(NBPManual, p.Cfg.NBPInterval, t)
	case id == 2101:
		p.StartNBP(t)
	case id == 2102:
		p.SetNBP(NBPOff, p.Cfg.NBPInterval, t)
	}
}

// textWidth measures a single line of text in the given font.
func textWidth(dc uintptr, s string, f uintptr) int32 {
	u := utf16.Encode([]rune(s))
	if len(u) == 0 {
		return 0
	}
	call(pSelectObject, dc, f)
	var sz [2]int32
	call(pGetTextExtentPoint32W, dc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)), uintptr(unsafe.Pointer(&sz[0])))
	return sz[0]
}

// handleSectorClick performs a hover-button action. Returns true if handled.
func handleSectorClick(hwnd uintptr, x, y int32) bool {
	for _, s := range sectors {
		if !s.rc.contains(x, y) {
			continue
		}
		for _, b := range s.btns {
			if !b.rc.contains(x, y) {
				continue
			}
			switch b.kind {
			case bStrip:
				openStrip(s.bed)
			case bRhythm, bActivate:
				openSetup(s.bed)
			case bTune:
				openTune(s.bed)
			case bLimits:
				openLimits(s.bed)
			case bSilence:
				silenceBed(s.bed)
			case bExpand:
				if expandedBed == s.bed {
					expandedBed = -1
				} else {
					expandedBed = s.bed
				}
			}
			return true
		}
		if s.lead.contains(x, y) {
			chooseLead(hwnd, s.bed, s.lead)
			return true
		}
		if s.nbp.contains(x, y) {
			chooseNBP(hwnd, s.bed, s.nbp)
			return true
		}
	}
	return false
}
