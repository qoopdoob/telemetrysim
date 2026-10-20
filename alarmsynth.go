package main

// Alarm tone synthesis, matched to recordings of a Philips central station:
//
//   Red:    961 Hz + 2881 Hz, 5 ms attack, hold 0.20 s, linear fade to silence
//           at 0.85 s; repeats every 1.023 s.
//   Yellow: 480 Hz harmonic stack (1x..6x at 0, 0, -12, -8, -2, -11 dB),
//           5 ms attack, hold 0.22 s, then -30 dB/s decay, cut at 0.67 s;
//           repeats every 2.05 s.
//   INOP:   480 Hz + 960 Hz (-14 dB), 3 ms attack, hold 0.18 s, fade to
//           silence at 0.31 s; repeats every 2.05 s.
//
// Envelope timings were fitted to the recordings to within ~2-3 dB RMS.

import (
	"encoding/binary"
	"math"
)

const (
	toneInop = iota + 1
	toneYellow
	toneRed
)

const synthRate = 22050

type partial struct{ freq, db float64 }

type alarmTone struct {
	parts  []partial
	period float64 // seconds between bursts
	length float64 // burst length (s)
	cycles int     // bursts per loop buffer
	env    func(t float64) float64
	gain   float64
}

var alarmTones = map[int]alarmTone{
	toneRed: {
		parts:  []partial{{961, -1}, {2881, 0}},
		period: 1.023, length: 0.85, cycles: 4, gain: 1.0,
		env: func(t float64) float64 {
			a := math.Min(t/0.005, 1)
			if t > 0.20 {
				a *= math.Max(0, 1-(t-0.20)/0.65)
			}
			return a
		},
	},
	toneYellow: {
		parts:  []partial{{480, 0}, {960, 0}, {1440, -12}, {1920, -8}, {2400, -2}, {2880, -11}},
		period: 2.05, length: 0.67, cycles: 2, gain: 0.85,
		env: func(t float64) float64 {
			a := math.Min(t/0.005, 1)
			if t > 0.22 {
				a *= math.Pow(10, -30*(t-0.22)/20)
			}
			return a * clamp((0.67-t)/0.03, 0, 1)
		},
	},
	toneInop: {
		parts:  []partial{{480, 0}, {960, -14}},
		period: 2.05, length: 0.31, cycles: 2, gain: 0.8,
		env: func(t float64) float64 {
			a := math.Min(t/0.003, 1)
			if t > 0.18 {
				a *= math.Max(0, 1-(t-0.18)/0.13)
			}
			return a
		},
	},
}

// synthTone renders one loopable buffer of the alarm as a WAV file image.
func synthTone(kind int, vol float64) []byte {
	at := alarmTones[kind]
	n := int(at.period * float64(at.cycles) * synthRate)
	pcm := make([]int16, n)
	norm := 0.0
	amps := make([]float64, len(at.parts))
	for i, p := range at.parts {
		amps[i] = math.Pow(10, p.db/20)
		norm += amps[i]
	}
	for c := 0; c < at.cycles; c++ {
		i0 := int(float64(c) * at.period * synthRate)
		m := int(at.length * synthRate)
		for i := 0; i < m && i0+i < n; i++ {
			t := float64(i) / synthRate
			v := 0.0
			for k, p := range at.parts {
				v += amps[k] * math.Sin(2*math.Pi*p.freq*t)
			}
			pcm[i0+i] = int16(32000 * vol * at.gain * at.env(t) * v / norm)
		}
	}
	return wavFromPCM(pcm, synthRate)
}

func wavFromPCM(pcm []int16, rate int) []byte {
	data := len(pcm) * 2
	b := make([]byte, 44+data)
	copy(b[0:], "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(36+data))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1) // PCM
	binary.LittleEndian.PutUint16(b[22:], 1) // mono
	binary.LittleEndian.PutUint32(b[24:], uint32(rate))
	binary.LittleEndian.PutUint32(b[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(data))
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(b[44+i*2:], uint16(s))
	}
	return b
}
