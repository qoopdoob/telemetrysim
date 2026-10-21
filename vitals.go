package main

// Additional vital signs: invasive arterial pressure (ABP), non-invasive
// blood pressure (NBP) and capnography (EtCO2). All are driven by the same
// simulated heartbeats and breaths as the ECG, pleth and respiration.

import "math"

// NBP modes
const (
	NBPOff = iota
	NBPManual
	NBPAuto
)

var NBPModeNames = []string{"Off", "Manual", "Auto"}
var NBPIntervals = []int{1, 3, 5, 10, 15, 30, 60, 240, 480} // minutes

// nbpMinutes is the auto-cycle interval in minutes (0 = default 15 min).
func nbpMinutes(c Config) int {
	if c.NBPInterval <= 0 {
		return 15
	}
	return c.NBPInterval
}

// NBPIntervalLabel formats an interval as on the monitor: "15 min", "1 hr".
func NBPIntervalLabel(min int) string {
	if min >= 60 && min%60 == 0 {
		return itoaS(min/60) + " hr"
	}
	return itoaS(min) + " min"
}

func itoaS(v int) string {
	if v < 10 {
		return string(rune('0' + v))
	}
	return itoaS(v/10) + string(rune('0'+v%10))
}

// NBPModeLabel is the mode text next to the NBP timestamp: "Auto 15 min"
// in auto mode, nothing in manual mode.
func NBPModeLabel(c Config) string {
	if c.NBPMode == NBPAuto {
		return "Auto " + NBPIntervalLabel(nbpMinutes(c))
	}
	return ""
}

const nbpCycle = 25.0 // seconds for one cuff measurement

// vitals holds the per-patient state for ABP, NBP and EtCO2.
type vitals struct {
	abuf [bufLen]float32 // ABP samples (mmHg)
	cbuf [bufLen]float32 // CO2 samples (mmHg)

	ABPSys, ABPDia, ABPMap int
	ABPValid               bool
	ABPPulsatile           bool

	// perfusion: 1 = the patient's set pressures, falling toward 0 while
	// there is no cardiac output, recovering when pulses return
	perf, perfTarget float64
	ampAvg           float64 // mean pulse strength over the last few seconds

	EtCO2Disp, AwRR int
	CO2Valid        bool

	NBPSys, NBPDia, NBPMap int
	NBPValid               bool    // a completed reading exists
	NBPAt                  float64 // sim time of the last reading
	NBPMeasuring           bool
	nbpStart, nbpNext      float64
	CuffP                  int
	NBPFailed              bool
	lastEt                 float64 // CO2 at the end of the previous expiration
	nbpN, nbpPulsing       int     // seconds of deflation / with pulses
	nbpPerfSum, nbpAmpSum  float64
}

// pressure the arterial system settles to with no cardiac output (mmHg)
const arrestFloor = 16.0

// updatePerfTarget derives the perfusion target from recent pulses (1/s):
// normal beats give the set pressures; weak pulses (e.g. V-tach with a pulse)
// lower them.
func (p *Patient) updatePerfTarget(now float64) {
	n, s := 0, 0.0
	for _, q := range p.pulses {
		if q.t <= now && q.t > now-6 {
			n++
			s += q.amp
		}
	}
	if n == 0 {
		p.ampAvg = 0
		return
	}
	p.ampAvg = s / float64(n)
	p.perfTarget = 1
	if p.ampAvg < 0.85 {
		p.perfTarget = 0.5 + 0.5*p.ampAvg/0.85
	}
}

// updatePerf advances the perfusion state by one sample. With no pulses
// (asystole, VF, pulseless VT) it decays; when pulses return it recovers
// over several seconds, so pressures climb back gradually.
func (p *Patient) updatePerf(t float64) {
	last := math.Inf(-1)
	for i := len(p.pulses) - 1; i >= 0; i-- {
		if p.pulses[i].t <= t {
			last = p.pulses[i].t
			break
		}
	}
	target := p.perfTarget
	if t-last > math.Max(2.0, 2.2*60/p.ventRate()) {
		target = 0
	}
	tau := 8.0
	if target < p.perf {
		tau = 20
	}
	p.perf += (target - p.perf) / (FS * tau)
}

func (p *Patient) ABPAt(k int64) float32 { return p.abuf[((k%bufLen)+bufLen)%bufLen] }
func (p *Patient) CO2At(k int64) float32 { return p.cbuf[((k%bufLen)+bufLen)%bufLen] }

// abpValue: arterial pressure (mmHg). Each perfusing beat gives a pressure
// pulse (sharp upstroke, dicrotic notch, diastolic run-off) scaled by the
// beat's filling (PVCs give weak pulses). With no cardiac output the
// pulsations stop and the pressure runs off toward ~16 mmHg; after a return
// of pulses it recovers gradually (perf).
func (p *Patient) abpValue(t float64) float64 {
	if !p.Cfg.ABPOn {
		return 0
	}
	sys, dia := float64(p.Cfg.ABPSys), float64(p.Cfg.ABPDia)
	if sys <= dia {
		sys = dia + 10
	}
	sys = arrestFloor + (sys-arrestFloor)*p.perf
	dia = arrestFloor + (dia-arrestFloor)*p.perf
	last := math.Inf(-1)
	pulse := 0.0
	// diastolic run-off continues until the next beat; offset it so the
	// trough of a normal beat lands on the set diastolic pressure
	tailRR := abpTail(60 / p.ventRate())
	for _, q := range p.pulses {
		ts := q.t - 0.12 // arterial upstroke precedes the finger pleth
		dt := t - ts
		if dt < 0 {
			continue
		}
		if ts > last {
			last = ts
		}
		if dt > 1.4 {
			continue
		}
		a := math.Min(q.amp, 1.15)
		s := abpShape(dt) - tailRR
		if v := a * s; v > pulse {
			pulse = v
		}
	}
	base := dia
	if gap := t - last; gap > 60/p.ventRate()*1.3 { // no beat: pressure runs off
		x := gap - 60/p.ventRate()*1.3
		base = arrestFloor + (dia-arrestFloor)*math.Exp(-x/3.0)
		pulse *= math.Exp(-x / 0.6)
	}
	if math.IsInf(last, -1) {
		base = arrestFloor
	}
	v := base + (sys-dia)*pulse
	if p.Cfg.Rhythm == RVFibCoarse { // fibrillating ventricles: a faint ripple, no pulse
		v += 0.8 * math.Sin(2*math.Pi*4.6*t+p.ph[2])
	}
	return v + 0.4*p.rng.NormFloat64()
}

// abpShape is one arterial pressure pulse (0 = diastolic, ~1 = systolic):
// sharp upstroke, rounded peak, dicrotic notch, then exponential run-off.
func abpShape(dt float64) float64 {
	return ag(dt, 0.10, 0.04, 0.15, 0.80) - g(dt, 0.30, 0.016, 0.07) + g(dt, 0.34, 0.04, 0.10) + abpTail(dt)
}

func abpTail(dt float64) float64 {
	return 0.34 * sstep(dt-0.09, 0.03) * math.Exp(-math.Max(0, dt-0.12)/0.42)
}

// co2Value: capnogram (mmHg). Near zero during inspiration, then a steep rise
// to an alveolar plateau that slopes gently up to the end-tidal value.
func (p *Patient) co2Value(t float64) float64 {
	if !p.Cfg.CO2On {
		return 0
	}
	if p.Cfg.Apnea || p.breathPer <= 0 {
		return 0.3 * math.Abs(p.rng.NormFloat64())
	}
	// no pulmonary blood flow -> little CO2 delivered to the lungs
	et := float64(p.Cfg.EtCO2) * p.breathEt * (0.15 + 0.85*p.perf)
	x := (t - p.breathStart) / p.breathPer
	f := p.breathF // follows this breath's inspiratory time
	var v float64
	switch {
	case x < 0.06: // end of the previous expiration: rapid inspiratory down-stroke
		v = p.lastEt * (1 - x/0.06)
	case x < f: // inspiration
		v = 0
	default: // expiration: steep rise, then a gently sloping alveolar plateau
		v = et * (1 - math.Exp(-(x-f)/0.035)) * (0.90 + 0.10*(x-f)/(1-f))
		p.lastEt = v
	}
	return v + 0.25*p.rng.NormFloat64()
}

// SetNBP changes the NBP mode/interval live (from the interval menu). The
// next auto reading is due one interval after the last reading.
func (p *Patient) SetNBP(mode, minutes int, now float64) {
	p.Cfg.NBPMode, p.Cfg.NBPInterval = mode, minutes
	switch mode {
	case NBPOff:
		p.NBPValid, p.NBPMeasuring, p.NBPFailed = false, false, false
	case NBPAuto:
		if p.NBPValid || p.NBPFailed {
			p.nbpNext = math.Max(now+5, p.NBPAt+float64(nbpMinutes(p.Cfg))*60)
		} else {
			p.nbpNext = now + 5
		}
	}
}

// StartNBP begins a cuff measurement.
func (p *Patient) StartNBP(now float64) {
	if p.Cfg.NBPMode == NBPOff || p.NBPMeasuring {
		return
	}
	p.NBPMeasuring, p.nbpStart = true, now
	p.nbpN, p.nbpPulsing, p.nbpPerfSum, p.nbpAmpSum = 0, 0, 0, 0
}

// updateNBP runs the cuff cycle and auto-interval scheduling (called 1/s).
func (p *Patient) updateNBP(now float64) {
	c := p.Cfg
	if c.NBPMode == NBPOff {
		p.NBPMeasuring, p.NBPValid = false, false
		return
	}
	if c.NBPMode == NBPAuto && !p.NBPMeasuring && now >= p.nbpNext {
		p.StartNBP(now)
	}
	if !p.NBPMeasuring {
		return
	}
	el := now - p.nbpStart
	target := float64(c.NBPSys + 30)
	switch {
	case el < 6: // inflate
		p.CuffP = int(target * el / 6)
	case el < nbpCycle: // step deflation: the cuff senses arterial pulsations
		p.CuffP = int(target - (target-float64(c.NBPDia-15))*(el-6)/(nbpCycle-6))
		p.nbpN++
		p.nbpPerfSum += p.perf
		for _, q := range p.pulses {
			if q.t > now-2 && q.t <= now {
				p.nbpPulsing++
				p.nbpAmpSum += p.ampAvg
				break
			}
		}
	default: // done
		p.NBPMeasuring, p.CuffP = false, 0
		p.NBPAt = now
		iv := float64(nbpMinutes(c)) * 60
		p.nbpNext = now + iv
		// a reading needs pulsatile flow for most of the deflation
		if p.nbpN == 0 || p.nbpPulsing*2 < p.nbpN {
			p.NBPFailed, p.NBPValid = true, false
			return
		}
		pf := p.nbpPerfSum / float64(p.nbpN)
		amp := clamp(p.nbpAmpSum/float64(p.nbpPulsing), 0.3, 1)
		dia := arrestFloor + (float64(c.NBPDia)-arrestFloor)*pf
		sys := dia + (arrestFloor+(float64(c.NBPSys)-arrestFloor)*pf-dia)*amp
		p.NBPFailed, p.NBPValid = false, true
		p.NBPSys = int(math.Round(sys + 4*p.rng.NormFloat64()))
		p.NBPDia = int(math.Round(dia + 3*p.rng.NormFloat64()))
		if p.NBPDia >= p.NBPSys-10 {
			p.NBPDia = p.NBPSys - 10
		}
		p.NBPMap = int(math.Round(float64(p.NBPDia) + float64(p.NBPSys-p.NBPDia)/3))
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// measureVitals derives ABP and EtCO2 numerics from their waveforms (1/s).
func (p *Patient) measureVitals(now float64) {
	last := p.Last()
	p.ABPValid, p.ABPPulsatile = false, false
	if p.Cfg.ABPOn && last > int64(4*FS) {
		n := int64(4 * FS)
		mx, mn, sum := -1e9, 1e9, 0.0
		for k := last - n; k <= last; k++ {
			v := float64(p.ABPAt(k))
			mx, mn, sum = math.Max(mx, v), math.Min(mn, v), sum+v
		}
		// the numerics follow the wave, pulsatile or not (e.g. 18/15 (16) in arrest)
		p.ABPValid = true
		p.ABPPulsatile = mx-mn >= 3 // non-pulsatile: amplitude below 3 mmHg
		p.ABPSys, p.ABPDia = int(math.Round(mx)), int(math.Round(mn))
		p.ABPMap = int(math.Round(sum / float64(n+1)))
	}
	p.CO2Valid = false
	if p.Cfg.CO2On {
		p.CO2Valid = true
		n := int64(12 * FS)
		mx := 0.0
		for k := last - n; k <= last; k++ {
			mx = math.Max(mx, float64(p.CO2At(k)))
		}
		p.EtCO2Disp = int(math.Round(mx))
		if mx < 3 { // apnea / disconnected
			p.EtCO2Disp = 0
		}
		// awRR from breath onsets (same breaths that drive the CO2)
		var b []float64
		for _, x := range p.breaths {
			if x <= now && x > now-60 {
				b = append(b, x)
			}
		}
		p.AwRR = 0
		if len(b) >= 2 && now-b[len(b)-1] < float64(p.lim().ApneaSec) && p.EtCO2Disp > 0 {
			k := len(b) - 1
			if k > 5 {
				k = 5
			}
			p.AwRR = int(math.Round(60 * float64(k) / (b[len(b)-1] - b[len(b)-1-k])))
		}
	}
}

// stMeasurable reports whether ST/QT can be measured on the current rhythm.
func (p *Patient) stMeasurable() bool {
	if p.Cfg.CardiacOff || !p.HRValid || p.HR == 0 || p.LeadsOff() {
		return false
	}
	switch p.Cfg.Rhythm {
	case RLeadsOff, RAsystole, RVTach, RTorsades, RVFibCoarse, RVFibFine, RIdioventricular, RPaced, RAVPaced:
		return false
	}
	return true
}
