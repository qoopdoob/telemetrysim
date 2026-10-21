package main

// Arrhythmia analysis and alarm generation following the Philips IntelliVue
// Instructions for Use (MX100/X3 "Patient Alarms and INOPs" chapter, and the
// MP-series "ECG, Arrhythmia, and ST Monitoring" chapter):
//
//   - Every beat is labelled N (supraventricular), S (premature SV), V
//     (ventricular) or P (paced), like the monitor's beat labels.
//   - Red arrhythmia alarms: Asystole > Vent Fib/Tach > VTach > Extreme
//     Brady/Tachy (latching follows the station's latching setting).
//   - Yellow arrhythmia alarms are "short yellow" (one-star) alarms in three
//     chains (PVC, beat detection, rate). Only the highest alarm in each
//     chain is announced; announcing starts a timeout (TimeOut 1st = 3 min,
//     TimeOut 2nd = 10 min) that blocks the same alarm and those below it.
//     The tone sounds briefly, the message latches for 3 minutes after the
//     condition ends (or clears at once when silenced and the condition is
//     gone). Irregular HR only occurs when no other arrhythmia alarm is.
//   - Rhythm and ectopic status messages are shown on the primary ECG.

import (
	"fmt"
	"math"
	"sort"
)

// Extreme rate clamps (adult factory defaults).
const (
	BradyClamp = 40
	TachyClamp = 200
)

// XBrady / XTachy are the extreme rate limits: the HR low/high limit moved
// by ∆ExtrBrady/∆ExtrTachy, clamped at the Brady/Tachy Clamp.
func (l AlarmLimits) XBrady() int {
	v := l.HRLow - l.ExtBradyD
	if v < BradyClamp {
		v = BradyClamp
	}
	if v > l.HRLow {
		v = l.HRLow
	}
	return v
}

func (l AlarmLimits) XTachy() int {
	v := l.HRHigh + l.ExtTachyD
	if v > TachyClamp {
		v = TachyClamp
	}
	if v < l.HRHigh {
		v = l.HRHigh
	}
	return v
}

// AlarmRank orders alarm levels: red (panic) > red > yellow > short yellow > INOP.
func AlarmRank(lvl int) int {
	switch lvl {
	case AlarmPanic:
		return 5
	case 3:
		return 4
	case 2:
		return 3
	case 1:
		return 2
	case 4:
		return 1
	}
	return 0
}

// AlarmNumeric selects the alarm text setting: Enhanced (the worst value and
// the limit, e.g. "*** Desat 78 < 80") or Standard ("*** Desat").
var AlarmNumeric = true

// Alarm latching (configuration setting): Off, Red Only, or Red & Yellow,
// separately for the visual and the audible alarm indications.
const (
	LatchOff = iota
	LatchRed
	LatchRedYellow
)

// LatchCombos are the combinations the monitor allows (visual, audible).
var LatchCombos = [][2]int{
	{LatchRedYellow, LatchRedYellow}, {LatchRedYellow, LatchRed}, {LatchRedYellow, LatchOff},
	{LatchRed, LatchRed}, {LatchRed, LatchOff}, {LatchOff, LatchOff},
}

var VisualLatch, AudibleLatch = LatchRed, LatchRed

func latches(mode, lvl int) bool {
	switch mode {
	case LatchRed:
		return lvl == 3 || lvl == AlarmPanic
	case LatchRedYellow:
		return lvl == 3 || lvl == AlarmPanic || lvl == 2
	}
	return false
}

// cand is one active alarm condition.
type cand struct {
	text, num string // text form, numeric form ("" = same as text)
	lvl       int
	ord       int  // display priority (higher first)
	fresh     bool // condition present now and not acknowledged (blue sector)
	tone      bool // the alarm (still) sounds
	src       int  // measurement (Meas*)
}

func (c cand) shown() string {
	if AlarmNumeric && c.num != "" {
		return c.num
	}
	return c.text
}

// beat label log entry
type bEntry struct {
	t   float64
	lab byte  // 'N', 'S', 'V', 'P'
	m   uint8 // QRS morphology of V beats (10 = VT, 11 = idioventricular/escape)
}

func (p *Patient) logBeat(t float64, k kind, v uint8) {
	lab, m := byte('N'), uint8(0)
	switch k {
	case kPVC:
		lab, m = 'V', v
	case kVT:
		lab, m = 'V', 10
	case kWide:
		lab, m = 'V', 11
	case kPaced:
		lab = 'P'
	}
	p.blog = append(p.blog, bEntry{t, lab, m})
	if len(p.blog) > 420 {
		p.blog = append(p.blog[:0], p.blog[len(p.blog)-330:]...)
	}
}

// yellow arrhythmia alarm definitions: chain, priority in the chain (0 =
// highest) and which timeout applies.
type arrDef struct{ chain, pri, to int }

// Chains (MX100/X3 IfU "Arrhythmia Alarm Chaining"): 0 PVC, 1 Missed Beat,
// 2 Arrhy HR, 3 AFib/HR. to: 1 = TimeOut 1st, 2 = TimeOut 2nd, 0 = none
// (AFIB and Irregular HR have no timeout period).
var arrDefs = map[string]arrDef{
	"Non-Sustain VT":   {0, 0, 1},
	"Vent Rhythm":      {0, 0, 1},
	"Run PVCs High":    {0, 1, 1},
	"Pair PVCs":        {0, 2, 1},
	"R-on-T PVCs":      {0, 3, 1},
	"Vent Bigeminy":    {0, 4, 2},
	"Vent Trigeminy":   {0, 5, 2},
	"PVCs/min High":    {0, 6, 2},
	"Multiform PVCs":   {0, 7, 2},
	"Pause":            {1, 0, 1},
	"Pacer Not Capt":   {1, 1, 1},
	"Pacer Not Pacing": {1, 1, 1},
	"Missed Beat":      {1, 1, 1},
	"SVT":              {2, 0, 1},
	"HR High":          {2, 1, 1},
	"HR Low":           {2, 1, 1},
	"AFIB":             {3, 0, 0},
	"End AFIB":         {3, 0, 0},
	"Irregular HR":     {3, 1, 0},
	"End Irregular HR": {3, 1, 0},
}

// ArrSwitches are the arrhythmia alarms that can be switched on/off
// individually (Setup Arrhy), with the alarms each switch controls.
var ArrSwitches = []struct {
	Name string
	IDs  []string
}{
	{"Non-Sustain", []string{"Non-Sustain VT"}},
	{"Vent Rhythm", []string{"Vent Rhythm"}},
	{"Run PVCs", []string{"Run PVCs High"}},
	{"Pair PVCs", []string{"Pair PVCs"}},
	{"R-on-T PVCs", []string{"R-on-T PVCs"}},
	{"V.Bigeminy", []string{"Vent Bigeminy"}},
	{"V.Trigeminy", []string{"Vent Trigeminy"}},
	{"Multif.PVCs", []string{"Multiform PVCs"}},
	{"Pacer n.Cap", []string{"Pacer Not Capt"}},
	{"Pacer n.Pac", []string{"Pacer Not Pacing"}},
	{"Pause", []string{"Pause"}},
	{"SVT", []string{"SVT"}},
	{"IrregularHR", []string{"Irregular HR", "End Irregular HR"}},
	{"Missed Beat", []string{"Missed Beat"}},
	{"PVCs/min", []string{"PVCs/min High"}},
	{"Afib", []string{"AFIB", "End AFIB"}},
}

// arrOff reports whether alarm id is switched off in the limits.
func (l AlarmLimits) arrOff(id string) bool {
	for i, sw := range ArrSwitches {
		for _, x := range sw.IDs {
			if x == id && l.ArrOff&(1<<uint(i)) != 0 {
				return true
			}
		}
	}
	return false
}

// HRStandardYellow configures HR High/Low as standard yellow (**) alarms,
// independent of the arrhythmia chains, instead of short yellow (*).
var HRStandardYellow = false

const learnSec = 16.0 // relearn: "Learning ECG", then "Learning Rhythm"

const (
	timeout1st   = 180.0 // TimeOut 1st (3 min)
	timeout2nd   = 600.0 // TimeOut 2nd (10 min)
	latchSec     = 180.0 // yellow arrhythmia alarms latch visually for 3 minutes
	shortToneSec = 6.0   // short yellow: lamp and tone active for six seconds
)

type chainAlarm struct {
	id, num             string
	pri                 int
	announced, lastSeen float64
	silenced            bool
}

type arrState struct {
	chains     [4]chainAlarm
	timeout    map[string]float64
	ect        map[string]float64 // ectopic status: last time seen
	lastScan   float64
	gapRep     float64 // beat after which a pause was already reported
	ronT       float64 // last early (R-on-T) V without a compensatory pause
	afOn       bool    // AFIB / Irregular HR episode in progress (for the "End" alarms)
	irrOn      bool
	afLast     float64 // last time AF / irregular rhythm was detected
	irrLast    float64
	learnStart float64 // start of the current arrhythmia learning phase
	stCat      string  // rhythm status (sinus/SV by rate) with 5-beat hysteresis
	stCand     string
	stCnt      int
}

// resetArrhythmia clears the analysis (e.g. ECG off / leads off / relearn).
func (p *Patient) resetArrhythmia() {
	p.arr = arrState{}
	p.RhythmStatus, p.EctopicStatus = "", ""
}

// relearn clears timeouts and announced arrhythmia alarms, as the monitor
// does when the rhythm is relearned.
// (automatic when ECG monitoring is switched on, the lead is changed, or a
// Leads Off longer than 60 s ends). Timeouts and templates are cleared; for
// ~16 s only Asystole, Vent Fib/Tach and HR alarms are active.
func (p *Patient) relearn() {
	p.resetTimeouts()
	p.learnPending = true
}

// resetTimeouts clears the timeout periods and announced arrhythmia alarms
// (Alarms off and on again). The AFIB / Irregular HR episode state is kept,
// so a conversion still gives its "End" alarm.
func (p *Patient) resetTimeouts() {
	a := p.arr
	p.resetArrhythmia()
	p.arr.afOn, p.arr.afLast, p.arr.irrOn, p.arr.irrLast = a.afOn, a.afLast, a.irrOn, a.irrLast
	p.arr.learnStart = a.learnStart
}

// Acknowledge (Silence): latched alarms whose condition has ended clear at
// once; alarms still present are silenced and clear when they end.
func (p *Patient) Acknowledge() {
	for k, l := range p.latch {
		if l.present {
			l.acked = true
		} else {
			delete(p.latch, k)
		}
	}
	a := &p.arr
	for i := range a.chains {
		ch := &a.chains[i]
		if ch.id == "" {
			continue
		}
		if ch.lastSeen < a.lastScan {
			ch.id = ""
		} else {
			ch.silenced = true
		}
	}
	p.computeHR(p.lastHRUpd)
}

// hold reports whether cond has been true continuously for d seconds.
func (p *Patient) hold(key string, cond bool, d, now float64) bool {
	if p.since == nil {
		p.since = map[string]float64{}
	}
	if !cond {
		delete(p.since, key)
		return false
	}
	s, ok := p.since[key]
	if !ok {
		p.since[key] = now
		s = now
	}
	return now-s >= d
}

// hasPWaves: the dominant supraventricular beats are preceded by P waves.
func hasPWaves(r Rhythm) bool {
	switch r {
	case RSinus, RSinusArrhythmia, RFirstDegree, RMobitzI, RMobitzII, RAPaced, RPaced:
		return true
	}
	return false
}

// arrhythmiaAlarms analyses the labelled beats (once a second) and returns
// the ECG alarm candidates; it also sets the status messages and PVC count.
func (p *Patient) arrhythmiaAlarms(now float64, lm AlarmLimits) []cand {
	a := &p.arr
	if a.timeout == nil {
		a.timeout, a.ect = map[string]float64{}, map[string]float64{}
	}
	prev := a.lastScan
	if prev == 0 || now-prev > 5 || now < prev {
		prev = now - 1
	}
	a.lastScan = now
	r := p.Cfg.Rhythm
	if p.learnPending {
		p.learnPending, a.learnStart = false, now
	}
	learning := a.learnStart > 0 && now-a.learnStart < learnSec

	var bl []bEntry
	for _, b := range p.blog {
		if b.t <= now {
			bl = append(bl, b)
		}
	}
	sort.Slice(bl, func(i, j int) bool { return bl[i].t < bl[j].t })
	n := len(bl)
	isV := func(i int) bool { return bl[i].lab == 'V' }
	inScan := func(t float64) bool { return t > prev && t <= now }
	det := map[string]string{} // yellow arrhythmia conditions -> numeric form
	mark := func(id, num string) {
		if !lm.arrOff(id) {
			det[id] = num
		}
	}
	seen := func(status string, t float64) {
		if t > a.ect[status] {
			a.ect[status] = t
		}
	}
	lastT := p.switchT - 100
	if n > 0 {
		lastT = bl[n-1].t
	}
	beating := now-lastT < 3

	// average and median R-R of the latest intervals between non-ventricular
	// beats before beat i
	typRR := func(i int) (mean, med float64, ok bool) {
		var iv []float64
		for j := i - 1; j >= 1 && len(iv) < 8; j-- {
			if bl[j].lab != 'V' && bl[j-1].lab != 'V' && bl[j].lab != 'S' {
				iv = append(iv, bl[j].t-bl[j-1].t)
			}
		}
		if len(iv) < 3 {
			return 0, 0, false
		}
		s := 0.0
		for _, x := range iv {
			s += x
		}
		sort.Float64s(iv)
		return s / float64(len(iv)), iv[len(iv)/2], true
	}

	// ---- red: fibrillation / asystole ----
	fib := (r == RVFibCoarse || r == RVFibFine || r == RTorsades) && now-p.switchT >= 4 // 4 consecutive seconds
	fibStart := (r == RVFibCoarse || r == RVFibFine || r == RTorsades) && !fib
	asys := !fib && !fibStart && now-math.Max(lastT, p.switchT-100) > lm.AsystoleSec && now-p.switchT > lm.AsystoleSec

	// ---- pauses and missed beats ----
	paced := IsPaced(r)
	for i := 1; i < n; i++ {
		if !inScan(bl[i].t) || a.gapRep == bl[i-1].t {
			continue
		}
		gap := bl[i].t - bl[i-1].t
		if gap > lm.PauseSec {
			if gap <= lm.AsystoleSec {
				mark("Pause", "")
				seen("Pause", bl[i].t)
			}
			continue
		}
		if isV(i - 1) {
			continue
		}
		mean, _, ok := typRR(i)
		if !ok || gap <= 1.75*mean { // 1.75 x the averaged R-R
			if !(ok && !paced && 60/mean > 120 && gap > 1.0) {
				continue
			}
		}
		switch {
		case paced: // paced patients: was there a pace pulse in the gap?
			if len(p.PacesBetween(bl[i-1].t+0.15, bl[i].t-0.15)) > 0 {
				mark("Pacer Not Capt", "")
				seen("Pacer Not Capt", bl[i].t)
			} else {
				mark("Pacer Not Pacing", "")
				seen("Pacer Not Pace", bl[i].t)
			}
		case r != RAfib: // AF is irregular by nature (its own AFIB alarm)
			mark("Missed Beat", "")
			seen("Pause", bl[i].t)
		}
	}
	if n > 0 && !fib && !fibStart {
		if gap := now - lastT; gap > lm.PauseSec && gap <= lm.AsystoleSec && a.gapRep != lastT {
			a.gapRep = lastT
			mark("Pause", "")
			seen("Pause", now)
		}
	}

	// ---- ventricular runs ----
	vtNow, vtOngoing, ventOngoing := false, false, false
	vtStart := 0.0
	for i := 0; i < n; {
		if !isV(i) {
			i++
			continue
		}
		s := i
		for i < n && isV(i) {
			i++
		}
		e := i - 1
		L := e - s + 1
		ongoing := e == n-1 && beating
		endT := now
		if e < n-1 {
			endT = bl[e+1].t
		}
		hr := 0.0
		if L >= 2 {
			hr = 60 * float64(L-1) / (bl[e].t - bl[s].t)
		}
		if !ongoing && e < n-1 && endT > now-60 {
			if L >= 3 {
				seen("Run PVCs", endT)
			} else if L == 2 {
				seen("Pair PVCs", endT)
			}
		}
		if !(ongoing || (e < n-1 && inScan(endT))) {
			continue
		}
		switch {
		case L >= lm.VTachRun && hr > float64(lm.VTachHR):
			vtNow = true
			if ongoing {
				vtOngoing, vtStart = true, bl[s].t
			}
		case hr > float64(lm.VTachHR) && L >= 3:
			if !ongoing {
				mark("Non-Sustain VT", "")
			}
		case L > lm.VentRhythmRun:
			mark("Vent Rhythm", "")
			ventOngoing = ongoing
		case L >= 3:
			if ongoing {
				ventOngoing = true
			} else {
				mark("Run PVCs High", "")
			}
		case L == 2:
			if !ongoing && s > 0 {
				mark("Pair PVCs", "")
			}
		}
	}

	// ---- R-on-T: early PVC (< 1/3 of the average R-R) with a compensatory
	// pause of 1.25 x average R-R, for HR < 100 ----
	for j := 1; j+1 < n; j++ {
		if !isV(j) || isV(j-1) || !inScan(bl[j+1].t) {
			continue
		}
		m, _, ok := typRR(j)
		if !ok || 60/m >= 100 || bl[j].t-bl[j-1].t >= m/3 {
			continue
		}
		// with a compensatory pause, or two such early Vs within 5 minutes
		if bl[j+1].t-bl[j].t >= 1.25*m || (a.ronT > 0 && bl[j].t-a.ronT < 300) {
			mark("R-on-T PVCs", "")
			seen("R-on-T PVCs", bl[j+1].t)
		}
		a.ronT = bl[j].t
	}

	// ---- dominant patterns ----
	seq := func(k int) string {
		if n < k {
			return ""
		}
		b := make([]byte, k)
		for i := 0; i < k; i++ {
			b[i] = 'N'
			if isV(n - k + i) {
				b[i] = 'V'
			}
		}
		return string(b)
	}
	s8, s9 := seq(8), seq(9)
	bigem := beating && (s8 == "NVNVNVNV" || s8 == "VNVNVNVN")
	trigem := beating && !bigem && (s9 == "NNVNNVNNV" || s9 == "NVNNVNNVN" || s9 == "VNNVNNVNN")
	if bigem {
		mark("Vent Bigeminy", "")
	} else if trigem {
		mark("Vent Trigeminy", "")
	}

	// ---- PVC rate and multiform PVCs (ectopic Vs, not sustained rhythms) ----
	pvc := 0
	cnt, recent := map[uint8]int{}, map[uint8]bool{}
	for i := max(0, n-300); i < n; i++ {
		if !isV(i) || bl[i].m >= 10 {
			continue
		}
		if bl[i].t > now-60 {
			pvc++
		}
		cnt[bl[i].m]++
		if i >= n-60 {
			recent[bl[i].m] = true
		}
	}
	p.PVCMin = pvc
	if pvc > lm.PVCsMin {
		mark("PVCs/min High", fmt.Sprintf("* PVCs/min %d>%d", pvc, lm.PVCsMin))
	}
	forms := 0
	for m, c := range cnt {
		if c >= 2 && recent[m] {
			forms++
		}
	}
	if forms >= 2 {
		mark("Multiform PVCs", "")
		seen("Multiform PVCs", now)
	}

	// ---- SVT: a run of SV beats longer than the SVT run limit, above the SVT HR limit ----
	svRun := 0
	for j := n - 1; j >= 0 && bl[j].lab != 'V' && bl[j].lab != 'P'; j-- {
		svRun++
	}
	if beating && svRun > lm.SVTRun && n-1-lm.SVTRun >= 0 {
		if span := bl[n-1].t - bl[n-1-lm.SVTRun].t; span > 0 && 60*float64(lm.SVTRun)/span > float64(lm.SVTHR) {
			mark("SVT", "")
		}
	}

	// ---- HR limits (rate chain; short yellow with arrhythmia on) ----
	// (rate alarms need the averaged HR to stay beyond the limit for a few
	// seconds, so a couple of early beats can't trigger them)
	hrOK := p.HRValid && p.HR > 0
	hrHigh := p.hold("hrhigh", hrOK && p.HR > lm.HRHigh, 4, now)
	hrLow := p.hold("hrlow", hrOK && p.HR < lm.HRLow, 4, now)
	var hrStd *cand // HR High/Low as standard yellow alarms (no chain, no timeout)
	switch {
	case hrHigh && HRStandardYellow:
		hrStd = &cand{text: "** HR High", num: "** HR " + fmt.Sprintf("%d>%d", p.worstVal("hrhi", p.HR, false), lm.HRHigh), lvl: 2, ord: 300}
	case hrLow && HRStandardYellow:
		hrStd = &cand{text: "** HR Low", num: "** HR " + fmt.Sprintf("%d<%d", p.worstVal("hrlo", p.HR, true), lm.HRLow), lvl: 2, ord: 300}
	case hrHigh:
		mark("HR High", fmt.Sprintf("* HR %d>%d", p.HR, lm.HRHigh))
	case hrLow:
		mark("HR Low", fmt.Sprintf("* HR %d<%d", p.HR, lm.HRLow))
	}

	// ---- irregular HR: consistently irregular N-N intervals ----
	var iv []float64
	for j := n - 1; j >= 1 && len(iv) < 12; j-- {
		if bl[j].lab == 'N' && bl[j-1].lab == 'N' {
			iv = append(iv, bl[j].t-bl[j-1].t)
		}
	}
	irr := false
	if beating && len(iv) >= 8 {
		// "consistently" irregular: the median beat-to-beat change, so a
		// single pause or early beat does not count
		sum := 0.0
		var d []float64
		for k := range iv {
			sum += iv[k]
			if k > 0 {
				d = append(d, math.Abs(iv[k]-iv[k-1]))
			}
		}
		sort.Float64s(d)
		irr = d[len(d)/2]/(sum/float64(len(iv))) > 0.07
	}
	irr = p.hold("irregular", irr, 8, now)

	// ---- red arrhythmia alarms (one at a time: highest of the red chain) ----
	var red *cand
	xb, xt := lm.XBrady(), lm.XTachy()
	xBrady := p.hold("xbrady", hrOK && p.HR < xb, 4, now)
	xTachy := p.hold("xtachy", hrOK && p.HR > xt, 4, now)
	switch {
	case asys:
		red = &cand{text: "*** Asystole", lvl: 3, ord: 450}
	case fib:
		red = &cand{text: "*** Vent Fib/Tach", lvl: 3, ord: 449}
	case vtNow && !learning:
		red = &cand{text: "*** VTach", lvl: 3, ord: 448}
	case xBrady: // Enhanced text: lowest measured value < limit
		red = &cand{text: "*** Extreme Brady", num: fmt.Sprintf("*** xBrady %d<%d", p.worstVal("xbrady", p.HR, true), xb), lvl: AlarmPanic, ord: 447}
	case xTachy:
		red = &cand{text: "*** Extreme Tachy", num: fmt.Sprintf("*** xTachy %d>%d", p.worstVal("xtachy", p.HR, false), xt), lvl: AlarmPanic, ord: 447}
	}
	// AFIB (atrial fibrillation waveform) / Irregular HR (other consistently
	// irregular rhythms), and their "End" alarms once no longer detected for
	// the end delay
	afEnd := float64(lm.AfEndDelay)
	afib := irr && r == RAfib
	irrHR := irr && r != RAfib && !a.afOn // (not while an AF episode is ending)
	if afib {
		a.afOn, a.afLast = true, now
		mark("AFIB", "")
	} else if a.afOn && now-a.afLast > afEnd {
		a.afOn = false
		mark("End AFIB", "")
	}
	if irrHR {
		a.irrOn, a.irrLast = true, now
		mark("Irregular HR", "")
	} else if a.irrOn && now-a.irrLast > afEnd {
		a.irrOn = false
		mark("End Irregular HR", "")
	}
	if learning { // only Asystole, Vent Fib/Tach and HR alarms while learning
		for id := range det {
			if id != "HR High" && id != "HR Low" {
				delete(det, id)
			}
		}
	}

	// ---- yellow arrhythmia alarm chains ----
	blocked := func(id string) bool {
		d := arrDefs[id]
		for x, dx := range arrDefs {
			if dx.chain == d.chain && dx.pri <= d.pri && a.timeout[x] > now {
				return true
			}
		}
		return false
	}
	announce := func(ch *chainAlarm, id string) {
		d := arrDefs[id]
		*ch = chainAlarm{id: id, num: det[id], pri: d.pri, announced: now, lastSeen: now}
		switch d.to {
		case 1:
			a.timeout[id] = now + timeout1st
		case 2:
			a.timeout[id] = now + timeout2nd
		}
	}
	for ci := range a.chains {
		ch := &a.chains[ci]
		best, bp := "", 99
		for id := range det {
			if d := arrDefs[id]; d.chain == ci && (d.pri < bp || (d.pri == bp && id < best)) {
				best, bp = id, d.pri
			}
		}
		if ch.id != "" {
			switch {
			case best != "" && bp < ch.pri: // a higher alarm in the chain supersedes
				announce(ch, best)
			case best != "" && best != ch.id && bp == ch.pri: // e.g. AFIB -> End AFIB
				announce(ch, best)
			case best == ch.id && arrDefs[best].to != 0 && a.timeout[best] <= now:
				// still (or again) present after the timeout period: the tone
				// sounds again as an alarm reminder, even if acknowledged
				announce(ch, best)
				p.RemindAt = now
			case best == ch.id:
				ch.num, ch.lastSeen = det[best], now
			}
			hold := latchSec
			if ch.silenced {
				hold = 0
			}
			if ch.lastSeen < now && now-ch.lastSeen > hold {
				ch.id = ""
			}
		}
		if ch.id == "" && best != "" && !blocked(best) {
			announce(ch, best)
		}
	}

	var out []cand
	if red != nil {
		red.fresh, red.tone = true, true
		out = append(out, *red)
	}
	if hrStd != nil {
		hrStd.fresh, hrStd.tone = true, true
		out = append(out, *hrStd)
	}
	if lm.ArrOff != 0 { // (if configured) some arrhythmia alarms switched off
		out = append(out, cand{text: "Some ECG AlarmsOff", lvl: 4, ord: 80})
	}
	// alarm conditions of equal severity from different chains: the most
	// recently occurring one is announced
	var pick *chainAlarm
	for i := range a.chains {
		ch := &a.chains[i]
		if ch.id != "" && (pick == nil || ch.announced > pick.announced) {
			pick = ch
		}
	}
	tone := false
	for _, ch := range a.chains {
		if ch.id != "" && !ch.silenced && now-ch.announced < shortToneSec {
			tone = true
		}
	}
	if pick != nil {
		out = append(out, cand{text: "* " + pick.id, num: pick.num, lvl: 1, ord: 200,
			fresh: now-pick.lastSeen < shortToneSec && !pick.silenced, tone: tone})
	}

	// ---- status messages on the ECG ----
	p.RhythmStatus = ""
	switch {
	case learning && now-a.learnStart < learnSec/2:
		p.RhythmStatus = "Learning ECG"
	case learning && !fib && !asys:
		p.RhythmStatus = "Learning Rhythm"
	case fib:
		p.RhythmStatus = "Vent Fib/Tach"
	case fibStart: // still confirming (4 s)
	case asys:
		p.RhythmStatus = "Asystole"
	case vtOngoing && now-vtStart > 15:
		p.RhythmStatus = "Sust V-Tach"
	case vtOngoing:
		p.RhythmStatus = "V-Tach"
	case ventOngoing:
		p.RhythmStatus = "Vent Rhythm"
	case bigem:
		p.RhythmStatus = "Vent Bigeminy"
	case trigem:
		p.RhythmStatus = "Vent Trigeminy"
	case !beating:
		p.RhythmStatus = ""
	default:
		pacedN := 0
		for i := max(0, n-8); i < n; i++ {
			if bl[i].lab == 'P' {
				pacedN++
			}
		}
		switch {
		case pacedN*2 >= min(n, 8) && n > 0:
			p.RhythmStatus = "Paced Rhythm"
		case r == RAfib:
			p.RhythmStatus = "Atrial Fib"
		case irr:
			p.RhythmStatus = "Irregular HR"
		default:
			base := "SV"
			if hasPWaves(r) && p.psvtEnd == 0 { // (no P waves during a PSVT episode)
				base = "Sinus"
			}
			cat := base + " Rhythm"
			if p.HR > 100 {
				cat = base + " Tachy"
			} else if p.HR > 0 && p.HR < 60 {
				cat = base + " Brady"
			}
			// the HR must be in the new range for five beats
			nb := 0
			for i := n - 1; i >= 0 && inScan(bl[i].t); i-- {
				nb++
			}
			switch {
			case a.stCat == "" || cat == a.stCat:
				a.stCat, a.stCnt = cat, 0
			case cat == a.stCand:
				a.stCnt += nb
				if a.stCnt >= 5 {
					a.stCat, a.stCnt = cat, 0
				}
			default:
				a.stCand, a.stCnt = cat, nb
			}
			p.RhythmStatus = a.stCat
		}
	}
	// ectopic status: within the last minute, in the monitor's order
	sv, pb := 0, 0
	for i := 0; i < n; i++ {
		if bl[i].t > now-60 {
			switch bl[i].lab {
			case 'S':
				sv++
			case 'P':
				pb++
			}
		}
	}
	p.EctopicStatus = ""
	for _, s := range []string{"Run PVCs", "Pair PVCs", "Pacer Not Capt", "Pacer Not Pace", "Pause", "R-on-T PVCs", "Multiform PVCs"} {
		if t, ok := a.ect[s]; ok && now-t < 60 {
			p.EctopicStatus = s
			break
		}
	}
	if p.EctopicStatus == "" {
		switch {
		case sv > 5:
			p.EctopicStatus = "Frequent SVPBs"
		case sv > 0 && pvc == 0 && hasPWaves(r) && p.RhythmStatus != "Paced Rhythm":
			p.EctopicStatus = "SVPBs"
		case sv > 0 && p.RhythmStatus == "Paced Rhythm":
			p.EctopicStatus = "SV Beats"
		case pb > 0 && p.RhythmStatus != "Paced Rhythm" && p.RhythmStatus != "":
			p.EctopicStatus = "Paced Beats"
		}
	}
	return out
}

// worstVal tracks the most extreme value of an alarm while it lasts (for the
// Enhanced alarm text, which shows the maximum deviation from the limit).
func (p *Patient) worstVal(key string, v int, low bool) int {
	if p.worst == nil {
		p.worst = map[string]int{}
	}
	w, ok := p.worst[key]
	if !ok || (low && v < w) || (!low && v > w) {
		w = v
		p.worst[key] = w
	}
	p.worstSeen[key] = true
	return w
}

// latchEntry is a latched patient alarm.
type latchEntry struct {
	c              cand
	present, acked bool
}
