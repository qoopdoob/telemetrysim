package main

// Rhythm quiz content (platform independent): the rhythms that can be asked,
// with a teaching point each, and the strip generator.

import "math/rand"

// quizItem is one rhythm that can be asked.
type quizItem struct {
	name, group, why string
	cfg              func(r *rand.Rand) Config
	need             func(bl []bEntry) bool // the strip must show this (nil = any)
	lead             string
}

func rIn(r *rand.Rand, lo, hi int) int { return lo + r.Intn(hi-lo+1) }

func hasLab(lab byte, n int) func(bl []bEntry) bool {
	return func(bl []bEntry) bool {
		c := 0
		for _, b := range bl {
			if b.lab == lab {
				c++
			}
		}
		return c >= n
	}
}

func hasGap(sec float64) func(bl []bEntry) bool {
	return func(bl []bEntry) bool {
		for i := 1; i < len(bl); i++ {
			if bl[i].t-bl[i-1].t > sec {
				return true
			}
		}
		return false
	}
}

func hasRunV(n int) func(bl []bEntry) bool {
	return func(bl []bEntry) bool {
		run := 0
		for _, b := range bl {
			if b.lab == 'V' {
				run++
				if run >= n {
					return true
				}
			} else {
				run = 0
			}
		}
		return false
	}
}

var quizPool = []quizItem{
	{name: "Normal sinus rhythm", group: "sinus", why: "Regular, rate 60-100, upright P before every narrow QRS, normal PR.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RSinus, Rate: rIn(r, 62, 95)} }},
	{name: "Sinus bradycardia", group: "sinus", why: "Sinus rhythm (P before every QRS, normal PR) at a rate below 60.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RSinus, Rate: rIn(r, 38, 54)} }},
	{name: "Sinus tachycardia", group: "sinus", why: "Sinus rhythm at a rate above 100 (usually up to ~150); P waves visible before each QRS.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RSinus, Rate: rIn(r, 108, 150)} }},
	{name: "Sinus arrhythmia", group: "sinus", why: "Sinus P waves, but the R-R interval varies cyclically (with breathing).",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RSinusArrhythmia, Rate: rIn(r, 58, 80)} }},
	{name: "Atrial fibrillation", group: "atrial", why: "Irregularly irregular R-R, no P waves - a wavy fibrillatory baseline.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RAfib, Rate: rIn(r, 70, 140)} }},
	{name: "Atrial flutter", group: "atrial", why: "Sawtooth flutter waves (~300/min) with a fixed or variable conduction ratio.",
		cfg: func(r *rand.Rand) Config {
			return Config{Rhythm: RAflutter, Rate: rIn(r, 280, 320), FlRatio: []int{Fl2to1, Fl3to1, Fl4to1, FlVariable}[r.Intn(4)]}
		}},
	{name: "Supraventricular tachycardia (SVT)", group: "atrial", why: "Regular narrow-complex tachycardia, usually 150-220, P waves not visible.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RSVT, Rate: rIn(r, 160, 220)} }},
	{name: "Junctional rhythm", group: "atrial", why: "Regular narrow QRS at 40-60 with inverted or absent P waves.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RJunctional, Rate: rIn(r, 40, 58)} }},
	{name: "Accelerated junctional rhythm", group: "atrial", why: "Junctional rhythm (inverted/absent P) at 60-100.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RJunctional, Rate: rIn(r, 65, 98)} }},
	{name: "First-degree AV block", group: "block", why: "Sinus rhythm with every P conducted, but a constant PR above 200 ms.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RFirstDegree, Rate: rIn(r, 55, 90)} }},
	{name: "Second-degree AV block type I (Wenckebach)", group: "block", why: "PR lengthens progressively until a P wave is not conducted (grouped beating).",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RMobitzI, Rate: rIn(r, 50, 70)} }},
	{name: "Second-degree AV block type II (Mobitz II)", group: "block", why: "Constant PR, then a P wave suddenly not conducted (dropped QRS).",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RMobitzII, Rate: rIn(r, 38, 55)} }},
	{name: "Third-degree (complete) heart block", group: "block", why: "P waves and QRS complexes are unrelated (AV dissociation); slow regular escape.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RThirdDegree, Rate: rIn(r, 28, 45)} }},
	{name: "Idioventricular rhythm", group: "vent", why: "Wide QRS, no P waves, rate 20-40 (accelerated: 40-100).",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RIdioventricular, Rate: rIn(r, 25, 40)} }},
	{name: "Ventricular tachycardia", group: "vent", why: "Regular wide-complex tachycardia, usually 140-250, no visible P waves.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RVTach, Rate: rIn(r, 150, 220)} }},
	{name: "Torsades de pointes", group: "vent", why: "Polymorphic VT whose QRS amplitude twists around the baseline.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RTorsades, Rate: rIn(r, 200, 260)} }},
	{name: "Ventricular fibrillation", group: "vent", why: "Chaotic irregular deflections with no identifiable QRS complexes.",
		cfg: func(r *rand.Rand) Config {
			return Config{Rhythm: []Rhythm{RVFibCoarse, RVFibFine}[r.Intn(2)]}
		}},
	{name: "Asystole", group: "vent", why: "No electrical activity - a flat line (check leads and gain in two leads).",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RAsystole} }},
	{name: "Ventricular paced rhythm", group: "paced", why: "A pacer spike before every wide QRS; no relation to P waves.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RPaced, Rate: rIn(r, 60, 80)} }},
	{name: "Atrial paced rhythm", group: "paced", why: "A pacer spike before each P wave, followed by a narrow conducted QRS.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RAPaced, Rate: rIn(r, 60, 80)} }},
	{name: "AV (dual-chamber) paced rhythm", group: "paced", why: "Two spikes per beat: one before the P wave and one before the wide QRS.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RAVPaced, Rate: rIn(r, 60, 80)} }},
	{name: "Pacemaker failure to capture", group: "paced", why: "Pacer spikes that are not followed by a QRS complex.",
		cfg: func(r *rand.Rand) Config {
			return Config{Rhythm: RPaced, Rate: rIn(r, 60, 75), PaceMode: PaceNoCapture}
		},
		need: hasGap(1.4)},
	{name: "Sinus rhythm with PVCs", group: "ectopy", why: "Sinus rhythm with occasional early, wide, bizarre beats and a compensatory pause.",
		cfg: func(r *rand.Rand) Config {
			return Config{Rhythm: RSinus, Rate: rIn(r, 65, 90), Ectopy: EctPVCUni, Pattern: PatOccasional, Freq: rIn(r, 8, 14)}
		}, need: hasLab('V', 1)},
	{name: "Sinus rhythm with ventricular bigeminy", group: "ectopy", why: "Every other beat is a PVC (normal, PVC, normal, PVC...).",
		cfg: func(r *rand.Rand) Config {
			return Config{Rhythm: RSinus, Rate: rIn(r, 65, 85), Ectopy: EctPVCUni, Pattern: PatBigeminy}
		}},
	{name: "Sinus rhythm with ventricular trigeminy", group: "ectopy", why: "Every third beat is a PVC (normal, normal, PVC...).",
		cfg: func(r *rand.Rand) Config {
			return Config{Rhythm: RSinus, Rate: rIn(r, 65, 85), Ectopy: EctPVCUni, Pattern: PatTrigeminy}
		}},
	{name: "Sinus rhythm with PVC couplets", group: "ectopy", why: "Pairs of consecutive PVCs interrupting a sinus rhythm.",
		cfg: func(r *rand.Rand) Config {
			return Config{Rhythm: RSinus, Rate: rIn(r, 65, 85), Ectopy: EctPVCUni, Pattern: PatCouplets, Freq: 8}
		}, need: hasRunV(2)},
	{name: "Sinus rhythm with a run of VT", group: "ectopy", why: "Three or more consecutive PVCs at a fast rate, then sinus rhythm resumes.",
		cfg: func(r *rand.Rand) Config {
			return Config{Rhythm: RSinus, Rate: rIn(r, 65, 85), Ectopy: EctPVCUni, Pattern: PatVTRuns, Freq: 6}
		}, need: hasRunV(4)},
	{name: "Sinus rhythm with PACs", group: "ectopy", why: "Early narrow beats with an abnormal P wave, then a non-compensatory pause.",
		cfg: func(r *rand.Rand) Config {
			return Config{Rhythm: RSinus, Rate: rIn(r, 65, 90), Ectopy: EctPAC, Pattern: PatOccasional, Freq: 12}
		}, need: hasLab('S', 1)},
	{name: "Sinus rhythm with sinus pause", group: "ectopy", why: "A pause in which no P wave appears, then sinus rhythm resumes.",
		cfg: func(r *rand.Rand) Config {
			return Config{Rhythm: RSinus, Rate: rIn(r, 62, 85), Ectopy: EctSinusPause, Freq: 6}
		}, need: hasGap(1.8)},
	{name: "Sinus rhythm with right bundle branch block", group: "conduction", lead: "V1", why: "Lead V1: wide QRS (>= 120 ms) with an rSR' ('M') pattern.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RSinus, Rate: rIn(r, 62, 90), Conduction: CondRBBB} }},
	{name: "Sinus rhythm with left bundle branch block", group: "conduction", lead: "V1", why: "Lead V1: wide QRS (>= 120 ms) with a deep, broad QS complex.",
		cfg: func(r *rand.Rand) Config { return Config{Rhythm: RSinus, Rate: rIn(r, 62, 90), Conduction: CondLBBB} }},
}

// quizStrip simulates the item's rhythm and returns a 10-second strip that
// shows what the question needs (e.g. a PVC), plus its pace pulses, lead and
// whether the requirement was met.
func quizStrip(r *rand.Rand, item quizItem) ([]float32, []float64, string, bool) {
	cfg := item.cfg(r)
	cfg.Tune = DefaultTune()
	cfg.Tune.Natural = true // beat-to-beat variation like a real patient
	if item.lead != "" {
		cfg.Lead = item.lead
	}
	p := NewPatient(cfg, 0, r.Int63())
	end := 14.0 + 4*r.Float64()
	ok := item.need == nil
	for end < 90 {
		p.Update(end)
		if item.need == nil {
			break
		}
		var bl []bEntry
		for _, b := range p.blog {
			if b.t > end-10 && b.t <= end {
				bl = append(bl, b)
			}
		}
		if item.need(bl) {
			ok = true
			break
		}
		end += 1.5
	}
	if ok && item.need != nil { // move the feature away from the right edge if it still fits
		for _, d := range []float64{4, 3, 2, 1} {
			var bl []bEntry
			p.Update(end + d)
			for _, b := range p.blog {
				if b.t > end+d-10 && b.t <= end+d {
					bl = append(bl, b)
				}
			}
			if item.need(bl) {
				end += d
				break
			}
		}
	}
	data, paces, _ := p.StripAt(end)
	return data, paces, p.Cfg.LeadName(), ok
}
