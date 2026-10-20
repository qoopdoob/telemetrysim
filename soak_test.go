package main

import (
	"math"
	"math/rand"
	"testing"
)

// Soak test: many patients, all rhythms/scenarios, 25 simulated minutes at the
// GUI's 30 ms timer step. Checks for panics, unbounded growth and NaN/Inf.
func TestSoak(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	var ps []*Patient
	now := 100.0
	for i := 0; i < 48; i++ {
		var c Config
		switch {
		case i < int(RCount):
			c = Config{Rhythm: Rhythm(i), Rate: Rhythms[i].DefRate, SpO2On: SpO2Monitored, SpO2: 96}
		case i%2 == 0:
			c = ERConfig(r, "")
		default:
			c = RandomConfig(r, "")
		}
		p := NewPatient(Config{Rhythm: RLeadsOff}, now, int64(i))
		p.Apply(c, now)
		ps = append(ps, p)
	}
	maxEv, maxPul, maxBeats := 0, 0, 0
	for step := 0; step < 25*60*33; step++ {
		now += 0.030
		for i, p := range ps {
			p.Update(now)
			if len(p.ev) > maxEv {
				maxEv = len(p.ev)
			}
			if len(p.pulses) > maxPul {
				maxPul = len(p.pulses)
			}
			if len(p.beats) > maxBeats {
				maxBeats = len(p.beats)
			}
			v := p.At(p.Last())
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Fatalf("patient %d rhythm %d: bad sample %v at %.1fs", i, p.Cfg.Rhythm, v, now)
			}
		}
		// reconfigure some patients now and then, like a user would
		if step%2000 == 0 {
			for i := 0; i < 6; i++ {
				ps[r.Intn(len(ps))].Apply(RandomConfig(r, ""), now)
			}
		}
		if step%(60*33) == 0 {
			t.Logf("t=%4.0fs maxEv=%d maxPulses=%d maxBeats=%d", now-100, maxEv, maxPul, maxBeats)
		}
	}
	if maxEv > 400 {
		t.Errorf("event list grows without bound: %d", maxEv)
	}
}
