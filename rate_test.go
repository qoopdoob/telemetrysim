package main

import (
	"math"
	"testing"
)

// Every rhythm must hold its set rate across its full allowed range.
func TestRatesAcrossRange(t *testing.T) {
	for r := Rhythm(0); r < RCount; r++ {
		info := Rhythms[r]
		if !info.UsesRate || r == RTorsades || r == RAfib {
			continue
		}
		ratios := []int{FlAuto}
		if r == RAflutter {
			ratios = []int{FlAuto, Fl2to1, Fl3to1, Fl4to1}
		}
		for _, ratio := range ratios {
			lo, hi, _ := RateRange(r, ratio)
			for rate := lo; rate <= hi; rate += 3 {
				if rate < 40 {
					continue // too few beats in the window to measure precisely
				}
				cfg := Config{Rhythm: r, Rate: rate, FlRatio: ratio}
				p := NewPatient(Config{Rhythm: RAsystole}, 100, int64(rate))
				p.Apply(cfg, 100)
				for tt := 100.0; tt <= 130; tt += 0.05 {
					p.Update(tt)
				}
				// count beats over the last 20 s
				n := 0
				for _, b := range p.beats {
					if b > 118 && b <= 130 {
						n++
					}
				}
				got := float64(n) * 5
				want := p.ventRate()
				if r == RMobitzI || r == RMobitzII || r == RThirdDegree {
					want = float64(rate)
				}
				if math.Abs(got-want)/want > 0.12 {
					t.Errorf("%s ratio %d rate %d: measured %.0f bpm, want ~%.0f", info.Name, ratio, rate, got, want)
				}
			}
		}
	}
}
