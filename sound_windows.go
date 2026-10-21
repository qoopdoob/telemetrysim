//go:build windows

package main

// Alarm audio: tones matched to recorded Philips central-station alarms
// (see alarmsynth.go), synthesized at startup and looped with PlaySound. Only the highest-priority
// unsilenced alarm on the board sounds, like a central station.

import (
	"syscall"
	"unsafe"
)

var (
	winmm      = syscall.NewLazyDLL("winmm.dll")
	pPlaySound = winmm.NewProc("PlaySoundW")
)

const (
	sndASYNC     = 0x0001
	sndNODEFAULT = 0x0002
	sndMEMORY    = 0x0004
	sndLOOP      = 0x0008
)

// Sound levels
const (
	sndNone = iota
	sndInop
	sndYellow
	sndRed
)

var (
	soundOn    = true
	volumeIdx  = 1 // 0 low, 1 medium, 2 high
	volumes    = []float64{0.12, 0.30, 0.65}
	wavs       = map[int][]byte{}
	playingLvl = sndNone
)

func buildWavs() {
	v := volumes[volumeIdx]
	wavs[sndInop] = synthTone(toneInop, v)
	wavs[sndYellow] = synthTone(toneYellow, v)
	wavs[sndRed] = synthTone(toneRed, v)
}

// setAlarmSound starts/stops the looping tone for the given level.
func setAlarmSound(lvl int) {
	if !soundOn {
		lvl = sndNone
	}
	if lvl == playingLvl {
		return
	}
	playingLvl = lvl
	if lvl == sndNone {
		call(pPlaySound, 0, 0, 0)
		return
	}
	w := wavs[lvl]
	call(pPlaySound, uintptr(unsafe.Pointer(&w[0])), 0, sndASYNC|sndMEMORY|sndLOOP|sndNODEFAULT)
}

// restartSound re-plays the current level (after a volume change).
func restartSound() {
	lvl := playingLvl
	setAlarmSound(sndNone)
	setAlarmSound(lvl)
}

// alarmSoundLevel maps a patient alarm level to a tone.
func alarmSoundLevel(lvl int) int {
	switch lvl {
	case AlarmPanic, 3: // panic alarms use the red alarm tone
		return sndRed
	case 2, 1: // short yellow arrhythmia alarms use the yellow tone (briefly)
		return sndYellow
	case 4:
		return sndInop
	}
	return sndNone
}
