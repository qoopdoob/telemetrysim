//go:build windows

package main

// Shared drawing for each monitored parameter (numerics and waves), used by
// both the patient sectors and the expanded view.

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// parameter kinds
const (
	kHR = iota
	kSpO2
	kRR
	kPulse
	kABP
	kNBP
	kCO2
	kPVCnum
	kST
	kQT
	kAwRR
)

var (
	lastNBPRect RECT // where the clickable NBP title was last drawn
	nbpHot      bool // mouse is over this sector's NBP title
	cRed        = rgb(255, 48, 48)
	cPink       = rgb(240, 130, 235)
	cYellow     = rgb(255, 255, 0)
)

// drawBP draws a pressure numeric: label (+ small sub-label) at top left,
// "sys/dia" large at the right and "(mean)" beneath it.
func drawBP(dc uintptr, r RECT, label, sub, sysdia, mean string, c uint32) {
	nh := float64(r.Bottom - r.Top)
	nw := float64(r.Right - r.Left)
	lab := int32(clamp(nh*0.20, float64(S(10)), float64(S(16))))
	lf := font(lab, true, "Segoe UI")
	if label == "NBP" { // the NBP title line is clickable (interval menu)
		lastNBPRect = RECT{r.Left + S(1), r.Top + S(1), r.Right - S(1), r.Top + S(4) + lab + lab/2}
		if nbpHot {
			fill(dc, lastNBPRect.Left, lastNBPRect.Top, lastNBPRect.Right, lastNBPRect.Bottom, rgb(80, 30, 80))
		}
	}
	text(dc, label, r.Left+S(4), r.Top+S(2), r.Right, r.Top+S(2)+lab*2, c, lf, DT_SINGLELINE)
	extra := int32(0) // height of a wrapped second label line
	if sub != "" {
		lw := textWidth(dc, label, lf)
		sx := r.Left + S(8) + lw
		avail := r.Right - S(2) - sx
		fs := lab * 9 / 10
		sf := font(fs, false, "Segoe UI")
		for fs > S(10) && textWidth(dc, sub, sf) > avail { // shrink a little first
			fs--
			sf = font(fs, false, "Segoe UI")
		}
		if i := strings.Index(sub, "  "); i > 0 && textWidth(dc, sub, sf) > avail {
			// still too long: time on the title line, "Auto 15 min" on the line below
			sf = font(lab*9/10, false, "Segoe UI")
			text(dc, sub[:i], sx, r.Top+S(2), r.Right-S(2), r.Top+S(2)+lab*2, c, sf, DT_SINGLELINE|DT_END_ELLIPSIS)
			ly := r.Top + S(3) + lab
			text(dc, sub[i+2:], r.Left+S(4), ly, r.Right-S(2), ly+lab+S(2), c, sf, DT_SINGLELINE|DT_END_ELLIPSIS)
			extra = lab
			if label == "NBP" {
				lastNBPRect.Bottom = ly + lab + S(2)
			}
		} else {
			text(dc, sub, sx, r.Top+S(2), r.Right-S(2), r.Top+S(2)+lab*2, c, sf, DT_SINGLELINE|DT_END_ELLIPSIS)
		}
	}
	// "sys/dia (mean)" on one line, e.g. "119/80 (95)"
	line := sysdia
	if mean != "" {
		line += " " + mean
	}
	top := r.Top + lab + S(2) + extra
	big := int32(float64(r.Bottom-top) * 0.70) // then shrink to fit the width
	bf := font(big, false, "Arial")
	for big > S(10) && textWidth(dc, line, bf) > int32(nw*0.95) { // shrink to fit
		big = big * 19 / 20
		bf = font(big, false, "Arial")
	}
	text(dc, line, r.Left, top, r.Right-S(6), r.Bottom, c, bf, DT_SINGLELINE|DT_VCENTER|DT_RIGHT)
}

// nbpTimeLabel is the wall-clock time of the last NBP reading.
func nbpTimeLabel(p *Patient) string {
	ago := time.Duration((now() - p.NBPAt) * float64(time.Second))
	return time.Now().Add(-ago).Format("15:04")
}

// drawParamNumeric draws one numeric box of the given kind.
func drawParamNumeric(dc uintptr, r RECT, kind int, p *Patient) {
	curHits = append(curHits, mhit{r, measOfKind(kind)})
	defer alarmsOffMark(dc, r, kind, p)
	lm := p.lim()
	it := strconv.Itoa
	q := func(ok bool, v int) string {
		if ok {
			return it(v)
		}
		return "-?-"
	}
	switch kind {
	case kHR:
		drawNumeric(dc, r, "HR", q(p.HRValid, p.HR), cGreen, it(lm.HRHigh), it(lm.HRLow))
	case kSpO2:
		drawNumeric(dc, r, "SpO₂", q(p.SpO2Valid, p.SpO2Disp), cCyan, "100", it(lm.SpO2Low))
	case kPulse:
		drawNumeric(dc, r, "Pulse", q(p.SpO2Valid && p.Pulse > 0, p.Pulse), cCyan, it(lm.HRHigh), it(lm.HRLow))
	case kRR:
		drawNumeric(dc, r, "RR", q(p.RRValid, p.RRDisp), cPurple, it(lm.RRHigh), it(lm.RRLow))
	case kAwRR:
		drawNumeric(dc, r, "awRR", q(p.CO2Valid, p.AwRR), cYellow, "", "")
	case kCO2:
		drawNumeric(dc, r, "etCO₂", q(p.CO2Valid, p.EtCO2Disp), cYellow, it(lm.EtCO2High), it(lm.EtCO2Low))
	case kABP:
		if p.ABPValid {
			drawBP(dc, r, "ABP", "", it(p.ABPSys)+"/"+it(p.ABPDia), "("+it(p.ABPMap)+")", cRed)
		} else {
			drawBP(dc, r, "ABP", "", "-?-", "("+it(p.ABPMap)+")", cRed)
		}
	case kNBP:
		switch {
		case p.NBPMeasuring:
			drawBP(dc, r, "NBP", "cuff", it(p.CuffP), "", cPink)
		case p.NBPValid: // e.g. "NBP 17:03 Auto 15 min"
			sub := nbpTimeLabel(p)
			if m := NBPModeLabel(p.Cfg); m != "" {
				sub += "  " + m
			}
			drawBP(dc, r, "NBP", sub, it(p.NBPSys)+"/"+it(p.NBPDia), "("+it(p.NBPMap)+")", cPink)
		default:
			drawBP(dc, r, "NBP", NBPModeLabel(p.Cfg), "-?-", "", cPink)
		}
	case kPVCnum:
		pvc := "-?-"
		if !p.LeadsOff() && p.HRValid {
			pvc = it(p.PVCMin)
		}
		drawNumeric(dc, r, "PVC/min", pvc, cGreen, "10", "")
	case kST:
		st := "-?-"
		if p.stMeasurable() {
			st = signed1(p.STDisplay())
		}
		drawNumeric(dc, r, "ST-"+p.Cfg.LeadName(), st, cGreen, signed1(lm.STHigh), signed1(lm.STLow))
	case kQT:
		qtc, qt := "-?-", ""
		if p.stMeasurable() {
			if a, b := p.QTDisplay(); a > 0 {
				qtc, qt = it(b), "QT "+it(a)
			}
		}
		drawNumeric(dc, r, "QTc", qtc, cGreen, qt, "<"+it(lm.QTcHigh))
	}
}

func signed1(v float64) string {
	s := strconv.FormatFloat(math.Abs(v), 'f', 1, 64)
	if v < 0 {
		return "-" + s
	}
	return "+" + s
}

// dashedH draws a dashed horizontal scale line.
func dashedH(dc uintptr, x0, x1, y int32, c uint32) {
	pn := pen(c, 0)
	for x := x0; x < x1; x += S(8) {
		line(dc, x, y, min(x+S(4), x1), y, pn)
	}
}

// drawABP renders the arterial pressure wave with its scale lines.
func drawABP(dc uintptr, p *Patient, r, tr RECT) {
	top := 150.0
	if p.Cfg.ABPSys > 140 {
		top = 200
	}
	if p.Cfg.ABPSys > 190 {
		top = 250
	}
	wh := float64(tr.Bottom - tr.Top)
	y0 := float64(tr.Bottom) - wh*0.08
	k := wh * 0.82 / top
	sc := rgb(150, 30, 30)
	dashedH(dc, tr.Left, tr.Right, int32(y0-top*k), sc)
	dashedH(dc, tr.Left, tr.Right, int32(y0), sc)
	sf := font(S(10), false, "Segoe UI")
	// scale numbers just inside the trace area, on their lines (clear of the "ABP" label)
	text(dc, strconv.Itoa(int(top)), tr.Left+S(2), int32(y0-top*k)-S(13), tr.Left+S(40), int32(y0-top*k), cRed, sf, DT_SINGLELINE)
	text(dc, "0", tr.Left+S(2), int32(y0)-S(13), tr.Left+S(40), int32(y0), cRed, sf, DT_SINGLELINE)
	sweepTrace(dc, p, tr, func(i int64) float64 { return y0 - float64(p.ABPAt(i))*k }, cRed, sweepSpeed)
}

// drawCO2 renders the capnogram (slow sweep, like respiration).
func drawCO2(dc uintptr, p *Patient, r, tr RECT) {
	top := 50.0
	if p.Cfg.EtCO2 > 45 {
		top = 80
	}
	wh := float64(tr.Bottom - tr.Top)
	y0 := float64(tr.Bottom) - wh*0.08
	k := wh * 0.80 / top
	sc := rgb(120, 120, 0)
	dashedH(dc, tr.Left, tr.Right, int32(y0-top*k), sc)
	sf := font(S(10), false, "Segoe UI")
	text(dc, strconv.Itoa(int(top)), tr.Left+S(2), int32(y0-top*k)-S(13), tr.Left+S(40), int32(y0-top*k), cYellow, sf, DT_SINGLELINE)
	sweepTrace(dc, p, tr, func(i int64) float64 { return y0 - float64(p.CO2At(i))*k }, cYellow, respSpeed())
}

// alarmsOffMark draws a small crossed-out alarm symbol at the top right of a
// numeric whose measurement has its alarms switched off.
func alarmsOffMark(dc uintptr, r RECT, kind int, p *Patient) {
	m := measOfKind(kind)
	if m == MeasArrhy {
		m = MeasECG
	}
	if p.MeasOff&(1<<uint(m)) == 0 {
		return
	}
	x, y, d := r.Right-S(14), r.Top+S(9), S(5)
	pn := pen(rgb(255, 60, 60), S(2))
	line(dc, x-d, y-d, x+d, y+d, pn)
	line(dc, x+d, y-d, x-d, y+d, pn)
}
