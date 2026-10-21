//go:build windows

package main

// Expanded patient view: docked across the bottom of the board at full width,
// showing every available wave (three ECG leads, Pleth, Resp) and every
// measurement (HR, PVC/min, ST, QT/QTc, SpO2, Pulse, RR) for one patient.

import (
	"strings"
)

// expandedLeads: the displayed lead first, then two of the recorded extras.
func expandedLeads(p *Patient) []string {
	out := []string{p.Cfg.LeadName()}
	for _, n := range ExtraLeads {
		if len(out) == 3 {
			break
		}
		if n != out[0] {
			out = append(out, n)
		}
	}
	return out
}

func drawExpanded(dc uintptr, bed int, rc RECT, t float64) {
	p := patients[bed]
	in := RECT{rc.Left, rc.Top, rc.Right, rc.Bottom}
	idx := len(sectors)
	hovered := hoverSec == idx
	sw := float64(in.Right - in.Left)
	curHits = nil

	alarmOn := showAlarms && p.AlarmText != ""
	aText, aLvl, isSil := shownAlarm(bed)
	alarmOn = alarmOn || (showAlarms && isSil)
	isSil = alarmOn && isSil
	// INOPs (technical) don't turn the sector blue; a latched short-yellow
	// arrhythmia message only does while its condition is present
	newAlarm := alarmOn && !isSil && p.AlarmLvl != 4 && (p.AlarmLvl != 1 || p.AlarmFresh)
	blink := int(t*4)%2 == 0
	bg := rgb(0, 0, 0)
	if newAlarm {
		bg = cAlarmBg
	}
	fill(dc, in.Left, in.Top, in.Right, in.Bottom, bg)
	geom := sectorGeom{bed: bed, rc: in}

	// header: bed, name, alarm box
	l1 := S(24)
	fh := int32(float64(l1) * 0.72)
	hf := font(fh, true, "Segoe UI")
	text(dc, strings.ToUpper(bedLabel(bed)), in.Left+S(6), in.Top, in.Left+S(90), in.Top+l1, cWhite, hf, DT_SINGLELINE|DT_VCENTER)
	text(dc, displayName(p), in.Left+S(90), in.Top, in.Left+int32(sw*0.4), in.Top+l1, cWhite, hf, DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	drawAlarmArea(dc, p, in.Right, in.Top, in.Top+l1, int32(sw*0.36), alarmOn, aText, aLvl, isSil, blink, fh)
	geom.alarm = lastAlarmRect

	ecgOn := !p.Cfg.CardiacOff
	spo2On := p.Cfg.SpO2On != SpO2Off
	respOn := !p.Cfg.RespRateOff // in the expanded view the resp wave is always shown when resp is monitored

	// ---- measurements: up to three columns (ECG / pressures / gas & SpO2) ----
	var colA, colB, colC []int
	if ecgOn {
		colA = []int{kHR, kPVCnum, kST, kQT}
	}
	if p.Cfg.ABPOn {
		colB = append(colB, kABP)
	}
	if p.Cfg.NBPMode != NBPOff {
		colB = append(colB, kNBP)
	}
	if spo2On {
		colC = append(colC, kSpO2, kPulse)
	}
	if respOn {
		colC = append(colC, kRR)
	}
	if p.Cfg.CO2On {
		colC = append(colC, kCO2, kAwRR)
	}
	var cols [][]int
	for _, c := range [][]int{colA, colB, colC} {
		if len(c) > 0 {
			cols = append(cols, c)
		}
	}
	colW := clamp(sw*0.13, float64(S(150)), float64(S(250)))
	numW := int32(colW * float64(max(len(cols), 1)))

	// ---- waves ----
	wave := RECT{in.Left, in.Top + l1 + S(2), in.Right - numW, in.Bottom - S(4)}
	type wv struct {
		kind int // kHR = ECG lead, kSpO2 = Pleth, kRR = Resp, kABP, kCO2
		lead string
		wt   float64
	}
	var ws []wv
	if ecgOn {
		for _, l := range expandedLeads(p) {
			ws = append(ws, wv{kHR, l, 1})
		}
	}
	if p.Cfg.ABPOn {
		ws = append(ws, wv{kABP, "", 1})
	}
	if spo2On {
		ws = append(ws, wv{kSpO2, "", 0.8})
	}
	if p.Cfg.CO2On {
		ws = append(ws, wv{kCO2, "", 0.8})
	}
	if respOn {
		ws = append(ws, wv{kRR, "", 0.8})
	}
	tot := 0.0
	for _, x := range ws {
		tot += x.wt
	}
	lf := font(S(13), true, "Segoe UI")
	labW := S(42)
	y := float64(wave.Top)
	firstECG := true
	for _, x := range ws {
		h := float64(wave.Bottom-wave.Top) * x.wt / tot
		r := RECT{wave.Left, int32(y), wave.Right, int32(y + h)}
		y += h
		tr := RECT{r.Left + labW, r.Top, r.Right, r.Bottom}
		curHits = append(curHits, mhit{tr, measOfKind(x.kind)})
		switch x.kind {
		case kHR:
			lr := RECT{r.Left + S(4), r.Top + S(1), r.Left + S(4) + textWidth(dc, x.lead, lf) + S(8), r.Top + S(18)}
			isFirst := firstECG
			if firstECG {
				geom.lead = lr // the displayed lead can be changed from here too
				if hovered {
					fill(dc, lr.Left, lr.Top, lr.Right, lr.Bottom, rgb(0, 70, 0))
				}
				firstECG = false
			}
			text(dc, x.lead, lr.Left+S(3), lr.Top, lr.Right, lr.Bottom, cGreen, lf, DT_SINGLELINE|DT_VCENTER)
			wh := float64(tr.Bottom - tr.Top)
			yb := float64(tr.Top) + wh*0.60
			mv := wh * 0.34
			if yb-mv > float64(lr.Bottom+S(2)) {
				bx := r.Left + S(18)
				fill(dc, bx, int32(yb-mv), bx+S(3), int32(yb), cGreen)
				text(dc, "1 mV", r.Left+S(2), int32(yb)+S(2), tr.Left, int32(yb)+S(15), cGreen, font(S(10), false, "Segoe UI"), DT_SINGLELINE|DT_CENTER)
			}
			drawSweepLead(dc, p, tr, x.lead)
			if lbl, bold := statusLabel(p); isFirst && lbl != "" {
				rf := font(S(14), bold, "Segoe UI")
				lw := textWidth(dc, lbl, rf)
				lx := tr.Left + int32(float64(tr.Right-tr.Left)*0.60) - lw/2
				text(dc, lbl, lx, r.Top+S(1), lx+lw+S(4), r.Top+S(20), cWhite, rf, DT_SINGLELINE|DT_VCENTER)
			}
		case kABP:
			text(dc, "ABP", r.Left+S(4), r.Top+S(1), r.Left+S(80), r.Top+S(18), cRed, lf, DT_SINGLELINE)
			drawABP(dc, p, r, tr)
		case kSpO2:
			text(dc, "Pleth", r.Left+S(4), r.Top+S(1), r.Left+S(80), r.Top+S(18), cCyan, lf, DT_SINGLELINE)
			drawPleth(dc, p, tr)
		case kCO2:
			text(dc, "CO\u2082", r.Left+S(4), r.Top+S(1), r.Left+S(80), r.Top+S(18), cYellow, lf, DT_SINGLELINE)
			drawCO2(dc, p, r, tr)
		case kRR:
			text(dc, "Resp", r.Left+S(4), r.Top+S(1), r.Left+S(80), r.Top+S(18), cPurple, lf, DT_SINGLELINE)
			drawResp(dc, p, tr)
		}
	}
	if len(ws) == 0 {
		text(dc, "No waveforms available", wave.Left, wave.Top, wave.Right, wave.Bottom, rgb(90, 90, 100), font(S(14), false, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_CENTER)
	}

	num := RECT{in.Right - numW, in.Top + l1 + S(2), in.Right, in.Bottom - S(4)}
	line(dc, num.Left, num.Top+S(4), num.Left, num.Bottom-S(4), pen(rgb(60, 60, 60), 1))
	if len(cols) == 0 {
		text(dc, "Not monitored", num.Left, num.Top, num.Right, num.Bottom, rgb(90, 90, 100), font(S(13), false, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_CENTER)
	}
	lastNBPRect, nbpHot = RECT{}, hoverNBPSec == idx
	for ci, c := range cols {
		cr := RECT{num.Left + int32(colW*float64(ci)) + S(4), num.Top, num.Left + int32(colW*float64(ci+1)), num.Bottom}
		wt := func(k int) float64 {
			switch k {
			case kHR, kSpO2, kABP, kNBP:
				return 1.3
			}
			return 0.8
		}
		tot := 0.0
		for _, k := range c {
			tot += wt(k)
		}
		yy := float64(cr.Top)
		for _, k := range c {
			h := float64(cr.Bottom-cr.Top) * wt(k) / tot
			drawParamNumeric(dc, RECT{cr.Left, int32(yy), cr.Right, int32(yy + h)}, k, p)
			yy += h
		}
	}

	geom.nbp = lastNBPRect

	// hover buttons (same set as the sector; the expand button collapses)
	if hovered {
		var specs []int
		if ecgOn {
			specs = append(specs, bStrip)
		}
		specs = append(specs, bRhythm, bReview, bMeas, bExpand)
		expandedNow = true
		geom.btns = layoutButtons(dc, in, specs, p, idx)
	}
	finishSector(dc, RECT{rc.Left - 1, rc.Top - 1, rc.Right, rc.Bottom}, hovered, geom)
}
