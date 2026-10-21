package main

// Scenario generators for the Patients menu.

import (
	"math"
	"math/rand"
)

func randIn(r *rand.Rand, lo, hi int) int {
	if hi <= lo {
		return lo
	}
	return lo + r.Intn(hi-lo+1)
}

// RandomConfig picks any rhythm (including lethal ones) with a plausible rate
// and, sometimes, random ectopy. The patient name is kept.
func RandomConfig(r *rand.Rand, name string) Config {
	c := Config{Name: name, SpO2: 98, RR: randIn(r, 8, 32)}
	if r.Float64() < 0.07 {
		c.Apnea = true
	}
	if r.Float64() < 0.7 {
		c.SpO2On, c.SpO2 = SpO2Monitored, randIn(r, 86, 100)
	}
	c.NBPSys = int(gauss(r, 125, 28, 60, 220))
	c.NBPDia = int(gauss(r, float64(c.NBPSys)*0.6, 10, 30, 125))
	if r.Float64() < 0.6 {
		c.NBPMode, c.NBPInterval = NBPAuto, NBPIntervals[r.Intn(len(NBPIntervals))]
	}
	if r.Float64() < 0.2 {
		c.ABPOn, c.ABPSys, c.ABPDia = true, c.NBPSys, c.NBPDia
	}
	if r.Float64() < 0.2 {
		c.CO2On, c.EtCO2 = true, int(gauss(r, 38, 8, 15, 70))
	}
	// any rhythm; now and then a patient with the leads off, and a few whose
	// electrodes come off intermittently
	c.Rhythm = Rhythm(1 + r.Intn(int(RCount)-1))
	if r.Float64() < 0.03 {
		c.Rhythm = RLeadsOff
	}
	c.LeadsOffEp = c.Rhythm != RLeadsOff && r.Float64() < 0.08
	info := Rhythms[c.Rhythm]
	if c.Rhythm == RAflutter {
		c.FlRatio = []int{FlAuto, Fl2to1, Fl3to1, Fl4to1, FlVariable}[r.Intn(5)]
	}
	if info.UsesRate {
		if c.Rhythm == RAflutter && c.FlRatio != FlAuto {
			c.Rate = randIn(r, 270, 330)
		} else {
			c.Rate = randIn(r, info.TypMin, info.TypMax)
		}
	}
	if IsPaced(c.Rhythm) && r.Float64() < 0.6 {
		c.PaceMode, c.IntrRate = PaceDemand, c.Rate+randIn(r, -20, 10)
	}
	if info.Ectopy && r.Float64() < 0.4 {
		c.Ectopy = 1 + r.Intn(4)
		if (c.Ectopy == EctPAC || c.Ectopy == EctPJC) && !info.AtrialEctopy {
			c.Ectopy = EctPVCUni
		}
		c.Pattern = r.Intn(len(PatternNames))
		if (c.Ectopy == EctPAC || c.Ectopy == EctPJC) && c.Pattern == PatVTRuns {
			c.Pattern = PatRuns
		}
		c.Freq = randIn(r, 2, 12)
		if SinusNode(c.Rhythm) && r.Float64() < 0.25 { // PSVT episodes or sinus pauses
			c.Ectopy, c.Pattern = EctPSVT, PatOccasional
			c.Freq = randIn(r, 6, 30) // per hour
			if r.Float64() < 0.5 {
				c.Ectopy, c.Freq = EctSinusPause, randIn(r, 1, 4)
			}
		}
	}
	return c
}

type erEntry struct {
	weight float64
	rhythm Rhythm
	lo, hi int
	ratio  int
}

// Rough mix of what is on an ED telemetry board at a random moment: mostly
// sinus rhythms, a fair amount of AF, some paced and block rhythms, an
// occasional leads-off, and no lethal arrhythmias.
var erMix = []erEntry{
	{40, RSinus, 60, 100, 0},  // normal sinus
	{14, RSinus, 101, 135, 0}, // sinus tachycardia
	{8, RSinus, 45, 59, 0},    // sinus bradycardia
	{3, RSinusArrhythmia, 60, 90, 0},
	{11, RAfib, 65, 105, 0}, // rate-controlled AF
	{4, RAfib, 110, 150, 0}, // AF with RVR
	{1.5, RAflutter, 280, 320, Fl2to1},
	{1, RAflutter, 280, 320, Fl4to1},
	{0.5, RAflutter, 280, 320, FlVariable},
	{4, RFirstDegree, 55, 85, 0},
	{2.5, RPaced, 60, 80, 0},
	{2.5, RAVPaced, 60, 80, 0},
	{1, RAPaced, 60, 80, 0},
	{1, RSVT, 150, 200, 0},
	{1, RJunctional, 40, 60, 0},
	{1, RMobitzI, 50, 70, 0},
	{0.5, RMobitzII, 40, 55, 0},
	{0.5, RThirdDegree, 30, 45, 0},
	{3, RLeadsOff, 0, 0, 0},
}

// ERConfig draws one realistic ED telemetry patient.
// ---- care-unit presets ----

const (
	UnitED = iota
	UnitMedSurg
	UnitStepdown
	UnitICU
	UnitCount
)

var UnitNames = []string{"ED", "Med-surg tele", "Step-down", "ICU"}

// UnitOdds are the adjustable percentages (0-100) for each unit, editable in
// Patients > Scenario odds.
type UnitOdds struct {
	SpO2, NBP, ABP, CO2, Artifact float64
	LeadsOff                      float64 // electrodes come off now and then
}

var Odds = [UnitCount]UnitOdds{
	UnitED:       {SpO2: 75, NBP: 85, ABP: 1, CO2: 1, Artifact: 20, LeadsOff: 8},
	UnitMedSurg:  {SpO2: 0, NBP: 0, ABP: 0, CO2: 0, Artifact: 28, LeadsOff: 15},
	UnitStepdown: {SpO2: 90, NBP: 90, ABP: 6, CO2: 5, Artifact: 15, LeadsOff: 8},
	UnitICU:      {SpO2: 98, NBP: 75, ABP: 45, CO2: 30, Artifact: 12, LeadsOff: 4},
}

type unitProfile struct {
	mix          []erEntry
	nbpIntervals []int   // auto-cycle intervals in minutes
	sysMean      float64 // systolic BP distribution
	sysSD        float64
	rrLo, rrHi   int
	tachypnea    float64 // chance of RR 23-32
	lowSpO2      float64 // chance SpO2 is 85-89 (borderline 90-93 is twice this)
	ectopy       float64 // scales the chance of ectopic beats
	leadsOffBias float64 // extra chance of leads off
}

// Med-surg tele: mostly stable rhythms; unstable ones (AF with RVR, 2:1
// flutter, SVT, Mobitz II, complete heart block) are rare.
var medSurgMix = []erEntry{
	{40, RSinus, 60, 100, 0}, {7, RSinus, 101, 115, 0}, {9, RSinus, 50, 59, 0},
	{2, RSinusArrhythmia, 55, 85, 0},
	{16, RAfib, 60, 100, 0}, {0.8, RAfib, 105, 125, 0},
	{1, RAflutter, 280, 320, Fl4to1}, {0.15, RAflutter, 280, 320, Fl2to1}, {0.2, RAflutter, 280, 320, FlVariable},
	{6, RFirstDegree, 50, 85, 0},
	{3, RPaced, 60, 80, 0}, {3.5, RAVPaced, 60, 80, 0}, {0.5, RAPaced, 60, 80, 0},
	{0.05, RSVT, 150, 180, 0}, {0.4, RJunctional, 45, 60, 0},
	{0.5, RMobitzI, 50, 70, 0}, {0.06, RMobitzII, 40, 55, 0}, {0.03, RThirdDegree, 35, 45, 0},
}

var stepdownMix = []erEntry{
	{30, RSinus, 60, 100, 0}, {13, RSinus, 101, 140, 0}, {6, RSinus, 45, 59, 0},
	{1, RSinusArrhythmia, 55, 90, 0},
	{12, RAfib, 65, 105, 0}, {6, RAfib, 110, 155, 0},
	{1.5, RAflutter, 280, 320, Fl2to1}, {1.5, RAflutter, 280, 320, Fl4to1}, {0.5, RAflutter, 280, 320, FlVariable},
	{5, RFirstDegree, 50, 85, 0},
	{3, RPaced, 60, 80, 0}, {4, RAVPaced, 60, 80, 0}, {1, RAPaced, 60, 80, 0},
	{1.5, RSVT, 150, 210, 0}, {1, RJunctional, 40, 65, 0},
	{1, RMobitzI, 45, 70, 0}, {0.6, RMobitzII, 38, 55, 0}, {0.4, RThirdDegree, 30, 45, 0},
	{0.5, RIdioventricular, 30, 45, 0},
}

// ICU: the sickest mix - more sinus tachycardia, AF with RVR, flutter, SVT,
// heart block and escape rhythms (lethal rhythms still left to the user).
var icuMix = []erEntry{
	{24, RSinus, 60, 100, 0}, {17, RSinus, 101, 145, 0}, {4, RSinus, 45, 59, 0},
	{11, RAfib, 65, 105, 0}, {7, RAfib, 110, 160, 0},
	{1.5, RAflutter, 280, 320, Fl2to1}, {1.5, RAflutter, 280, 320, Fl4to1}, {0.5, RAflutter, 280, 320, FlVariable},
	{4, RFirstDegree, 50, 85, 0},
	{2.5, RPaced, 60, 90, 0}, {3.5, RAVPaced, 70, 90, 0}, {1, RAPaced, 70, 90, 0},
	{1.5, RSVT, 150, 210, 0}, {1.5, RJunctional, 40, 70, 0},
	{0.8, RMobitzI, 45, 70, 0}, {0.6, RMobitzII, 35, 55, 0}, {0.6, RThirdDegree, 28, 45, 0},
	{0.8, RIdioventricular, 30, 50, 0},
}

var unitProfiles = [UnitCount]unitProfile{
	UnitED:       {erMix, []int{10, 15, 15, 15, 30}, 132, 20, 12, 22, 0.12, 0.07, 1.0, 0.02},
	UnitMedSurg:  {medSurgMix, []int{30, 60, 240, 240}, 128, 18, 14, 22, 0.08, 0.05, 1.3, 0.06},
	UnitStepdown: {stepdownMix, []int{60, 60, 60, 60, 30, 15, 240}, 126, 24, 14, 26, 0.18, 0.10, 1.2, 0.02},
	UnitICU:      {icuMix, []int{15, 15, 30, 60, 60, 60, 5}, 118, 26, 12, 28, 0.22, 0.14, 1.3, 0.01},
}

// ERConfig draws one realistic ED telemetry patient.
func ERConfig(r *rand.Rand, name string) Config { return UnitConfig(r, name, UnitED) }

// UnitConfig draws one patient typical of the given care unit.
func UnitConfig(r *rand.Rand, name string, unit int) Config {
	pr := unitProfiles[unit]
	od := Odds[unit]
	chance := func(pct float64) bool { return r.Float64()*100 < pct }

	total := 0.0
	for _, e := range pr.mix {
		total += e.weight
	}
	x := r.Float64() * total
	e := pr.mix[len(pr.mix)-1]
	for _, it := range pr.mix {
		if x < it.weight {
			e = it
			break
		}
		x -= it.weight
	}
	if r.Float64() < pr.leadsOffBias {
		e = erEntry{1, RLeadsOff, 0, 0, 0}
	}
	c := Config{Name: name, Rhythm: e.rhythm, FlRatio: e.ratio, SpO2: 98, RR: randIn(r, pr.rrLo, pr.rrHi)}
	if r.Float64() < pr.tachypnea {
		c.RR = randIn(r, 23, 32)
	}
	// pulse oximetry
	if chance(od.SpO2) {
		c.SpO2On = SpO2Monitored
		z := r.Float64()
		switch {
		case z < pr.lowSpO2:
			c.SpO2 = randIn(r, 85, 89)
		case z < 3*pr.lowSpO2:
			c.SpO2 = randIn(r, 90, 93)
		default:
			c.SpO2 = randIn(r, 94, 100)
		}
	} else if od.SpO2 > 0 && r.Float64() < 0.05 { // (units without pulse oximetry never show it)
		c.SpO2On = SpO2SensorOff
	}
	if Rhythms[e.rhythm].UsesRate {
		c.Rate = randIn(r, e.lo, e.hi)
	}
	c.Tune = naturalTune(r, c)
	// a wide QRS on a conducted rhythm is usually a bundle branch block
	if c.Tune.QRSms >= 120 && UsesConduction(c.Rhythm) && r.Float64() < 0.8 {
		c.Conduction = CondRBBB
		if r.Float64() < 0.4 {
			c.Conduction = CondLBBB
		}
		c.Tune.QRSms = 0 // the block sets the QRS width
	}
	// blood pressure
	c.NBPSys = int(gauss(r, pr.sysMean, pr.sysSD, 80, 210))
	c.NBPDia = int(gauss(r, float64(c.NBPSys)*0.58, 8, 40, 115))
	if chance(od.NBP) {
		c.NBPMode = NBPAuto
		c.NBPInterval = pr.nbpIntervals[r.Intn(len(pr.nbpIntervals))]
	}
	if chance(od.ABP) { // arterial line
		c.ABPOn = true
		c.ABPSys, c.ABPDia = c.NBPSys+randIn(r, -6, 8), c.NBPDia+randIn(r, -6, 4)
	}
	if chance(od.CO2) { // capnography (sedation, PCA, respiratory concern)
		c.CO2On = true
		c.EtCO2 = int(gauss(r, 38, 5, 25, 55))
	}
	// a few patients' electrodes come off now and then
	c.LeadsOffEp = e.rhythm != RLeadsOff && chance(od.LeadsOff)
	// artifact, mostly mild/moderate, in proportions typical of telemetry
	if e.rhythm != RLeadsOff && chance(od.Artifact) {
		c.Artifact = []int{ArtTremor, ArtTremor, ArtTremor, ArtWander, ArtWander, ArtWander,
			ArtMotion, ArtMotion, ArtMotion, ArtLoose, ArtLoose, Art60Hz}[r.Intn(12)]
		c.ArtLevel = []int{0, 0, 1, 1, 1, 2}[r.Intn(6)]
	}
	info := Rhythms[c.Rhythm]
	if IsPaced(c.Rhythm) { // most real pacemakers run in demand mode
		c.PaceMode, c.IntrRate = PaceDemand, c.Rate+randIn(r, -20, 10)
	}
	if !info.Ectopy {
		return c
	}
	// ectopy overlay: most patients have none
	y := r.Float64() / pr.ectopy
	switch {
	case y < 0.15:
		c.Ectopy, c.Pattern, c.Freq = EctPVCUni, PatOccasional, randIn(r, 2, 6)
	case y < 0.22 && info.AtrialEctopy:
		c.Ectopy, c.Pattern, c.Freq = EctPAC, PatOccasional, randIn(r, 2, 6)
	case y < 0.25:
		c.Ectopy, c.Pattern = EctPVCUni, []int{PatBigeminy, PatTrigeminy, PatQuadrigeminy}[r.Intn(3)]
	case y < 0.28 && unit != UnitMedSurg: // multifocal PVCs / couplets: not on med-surg
		c.Ectopy, c.Pattern, c.Freq = EctPVCMulti, PatOccasional, randIn(r, 2, 5)
	case y < 0.30 && unit != UnitMedSurg:
		c.Ectopy, c.Pattern, c.Freq = EctPVCUni, PatCouplets, randIn(r, 1, 2)
	case y < 0.315 && SinusNode(c.Rhythm): // sinus pauses (sick sinus), mostly older patients
		c.Ectopy, c.Pattern, c.Freq = EctSinusPause, PatOccasional, 1
	case y < 0.322 && SinusNode(c.Rhythm) && unit != UnitMedSurg: // PSVT, a few episodes an hour
		c.Ectopy, c.Pattern, c.Freq = EctPSVT, PatOccasional, randIn(r, 2, 6)
	}
	return c
}

// gauss returns a normal deviate clamped to [lo, hi].
func gauss(r *rand.Rand, mean, sd, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, mean+sd*r.NormFloat64()))
}

// naturalTune gives a generated ED patient their own ECG "fingerprint":
// intervals, wave amplitudes and ST position drawn from realistic adult
// ranges, with the occasional bundle branch block, long QT, T inversion or
// ST change. Natural also turns on beat-to-beat variability.
func naturalTune(r *rand.Rand, c Config) Tune {
	t := DefaultTune()
	t.Natural = true

	// PR: only used where atria conduct normally
	switch c.Rhythm {
	case RSinus, RSinusArrhythmia, RAPaced, RMobitzII:
		t.PRms = int(gauss(r, 158, 20, 120, 200))
	case RFirstDegree:
		t.PRms = int(gauss(r, 250, 30, 210, 340))
	case RMobitzI:
		t.PRms = int(gauss(r, 175, 15, 150, 210))
	}

	// QRS width: ~10% wide (mostly turned into a bundle branch block by the caller)
	if r.Float64() < 0.10 {
		t.QRSms = int(gauss(r, 132, 10, 120, 160))
	} else {
		t.QRSms = int(gauss(r, 88, 9, 70, 110))
	}

	// QT: normally rate-corrected; ~12% prolonged (QTc 470-530 ms)
	if r.Float64() < 0.12 {
		vr := float64(c.Rate)
		if c.Rhythm == RAflutter && c.FlRatio != FlAuto {
			vr /= []float64{1, 2, 3, 4, 5, 3}[c.FlRatio]
		}
		if vr < 30 {
			vr = 75
		}
		qtc := gauss(r, 495, 18, 470, 530)
		t.QTms = int(math.Round(qtc * math.Sqrt(60/vr)))
	}

	// amplitudes vary with body habitus, lead placement, etc.
	t.PAmp = int(gauss(r, 100, 15, 65, 140))
	t.QRSAmp = int(gauss(r, 100, 22, 55, 160))
	t.TAmp = int(gauss(r, 100, 22, 50, 150))

	// ST: normally within half a millimetre of baseline
	t.STmm = math.Round(gauss(r, 0, 0.3, -0.5, 0.5)*2)/2 + 0 // +0 avoids "-0"
	switch x := r.Float64(); {
	case x < 0.05: // T-wave inversion (ischaemia, strain, old infarct...)
		t.TAmp = -int(gauss(r, 55, 15, 30, 90))
	case x < 0.08: // ST depression
		t.STmm = -math.Round(gauss(r, 1.5, 0.4, 1, 2.5)*2) / 2
	case x < 0.095: // ST elevation (rare: pericarditis / STEMI)
		t.STmm = math.Round(gauss(r, 2, 0.5, 1, 3.5)*2) / 2
	}
	return t
}
