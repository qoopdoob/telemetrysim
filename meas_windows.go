//go:build windows

package main

// Measurement setup windows, in the style of the IntelliVue "Setup <label>"
// menus: a titled panel with one row per setting ("High Limit : 100"). Click
// a row to pick a value from a pop-up list, or to run its action. Opened by
// clicking a patient's wave or numeric, or from the Measurements button.

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unsafe"
)

// MeasArrhy is the arrhythmia setup (UI only; its alarms belong to ECG).
const MeasArrhy = 6

var measTitles = map[int]string{
	MeasECG: "Setup ECG", MeasArrhy: "Setup Arrhythmia", MeasSpO2: "Setup SpO₂",
	MeasResp: "Setup Resp", MeasNBP: "Setup NBP", MeasABP: "Setup ABP", MeasCO2: "Setup CO₂",
}

// measOrder is the order of the Measurements menu.
var measOrder = []int{MeasECG, MeasArrhy, MeasSpO2, MeasResp, MeasNBP, MeasABP, MeasCO2}

// mItem is one row of a setup window.
type mItem struct {
	label   string
	value   func(p *Patient) string   // current value ("" for an action row)
	choices func(p *Patient) []string // pop-up list (nil = action row)
	set     func(p *Patient, i int)   // apply choice i
	action  func(p *Patient)          // action row
	group   int                       // rows with the same non-zero group get a bracket (limit pairs)
	alarm   bool                      // the "Alarms" row (alarm symbol)
}

var (
	measHwnd   uintptr
	measBed    = -1
	measKind   = -1
	measItems  []mItem
	measScroll int
	measHover  = -1
	measBuf    backBuf
)

const (
	wmNCHitTest  = 0x0084
	wmMouseWheel = 0x020A
	htCaption    = 2
	htClient     = 1
	measRows     = 12 // visible rows before scrolling
)

// ---- choice helpers ----

func intList(lo, hi, step int) []string {
	var o []string
	for v := lo; v <= hi; v += step {
		o = append(o, strconv.Itoa(v))
	}
	return o
}

func floatList(lo, hi, step float64, signed bool) []string {
	var o []string
	for v := lo; v <= hi+1e-9; v += step {
		if signed {
			o = append(o, signed1(math.Round(v*10)/10))
		} else {
			o = append(o, strconv.FormatFloat(math.Round(v*10)/10, 'f', 1, 64))
		}
	}
	return o
}

func atoi(s string) int { v, _ := strconv.Atoi(strings.Fields(s + " 0")[0]); return v }

func atof(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimPrefix(strings.Fields(s + " 0")[0], "+"), 64)
	return v
}

var onOff = []string{"On", "Off"}

func onOffStr(on bool) string {
	if on {
		return "On"
	}
	return "Off"
}

// ownLim gives the patient its own copy of the limits (as when limits are
// changed at the bedside) and returns it.
func ownLim(p *Patient) *AlarmLimits {
	if p.Lim == nil {
		l := Limits
		p.Lim = &l
	}
	return p.Lim
}

// intLimit is a row editing one integer limit.
func intLimit(label string, f func(l *AlarmLimits) *int, lo, hi, step, group int, fix func(l *AlarmLimits)) mItem {
	return mItem{label: label, group: group,
		value:   func(p *Patient) string { l := p.lim(); return strconv.Itoa(*f(&l)) },
		choices: func(p *Patient) []string { return intList(lo, hi, step) },
		set: func(p *Patient, i int) {
			l := ownLim(p)
			*f(l) = lo + i*step
			if fix != nil {
				fix(l)
			}
		}}
}

func floatLimit(label, unit string, f func(l *AlarmLimits) *float64, lo, hi, step float64, signed bool, group int, fix func(l *AlarmLimits)) mItem {
	return mItem{label: label, group: group,
		value: func(p *Patient) string {
			l := p.lim()
			if signed {
				return signed1(*f(&l)) + unit
			}
			return strconv.FormatFloat(*f(&l), 'f', 1, 64) + unit
		},
		choices: func(p *Patient) []string { return floatList(lo, hi, step, signed) },
		set: func(p *Patient, i int) {
			l := ownLim(p)
			*f(l) = math.Round((lo+float64(i)*step)*10) / 10
			if fix != nil {
				fix(l)
			}
		}}
}

// alarmsRow switches one measurement's alarms on or off.
func alarmsRow(m int) mItem {
	return mItem{label: "Alarms", alarm: true,
		value:   func(p *Patient) string { return onOffStr(p.MeasOff&(1<<uint(m)) == 0) },
		choices: func(*Patient) []string { return onOff },
		set: func(p *Patient, i int) {
			if i == 0 {
				p.MeasOff &^= 1 << uint(m)
			} else {
				p.MeasOff |= 1 << uint(m)
			}
			p.LogEvent(strings.TrimPrefix(measTitles[m], "Setup ")+" Alarms "+onOff[i], 4)
		}}
}

// cfgRow edits a simulator setting of the patient (a "patient value").
func cfgRow(label string, value func(c *Config) string, choices []string, set func(p *Patient, s string)) mItem {
	return mItem{label: label,
		value:   func(p *Patient) string { return value(&p.Cfg) },
		choices: func(*Patient) []string { return choices },
		set:     func(p *Patient, i int) { set(p, choices[i]) }}
}

// applyCfg applies a configuration change that affects the rhythm engine.
func applyCfg(p *Patient, f func(c *Config)) {
	c := p.Cfg
	f(&c)
	p.Apply(c, now())
}

// measMenu builds the rows of a setup window.
func measMenu(kind int) []mItem {
	it := strconv.Itoa
	switch kind {
	case MeasECG:
		return []mItem{
			cfgRow("ECG", func(c *Config) string { return onOffStr(!c.CardiacOff) }, onOff,
				func(p *Patient, s string) { applyCfg(p, func(c *Config) { c.CardiacOff = s == "Off" }) }),
			{label: "Primary Lead", value: func(p *Patient) string { return p.Cfg.LeadName() },
				choices: func(*Patient) []string { return LeadNames },
				set:     func(p *Patient, i int) { p.SetLead(LeadNames[i]) }},
			alarmsRow(MeasECG),
			intLimit("HR High Limit", func(l *AlarmLimits) *int { return &l.HRHigh }, 20, 300, 5, 1,
				func(l *AlarmLimits) { l.HRLow = min(l.HRLow, l.HRHigh-5) }),
			intLimit("HR Low Limit", func(l *AlarmLimits) *int { return &l.HRLow }, 15, 295, 5, 1,
				func(l *AlarmLimits) { l.HRHigh = max(l.HRHigh, l.HRLow+5) }),
			intLimit("∆ExtrTachy", func(l *AlarmLimits) *int { return &l.ExtTachyD }, 0, 50, 5, 0, nil),
			intLimit("∆ExtrBrady", func(l *AlarmLimits) *int { return &l.ExtBradyD }, 0, 50, 5, 0, nil),
			{label: "Extreme Tachy / Brady", value: func(p *Patient) string {
				l := p.lim()
				return fmt.Sprintf("> %d / < %d", l.XTachy(), l.XBrady())
			}},
			floatLimit("ST High Limit", " mm", func(l *AlarmLimits) *float64 { return &l.STHigh }, -9.8, 10, 0.2, true, 2,
				func(l *AlarmLimits) { l.STLow = math.Min(l.STLow, l.STHigh-0.2) }),
			floatLimit("ST Low Limit", " mm", func(l *AlarmLimits) *float64 { return &l.STLow }, -10, 9.8, 0.2, true, 2,
				func(l *AlarmLimits) { l.STHigh = math.Max(l.STHigh, l.STLow+0.2) }),
			intLimit("QTc High Limit", func(l *AlarmLimits) *int { return &l.QTcHigh }, 300, 700, 10, 0, nil),
			{label: "Artifact (sim)", value: func(p *Patient) string { return ArtifactNames[p.Cfg.Artifact] },
				choices: func(*Patient) []string { return ArtifactNames },
				set:     func(p *Patient, i int) { p.SetArtifact(i, p.Cfg.ArtLevel) }},
			{label: "Artifact Level (sim)", value: func(p *Patient) string { return ArtLevelNames[p.Cfg.ArtLevel] },
				choices: func(*Patient) []string { return ArtLevelNames },
				set:     func(p *Patient, i int) { p.SetArtifact(p.Cfg.Artifact, i) }},
			{label: "Fine Tune Waveform...", action: func(p *Patient) { openTune(measBed) }},
			{label: "Setup Arrhythmia...", action: func(p *Patient) { openMeas(measBed, MeasArrhy, -1, -1) }},
		}
	case MeasArrhy:
		items := []mItem{
			intLimit("V-Tach HR", func(l *AlarmLimits) *int { return &l.VTachHR }, 20, 300, 5, 0, nil),
			intLimit("V-Tach Run", func(l *AlarmLimits) *int { return &l.VTachRun }, 3, 99, 1, 0, nil),
			intLimit("Vent Rhythm", func(l *AlarmLimits) *int { return &l.VentRhythmRun }, 2, 99, 1, 0, nil),
			intLimit("SVT HR", func(l *AlarmLimits) *int { return &l.SVTHR }, 120, 300, 5, 0, nil),
			intLimit("SVT Run", func(l *AlarmLimits) *int { return &l.SVTRun }, 3, 99, 1, 0, nil),
			intLimit("PVCs/min", func(l *AlarmLimits) *int { return &l.PVCsMin }, 1, 99, 1, 0, nil),
			floatLimit("Asystole Thresh.", " sec", func(l *AlarmLimits) *float64 { return &l.AsystoleSec }, 2.5, 10, 0.5, false, 0,
				func(l *AlarmLimits) { l.PauseSec = math.Min(l.PauseSec, l.AsystoleSec-0.5) }),
			floatLimit("Pause Threshold", " sec", func(l *AlarmLimits) *float64 { return &l.PauseSec }, 1.5, 5, 0.5, false, 0,
				func(l *AlarmLimits) { l.AsystoleSec = math.Max(l.AsystoleSec, l.PauseSec+0.5) }),
			{label: "Afib/IHR End Dly", value: func(p *Patient) string { return delayLabel(p.lim().AfEndDelay) },
				choices: func(*Patient) []string {
					var o []string
					for _, v := range afDelays {
						o = append(o, delayLabel(v))
					}
					return o
				},
				set: func(p *Patient, i int) { ownLim(p).AfEndDelay = afDelays[i] }},
			{label: "All Yellow Off", action: func(p *Patient) { ownLim(p).ArrOff = 1<<uint(len(ArrSwitches)) - 1 }},
			{label: "All Yellow On", action: func(p *Patient) { ownLim(p).ArrOff = 0 }},
		}
		for i, sw := range ArrSwitches {
			bit := uint32(1) << uint(i)
			items = append(items, mItem{label: sw.Name,
				value:   func(p *Patient) string { return onOffStr(p.lim().ArrOff&bit == 0) },
				choices: func(*Patient) []string { return onOff },
				set: func(p *Patient, k int) {
					if k == 0 {
						ownLim(p).ArrOff &^= bit
					} else {
						ownLim(p).ArrOff |= bit
					}
				}})
		}
		return items
	case MeasSpO2:
		return []mItem{
			cfgRow("SpO₂", func(c *Config) string { return []string{"Off", "On", "Sensor Off"}[c.SpO2On] },
				[]string{"On", "Sensor Off", "Off"}, func(p *Patient, s string) {
					p.Cfg.SpO2On = map[string]int{"On": SpO2Monitored, "Sensor Off": SpO2SensorOff, "Off": SpO2Off}[s]
				}),
			cfgRow("Patient SpO₂ (sim)", func(c *Config) string { return it(c.SpO2) + " %" }, intList(50, 100, 1),
				func(p *Patient, s string) { p.Cfg.SpO2 = atoi(s) }),
			intLimit("High Limit", func(l *AlarmLimits) *int { return &l.SpO2High }, 51, 100, 1, 1,
				func(l *AlarmLimits) { l.SpO2Low = min(l.SpO2Low, l.SpO2High-1); l.Desat = min(l.Desat, l.SpO2Low) }),
			intLimit("Low Limit", func(l *AlarmLimits) *int { return &l.SpO2Low }, 50, 99, 1, 1,
				func(l *AlarmLimits) { l.SpO2High = max(l.SpO2High, l.SpO2Low+1); l.Desat = min(l.Desat, l.SpO2Low) }),
			intLimit("Desat Limit", func(l *AlarmLimits) *int { return &l.Desat }, 50, 99, 1, 0,
				func(l *AlarmLimits) { l.SpO2Low = max(l.SpO2Low, l.Desat) }),
			alarmsRow(MeasSpO2),
			{label: "Pulse (SpO₂) limits", value: func(p *Patient) string {
				l := p.lim()
				return fmt.Sprintf("%d / %d (= HR)", l.HRHigh, l.HRLow)
			}},
		}
	case MeasResp:
		return []mItem{
			cfgRow("Resp (RR numeric)", func(c *Config) string { return onOffStr(!c.RespRateOff) }, onOff,
				func(p *Patient, s string) {
					p.Cfg.RespRateOff = s == "Off"
					if p.Cfg.RespRateOff {
						p.Cfg.RespWaveOn = false
					}
				}),
			cfgRow("Resp Wave", func(c *Config) string { return onOffStr(c.RespWaveOn) }, onOff,
				func(p *Patient, s string) { p.Cfg.RespWaveOn = s == "On" && !p.Cfg.RespRateOff }),
			cfgRow("Patient Resp Rate (sim)", func(c *Config) string {
				if c.Apnea {
					return "0 (apnea)"
				}
				return it(c.RR) + " rpm"
			}, intList(0, 60, 1), func(p *Patient, s string) {
				v := atoi(s)
				applyCfg(p, func(c *Config) { c.RR, c.Apnea = v, v == 0 })
			}),
			intLimit("High Limit", func(l *AlarmLimits) *int { return &l.RRHigh }, 5, 100, 1, 1,
				func(l *AlarmLimits) { l.RRLow = min(l.RRLow, l.RRHigh-1) }),
			intLimit("Low Limit", func(l *AlarmLimits) *int { return &l.RRLow }, 0, 99, 1, 1,
				func(l *AlarmLimits) { l.RRHigh = max(l.RRHigh, l.RRLow+1) }),
			intLimit("Apnea Time", func(l *AlarmLimits) *int { return &l.ApneaSec }, 10, 40, 5, 0, nil),
			alarmsRow(MeasResp),
		}
	case MeasNBP:
		return []mItem{
			{label: "Mode", value: func(p *Patient) string { return NBPModeNames[p.Cfg.NBPMode] },
				choices: func(*Patient) []string { return NBPModeNames },
				set:     func(p *Patient, i int) { p.SetNBP(i, nbpMinutes(p.Cfg), now()) }},
			{label: "Repeat Time", value: func(p *Patient) string { return NBPIntervalLabel(nbpMinutes(p.Cfg)) },
				choices: func(*Patient) []string {
					var o []string
					for _, m := range NBPIntervals {
						o = append(o, NBPIntervalLabel(m))
					}
					return o
				},
				set: func(p *Patient, i int) { p.SetNBP(p.Cfg.NBPMode, NBPIntervals[i], now()) }},
			{label: "Start NBP", action: func(p *Patient) {
				if p.Cfg.NBPMode == NBPOff {
					p.SetNBP(NBPManual, nbpMinutes(p.Cfg), now())
				}
				p.StartNBP(now())
			}},
			cfgRow("Patient Systolic (sim)", func(c *Config) string { return it(c.NBPSys) + " mmHg" }, intList(40, 280, 5),
				func(p *Patient, s string) {
					p.Cfg.NBPSys = atoi(s)
					p.Cfg.NBPDia = min(p.Cfg.NBPDia, p.Cfg.NBPSys-10)
				}),
			cfgRow("Patient Diastolic (sim)", func(c *Config) string { return it(c.NBPDia) + " mmHg" }, intList(15, 180, 5),
				func(p *Patient, s string) {
					p.Cfg.NBPDia = atoi(s)
					p.Cfg.NBPSys = max(p.Cfg.NBPSys, p.Cfg.NBPDia+10)
				}),
			intLimit("NBPs High Limit", func(l *AlarmLimits) *int { return &l.NBPsHigh }, 45, 270, 5, 1,
				func(l *AlarmLimits) { l.NBPsLow = min(l.NBPsLow, l.NBPsHigh-5) }),
			intLimit("NBPs Low Limit", func(l *AlarmLimits) *int { return &l.NBPsLow }, 40, 265, 5, 1,
				func(l *AlarmLimits) { l.NBPsHigh = max(l.NBPsHigh, l.NBPsLow+5) }),
			alarmsRow(MeasNBP),
		}
	case MeasABP:
		return []mItem{
			cfgRow("ABP", func(c *Config) string { return onOffStr(c.ABPOn) }, onOff,
				func(p *Patient, s string) { p.Cfg.ABPOn = s == "On" }),
			cfgRow("Patient Systolic (sim)", func(c *Config) string { return it(c.ABPSys) + " mmHg" }, intList(40, 280, 5),
				func(p *Patient, s string) {
					p.Cfg.ABPSys = atoi(s)
					p.Cfg.ABPDia = min(p.Cfg.ABPDia, p.Cfg.ABPSys-10)
				}),
			cfgRow("Patient Diastolic (sim)", func(c *Config) string { return it(c.ABPDia) + " mmHg" }, intList(15, 180, 5),
				func(p *Patient, s string) {
					p.Cfg.ABPDia = atoi(s)
					p.Cfg.ABPSys = max(p.Cfg.ABPSys, p.Cfg.ABPDia+10)
				}),
			intLimit("ABPs High Limit", func(l *AlarmLimits) *int { return &l.ABPsHigh }, 45, 300, 5, 1,
				func(l *AlarmLimits) {
					l.ABPsLow = min(l.ABPsLow, l.ABPsHigh-5)
					l.ABPsXHigh = max(l.ABPsXHigh, l.ABPsHigh)
				}),
			intLimit("ABPs Low Limit", func(l *AlarmLimits) *int { return &l.ABPsLow }, 20, 295, 5, 1,
				func(l *AlarmLimits) {
					l.ABPsHigh = max(l.ABPsHigh, l.ABPsLow+5)
					l.ABPsXLow = min(l.ABPsXLow, l.ABPsLow)
				}),
			intLimit("Extreme High", func(l *AlarmLimits) *int { return &l.ABPsXHigh }, 50, 300, 5, 2,
				func(l *AlarmLimits) { l.ABPsHigh = min(l.ABPsHigh, l.ABPsXHigh) }),
			intLimit("Extreme Low", func(l *AlarmLimits) *int { return &l.ABPsXLow }, 15, 250, 5, 2,
				func(l *AlarmLimits) { l.ABPsLow = max(l.ABPsLow, l.ABPsXLow) }),
			alarmsRow(MeasABP),
		}
	case MeasCO2:
		return []mItem{
			cfgRow("CO₂", func(c *Config) string { return onOffStr(c.CO2On) }, onOff,
				func(p *Patient, s string) { p.Cfg.CO2On = s == "On" }),
			cfgRow("Patient etCO₂ (sim)", func(c *Config) string { return it(c.EtCO2) + " mmHg" }, intList(5, 99, 1),
				func(p *Patient, s string) { p.Cfg.EtCO2 = atoi(s) }),
			intLimit("etCO₂ High Limit", func(l *AlarmLimits) *int { return &l.EtCO2High }, 11, 99, 1, 1,
				func(l *AlarmLimits) { l.EtCO2Low = min(l.EtCO2Low, l.EtCO2High-1) }),
			intLimit("etCO₂ Low Limit", func(l *AlarmLimits) *int { return &l.EtCO2Low }, 0, 98, 1, 1,
				func(l *AlarmLimits) { l.EtCO2High = max(l.EtCO2High, l.EtCO2Low+1) }),
			{label: "awRR limits", value: func(p *Patient) string {
				l := p.lim()
				return fmt.Sprintf("%d / %d (= RR)", l.RRHigh, l.RRLow)
			}},
			alarmsRow(MeasCO2),
		}
	}
	return nil
}

var afDelays = []int{10, 20, 30, 60, 120, 180, 300, 600}

func delayLabel(sec int) string {
	if sec >= 60 && sec%60 == 0 {
		return strconv.Itoa(sec/60) + " min"
	}
	return strconv.Itoa(sec) + " sec"
}

// measOfKind maps a numeric/wave kind to its setup window.
func measOfKind(k int) int {
	switch k {
	case kSpO2, kPulse:
		return MeasSpO2
	case kRR:
		return MeasResp
	case kNBP:
		return MeasNBP
	case kABP:
		return MeasABP
	case kCO2, kAwRR:
		return MeasCO2
	case kPVCnum:
		return MeasArrhy
	}
	return MeasECG
}

// ---- window ----

func measGeom() (w, title, row, bar int32) {
	return S(400), S(34), S(34), S(30)
}

// openMeas opens the setup window of one measurement for a bed, near the
// screen point (x, y) of the main window (or centred when x < 0).
func openMeas(bed, kind int, x, y int32) {
	if setupHwnd != 0 || limitsHwnd != 0 || tuneHwnd != 0 || oddsHwnd != 0 {
		return
	}
	var keep RECT
	reuse := measHwnd != 0
	if reuse {
		call(pGetWindowRect, measHwnd, uintptr(unsafe.Pointer(&keep)))
		closeMeas()
	}
	measBed, measKind, measItems, measScroll, measHover = bed, kind, measMenu(kind), 0, -1
	w, title, row, bar := measGeom()
	n := min(len(measItems), measRows)
	h := title + row*int32(n) + S(6)
	if len(measItems) > measRows {
		h += bar
	}
	var mr RECT
	call(pGetWindowRect, mainHwnd, uintptr(unsafe.Pointer(&mr)))
	px, py := mr.Left+(mr.Right-mr.Left-w)/2, mr.Top+(mr.Bottom-mr.Top-h)/2
	switch {
	case reuse:
		px, py = keep.Left, keep.Top
	case x >= 0:
		pt := POINT{x, y}
		call(pClientToScreen, mainHwnd, uintptr(unsafe.Pointer(&pt)))
		px, py = pt.X+S(12), pt.Y-S(40)
		if px+w > mr.Right {
			px = pt.X - w - S(12)
		}
		if py+h > mr.Bottom {
			py = mr.Bottom - h - S(8)
		}
		if py < mr.Top {
			py = mr.Top + S(8)
		}
	}
	measHwnd = call(pCreateWindowExW, 0x00000080, // WS_EX_TOOLWINDOW
		uintptr(unsafe.Pointer(wstr("TeleSimMeas"))), uintptr(unsafe.Pointer(wstr(measTitles[kind]))),
		0x80000000|0x00800000, // WS_POPUP | WS_BORDER
		uintptr(px), uintptr(py), uintptr(w), uintptr(h), mainHwnd, 0, hInst, 0)
	call(pShowWindow, measHwnd, SW_SHOW)
}

func closeMeas() {
	if measHwnd == 0 {
		return
	}
	h := measHwnd
	measHwnd = 0
	measBuf.free()
	call(pDestroyWindow, h)
}

// measRowAt returns the item index under client point y (-1 = none, -2 = close).
func measRowAt(x, y int32) int {
	w, title, row, _ := measGeom()
	if y < title {
		if x > w-S(40) {
			return -2
		}
		return -1
	}
	i := int((y - title) / row)
	if i >= min(len(measItems), measRows) {
		return -1
	}
	return i + measScroll
}

func measProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch uint32(msg) {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc := call(pBeginPaint, hwnd, uintptr(unsafe.Pointer(&ps)))
		var rc RECT
		call(pGetClientRect, hwnd, uintptr(unsafe.Pointer(&rc)))
		if rc.Right > 0 && rc.Bottom > 0 && measBed >= 0 {
			measBuf.ensure(hdc, rc.Right, rc.Bottom)
			drawMeas(measBuf.dc, rc.Right, rc.Bottom)
			call(pBitBlt, hdc, 0, 0, uintptr(rc.Right), uintptr(rc.Bottom), measBuf.dc, 0, 0, SRCCOPY)
		}
		call(pEndPaint, hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmNCHitTest: // drag the window by its title bar
		var pt POINT
		pt.X, pt.Y = int32(int16(lp&0xffff)), int32(int16((lp>>16)&0xffff))
		call(pScreenToClient, hwnd, uintptr(unsafe.Pointer(&pt)))
		_, title, _, _ := measGeom()
		w, _, _, _ := measGeom()
		if pt.Y >= 0 && pt.Y < title && pt.X < w-S(40) {
			return htCaption
		}
		return htClient
	case WM_MOUSEMOVE:
		if i := measRowAt(lpX(lp), lpY(lp)); i != measHover {
			measHover = i
			call(pInvalidateRect, hwnd, 0, 0)
		}
		return 0
	case wmMouseWheel:
		d := int(int16(wp >> 16))
		if d < 0 {
			measScrollBy(1)
		} else {
			measScrollBy(-1)
		}
		return 0
	case WM_LBUTTONDOWN:
		x, y := lpX(lp), lpY(lp)
		w, title, row, bar := measGeom()
		n := int32(min(len(measItems), measRows))
		if len(measItems) > measRows && y >= title+row*n { // scroll buttons
			if x < w/2 {
				measScrollBy(-measRows / 2)
			} else {
				measScrollBy(measRows / 2)
			}
			_ = bar
			return 0
		}
		i := measRowAt(x, y)
		if i == -2 {
			closeMeas()
			return 0
		}
		if i >= 0 {
			measClick(hwnd, i, title+row*int32(i-measScroll))
		}
		return 0
	case WM_KEYDOWN:
		if wp == VK_ESCAPE {
			closeMeas()
		}
		return 0
	}
	return call(pDefWindowProcW, hwnd, msg, wp, lp)
}

func measScrollBy(d int) {
	maxS := max(0, len(measItems)-measRows)
	measScroll = max(0, min(maxS, measScroll+d))
	call(pInvalidateRect, measHwnd, 0, 0)
}

// measClick opens the pop-up list of a row (or runs its action).
func measClick(hwnd uintptr, i int, rowTop int32) {
	p := patients[measBed]
	it := measItems[i]
	switch {
	case it.action != nil:
		it.action(p)
	case it.choices != nil:
		ch := it.choices(p)
		cur := it.value(p)
		m := call(pCreatePopupMenu)
		perCol := 20
		for k, c := range ch {
			f := uintptr(MF_STRING)
			if c == cur || strings.Fields(cur + " ")[0] == c {
				f |= MF_CHECKED
			}
			if k > 0 && k%perCol == 0 {
				f |= 0x00000020 // MF_MENUBARBREAK: a new column
			}
			call(pAppendMenuW, m, f, uintptr(2000+k), uintptr(unsafe.Pointer(wstr(c))))
		}
		w, _, row, _ := measGeom()
		pt := POINT{w / 2, rowTop + row}
		call(pClientToScreen, hwnd, uintptr(unsafe.Pointer(&pt)))
		id := int(call(pTrackPopupMenu, m, 0x0100|0x0080, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0))
		call(pDestroyMenu, m)
		if id >= 2000 && id < 2000+len(ch) {
			it.set(p, id-2000)
		}
	default:
		return // information only
	}
	p.computeHR(now()) // re-evaluate alarms with the new setting
	call(pInvalidateRect, hwnd, 0, 0)
}

// drawMeas paints the setup window.
func drawMeas(dc uintptr, W, H int32) {
	p := patients[measBed]
	w, title, row, bar := measGeom()
	_ = w
	black := rgb(20, 20, 20)
	fill(dc, 0, 0, W, H, rgb(226, 226, 226))
	// title bar: measurement symbol, title, close
	fill(dc, 0, 0, W, title, rgb(204, 204, 204))
	fill(dc, 0, title-1, W, title, rgb(150, 150, 150))
	ib := RECT{S(8), S(7), S(30), title - S(7)}
	fill(dc, ib.Left, ib.Top, ib.Right, ib.Bottom, rgb(90, 90, 90))
	fill(dc, ib.Left+S(2), ib.Top+S(2), ib.Right-S(2), ib.Bottom-S(2), rgb(226, 226, 226))
	text(dc, "≡", ib.Left, ib.Top-S(2), ib.Right, ib.Bottom, black, font(S(14), true, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_CENTER)
	tt := measTitles[measKind] + "  –  " + bedLabel(measBed)
	text(dc, tt, S(36), 0, W-S(40), title, black, font(S(16), false, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_CENTER|DT_END_ELLIPSIS)
	xc := pen(black, int32(math.Max(1, math.Round(1.6*dpiScale))))
	cx, cy, d := W-S(20), title/2, S(6)
	if measHover == -2 {
		fill(dc, W-S(36), S(4), W-S(4), title-S(4), rgb(235, 235, 235))
	}
	line(dc, cx-d, cy-d, cx+d, cy+d, xc)
	line(dc, cx+d, cy-d, cx-d, cy+d, xc)

	lf := font(S(15), false, "Segoe UI")
	n := min(len(measItems), measRows)
	colon := int32(float64(W) * 0.62)
	grp := pen(rgb(60, 60, 60), int32(math.Max(1, math.Round(1.2*dpiScale))))
	for k := 0; k < n; k++ {
		i := k + measScroll
		it := measItems[i]
		y := title + row*int32(k)
		if i == measHover && (it.choices != nil || it.action != nil) {
			fill(dc, S(2), y+1, W-S(2), y+row-1, rgb(196, 210, 232))
		}
		fill(dc, S(10), y+row-1, W-S(10), y+row, rgb(196, 196, 196))
		c := black
		if it.choices == nil && it.action == nil {
			c = rgb(90, 90, 90) // information only
		}
		text(dc, it.label, S(40), y, colon-S(8), y+row, c, lf, DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
		if it.value != nil {
			text(dc, ":", colon-S(4), y, colon+S(8), y+row, c, lf, DT_SINGLELINE|DT_VCENTER)
			text(dc, it.value(p), colon+S(16), y, W-S(10), y+row, c, lf, DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
		}
		if it.alarm { // alarm symbol (triangle) before "Alarms"
			tx, ty := S(22), y+row/2
			polyline(dc, []POINT{{tx - S(7), ty + S(6)}, {tx, ty - S(7)}, {tx + S(7), ty + S(6)}, {tx - S(7), ty + S(6)}}, grp)
			text(dc, "!", tx-S(4), ty-S(5), tx+S(4), ty+S(6), black, font(S(10), true, "Segoe UI"), DT_SINGLELINE|DT_CENTER)
		}
		// bracket joining a high/low limit pair
		if it.group != 0 {
			prev := i > measScroll && measItems[i-1].group == it.group
			next := k+1 < n && measItems[i+1].group == it.group
			bx := S(20)
			top, bot := y+row/2, y+row/2
			if prev {
				top = y
			}
			if next {
				bot = y + row
			}
			line(dc, bx, top, bx, bot, grp)
			if !prev || !next {
				line(dc, bx, y+row/2, bx+S(8), y+row/2, grp)
			}
		}
	}
	if len(measItems) > measRows { // scroll buttons (as on the monitor)
		y := title + row*int32(n) + S(2)
		half := W / 2
		for b := 0; b < 2; b++ {
			l := int32(b) * half
			fill(dc, l+S(4), y, l+half-S(4), y+bar, rgb(200, 200, 200))
			sym := "▲"
			if b == 1 {
				sym = "▼"
			}
			text(dc, sym, l, y, l+half, y+bar, black, font(S(13), false, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_CENTER)
		}
	}
}

// ---- Measurements button: list of setup windows ----

func chooseMeas(hwnd uintptr, bed int, r RECT) {
	m := call(pCreatePopupMenu)
	for i, k := range measOrder {
		call(pAppendMenuW, m, MF_STRING, uintptr(3000+i), uintptr(unsafe.Pointer(wstr(measTitles[k]+"..."))))
	}
	call(pAppendMenuW, m, MF_SEPARATOR, 0, 0)
	call(pAppendMenuW, m, MF_STRING, 3100, uintptr(unsafe.Pointer(wstr("Alarm limits table..."))))
	pt := POINT{r.Left, r.Top}
	call(pClientToScreen, hwnd, uintptr(unsafe.Pointer(&pt)))
	id := int(call(pTrackPopupMenu, m, 0x0100|0x0080|0x0020, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)) // ...|TPM_BOTTOMALIGN
	call(pDestroyMenu, m)
	switch {
	case id >= 3000 && id < 3000+len(measOrder):
		openMeas(bed, measOrder[id-3000], r.Left, r.Top)
	case id == 3100:
		closeMeas()
		openLimits(bed)
	}
}
