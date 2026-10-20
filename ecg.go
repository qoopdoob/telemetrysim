package main

// ECG rhythm engine: beat scheduling, ectopy, waveform synthesis, HR and alarms.
// Platform independent (used by the Windows GUI and by the preview renderer).

import (
	"fmt"
	"math"
	"math/rand"
)

const FS = 250.0        // samples per second
const bufLen = 250 * 64 // 64 s ring buffer (slow resp sweeps, wide expanded view)

type Rhythm int

const (
	RLeadsOff Rhythm = iota
	RAsystole
	RSinus
	RSinusArrhythmia
	RAfib
	RAflutter
	RSVT
	RJunctional
	RFirstDegree
	RMobitzI
	RMobitzII
	RThirdDegree
	RIdioventricular
	RVTach
	RTorsades
	RVFibCoarse
	RVFibFine
	RPaced
	RAPaced
	RAVPaced
	RCount
)

type rhythmInfo struct {
	Name             string
	DefRate          int
	MinRate, MaxRate int
	UsesRate         bool
	Ectopy           bool // PVCs allowed
	AtrialEctopy     bool // PACs / PJCs allowed
	TypMin, TypMax   int  // typical range used when randomizing
}

var Rhythms = []rhythmInfo{
	{"Leads Off (ECG disconnected)", 0, 0, 0, false, false, false, 0, 0},
	{"Asystole", 0, 0, 0, false, false, false, 0, 0},
	{"Sinus Rhythm (brady / normal / tachy by rate)", 75, 20, 200, true, true, true, 45, 140},
	{"Sinus Arrhythmia", 70, 30, 130, true, true, true, 55, 95},
	{"Atrial Fibrillation", 95, 30, 200, true, true, false, 60, 160},
	{"Atrial Flutter", 75, 50, 175, true, true, false, 70, 150},
	{"Supraventricular Tachycardia (SVT)", 180, 130, 280, true, true, false, 150, 220},
	{"Junctional Rhythm (set rate for accel./tachy)", 50, 25, 160, true, true, true, 40, 90},
	{"1st Degree AV Block", 70, 30, 150, true, true, true, 50, 90},
	{"2nd Degree AV Block Type I (Wenckebach)", 60, 30, 120, true, true, true, 45, 75},
	{"2nd Degree AV Block Type II (Mobitz II)", 50, 25, 110, true, true, true, 35, 60},
	{"3rd Degree (Complete) Heart Block", 38, 20, 70, true, true, false, 25, 50},
	{"Idioventricular Rhythm", 35, 15, 110, true, true, false, 25, 45},
	{"Ventricular Tachycardia (monomorphic)", 170, 110, 260, true, false, false, 140, 220},
	{"Torsades de Pointes", 220, 160, 300, true, false, false, 200, 260},
	{"Ventricular Fibrillation (coarse)", 0, 0, 0, false, false, false, 0, 0},
	{"Ventricular Fibrillation (fine)", 0, 0, 0, false, false, false, 0, 0},
	{"Ventricular Paced Rhythm (VVI)", 70, 30, 120, true, true, false, 60, 80},
	{"Atrial Paced Rhythm (AAI)", 70, 40, 130, true, true, false, 60, 80},
	{"AV (Dual-Chamber) Paced Rhythm (DDD)", 70, 40, 130, true, true, false, 60, 80},
}

// RateAdjustable reports whether the rate box applies to a rhythm. For Leads
// Off the heart is still beating, so the rate sets the SpO2 pulse rate.
func RateAdjustable(r Rhythm) bool { return Rhythms[r].UsesRate || r == RLeadsOff }

// IsPaced reports whether a rhythm comes from a pacemaker.
func IsPaced(r Rhythm) bool { return r == RPaced || r == RAPaced || r == RAVPaced }

// Pacing modes
const (
	PaceFixed  = iota // 100% paced at the set rate
	PaceDemand        // paces only when the intrinsic rate falls below the set rate
)

var PaceModeNames = []string{"Fixed (100% paced)", "Demand (paces below set rate)"}

// AlarmLimits are the station-wide alarm limits. Crossing an "Ext" limit
// raises a red panic alarm.
type AlarmLimits struct {
	HRLow, HRHigh       int
	HRExtLow, HRExtHigh int
	SpO2Low, SpO2ExtLow int
	RRLow, RRHigh       int
	ApneaSec            int
	STHigh, STLow       float64 // mm
	QTcHigh             int     // ms
	NBPsHigh, NBPsLow   int
	ABPsHigh, ABPsLow   int
	EtCO2High, EtCO2Low int
}

var Limits = AlarmLimits{HRLow: 50, HRHigh: 120, HRExtLow: 40, HRExtHigh: 160, SpO2Low: 90, SpO2ExtLow: 85,
	RRLow: 8, RRHigh: 30, ApneaSec: 20, STHigh: 2, STLow: -2, QTcHigh: 500,
	NBPsHigh: 160, NBPsLow: 90, ABPsHigh: 160, ABPsLow: 90, EtCO2High: 50, EtCO2Low: 30}

// ---- ECG leads ----

// leadP scales each waveform component for one ECG lead (lead II = 1).
type leadP struct {
	p, q, r, s, t float64 // P, Q, R, S and T of narrow (supraventricular) beats
	wide          float64 // polarity/size of wide complexes (PVC, VT, paced, escape)
	fl, af, vf    float64 // flutter waves, fibrillatory waves, VF/torsades
	pBi           bool    // biphasic P (V1)
}

var LeadNames = []string{"I", "II", "III", "aVR", "aVL", "aVF", "V1", "V2", "V3", "V4", "V5", "V6"}

var leadTable = map[string]leadP{
	"I":   {0.6, 0.5, 0.65, 0.4, 0.6, 0.6, 0.25, 0.5, 0.8, false},
	"II":  {1, 1, 1, 1, 1, 1, 1, 1, 1, false},
	"III": {0.45, 0.6, 0.55, 0.9, 0.35, 0.45, 1, 0.8, 0.9, false},
	"aVR": {-0.8, 5.5, 0.12, 0.8, -0.8, -0.8, -0.6, 0.5, 0.8, false},
	"aVL": {0.1, 0.8, 0.3, 1.4, 0.15, -0.2, -0.4, 0.4, 0.7, false},
	"aVF": {0.75, 0.8, 0.8, 0.8, 0.7, 0.75, 1, 0.8, 0.9, false},
	"V1":  {0.55, 0, 0.2, 3.8, -0.3, -1, -0.7, 1.8, 1.1, true},
	"V2":  {0.5, 0, 0.45, 4.2, 1.4, -0.8, 0.3, 1.2, 1.2, false},
	"V3":  {0.45, 0, 0.9, 2.6, 1.3, -0.2, 0.25, 1, 1.1, false},
	"V4":  {0.45, 0.3, 1.4, 1.3, 1.15, 0.5, 0.2, 0.7, 1, false},
	"V5":  {0.45, 0.6, 1.35, 0.6, 0.95, 0.85, 0.2, 0.6, 0.9, false},
	"V6":  {0.45, 0.7, 1.05, 0.3, 0.75, 1, 0.2, 0.5, 0.8, false},
}

// LeadName returns the patient's displayed lead ("" means lead II).
func (c Config) LeadName() string {
	if _, ok := leadTable[c.Lead]; ok {
		return c.Lead
	}
	return "II"
}

// Respiration display: the waveform depends on the rate number (it can only
// be shown while the rate is shown). Resp alarms are disabled only when both
// the rate and the waveform are turned off.
func (c Config) RespWaveShown() bool { return c.RespWaveOn && !c.RespRateOff }
func (c Config) RespShown() bool     { return !c.RespRateOff || c.RespWaveShown() }
func (c Config) RespAlarms() bool    { return c.RespShown() }

// AlarmPanic is the alarm level used for extreme-limit (panic) alarms.
const AlarmPanic = 5

// Atrial flutter conduction ratio settings.
const (
	FlAuto = iota
	Fl2to1
	Fl3to1
	Fl4to1
	Fl5to1
	FlVariable
)

var FlutterRatioNames = []string{
	"Auto (from heart rate)",
	"2:1",
	"3:1",
	"4:1",
	"5:1",
	"Variable (2:1 - 4:1)",
}

// RateRange returns the allowed and default rate for a rhythm. For atrial
// flutter with a set conduction ratio the rate is the atrial flutter-wave rate.
func RateRange(r Rhythm, flRatio int) (min, max, def int) {
	if r == RAflutter && flRatio != FlAuto {
		return 240, 350, 300
	}
	if r == RLeadsOff { // ECG disconnected: the rate drives the pulse oximeter
		return 20, 250, 78
	}
	i := Rhythms[r]
	return i.MinRate, i.MaxRate, i.DefRate
}

const (
	EctNone = iota
	EctPVCUni
	EctPVCMulti
	EctPAC
	EctPJC
)

var EctopyNames = []string{"None", "PVCs - unifocal", "PVCs - multifocal", "PACs", "PJCs"}

const (
	PatOccasional = iota
	PatBigeminy
	PatTrigeminy
	PatQuadrigeminy
	PatCouplets
	PatRuns
)

var PatternNames = []string{
	"Occasional (random)",
	"Bigeminy (every 2nd beat)",
	"Trigeminy (every 3rd beat)",
	"Quadrigeminy (every 4th beat)",
	"Couplets (pairs)",
	"Runs of 3",
}

// PatternUsesFreq reports whether the per-minute frequency applies to a pattern.
func PatternUsesFreq(p int) bool { return p == PatOccasional || p == PatCouplets || p == PatRuns }

type Config struct {
	Name     string
	Rhythm   Rhythm
	Rate     int
	Ectopy   int
	Pattern  int
	Freq     int // ectopic events per minute (occasional / couplets / runs)
	FlRatio  int // atrial flutter conduction ratio (Fl* constants)
	SpO2On   int // SpO2Off / SpO2Monitored / SpO2SensorOff
	SpO2     int // target saturation (%)
	PaceMode int // PaceFixed / PaceDemand (paced rhythms)
	IntrRate int // intrinsic sinus rate under a demand pacemaker
	Artifact int // Art* constants
	ArtLevel int // 0 mild, 1 moderate, 2 severe
	Tune     Tune
	Lead     string // displayed ECG lead ("" = II)

	CardiacOff  bool // ECG monitoring hidden (no ECG alarms)
	RespRateOff bool // hide the RR numeric (default shown)
	RespWaveOn  bool // show the respiration waveform (default hidden)
	RR          int  // breaths per minute (0 = default 16 unless Apnea)
	Apnea       bool

	NBPMode        int // NBPOff / NBPManual / NBPAuto
	NBPInterval    int // auto-cycle interval in minutes (0 = 15)
	NBPSys, NBPDia int // patient's true blood pressure (measured by the cuff)
	ABPOn          bool
	ABPSys, ABPDia int
	CO2On          bool
	EtCO2          int // end-tidal CO2 (mmHg)
}

// Artifact types
const (
	ArtNone = iota
	ArtTremor
	Art60Hz
	ArtWander
	ArtMotion
	ArtLoose
)

var ArtifactNames = []string{
	"None",
	"Muscle tremor (somatic)",
	"60 Hz electrical interference",
	"Wandering baseline (breathing)",
	"Patient movement (bursts)",
	"Loose lead / poor contact",
}

var ArtLevelNames = []string{"Mild", "Moderate", "Severe"}

// Tune holds per-patient waveform fine-tuning. Interval fields of 0 mean
// "automatic" (the rhythm's own value); amplitudes are percentages.
type Tune struct {
	Set                bool // false = all defaults
	Natural            bool // generated patient variation: adds beat-to-beat variability
	UserSet            bool // edited in the Tune window (shows the cyan dot)
	PRms, QRSms, QTms  int
	PAmp, QRSAmp, TAmp int     // percent (100 = normal; negative T = inverted)
	STmm               float64 // ST elevation (+) / depression (-) in mm at 10 mm/mV
}

func DefaultTune() Tune { return Tune{Set: true, PAmp: 100, QRSAmp: 100, TAmp: 100} }

// shape is the resolved form of Tune used while drawing.
type shape struct {
	p, qrs, t float64 // amplitude scales
	w         float64 // QRS width factor
	qt        float64 // QT override (s), 0 = auto
	st        float64 // ST offset (mV)
}

func (tn Tune) shape() shape {
	if !tn.Set {
		return shape{1, 1, 1, 1, 0, 0}
	}
	sh := shape{float64(tn.PAmp) / 100, float64(tn.QRSAmp) / 100, float64(tn.TAmp) / 100, 1, 0, tn.STmm / 10}
	if tn.QRSms > 0 {
		sh.w = clamp(float64(tn.QRSms)/80, 0.5, 3)
	}
	if tn.QTms > 0 {
		sh.qt = float64(tn.QTms) / 1000
	}
	return sh
}

const (
	SpO2Off = iota
	SpO2Monitored
	SpO2SensorOff
)

var SpO2ModeNames = []string{"Not monitored", "Monitored", "Sensor off"}

type pulse struct{ t, amp float64 }

type kind uint8

const (
	kP kind = iota
	kPEct
	kPInv
	kNarrow
	kWide
	kPVC
	kVT
	kPaced
)

type event struct {
	t   float64
	k   kind
	v   uint8
	amp float64
	rr  float64
}

type Patient struct {
	Cfg Config
	rng *rand.Rand

	ev     []event
	beats  []float64
	pvcs   []float64
	paces  []float64 // pacer spike times (drawn as markers, not voltage)
	pulses []pulse   // pleth pulses (one per perfusing ventricular beat)
	sh     shape     // resolved fine-tuning
	vitals
	ld leadP // displayed lead

	// respiration
	breathStart, breathPer, nextBreath, breathAmp float64
	breathF, breathEt                             float64 // this breath: inspiratory fraction, etCO2 factor
	// the patient's own breathing pattern (set once per patient)
	respIE, respK, respDepth, respBaseT, respBasePh float64
	respDrift                                       float64 // slowly varying breath-to-breath state
	breaths                                         []float64
	rbuf                                            [bufLen]float32
	RRDisp                                          int
	RRValid                                         bool
	ApneaNow                                        bool

	// artifact state
	artOn, artOff float64 // current motion burst / dropout window
	artNext       float64 // when the next burst/dropout may start
	artF          float64 // burst frequency
	artLevel      float64 // dropout level

	nextP, nextV, lastV float64
	pIdx                int
	lastA, intrP        float64 // pacemaker: last atrial event, next intrinsic P
	domCount            int
	flN                 int
	flFreq              float64
	switchT             float64
	ph                  [6]float64

	startT float64
	gen    int64
	buf    [bufLen]float32
	pbuf   [bufLen]float32    // pleth samples
	ebuf   [3][bufLen]float32 // fixed extra ECG leads (ExtraLeads) for the expanded view
	PVCMin int                // PVCs in the last minute

	SpO2Disp  int
	SpO2Valid bool

	Lim *AlarmLimits // per-patient alarm limits (nil = station-wide)

	HR        int
	HRValid   bool
	Pulse     int // pulse rate from SpO2 (0 = none)
	lastHRUpd float64
	AlarmText string // displayed message, with value and limit (e.g. "***xBrady 35 <40")
	AlarmKey  string // stable identity of the alarm (used for silencing)
	AlarmLvl  int
}

func NewPatient(cfg Config, now float64, seed int64) *Patient {
	p := &Patient{rng: rand.New(rand.NewSource(seed))}
	r := p.rng
	p.respIE = 0.32 + 0.12*r.Float64()   // inspiratory fraction of each breath
	p.respK = 3.5 + 4.5*r.Float64()      // how quickly expiration empties
	p.respDepth = 0.85 + 0.3*r.Float64() // typical tidal depth
	p.respBaseT = 30 + 30*r.Float64()    // slow baseline (FRC) drift period
	p.respBasePh = 2 * math.Pi * r.Float64()
	p.breathF, p.breathEt = p.respIE, 1
	p.startT = now - 12
	p.lastV = p.startT - 10
	p.Apply(cfg, p.startT)
	p.Update(now)
	return p
}

// SetTune changes waveform fine-tuning without restarting the rhythm.
func (p *Patient) SetTune(tn Tune) {
	p.Cfg.Tune = tn
	p.sh = tn.shape()
}

// SetLead changes the displayed ECG lead without restarting the rhythm.
func (p *Patient) SetLead(name string) {
	p.Cfg.Lead = name
	p.ld = leadTable[p.Cfg.LeadName()]
}

// SetArtifact changes artifact without restarting the rhythm.
func (p *Patient) SetArtifact(kind, level int) {
	p.Cfg.Artifact, p.Cfg.ArtLevel = kind, level
	p.artOn, p.artOff, p.artNext = 0, 0, 0
}

// prOverride returns the tuned PR interval (s), or def if not tuned.
func (p *Patient) prOverride(def float64) float64 {
	if p.Cfg.Tune.Set && p.Cfg.Tune.PRms > 0 {
		return float64(p.Cfg.Tune.PRms) / 1000
	}
	return def
}

func (p *Patient) atrialStream() bool {
	switch p.Cfg.Rhythm {
	case RSinus, RSinusArrhythmia, RFirstDegree, RMobitzI, RMobitzII, RThirdDegree,
		RLeadsOff, // leads off: heart still beats (drives pleth), ECG shows flat
		RAPaced, RAVPaced:
		return true
	case RPaced:
		return p.Cfg.PaceMode == PaceDemand // intrinsic sinus under a VVI pacer
	}
	return false
}

func (p *Patient) ventStream() bool {
	switch p.Cfg.Rhythm {
	case RAfib, RAflutter, RSVT, RJunctional, RIdioventricular, RVTach, RPaced, RThirdDegree:
		return true
	}
	return false
}

func (p *Patient) rateFor(c Config) float64 {
	if c.Rate < 10 {
		return 10
	}
	return float64(c.Rate)
}

func (p *Patient) rate() float64 {
	if p.Cfg.Rhythm == RLeadsOff && p.Cfg.Rate == 0 {
		return 78
	}
	r := float64(p.Cfg.Rate)
	if r < 10 {
		r = 10
	}
	return r
}

// Apply switches the patient to a new configuration at time t.
func (p *Patient) Apply(cfg Config, t float64) {
	if cfg.Rhythm == RLeadsOff && cfg.Rate == 0 {
		cfg.Rate = 78
	}
	if RateAdjustable(cfg.Rhythm) {
		lo, hi, _ := RateRange(cfg.Rhythm, cfg.FlRatio)
		if cfg.Rate < lo {
			cfg.Rate = lo
		}
		if cfg.Rate > hi {
			cfg.Rate = hi
		}
	}
	if cfg.RR == 0 && !cfg.Apnea {
		cfg.RR = 16
	}
	if cfg.NBPSys == 0 {
		cfg.NBPSys, cfg.NBPDia = 120, 75
	}
	if cfg.ABPSys == 0 {
		cfg.ABPSys, cfg.ABPDia = 120, 70
	}
	if cfg.EtCO2 == 0 {
		cfg.EtCO2 = 38
	}
	if cfg.NBPMode != NBPOff && (p.Cfg.NBPMode == NBPOff || cfg.NBPInterval != p.Cfg.NBPInterval || p.nbpNext == 0) {
		p.nbpNext = t + 5 // first auto reading shortly after it is switched on
	}
	if cfg.NBPMode == NBPOff {
		p.NBPValid, p.NBPMeasuring, p.NBPFailed = false, false, false
	}
	p.Cfg = cfg
	p.sh = cfg.Tune.shape()
	p.ld = leadTable[cfg.LeadName()]
	p.breaths = keepUpTo(p.breaths, t)
	if p.nextBreath == 0 || p.nextBreath > t+60/float64(max(cfg.RR, 1)) {
		p.nextBreath = t + 0.3
	}
	// drop anything scheduled in the future
	ev := p.ev[:0]
	for _, e := range p.ev {
		if e.t <= t {
			ev = append(ev, e)
		}
	}
	p.ev = ev
	p.beats = keepUpTo(p.beats, t)
	p.pvcs = keepUpTo(p.pvcs, t)
	p.paces = keepUpTo(p.paces, t)
	pl := p.pulses[:0]
	for _, q := range p.pulses {
		if q.t <= t {
			pl = append(pl, q)
		}
	}
	p.pulses = pl
	if cfg.SpO2 == 0 {
		p.Cfg.SpO2 = 98
	}
	p.lastV = p.startT - 10
	if n := len(p.beats); n > 0 {
		p.lastV = p.beats[n-1]
	}
	p.switchT = t
	p.pIdx, p.domCount = 0, 0
	for i := range p.ph {
		p.ph[i] = p.rng.Float64() * 2 * math.Pi
	}
	start := t + 0.2 + 0.3*p.rng.Float64()
	p.nextP = start
	p.nextV = math.Max(start, p.lastV+0.4)
	p.lastA = start - 60/p.rateFor(cfg)
	p.intrP = start
	if IsPaced(cfg.Rhythm) && cfg.PaceMode == PaceDemand && cfg.Rhythm == RPaced {
		p.nextV = math.Max(start, p.lastV+60/p.rateFor(cfg))
	}
	if cfg.Rhythm == RAflutter && cfg.FlRatio != FlAuto {
		p.flFreq = p.rate() / 60 // rate field = atrial flutter rate
		p.flN = map[int]int{Fl2to1: 2, Fl3to1: 3, Fl4to1: 4, Fl5to1: 5, FlVariable: 3}[cfg.FlRatio]
	} else if cfg.Rhythm == RAflutter {
		best := 2
		for n := 2; n <= 6; n++ {
			if math.Abs(p.rate()*float64(n)/60-5) < math.Abs(p.rate()*float64(best)/60-5) {
				best = n
			}
		}
		p.flN = best
		p.flFreq = p.rate() * float64(best) / 60
	}
}

func keepUpTo(s []float64, t float64) []float64 {
	o := s[:0]
	for _, v := range s {
		if v <= t {
			o = append(o, v)
		}
	}
	return o
}

func (p *Patient) push(e event) {
	if e.amp == 0 {
		e.amp = 1
	}
	p.ev = append(p.ev, e)
}

func (p *Patient) beat(t float64, k kind, v uint8, amp float64) {
	if p.Cfg.Tune.Natural {
		// QRS height sways with breathing (electrical axis shifts as the
		// chest moves) plus a little random beat-to-beat variation
		ph := 0.0
		if p.breathPer > 0 {
			ph = 2 * math.Pi * (t - p.breathStart) / p.breathPer
		}
		amp *= 1 + 0.07*math.Sin(ph) + 0.025*p.rng.NormFloat64()
	}
	p.push(event{t: t, k: k, v: v, amp: amp, rr: 60 / p.ventRate()})
	p.beats = append(p.beats, t)
	if k == kPaced {
		p.paces = append(p.paces, t-0.05)
	}
	// pleth pulse: early beats (short filling time) give a smaller pulse
	nom := 60 / p.ventRate()
	pa := clamp(0.25+0.75*(t-p.lastV)/nom, 0.15, 1.3)
	if k == kVT {
		pa *= 0.4
	}
	p.pulses = append(p.pulses, pulse{t + 0.22, pa})
	if t > p.lastV {
		p.lastV = t
	}
}

func (p *Patient) schedule(until float64) {
	inf := math.Inf(1)
	for {
		ta, tv := inf, inf
		if p.atrialStream() {
			ta = p.nextP
		}
		if p.ventStream() {
			tv = p.nextV
		}
		if math.Min(ta, tv) > until {
			return
		}
		if ta <= tv {
			p.doP()
		} else {
			p.doV()
		}
	}
}

// intrinsicPP returns the next intrinsic P-P interval under a demand pacemaker.
// The intrinsic rate drifts slowly so pacing engages intermittently.
func (p *Patient) intrinsicPP(t float64) float64 {
	ir := float64(p.Cfg.IntrRate)
	if ir < 20 {
		ir = 20
	}
	return 60 / ir * (1 + 0.10*math.Sin(2*math.Pi*t/28+p.ph[1]) + 0.015*(2*p.rng.Float64()-1))
}

// doPacedA handles AAI and DDD pacing: an atrial event (sensed or paced),
// then either intrinsic conduction (AAI) or a paced ventricle (DDD).
func (p *Patient) doPacedA() {
	la := 60 / p.rate()
	tPace := p.lastA + la
	demand := p.Cfg.PaceMode == PaceDemand
	var tA float64 // atrial event time (spike, or intrinsic P onset)
	paced := true
	if demand && p.intrP < tPace {
		tA, paced = p.intrP, false
		p.push(event{t: tA + 0.05, k: kP})
	} else {
		tA = tPace
		p.paces = append(p.paces, tA)
		p.push(event{t: tA + 0.06, k: kP, amp: 1.1})
	}
	p.lastA = tA
	p.intrP = tA + p.intrinsicPP(tA)
	if demand {
		p.nextP = math.Min(p.intrP, tA+la)
	} else {
		p.nextP = tA + la
	}
	if p.Cfg.Rhythm == RAPaced {
		pr := p.prOverride(0.16)
		if paced {
			pr += 0.04 // spike to QRS onset
		}
		tR := tA + pr + 0.04
		if tR < p.lastV+p.refractory() {
			return
		}
		p.beat(tR, kNarrow, 0, 1)
		p.afterDominant(tR, la, pr)
		return
	}
	// DDD: AV delay then ventricular pace (intrinsic AV conduction assumed slow)
	avd := 0.15
	if paced {
		avd = 0.18
	}
	tR := tA + avd + 0.05
	if tR < p.lastV+p.refractory() {
		return
	}
	p.beat(tR, kPaced, 0, 1)
	p.afterDominant(tR, la, avd)
}

func (p *Patient) doP() {
	r := p.Cfg.Rhythm
	if r == RAPaced || r == RAVPaced {
		p.doPacedA()
		return
	}
	tP := p.nextP
	atrialRate := p.rate()
	switch r {
	case RMobitzI:
		atrialRate = p.rate() * 4 / 3
	case RMobitzII:
		atrialRate = p.rate() * 3 / 2
	case RThirdDegree:
		atrialRate = 78
	}
	pp := 60 / atrialRate
	switch r {
	case RPaced: // intrinsic sinus under a demand VVI pacer
		pp = p.intrinsicPP(tP)
	case RSinusArrhythmia:
		pp *= 1 + 0.17*math.Sin(2*math.Pi*tP/4.3+p.ph[0])
	default:
		pp *= 1 + 0.012*(2*p.rng.Float64()-1)
	}
	p.nextP = tP + pp
	p.push(event{t: tP, k: kP})

	conduct, pr := true, p.prOverride(0.16)
	switch r {
	case RFirstDegree:
		pr = p.prOverride(0.30)
	case RMobitzI:
		i := p.pIdx % 4
		p.pIdx++
		if i == 3 {
			conduct = false
		} else {
			pr = p.prOverride(0.18) + []float64{0, 0.11, 0.17}[i]
		}
	case RMobitzII:
		i := p.pIdx % 3
		p.pIdx++
		pr = p.prOverride(0.18)
		if i == 2 {
			conduct = false
		}
	case RThirdDegree:
		conduct = false
	}
	if !conduct {
		return
	}
	if p.Cfg.Tune.Natural {
		pr += 0.004 * p.rng.NormFloat64() // a few ms of PR jitter
	}
	tR := tP - 0.05 + pr + 0.04
	if tR < p.lastV+p.refractory() {
		return // ventricle refractory (e.g. after a PVC) -> compensatory pause
	}
	if r == RPaced && tR > p.lastV+60/p.rate() {
		return // VVI pacer escape interval expires first; the pacer captures
	}
	p.beat(tR, kNarrow, 0, 1)
	p.afterDominant(tR, 60/p.rate(), pr)
}

func (p *Patient) doV() {
	r := p.Cfg.Rhythm
	t := p.nextV
	rr := 60 / p.rate()
	jit := func(a float64) float64 { return 1 + a*(2*p.rng.Float64()-1) }
	switch r {
	case RAfib:
		p.beat(t, kNarrow, 0, 0.88+0.22*p.rng.Float64())
		n := rr * (0.55 + 0.9*p.rng.Float64())
		p.nextV = t + math.Max(n, 0.30)
	case RAflutter:
		p.beat(t, kNarrow, 0, 1)
		n := p.flN
		if p.Cfg.FlRatio == FlVariable {
			n = []int{2, 2, 3, 3, 4, 4, 4}[p.rng.Intn(7)]
		}
		rr = float64(p.flN) / p.flFreq
		p.nextV = t + float64(n)/p.flFreq
	case RSVT:
		p.beat(t, kNarrow, 0, 0.95)
		p.nextV = t + rr*jit(0.004)
	case RJunctional:
		p.push(event{t: t - 0.10, k: kPInv})
		p.beat(t, kNarrow, 0, 1)
		p.nextV = t + rr*jit(0.01)
	case RIdioventricular:
		p.beat(t, kWide, 0, 1)
		p.nextV = t + rr*jit(0.015)
	case RThirdDegree:
		if p.rate() >= 40 {
			p.beat(t, kNarrow, 0, 1)
		} else {
			p.beat(t, kWide, 0, 1)
		}
		p.nextV = t + rr*jit(0.01)
	case RVTach:
		p.beat(t, kVT, 0, 1)
		p.nextV = t + rr*jit(0.008)
		return
	case RPaced:
		if p.Cfg.PaceMode == PaceDemand && t < p.lastV+rr-1e-6 {
			p.nextV = p.lastV + rr // an intrinsic beat was sensed: reset the timer
			return
		}
		p.beat(t, kPaced, 0, 1)
		p.nextV = t + rr
	}
	p.afterDominant(t, rr, 0.16)
}

func (p *Patient) ectopicCount() int {
	c := p.Cfg
	info := Rhythms[c.Rhythm]
	switch c.Ectopy {
	case EctNone:
		return 0
	case EctPVCUni, EctPVCMulti:
		if !info.Ectopy {
			return 0
		}
	case EctPAC, EctPJC:
		if !info.AtrialEctopy {
			return 0
		}
	}
	switch c.Pattern {
	case PatBigeminy:
		return 1
	case PatTrigeminy, PatQuadrigeminy:
		need := 2
		if c.Pattern == PatQuadrigeminy {
			need = 3
		}
		p.domCount++
		if p.domCount >= need {
			p.domCount = 0
			return 1
		}
		return 0
	}
	prob := float64(c.Freq) / p.ventRate()
	if prob > 0.9 {
		prob = 0.9
	}
	if p.rng.Float64() >= prob {
		return 0
	}
	switch c.Pattern {
	case PatCouplets:
		return 2
	case PatRuns:
		return 3
	}
	return 1
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// afterDominant inserts ectopic beats after a dominant beat at tR.
func (p *Patient) afterDominant(tR, rr, pr float64) {
	n := p.ectopicCount()
	if n == 0 {
		return
	}
	r := p.Cfg.Rhythm
	switch p.Cfg.Ectopy {
	case EctPVCUni, EctPVCMulti:
		t := tR + clamp(0.5*rr, 0.22, 0.56)
		last := t
		for i := 0; i < n; i++ {
			var v uint8
			if p.Cfg.Ectopy == EctPVCMulti {
				v = uint8(p.rng.Intn(3))
			}
			p.beat(t, kPVC, v, 1)
			p.pvcs = append(p.pvcs, t)
			last = t
			t += 0.40
		}
		switch r {
		case RAfib:
			p.nextV = last + rr*(0.9+0.5*p.rng.Float64())
		case RAflutter:
			k := math.Ceil((last + 0.45 - tR) / rr)
			if k < 2 {
				k = 2
			}
			p.nextV = tR + k*rr
		default:
			if p.ventStream() {
				p.nextV = math.Max(p.nextV, last+rr)
			}
		}
	case EctPAC, EctPJC:
		coupling := clamp(0.62*rr, 0.34, 0.75)
		t := tR + coupling // ectopic R time
		last := t
		for i := 0; i < n; i++ {
			if p.Cfg.Ectopy == EctPAC {
				p.push(event{t: t - 0.13, k: kPEct})
			} else {
				p.push(event{t: t + 0.10, k: kPInv, amp: 0.8})
			}
			p.beat(t, kNarrow, 0, 0.97)
			last = t
			t += math.Max(0.36, 0.6*rr)
		}
		if p.atrialStream() {
			// ectopic atrial/junctional beat resets the sinus node
			p.nextP = last - (pr - 0.01) + 60/p.atrialRateNominal()
			p.pIdx = 0
		}
		if p.ventStream() {
			p.nextV = last + rr
		}
	}
}

// refractory is how long after a ventricular beat a conducted impulse is
// blocked. It scales with the fastest expected conducted rate so fast sinus
// rhythms are never blocked, while a PVC still blocks the next sinus beat
// (giving the compensatory pause).
func (p *Patient) refractory() float64 {
	pp := 60 / p.atrialRateNominal()
	if p.Cfg.Rhythm == RPaced && p.Cfg.PaceMode == PaceDemand {
		ir := float64(p.Cfg.IntrRate)
		if ir < 20 {
			ir = 20
		}
		pp = math.Min(pp, 60/ir)
	}
	return math.Min(0.42, 0.75*pp)
}

// ventRate is the nominal ventricular rate (bpm).
func (p *Patient) ventRate() float64 {
	if p.Cfg.Rhythm == RAflutter && p.flN > 0 {
		return p.flFreq * 60 / float64(p.flN)
	}
	return p.rate()
}

func (p *Patient) atrialRateNominal() float64 {
	switch p.Cfg.Rhythm {
	case RMobitzI:
		return p.rate() * 4 / 3
	case RMobitzII:
		return p.rate() * 3 / 2
	case RThirdDegree:
		return 78
	}
	return p.rate()
}

func g(dt, mu, sig, a float64) float64 {
	x := (dt - mu) / sig
	if x > 6 || x < -6 {
		return 0
	}
	return a * math.Exp(-0.5*x*x)
}

func ag(dt, mu, sl, sr, a float64) float64 {
	if dt < mu {
		return g(dt, mu, sl, a)
	}
	return g(dt, mu, sr, a)
}

// smooth step from 0 to 1 centred on x=0 with half-width w
func sstep(x, w float64) float64 { return 0.5 + 0.5*math.Tanh(x/w) }

func morph(e *event, dt float64, sh *shape, ld *leadP) float64 {
	q := sh.qrs * ld.wide
	switch e.k {
	case kP:
		v := g(dt, 0, 0.022, 0.15)
		if ld.pBi { // V1: biphasic P
			v = g(dt, -0.01, 0.016, 0.15) + g(dt, 0.03, 0.016, -0.12)
		}
		return sh.p * ld.p * v
	case kPEct:
		return sh.p * ld.p * (g(dt, 0, 0.017, 0.14) + g(dt, 0.032, 0.014, -0.05))
	case kPInv:
		return sh.p * ld.p * e.amp * g(dt, 0, 0.02, -0.11)
	case kNarrow:
		qt := clamp(0.40*math.Sqrt(e.rr), 0.24, 0.46)
		if sh.qt > 0 {
			qt = sh.qt
		}
		sl := 0.03 + 0.035*clamp(e.rr/0.8, 0.4, 1)
		w := sh.w
		v := sh.qrs * e.amp * (g(dt, -0.03*w, 0.009*w, -0.10*ld.q) + g(dt, 0, 0.011*w, 1.3*ld.r) + g(dt, 0.027*w, 0.010*w, -0.32*ld.s))
		tp := qt - 0.11 + 0.04*(w-1) // T follows a wider QRS
		v += sh.t * ld.t * ag(dt, tp, sl, 0.04, 0.30)
		if sh.st != 0 { // ST shift: from the J point through the T wave
			j := 0.045 * w
			v += sh.st * sstep(dt-j, 0.008) * sstep(tp+0.05-dt, 0.03)
		}
		return v
	case kWide:
		return q * (g(dt, 0, 0.030, 1.0) + g(dt, 0.065, 0.028, -0.45) + g(dt, 0.32, 0.07, -0.35))
	case kPVC:
		switch e.v {
		case 1:
			return q * (g(dt, 0, 0.032, -1.35) + g(dt, 0.065, 0.03, 0.45) + g(dt, 0.33, 0.07, 0.5))
		case 2:
			return q * (g(dt, -0.02, 0.024, 0.95) + g(dt, 0.04, 0.028, 1.15) + g(dt, 0.10, 0.025, -0.3) + g(dt, 0.35, 0.075, -0.45))
		}
		return q * (g(dt, 0, 0.035, 1.6) + g(dt, 0.075, 0.03, -0.35) + g(dt, 0.33, 0.075, -0.55))
	case kVT:
		return q * (g(dt, 0, 0.04, 1.45) + g(dt, 0.08, 0.035, -0.55) + g(dt, math.Min(0.27, 0.62*e.rr), 0.06, -0.5))
	case kPaced:
		return q * (g(dt, 0.01, 0.034, 1.25) + g(dt, 0.08, 0.03, -0.4) + g(dt, 0.33, 0.07, -0.4))
	}
	return 0
}

func (p *Patient) value(t float64) float64 {
	if p.Cfg.Rhythm == RLeadsOff {
		return 0 // disconnected: perfectly flat, no noise or wander
	}
	v := 0.0
	for i := range p.ev {
		e := &p.ev[i]
		dt := t - e.t
		if dt < -0.35 || dt > 0.8 {
			continue
		}
		v += morph(e, dt, &p.sh, &p.ld)
	}
	tw := 2 * math.Pi
	ts := t - p.switchT
	switch p.Cfg.Rhythm {
	case RAfib:
		v += p.ld.af * (0.035*math.Sin(tw*5.3*t+p.ph[0]) + 0.03*math.Sin(tw*6.7*t+p.ph[1]) +
			0.025*math.Sin(tw*4.4*t+p.ph[2]) + 0.02*math.Sin(tw*8.1*t+p.ph[3])) *
			(0.8 + 0.3*math.Sin(tw*0.3*t))
	case RAflutter:
		ph := ts*p.flFreq + 0.3
		f := ph - math.Floor(ph)
		if f < 0.78 {
			v += p.ld.fl * (0.13 - 0.30*f/0.78)
		} else {
			v += p.ld.fl * (-0.17 + 0.30*(f-0.78)/0.22)
		}
	case RVFibCoarse, RVFibFine:
		a := 0.75
		if p.Cfg.Rhythm == RVFibFine {
			a = 0.17
		}
		env := a * (0.75 + 0.25*math.Sin(tw*0.23*t+p.ph[0]) + 0.15*math.Sin(tw*0.61*t+p.ph[1]))
		v += p.ld.vf * env * (0.6*math.Sin(tw*4.6*t+1.8*math.Sin(tw*0.4*t+p.ph[2])) +
			0.35*math.Sin(tw*6.3*t+p.ph[3]) + 0.2*math.Sin(tw*2.9*t+p.ph[4]))
	case RTorsades:
		f := p.rate() / 60
		env := math.Sin(tw*ts/3.2 + 0.4)
		v += p.ld.vf * 1.5 * env * (math.Sin(tw*f*t) + 0.35*math.Sin(2*tw*f*t+0.7))
	}
	// baseline wander + noise
	v += 0.04*math.Sin(tw*0.21*t+p.ph[5]) + 0.025*math.Sin(tw*0.07*t)
	v += 0.010 * p.rng.NormFloat64()
	return p.artifact(t, v)
}

// artifact adds the selected ECG artifact to signal v at time t.
func (p *Patient) artifact(t, v float64) float64 {
	if p.Cfg.Artifact == ArtNone {
		return v
	}
	L := []float64{0.5, 1.0, 1.8}[p.Cfg.ArtLevel%3]
	tw := 2 * math.Pi
	r := p.rng
	switch p.Cfg.Artifact {
	case ArtTremor: // irregular high-frequency fuzz, waxing and waning
		mod := 0.6 + 0.4*math.Sin(tw*0.17*t+p.ph[2])
		v += L * mod * (0.07*r.NormFloat64() + 0.04*math.Sin(tw*31*t+p.ph[3]) + 0.03*math.Sin(tw*43*t))
	case Art60Hz: // regular fine oscillation thickening the baseline
		v += L * 0.06 * math.Sin(tw*60*t)
	case ArtWander: // slow, large baseline swings
		v += L * (0.45*math.Sin(tw*0.24*t+p.ph[4]) + 0.25*math.Sin(tw*0.09*t+p.ph[1]))
	case ArtMotion: // bursts of large irregular deflections (can mimic VT/VF)
		if t >= p.artOff && t >= p.artNext {
			p.artOn = t
			p.artOff = t + 1.2 + 2.5*r.Float64()
			p.artNext = p.artOff + (4+10*r.Float64())/L
			p.artF = 3 + 4*r.Float64()
		}
		if t < p.artOff {
			x := (t - p.artOn) / (p.artOff - p.artOn)
			env := math.Sin(math.Pi * x)
			v += L * env * (0.7*math.Sin(tw*p.artF*t+1.5*math.Sin(tw*0.9*t)) +
				0.35*math.Sin(tw*p.artF*1.7*t+p.ph[0]) + 0.15*r.NormFloat64() + 0.4*math.Sin(tw*0.6*t))
		}
	case ArtLoose: // sharp spikes, jumps and brief signal dropouts
		if t >= p.artOff && t >= p.artNext {
			p.artOn = t
			p.artOff = t + 0.15 + 0.5*r.Float64()
			p.artNext = p.artOff + (2+5*r.Float64())/L
			p.artLevel = (2*r.Float64() - 1) * 1.2 * L
		}
		if t < p.artOff {
			return p.artLevel + 0.03*r.NormFloat64() // contact lost: signal replaced
		}
		if r.Float64() < 0.004*L {
			v += (2*r.Float64() - 1) * 2.0 * L // sharp spike
		}
	}
	return v
}

// Update generates samples up to time now and refreshes HR/alarms once a second.
func (p *Patient) Update(now float64) {
	kmax := int64((now - p.startT) * FS)
	if kmax-p.gen > bufLen {
		p.gen = kmax - bufLen
	}
	for ; p.gen <= kmax; p.gen++ {
		t := p.startT + float64(p.gen)/FS
		p.schedule(t + 1.5)
		p.buf[p.gen%bufLen] = float32(p.value(t))
		p.pbuf[p.gen%bufLen] = float32(p.plethValue(t))
		p.rbuf[p.gen%bufLen] = float32(p.respValue(t))
		p.abuf[p.gen%bufLen] = float32(p.abpValue(t))
		p.cbuf[p.gen%bufLen] = float32(p.co2Value(t))
		// extra leads (same beats, different lead projection)
		saved := p.ld
		for i, n := range ExtraLeads {
			p.ld = leadTable[n]
			p.ebuf[i][p.gen%bufLen] = float32(p.value(t))
		}
		p.ld = saved
	}
	// prune
	cut := 0
	for cut < len(p.ev) && p.ev[cut].t < now-2 {
		cut++
	}
	if cut > 0 {
		// events are mostly time ordered; keep any that are still recent
		keep := p.ev[:0]
		for _, e := range p.ev {
			if e.t >= now-2 {
				keep = append(keep, e)
			}
		}
		p.ev = keep
	}
	p.beats = pruneBefore(p.beats, now-15)
	p.pvcs = pruneBefore(p.pvcs, now-60)
	p.paces = pruneBefore(p.paces, now-15)
	p.breaths = pruneBefore(p.breaths, now-90)
	i := 0
	for i < len(p.pulses) && p.pulses[i].t < now-8 {
		i++
	}
	p.pulses = append(p.pulses[:0], p.pulses[i:]...)

	if now-p.lastHRUpd >= 1 {
		p.lastHRUpd = now
		p.computeHR(now)
	}
}

func pruneBefore(s []float64, t float64) []float64 {
	i := 0
	for i < len(s) && s[i] < t {
		i++
	}
	if i == 0 {
		return s
	}
	return append(s[:0], s[i:]...)
}

func (p *Patient) computeHR(now float64) {
	var past []float64
	for _, b := range p.beats {
		if b <= now && b > now-10 {
			past = append(past, b)
		}
	}
	r := p.Cfg.Rhythm
	p.HRValid = !(r == RVFibCoarse || r == RVFibFine || r == RTorsades || r == RLeadsOff)
	if len(past) < 2 || now-past[len(past)-1] > 4.0 {
		p.HR = 0
	} else {
		n := len(past) - 1
		if n > 8 {
			n = 8
		}
		span := past[len(past)-1] - past[len(past)-1-n]
		p.HR = int(math.Round(60 * float64(n) / span))
	}
	pvcMin := 0
	for _, t := range p.pvcs {
		if t <= now {
			pvcMin++
		}
	}
	p.PVCMin = pvcMin
	// SpO2 numeric: valid while pulses are being detected
	p.SpO2Valid = false
	p.Pulse = 0
	if p.Cfg.SpO2On == SpO2Monitored {
		var pt []float64
		for _, q := range p.pulses {
			if q.t <= now && q.t > now-10 {
				pt = append(pt, q.t)
			}
			if q.t <= now && q.t > now-5 {
				p.SpO2Valid = true
			}
		}
		// pulse rate from the pleth (like the "Pulse" numeric on a monitor)
		if p.SpO2Valid && len(pt) >= 2 {
			n := len(pt) - 1
			if n > 8 {
				n = 8
			}
			p.Pulse = int(math.Round(60 * float64(n) / (pt[len(pt)-1] - pt[len(pt)-1-n])))
		}
		if p.SpO2Disp == 0 || p.SpO2Disp < p.Cfg.SpO2-1 || p.SpO2Disp > p.Cfg.SpO2+1 {
			p.SpO2Disp = p.Cfg.SpO2
		} else if p.rng.Float64() < 0.25 {
			p.SpO2Disp = p.Cfg.SpO2 + p.rng.Intn(3) - 1
		}
		if p.SpO2Disp > 100 {
			p.SpO2Disp = 100
		}
	}

	// Collect alarm candidates; show the highest priority one.
	// Priority: red (3) > yellow (2) > INOP (4) > short yellow (1).
	type cand struct {
		txt, det string
		lvl      int
	}
	var cs []cand
	add := func(t string, l int, det ...string) {
		d := ""
		if len(det) > 0 {
			d = det[0]
		}
		cs = append(cs, cand{t, d, l})
	}
	lt := func(v, lim int) string { return fmt.Sprintf(" %d <%d", v, lim) }
	gt := func(v, lim int) string { return fmt.Sprintf(" %d >%d", v, lim) }
	lm := p.lim()
	p.updateNBP(now)
	p.measureVitals(now)
	p.computeRR(now)
	ecgOn := !p.Cfg.CardiacOff
	switch {
	case !ecgOn:
		// cardiac monitoring hidden: no ECG alarms
	case r == RLeadsOff:
		add("ECG Leads Off", 4)
	case !p.HRValid:
		add("***VFib/VTach", 3)
	case p.HR == 0:
		add("***Asystole", 3)
	case r == RVTach:
		add("***VTach", 3)
	case p.HR < lm.HRExtLow:
		add("***xBrady", AlarmPanic, lt(p.HR, lm.HRExtLow))
	case p.HR > lm.HRExtHigh:
		add("***xTachy", AlarmPanic, gt(p.HR, lm.HRExtHigh))
	case pvcMin > 10:
		add("**PVCs/min High", 2, gt(pvcMin, 10))
	case p.HR > lm.HRHigh:
		add("**HR High", 2, gt(p.HR, lm.HRHigh))
	case p.HR < lm.HRLow:
		add("**HR Low", 2, lt(p.HR, lm.HRLow))
	case r == RSinusArrhythmia || r == RMobitzI || r == RMobitzII ||
		(r == RAflutter && p.Cfg.FlRatio == FlVariable):
		// irregular rhythms other than AF (AF is labelled "Atrial Fib" instead)
		add("*Irregular HR", 1)
	}
	// Without a usable ECG (cardiac monitoring off, or leads off) the pulse
	// rate from the oximeter is the heart-rate alarm source, as on a monitor.
	if (!ecgOn || r == RLeadsOff) && p.Cfg.SpO2On == SpO2Monitored && p.SpO2Valid {
		switch {
		case p.Pulse < lm.HRExtLow:
			add("***xBrady (Pulse)", AlarmPanic, lt(p.Pulse, lm.HRExtLow))
		case p.Pulse > lm.HRExtHigh:
			add("***xTachy (Pulse)", AlarmPanic, gt(p.Pulse, lm.HRExtHigh))
		case p.Pulse > lm.HRHigh:
			add("**Pulse High", 2, gt(p.Pulse, lm.HRHigh))
		case p.Pulse < lm.HRLow:
			add("**Pulse Low", 2, lt(p.Pulse, lm.HRLow))
		}
	}
	switch p.Cfg.SpO2On {
	case SpO2SensorOff:
		add("SpO2 Sensor Off", 4)
	case SpO2Monitored:
		switch {
		case !p.SpO2Valid:
			add("SpO2 Non-Pulsatile", 4)
		case p.SpO2Disp < lm.SpO2ExtLow:
			add("***Desat", AlarmPanic, lt(p.SpO2Disp, lm.SpO2ExtLow))
		case p.SpO2Disp < lm.SpO2Low:
			add("**SpO2 Low", 2, lt(p.SpO2Disp, lm.SpO2Low))
		}
	}
	if p.Cfg.RespAlarms() {
		switch {
		case !p.RRValid:
			if !ecgOn {
				add("Resp Leads Off", 4)
			}
		case p.ApneaNow:
			add("***Apnea", 3)
		case p.RRDisp > lm.RRHigh:
			add("**RR High", 2, gt(p.RRDisp, lm.RRHigh))
		case p.RRDisp > 0 && p.RRDisp < lm.RRLow:
			add("**RR Low", 2, lt(p.RRDisp, lm.RRLow))
		}
	}
	// ST / QT (from the displayed lead)
	if ecgOn && p.stMeasurable() {
		st := p.STDisplay()
		key := "**ST-" + p.Cfg.LeadName()
		switch {
		case st > lm.STHigh:
			add(key, 2, fmt.Sprintf(" %+.1f >%+.1f", st, lm.STHigh))
		case st < lm.STLow:
			add(key, 2, fmt.Sprintf(" %+.1f <%+.1f", st, lm.STLow))
		}
		if _, qc := p.QTDisplay(); qc > lm.QTcHigh {
			add("**QTc High", 2, gt(qc, lm.QTcHigh))
		}
	}
	// NBP (last reading)
	if p.Cfg.NBPMode != NBPOff {
		switch {
		case p.NBPFailed:
			add("NBP Measure Failed", 4)
		case p.NBPValid && p.NBPSys > lm.NBPsHigh:
			add("**NBPs High", 2, gt(p.NBPSys, lm.NBPsHigh))
		case p.NBPValid && p.NBPSys < lm.NBPsLow:
			add("**NBPs Low", 2, lt(p.NBPSys, lm.NBPsLow))
		}
	}
	// ABP
	if p.Cfg.ABPOn {
		switch {
		case !p.ABPValid:
			add("ABP Non-Pulsatile", 4)
		case p.ABPSys > lm.ABPsHigh:
			add("**ABPs High", 2, gt(p.ABPSys, lm.ABPsHigh))
		case p.ABPSys < lm.ABPsLow:
			add("**ABPs Low", 2, lt(p.ABPSys, lm.ABPsLow))
		}
	}
	// EtCO2 (apnea is detected from the capnogram as well)
	if p.Cfg.CO2On {
		switch {
		case (p.EtCO2Disp == 0 || p.AwRR == 0) && now-p.switchT > 15:
			add("***Apnea", 3)
		case p.EtCO2Disp > lm.EtCO2High:
			add("**etCO2 High", 2, gt(p.EtCO2Disp, lm.EtCO2High))
		case p.EtCO2Disp < lm.EtCO2Low:
			add("**etCO2 Low", 2, lt(p.EtCO2Disp, lm.EtCO2Low))
		}
	}
	rank := map[int]int{AlarmPanic: 5, 3: 4, 2: 3, 4: 2, 1: 1}
	p.AlarmText, p.AlarmKey, p.AlarmLvl = "", "", 0
	best := 0
	for _, c := range cs {
		if rank[c.lvl] > best {
			best = rank[c.lvl]
			p.AlarmText, p.AlarmKey, p.AlarmLvl = c.txt+c.det, c.txt, c.lvl
		}
	}
}

// respValue synthesizes the impedance respiration wave: a quicker inspiration
// rise and an exponential expiration, with breath-to-breath variability.
func (p *Patient) respValue(t float64) float64 {
	if p.Cfg.Apnea {
		p.nextBreath = t + 1
		if p.Cfg.Rhythm == RLeadsOff {
			return 0
		}
		return 0.03*math.Sin(2*math.Pi*p.ventRate()/60*t) + 0.01*p.rng.NormFloat64()
	}
	for t >= p.nextBreath {
		p.newBreath()
	}
	if p.Cfg.Rhythm == RLeadsOff {
		return 0 // impedance resp is measured through the ECG electrodes
	}
	x := (t - p.breathStart) / p.breathPer
	f := p.breathF
	var v float64
	if x < f { // inspiration: smooth rise, a little faster at the start
		u := x / f
		v = 0.5 - 0.5*math.Cos(math.Pi*math.Pow(u, 0.85))
	} else { // passive expiration, then an end-expiratory pause
		k := p.respK
		v = (math.Exp(-k*(x-f)/(1-f)) - math.Exp(-k)) / (1 - math.Exp(-k))
	}
	v *= p.breathAmp
	// slow baseline (lung volume) drift and the small cardiac ripple that
	// impedance pneumography picks up from each heartbeat
	v += 0.07 * math.Sin(2*math.Pi*t/p.respBaseT+p.respBasePh)
	v += 0.03 * math.Sin(2*math.Pi*p.ventRate()/60*t+p.ph[2])
	return v + 0.01*p.rng.NormFloat64()
}

// newBreath starts the next breath with natural breath-to-breath variation:
// a slowly drifting rate and depth (deeper breaths tend to be slower), small
// changes in timing, an occasional sigh and the odd shallow breath.
func (p *Patient) newBreath() {
	r := p.rng
	p.breathStart = p.nextBreath
	p.respDrift = 0.8*p.respDrift + 0.6*r.NormFloat64()*0.6
	per := 60 / float64(max(p.Cfg.RR, 1)) * (1 + 0.06*p.respDrift + 0.04*r.NormFloat64())
	amp := p.respDepth * (1 + 0.07*p.respDrift + 0.07*r.NormFloat64())
	switch x := r.Float64(); {
	case x < per/100: // a sigh roughly every 1.5-2 minutes
		amp *= 1.7 + 0.3*r.Float64()
		per *= 1.3 + 0.2*r.Float64()
	case x < per/100+0.04: // a shallow, quicker breath
		amp *= 0.5 + 0.15*r.Float64()
		per *= 0.75
	}
	p.breathPer = clamp(per, 0.6, 30)
	p.breathAmp = clamp(amp, 0.25, 2.2)
	p.breathF = clamp(p.respIE*(1+0.08*r.NormFloat64()), 0.24, 0.52)
	p.breathEt = 1 + 0.025*r.NormFloat64()
	p.nextBreath = p.breathStart + p.breathPer
	p.breaths = append(p.breaths, p.breathStart)
}

// ExtraLeads are recorded continuously so the expanded patient view can show
// several leads at once without waiting for the buffers to fill.
var ExtraLeads = []string{"II", "V1", "III"}

// LeadAt returns sample k of the named lead: the displayed lead's own buffer,
// or one of the ExtraLeads. ok is false if that lead is not recorded.
func (p *Patient) LeadAt(name string, k int64) (v float32, ok bool) {
	kk := ((k % bufLen) + bufLen) % bufLen
	if name == p.Cfg.LeadName() {
		return p.buf[kk], true
	}
	for i, n := range ExtraLeads {
		if n == name {
			return p.ebuf[i][kk], true
		}
	}
	return 0, false
}

// QTDisplay returns the measured QT and Bazett QTc in ms (0 if not measurable).
func (p *Patient) QTDisplay() (qt, qtc int) {
	if !p.HRValid || p.HR <= 0 || p.Cfg.CardiacOff {
		return 0, 0
	}
	switch p.Cfg.Rhythm {
	case RLeadsOff, RAsystole, RVTach, RTorsades, RVFibCoarse, RVFibFine:
		return 0, 0
	}
	rr := 60 / float64(p.HR)
	q := clamp(0.40*math.Sqrt(60/p.ventRate()), 0.24, 0.46)
	if p.sh.qt > 0 {
		q = p.sh.qt
	}
	return int(math.Round(q * 1000)), int(math.Round(q / math.Sqrt(rr) * 1000))
}

// STDisplay returns the ST deviation in mm for the displayed lead.
func (p *Patient) STDisplay() float64 { return math.Round(p.sh.st*100) / 10 }

// RespAt returns respiration sample k.
func (p *Patient) RespAt(k int64) float32 { return p.rbuf[((k%bufLen)+bufLen)%bufLen] }

// lim returns the alarm limits in force for this patient.
func (p *Patient) lim() AlarmLimits {
	if p.Lim != nil {
		return *p.Lim
	}
	return Limits
}

// computeRR updates the displayed respiratory rate and apnea state.
func (p *Patient) computeRR(now float64) {
	p.RRValid = p.Cfg.Rhythm != RLeadsOff
	var past []float64
	for _, b := range p.breaths {
		if b <= now && b > now-60 {
			past = append(past, b)
		}
	}
	last := p.switchT // apnea timer starts when the bed was (re)configured
	if len(past) > 0 && past[len(past)-1] > last {
		last = past[len(past)-1]
	}
	p.ApneaNow = now-last > float64(p.lim().ApneaSec)
	switch {
	case p.ApneaNow:
		p.RRDisp = 0
	case len(past) < 2:
		p.RRDisp = p.Cfg.RR // not enough breaths yet: show the set rate
	default:
		n := len(past) - 1
		if n > 5 {
			n = 5
		}
		span := past[len(past)-1] - past[len(past)-1-n]
		p.RRDisp = int(math.Round(60 * float64(n) / span))
	}
}

// plethValue synthesizes the pulse-oximeter plethysmogram (arbitrary units, 0..~1.3).
func (p *Patient) plethValue(t float64) float64 {
	if p.Cfg.SpO2On != SpO2Monitored {
		return 0
	}
	v := 0.0
	for _, q := range p.pulses {
		dt := t - q.t
		if dt < -0.2 || dt > 1.2 {
			continue
		}
		w := q.amp * (ag(dt, 0.10, 0.045, 0.17, 1.0) + g(dt, 0.37, 0.05, 0.14))
		if w > v {
			v = w
		}
	}
	if p.Cfg.Artifact == ArtMotion && t < p.artOff { // movement also disturbs the pleth
		v += 0.35 * math.Sin(2*math.Pi*p.artF*0.7*t) * math.Sin(math.Pi*(t-p.artOn)/(p.artOff-p.artOn))
	}
	return v + 0.006*p.rng.NormFloat64()
}

// PlethAt returns the pleth sample k.
func (p *Patient) PlethAt(k int64) float32 { return p.pbuf[((k%bufLen)+bufLen)%bufLen] }

// PacesBetween returns pacer spike times in [t0, t1].
func (p *Patient) PacesBetween(t0, t1 float64) []float64 {
	var o []float64
	for _, x := range p.paces {
		if x >= t0 && x <= t1 {
			o = append(o, x)
		}
	}
	return o
}

// Sample access for display.
func (p *Patient) Last() int64            { return p.gen - 1 }
func (p *Patient) TimeOf(k int64) float64 { return p.startT + float64(k)/FS }
func (p *Patient) At(k int64) float32     { return p.buf[((k%bufLen)+bufLen)%bufLen] }

// Snapshot returns the most recent `secs` seconds of samples.
func (p *Patient) Snapshot(secs float64) []float32 {
	n := int64(secs * FS)
	out := make([]float32, n)
	last := p.Last()
	for i := int64(0); i < n; i++ {
		out[i] = p.At(last - n + 1 + i)
	}
	return out
}
