//go:build windows

package main

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

func init() { runtime.LockOSThread() }

const maxBeds = 16

var (
	hInst      uintptr
	mainHwnd   uintptr
	mainMenu   uintptr
	viewMenu   uintptr
	alarmMenu  uintptr
	t0         = time.Now()
	patients   [maxBeds]*Patient
	nBeds      = 12
	showAlarms = true
	dpiScale   = 1.0
	mainBuf    backBuf
	sectors    []sectorGeom
	strips     = map[uintptr]*Strip{}
	standby    [maxBeds]bool
	acks       [maxBeds]ackState // acknowledged (silenced) alarm per bed
	colsSet    = 3               // 0 = auto
	silenceAll RECT              // top-bar "Silence All" button
	anyAudible bool
)

type sectorGeom struct {
	bed   int
	rc    RECT
	lead  RECT
	nbp   RECT   // clickable NBP title (interval menu)
	btns  []hbtn // hover buttons (only while the mouse is over the sector)
	hits  []mhit // waves and numerics: click to open that measurement's setup
	alarm RECT   // the alarm message box (double-click: alarm strip)
}

// mhit is a clickable wave or numeric area.
type mhit struct {
	rc   RECT
	meas int
}

// curHits collects the clickable areas while a sector is being drawn.
var curHits []mhit

// pendingClick is a single click on a wave/numeric, acted on once it is
// clear it is not the first half of a double-click (which silences).
var pendingClick struct {
	on        bool
	bed, meas int
	x, y      int32
	at        float64
}

type Strip struct {
	data    []float32
	bed     int
	name    string
	when    string
	hr      string
	alarm   string
	lvl     int
	spo2    string
	lead    string
	paces   []float64 // seconds from start of strip
	buf     backBuf
	status  string // rhythm / ectopic status when recorded
	ev      *Event // the saved event this strip shows (nil = not saved)
	cal     calipers
	panel   stripPanel
	at      time.Time  // alarm onset (alarm strips) / recording time
	isAlarm bool       // strip of the rhythm that triggered an alarm
	onset   float64    // alarm onset, seconds from the strip start (0 = none)
	quiz    *quizState // rhythm quiz strip
}

func now() float64      { return 100 + time.Since(t0).Seconds() }
func S(v float64) int32 { return int32(math.Round(v * dpiScale)) }

func bedLabel(i int) string { return fmt.Sprintf("Bed %d", i+1) }

// Menu IDs
const (
	idExit      = 100
	idResetAll  = 101
	idHelp      = 102
	idSetupBase = 110 // +bed
	idBedsBase  = 200 // +count
	idAlarms    = 300
	idAllOn     = 103
	idAllOff    = 104
	idRandom    = 105
	idERMix     = 106
	idSilAll    = 107
	idLimits    = 108
	idMedSurg   = 130
	idStepdown  = 131
	idOdds      = 132
	idICU       = 133
	idSound     = 310
	idFPS       = 311
	idRhyLbl    = 312
	idNumAlarm  = 313
	idLatchBase = 340 // +0..5
	idHRStd     = 314
	idEvents    = 315
	idQuiz      = 316
	idVolBase   = 320 // +0..2
	idColsBase  = 400 // +0 auto, +1..4
	idSpeedBase = 600 // +0..2
)

var sweepSpeeds = []float64{12.5, 25, 50}

var (
	latchMenu  uintptr
	measTick   int
	lowerSince float64 // when the board's alarm level first dropped
	forceSound bool    // apply the next sound change immediately
)

var (
	showFPS       bool
	fpsVal, fpsT0 float64
	fpsCount      int
)

var scenRng = rand.New(rand.NewSource(time.Now().UnixNano()))

var bedCounts = []int{4, 6, 8, 12, 16}

func main() {
	initCrashLog()
	defer cleanupCrashLog()
	call(pSetProcessDPIAware)
	hInst = call(pGetModuleHandleW, 0)
	sdc := call(pGetDC, 0)
	dpiScale = float64(call(pGetDeviceCaps, sdc, 90)) / 96
	call(pReleaseDC, 0, sdc)
	if dpiScale < 1 {
		dpiScale = 1
	}

	n := now()
	for i := range standby {
		standby[i] = true // every bed starts in standby
	}
	for i := range patients {
		patients[i] = NewPatient(Config{Rhythm: RLeadsOff}, n, int64(i*7919+13))
	}

	registerClass("TeleSimCentral", syscallCallback(mainProc), 0, 0x0008) // CS_DBLCLKS
	registerClass("TeleSimStrip", syscallCallback(stripProc), 0, CS_HREDRAW|CS_VREDRAW)
	registerClass("TeleSimSetup", syscallCallback(setupProc), 16, 0) // COLOR_BTNFACE+1
	registerClass("TeleSimLimits", syscallCallback(limitsProc), 16, 0)
	registerClass("TeleSimTune", syscallCallback(tuneProc), 16, 0)
	registerClass("TeleSimOdds", syscallCallback(oddsProc), 16, 0)
	registerClass("TeleSimMeas", syscallCallback(measProc), 0, 0)
	registerClass("TeleSimReview", syscallCallback(reviewProc), 16, 0)
	registerClass("TeleSimEventList", syscallCallback(eventListProc), 16, 0)

	sw := int32(call(pGetSystemMetrics, 0))
	sh := int32(call(pGetSystemMetrics, 1))
	w, h := sw*85/100, sh*85/100
	mainHwnd = call(pCreateWindowExW, 0, uintptr(unsafe.Pointer(wstr("TeleSimCentral"))), uintptr(unsafe.Pointer(wstr("Central Telemetry Station Simulator"))),
		WS_OVERLAPPEDWINDOW, uintptr((sw-w)/2), uintptr((sh-h)/2), uintptr(w), uintptr(h), 0, 0, hInst, 0)
	buildWavs()
	buildMenu()
	call(pShowWindow, mainHwnd, SW_SHOW)
	call(pUpdateWindow, mainHwnd)
	call(pSetTimer, mainHwnd, 1, 30, 0)

	var msg MSG
	for {
		r := int32(call(pGetMessageW, uintptr(unsafe.Pointer(&msg)), 0, 0, 0))
		if r <= 0 {
			break
		}
		if setupHwnd != 0 && call(pIsDialogMessageW, setupHwnd, uintptr(unsafe.Pointer(&msg))) != 0 {
			continue
		}
		if limitsHwnd != 0 && call(pIsDialogMessageW, limitsHwnd, uintptr(unsafe.Pointer(&msg))) != 0 {
			continue
		}
		if tuneHwnd != 0 && call(pIsDialogMessageW, tuneHwnd, uintptr(unsafe.Pointer(&msg))) != 0 {
			continue
		}
		if oddsHwnd != 0 && call(pIsDialogMessageW, oddsHwnd, uintptr(unsafe.Pointer(&msg))) != 0 {
			continue
		}
		call(pTranslateMessage, uintptr(unsafe.Pointer(&msg)))
		call(pDispatchMessageW, uintptr(unsafe.Pointer(&msg)))
	}
}

func buildMenu() {
	mainMenu = call(pCreateMenu)
	file := call(pCreatePopupMenu)
	call(pAppendMenuW, file, MF_STRING, idExit, uintptr(unsafe.Pointer(wstr("E&xit"))))
	call(pAppendMenuW, mainMenu, MF_POPUP, file, uintptr(unsafe.Pointer(wstr("&File"))))

	beds := call(pCreatePopupMenu)
	for i := 0; i < maxBeds; i++ {
		call(pAppendMenuW, beds, MF_STRING, uintptr(idSetupBase+i), uintptr(unsafe.Pointer(wstr(fmt.Sprintf("Set up %s...", bedLabel(i))))))
	}
	call(pAppendMenuW, beds, MF_SEPARATOR, 0, 0)
	call(pAppendMenuW, beds, MF_STRING, idAllOn, uintptr(unsafe.Pointer(wstr("Activate all beds"))))
	call(pAppendMenuW, beds, MF_STRING, idAllOff, uintptr(unsafe.Pointer(wstr("Put all beds in Standby"))))
	call(pAppendMenuW, beds, MF_SEPARATOR, 0, 0)
	call(pAppendMenuW, beds, MF_STRING, idRandom, uintptr(unsafe.Pointer(wstr("Randomize every patient's rhythm"))))
	call(pAppendMenuW, beds, MF_STRING, idERMix, uintptr(unsafe.Pointer(wstr("Simulate typical ER telemetry board"))))
	call(pAppendMenuW, beds, MF_STRING, idMedSurg, uintptr(unsafe.Pointer(wstr("Simulate med-surg telemetry floor"))))
	call(pAppendMenuW, beds, MF_STRING, idStepdown, uintptr(unsafe.Pointer(wstr("Simulate step-down unit"))))
	call(pAppendMenuW, beds, MF_STRING, idICU, uintptr(unsafe.Pointer(wstr("Simulate ICU"))))
	call(pAppendMenuW, beds, MF_STRING, idOdds, uintptr(unsafe.Pointer(wstr("Scenario odds..."))))
	call(pAppendMenuW, beds, MF_SEPARATOR, 0, 0)
	call(pAppendMenuW, beds, MF_STRING, idResetAll, uintptr(unsafe.Pointer(wstr("Reset all beds to Leads Off"))))
	call(pAppendMenuW, mainMenu, MF_POPUP, beds, uintptr(unsafe.Pointer(wstr("&Patients"))))

	viewMenu = call(pCreatePopupMenu)
	for _, c := range bedCounts {
		call(pAppendMenuW, viewMenu, MF_STRING, uintptr(idBedsBase+c), uintptr(unsafe.Pointer(wstr(fmt.Sprintf("%d patient sectors", c)))))
	}
	call(pAppendMenuW, viewMenu, MF_SEPARATOR, 0, 0)
	call(pAppendMenuW, viewMenu, MF_STRING, idColsBase, uintptr(unsafe.Pointer(wstr("Columns: Auto"))))
	for c := 1; c <= 4; c++ {
		lbl := fmt.Sprintf("Columns: %d", c)
		call(pAppendMenuW, viewMenu, MF_STRING, uintptr(idColsBase+c), uintptr(unsafe.Pointer(wstr(lbl))))
	}
	call(pAppendMenuW, viewMenu, MF_SEPARATOR, 0, 0)
	for i, sp := range sweepSpeeds {
		lbl := fmt.Sprintf("Sweep speed: %g mm/s (resp %g mm/s)", sp, sp/4)
		if sp == 25 {
			lbl += "  - standard"
		}
		call(pAppendMenuW, viewMenu, MF_STRING, uintptr(idSpeedBase+i), uintptr(unsafe.Pointer(wstr(lbl))))
	}
	call(pAppendMenuW, viewMenu, MF_SEPARATOR, 0, 0)
	call(pAppendMenuW, viewMenu, MF_STRING, idRhyLbl, uintptr(unsafe.Pointer(wstr("Show rhythm labels"))))
	call(pAppendMenuW, viewMenu, MF_STRING, idFPS, uintptr(unsafe.Pointer(wstr("Show frame rate"))))
	call(pAppendMenuW, viewMenu, MF_STRING, idAlarms, uintptr(unsafe.Pointer(wstr("Show alarm messages"))))
	call(pAppendMenuW, mainMenu, MF_POPUP, viewMenu, uintptr(unsafe.Pointer(wstr("&View"))))

	alarmMenu = call(pCreatePopupMenu)
	call(pAppendMenuW, alarmMenu, MF_STRING, idSilAll, uintptr(unsafe.Pointer(wstr("Silence all alarms"))))
	call(pAppendMenuW, alarmMenu, MF_STRING, idLimits, uintptr(unsafe.Pointer(wstr("Alarm and arrhythmia limits..."))))
	call(pAppendMenuW, alarmMenu, MF_STRING, idNumAlarm, uintptr(unsafe.Pointer(wstr("Alarm text: Enhanced (value and limit, e.g. ** SpO2 88<90)"))))
	latchMenu = call(pCreatePopupMenu)
	lname := []string{"Off", "Red Only", "Red&Yellow"}
	for i, c := range LatchCombos {
		call(pAppendMenuW, latchMenu, MF_STRING, uintptr(idLatchBase+i), uintptr(unsafe.Pointer(wstr("Visual: "+lname[c[0]]+"   Audible: "+lname[c[1]]))))
	}
	call(pAppendMenuW, alarmMenu, MF_POPUP, latchMenu, uintptr(unsafe.Pointer(wstr("Alarm latching"))))
	call(pAppendMenuW, alarmMenu, MF_STRING, idHRStd, uintptr(unsafe.Pointer(wstr("HR High/Low as standard yellow (** instead of short yellow *)"))))
	call(pAppendMenuW, alarmMenu, MF_SEPARATOR, 0, 0)
	call(pAppendMenuW, alarmMenu, MF_STRING, idSound, uintptr(unsafe.Pointer(wstr("Alarm sounds on"))))
	for i, n := range []string{"Volume: Low", "Volume: Medium", "Volume: High"} {
		call(pAppendMenuW, alarmMenu, MF_STRING, uintptr(idVolBase+i), uintptr(unsafe.Pointer(wstr(n))))
	}
	call(pAppendMenuW, mainMenu, MF_POPUP, alarmMenu, uintptr(unsafe.Pointer(wstr("&Alarms"))))
	evMenu := call(pCreatePopupMenu)
	call(pAppendMenuW, evMenu, MF_STRING, idEvents, uintptr(unsafe.Pointer(wstr("Event Log..."))))
	call(pAppendMenuW, mainMenu, MF_POPUP, evMenu, uintptr(unsafe.Pointer(wstr("&Events"))))
	quizMenu := call(pCreatePopupMenu)
	call(pAppendMenuW, quizMenu, MF_STRING, idQuiz, uintptr(unsafe.Pointer(wstr("Rhythm Quiz (10 strips)..."))))
	call(pAppendMenuW, mainMenu, MF_POPUP, quizMenu, uintptr(unsafe.Pointer(wstr("&Quiz"))))

	help := call(pCreatePopupMenu)
	call(pAppendMenuW, help, MF_STRING, idHelp, uintptr(unsafe.Pointer(wstr("How to use..."))))
	call(pAppendMenuW, mainMenu, MF_POPUP, help, uintptr(unsafe.Pointer(wstr("&Help"))))
	call(pSetMenu, mainHwnd, mainMenu)
	updateChecks()
}

func updateChecks() {
	for _, c := range bedCounts {
		f := uintptr(0)
		if c == nBeds {
			f = MF_CHECKED
		}
		call(pCheckMenuItem, viewMenu, uintptr(idBedsBase+c), f)
	}
	f := uintptr(0)
	if showAlarms {
		f = MF_CHECKED
	}
	call(pCheckMenuItem, viewMenu, idAlarms, f)
	for c := 0; c <= 4; c++ {
		f := uintptr(0)
		if c == colsSet {
			f = MF_CHECKED
		}
		call(pCheckMenuItem, viewMenu, uintptr(idColsBase+c), f)
	}
	f = 0
	if showRhythm {
		f = MF_CHECKED
	}
	call(pCheckMenuItem, viewMenu, idRhyLbl, f)
	f = 0
	if showFPS {
		f = MF_CHECKED
	}
	call(pCheckMenuItem, viewMenu, idFPS, f)
	for i, sp := range sweepSpeeds {
		f := uintptr(0)
		if sp == sweepSpeed {
			f = MF_CHECKED
		}
		call(pCheckMenuItem, viewMenu, uintptr(idSpeedBase+i), f)
	}
	f = 0
	if soundOn {
		f = MF_CHECKED
	}
	call(pCheckMenuItem, alarmMenu, idSound, f)
	f = 0
	if AlarmNumeric {
		f = MF_CHECKED
	}
	call(pCheckMenuItem, alarmMenu, idNumAlarm, f)
	f = 0
	if HRStandardYellow {
		f = MF_CHECKED
	}
	call(pCheckMenuItem, alarmMenu, idHRStd, f)
	for i, c := range LatchCombos {
		f := uintptr(0)
		if c[0] == VisualLatch && c[1] == AudibleLatch {
			f = MF_CHECKED
		}
		call(pCheckMenuItem, latchMenu, uintptr(idLatchBase+i), f)
	}
	for i := 0; i < 3; i++ {
		f := uintptr(0)
		if i == volumeIdx {
			f = MF_CHECKED
		}
		call(pCheckMenuItem, alarmMenu, uintptr(idVolBase+i), f)
	}
}

// updateAlarmState clears silences whose alarm ended or changed, and drives
// the central-station tone for the highest-priority unsilenced alarm.
func updateAlarmState() {
	best := sndNone
	anyAudible = false
	for i := 0; i < maxBeds; i++ {
		p := patients[i]
		// An acknowledged alarm stays paused (and its message frozen) until the
		// condition clears or a HIGHER-priority alarm appears (e.g. yellow -> red).
		// The condition must stay clear for 5 s (values hovering at a limit
		// shouldn't re-arm it); escalation re-arms immediately.
		if acks[i].on {
			switch {
			case alarmRank(p.AlarmLvl) > acks[i].rank, p.RemindAt > acks[i].at:
				// escalation, or an arrhythmia alarm reminder after its timeout
				acks[i] = ackState{}
			case p.AlarmText == "":
				if acks[i].clearSince == 0 {
					acks[i].clearSince = now()
				} else if now()-acks[i].clearSince >= 5 {
					acks[i] = ackState{}
				}
			default:
				acks[i].clearSince = 0
			}
		}
		// short yellow (arrhythmia) alarms sound only briefly when announced
		// short yellow (arrhythmia) alarms sound only briefly; latched alarms
		// sound only with audible latching (ToneLvl = 0 when silent)
		if i >= nBeds || standby[i] || !showAlarms || p.AlarmText == "" || p.ToneLvl == 0 || acks[i].on {
			continue
		}
		anyAudible = true
		if l := alarmSoundLevel(p.ToneLvl); l > best {
			best = l
		}
	}
	// Escalate immediately; step the tone down only after the quieter state
	// has lasted 2 s, so alarms flickering at a limit don't restart the sound
	// (each restart can briefly stall the display).
	t := now()
	if best >= playingLvl || !soundOn || forceSound {
		forceSound = false
		lowerSince = 0
		setAlarmSound(best)
	} else if lowerSince == 0 {
		lowerSince = t
	} else if t-lowerSince >= 2 {
		lowerSince = 0
		setAlarmSound(best)
	}
}

// chooseLead pops up a menu of ECG leads under the lead label.
func chooseLead(hwnd uintptr, bed int, r RECT) {
	m := call(pCreatePopupMenu)
	cur := patients[bed].Cfg.LeadName()
	for i, n := range LeadNames {
		f := uintptr(MF_STRING)
		if n == cur {
			f |= MF_CHECKED
		}
		if i == 6 {
			call(pAppendMenuW, m, MF_SEPARATOR, 0, 0)
		}
		call(pAppendMenuW, m, f, uintptr(1000+i), uintptr(unsafe.Pointer(wstr("Lead "+n))))
	}
	pt := POINT{r.Left, r.Bottom}
	call(pClientToScreen, hwnd, uintptr(unsafe.Pointer(&pt)))
	id := int(call(pTrackPopupMenu, m, 0x0100|0x0080, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)) // TPM_RETURNCMD|TPM_NONOTIFY
	call(pDestroyMenu, m)
	if id >= 1000 && id < 1000+len(LeadNames) {
		patients[bed].SetLead(LeadNames[id-1000])
	}
}

// ackState is an acknowledged alarm: the message and level shown at the
// moment it was silenced.
type ackState struct {
	on         bool
	text       string
	lvl, rank  int
	clearSince float64 // when the condition went away (0 = still present)
	at         float64 // when it was acknowledged
}

// alarmRank orders alarm levels by priority (panic > red > yellow > short yellow > INOP).
func alarmRank(lvl int) int { return AlarmRank(lvl) }

// shownAlarm returns the alarm message/level to display for a bed, and
// whether it is acknowledged (frozen while acknowledged).
func shownAlarm(bed int) (text string, lvl int, ack bool) {
	if a := acks[bed]; a.on {
		return a.text, a.lvl, true
	}
	p := patients[bed]
	return p.AlarmText, p.AlarmLvl, false
}

func silenceBed(i int) {
	p := patients[i]
	if p.AlarmText != "" && !acks[i].on {
		p.LogEvent("Alarm acknowledged: "+p.AlarmText, 4)
		p.Acknowledge() // latched red / yellow arrhythmia alarms whose condition ended clear now
		if p.AlarmText != "" {
			acks[i] = ackState{on: true, text: p.AlarmText, lvl: p.AlarmLvl, rank: alarmRank(p.AlarmLvl), at: now()}
		}
	}
	forceSound = true // pressing Silence takes effect immediately
	updateAlarmState()
}

const helpText = `Central Telemetry Station Simulator

PATIENT SECTORS
- Hover over a patient to show its buttons (as at the Information Center):
    [strip icon]    10-second freeze-frame on ECG paper (25 mm/s, 10 mm/mV)
                    with virtual calipers and Save Event (see EVENTS)
    Manage Patient  name, rhythm, rate, ectopy/events, pacing, conduction,
                    electrodes-off episodes, Alarms Off, standby
    Review          the patient's alarm history (date, time, message)
    Measurements    Setup ECG / Arrhythmia / SpO2 / Resp / NBP / ABP / CO2
                    windows, and the full alarm limits table
    [window icon]   expand: dock this patient full-width at the bottom of
                    the board with every wave and measurement; press again
                    to close.
- Click a wave or numeric to open that measurement's Setup window (e.g.
  Setup SpO2: SpO2 on/off, the patient's SpO2, High / Low / Desat limits,
  Alarms On/Off). Click a row to pick a value from its pop-up list; drag
  the window by its title; X or Esc closes it. Changing a limit gives the
  patient its own limits. "(sim)" rows set the simulated patient's value.
- Setup ECG also holds the lead, HR / extreme / ST / QTc limits, artifact
  and Fine Tune; Setup Arrhythmia holds the arrhythmia limits and the
  individual arrhythmia alarm switches.
- A numeric whose measurement alarms are off shows a red X; a patient with
  all alarms off shows an X beside the bed label and "Alarms Off".
- Double-click a patient to silence (acknowledge) its alarm; double-click
  the alarm message itself to also open the alarm's 10-second strip.
- Click the lead label (e.g. "II") to choose I, II, III, aVR, aVL, aVF, V1-V6.
- Every bed starts in Standby: hover and press "Activate Bed".

ALARMS (texts, priorities and criteria follow the Philips IntelliVue
MX100/X3 Instructions for Use, "Patient Alarms and INOPs")
- Message box at top right: red *** (e.g. *** Asystole, *** VTach,
  *** Desat), yellow ** (e.g. ** SpO2 Low), one-star yellow arrhythmia
  alarms * (e.g. * Pair PVCs, * AFIB, * HR High) and cyan INOPs (e.g.
  ECG Leads Off, SpO2 No Pulse, NBP Measure Failed).
- Alarm text Enhanced (Alarms menu): worst value and limit, e.g.
  *** Desat 78 < 80, *** xBrady 35<40, *** Apnea 0:25. Several alarms of
  the top priority alternate every 2 s.
- Alarm latching (Alarms menu, as configured on the monitor): visual and
  audible latching Red Only / Red&Yellow / Off. A latched alarm keeps its
  message (and tone, if audible latching) after the condition ends until
  it is acknowledged. INOPs never latch.
- One-star arrhythmia alarms: lamp and tone for 6 s, message up to 3 min
  after the condition ends. Four chains (PVC / Missed Beat / Arrhy HR /
  AFib-HR): only the highest in each chain is announced; a timeout (TimeOut
  1st 3 min, 2nd 10 min) blocks the same or a lower alarm, and when it
  expires a condition still present sounds again as a reminder (even if
  acknowledged). AFIB / Irregular HR have no timeout; * End AFIB / * End
  Irregular HR follow after the end delay (60 s, adjustable).
- Arrhythmia relearn (Learning ECG / Learning Rhythm, ~16 s; only Asystole,
  Vent Fib/Tach and HR alarms meanwhile) happens when ECG monitoring is
  switched on, the lead is changed, or a Leads Off of > 60 s ends.
- Alarms Off then On resets the arrhythmia timeouts. Individual arrhythmia
  alarms can be switched off in Alarm Limits ("Some ECG AlarmsOff").
- HR High/Low are short yellow (*) by default, or standard yellow (**,
  no timeout, no chain) via the Alarms menu.
- Rhythm status (Sinus Rhythm, Vent Bigeminy, Paced Rhythm, ...) and
  ectopic status (Pair PVCs, Pause, Pacer Not Capt, ...) show on the ECG.
- A new alarm turns the sector blue. Silencing (double-click, bell button,
  Silence All) returns it to black and freezes the message with a check
  mark until the condition clears or a HIGHER-priority alarm appears.
- Alarms Off (Manage Patient): switches all of this patient's alarms off
  (X beside the bed label, "Alarms Off"; INOPs still appear, silently).
  Each Setup window also has Alarms On/Off for that measurement alone
  (ECG alarms off shows the INOP "ECG/Arrh AlarmsOff").
- Alarm limits: HR, extreme deltas, V-Tach HR/run, vent rhythm run, SVT
  HR/run, PVCs/min, asystole and pause thresholds, ST, QTc (alarms after
  5 min), SpO2/desat, RR/apnea, NBP, ABP and ABP extreme (red), etCO2.

MONITORING
- ECG (green) with 1 mV calibration bar and rhythm label; Pleth/SpO2 (cyan);
  RR (purple) with optional Resp wave. Each can be turned off per patient;
  hidden parameters do not alarm (resp alarms stop only when both the rate
  and wave are off). Pulse-ox-only patients show a cyan Pulse instead of HR.
- Waves sweep at real speeds (ECG/Pleth/ABP 25 mm/s, Resp/CO2 6.25 mm/s).
- ST and QT are measured on the displayed lead (ST-II etc., QTc by Bazett);
  alarms: **ST-<lead> above/below the ST limits, **QTc High.
- NBP (pink, Setup NBP): Off / Manual / Auto every
  5-60 min. A cuff cycle takes ~25 s (cuff pressure shown), then sys/dia
  (mean) and the time of the reading. "Measure NBP now" starts one. With no
  pulse the reading fails (NBP MEASURE FAILED); with weak pulses it reads
  low. Alarms: **NBPs HIGH/LOW.
- ABP (red, optional arterial line): beat-by-beat waveform with dicrotic
  notch, 0/150 scale lines and sys/dia (mean) measured from the wave. PVCs
  give weak pulses; V-tach with a pulse lowers the pressure. With no
  cardiac output (asystole, VF, torsades, pulseless V-tach) the pulsations
  stop and the pressure runs off to ~17/15 (16) within ~15 s; when a
  perfusing rhythm returns it recovers over ~20-30 s. **ABPs HIGH/LOW.
- Ectopy / events (Manage Patient): PVC couplets, triplets and runs of VT
  (4-12 beats, ~180/min: 5+ alarm *** VTach), PSVT episodes (sudden SVT
  150-220/min for 8-60 s, started by a PAC; set per hour) and sinus pauses
  (1.6-3.6 s; over 2 s alarm * Pause).
- Bundle branch block: Manage Patient > Conduction: RBBB (rSR' in V1,
  broad S in I/V6) or LBBB (broad notched R in I/V6, deep QS in V1,
  discordant T) for any rhythm with conducted beats.
- Pacing modes include intermittent failure to capture / to pace
  (* Pacer Not Capt / * Pacer Not Pacing).
- Pulseless V-tach: tick "Pulseless" in Manage Patient when V-tach is
  selected - no pleth (SpO2 NON-PULSAT.), falling ABP and EtCO2, NBP fails.
- EtCO2 (yellow, optional capnography): capnogram per breath, etCO2 and
  awRR. Works even with ECG leads off. EtCO2 falls when there is no cardiac
  output. **etCO2 HIGH/LOW, **awRR HIGH/LOW, ***APNEA.

SCENARIOS (Patients menu)
- Randomize: any rhythm including lethal ones.
- Typical ER board / med-surg tele floor / step-down unit: realistic rhythm
  mixes for each unit (med-surg: older, stable patients, more rate-
  controlled AF and pacemakers, unstable rhythms rare, no SpO2 or NBP; step-down: sicker, more tachycardia and
  RVR, SpO2 and NBP on most, a few arterial lines), natural per-patient
  variation, some artifact, no lethal rhythms.
- ICU: sickest mix (more tachycardia, RVR, blocks, escape rhythms), SpO2
  on nearly everyone, many arterial lines and capnography (ventilated).
- Click the NBP title ("NBP 17:03 Auto 15 min") on any patient for a menu:
  Auto 1 min ... 8 hr, Manual, Measure NBP now, Turn NBP off.
- Scenario odds: % of patients per unit with SpO2, auto NBP, ABP, EtCO2,
  artifact and electrodes that come off now and then.
- Every preset (and Randomize) includes a few patients with the leads off
  and a few whose electrodes come off intermittently (ECG Leads Off for
  ~10 s - 4 min, then reattached; > 60 s triggers an arrhythmia relearn).
  Tick "Electrodes come off now and then" in Manage Patient for any patient.

CALIPERS AND EVENT LOG
- Double-click a red or yellow alarm message to open the 10-second strip of
  the rhythm that triggered it (6 s before to 4 s after the onset, marked
  "alarm"); this also acknowledges the alarm.
- On any strip, drag with the LEFT mouse button to measure an interval
  (ms and rate per minute), with the RIGHT button for an amplitude (mV and
  mm). Drag a caliper line to move it; Clear Calipers removes them.
- Below the strip: Rhythm, HR, PR, QRS, QT, QTc, R-R and ST boxes and a
  free-text Note. Click into a box, then "<- Caliper into box" to copy the
  caliper reading (a rate for HR, mm for ST). QTc is calculated (Bazett) if
  left empty. "Save to Event Log" stores the strip with the alarm name and
  time; save again to update it, Delete Event removes it.
- Events > Event Log lists the events (newest first: time, bed, patient,
  alarm, interpretation, note); open one to view, measure or edit it. The
  log is kept for this session only and is cleared when the program closes.

RHYTHM QUIZ (Quiz menu)
- Ten 10-second strips generated by the simulator (31 rhythms: sinus,
  atrial, junctional, AV blocks, ventricular, paced, ectopy, bundle branch
  blocks), each with four answers - mostly similar rhythms as distractors.
  Pick one and press Check Answer to see the rhythm and a teaching point;
  Next Rhythm moves on. Score is shown at the top; calipers work on the
  strip. New Quiz starts over.

VIEW MENU: number of sectors, columns, sweep speed, frame rate, alarm
messages and rhythm labels on/off (turn messages off to quiz recognition).

For education and training only. Not a medical device.`

func mainProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch uint32(msg) {
	case WM_TIMER:
		t := now()
		for _, p := range patients {
			p.Update(t)
		}
		updateAlarmState()
		updateHover(hwnd)
		call(pInvalidateRect, hwnd, 0, 0)
		if pendingClick.on && t-pendingClick.at > 0.45 { // not a double-click
			pendingClick.on = false
			openMeas(pendingClick.bed, pendingClick.meas, pendingClick.x, pendingClick.y)
		}
		if measHwnd != 0 && int(t*4) != measTick { // keep live values current
			measTick = int(t * 4)
			call(pInvalidateRect, measHwnd, 0, 0)
		}
		refreshReview()
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc := call(pBeginPaint, hwnd, uintptr(unsafe.Pointer(&ps)))
		var rc RECT
		call(pGetClientRect, hwnd, uintptr(unsafe.Pointer(&rc)))
		if rc.Right > 0 && rc.Bottom > 0 {
			calibrate(hwnd)
			fpsCount++
			if t := now(); t-fpsT0 >= 1 {
				fpsVal, fpsCount, fpsT0 = float64(fpsCount)/(t-fpsT0), 0, t
			}
			mainBuf.ensure(hdc, rc.Right, rc.Bottom)
			drawMain(mainBuf.dc, rc.Right, rc.Bottom)
			call(pBitBlt, hdc, 0, 0, uintptr(rc.Right), uintptr(rc.Bottom), mainBuf.dc, 0, 0, SRCCOPY)
		}
		call(pEndPaint, hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_MOUSEMOVE:
		updateHover(hwnd)
		x, y := lpX(lp), lpY(lp)
		hoverSilAll = silenceAll.contains(x, y)
		return 0
	case WM_SETCURSOR:
		if loword(lp) == HTCLIENT {
			var pt POINT
			call(pGetCursorPos, uintptr(unsafe.Pointer(&pt)))
			call(pScreenToClient, hwnd, uintptr(unsafe.Pointer(&pt)))
			cur := uintptr(IDC_ARROW)
			if silenceAll.contains(pt.X, pt.Y) {
				cur = IDC_HAND
			}
			for _, s := range sectors {
				if s.lead.contains(pt.X, pt.Y) || s.nbp.contains(pt.X, pt.Y) {
					cur = IDC_HAND
				}
				for _, b := range s.btns {
					if b.rc.contains(pt.X, pt.Y) {
						cur = IDC_HAND
					}
				}
			}
			call(pSetCursor, call(pLoadCursorW, 0, cur))
			return 1
		}
	case WM_LBUTTONDOWN:
		x, y := lpX(lp), lpY(lp)
		if silenceAll.contains(x, y) {
			for i := 0; i < maxBeds; i++ {
				silenceBed(i)
			}
			return 0
		}
		handleSectorClick(hwnd, x, y)
		return 0
	case WM_LBUTTONDBLCLK:
		// double-click a patient to silence (acknowledge) its alarm
		pendingClick.on = false
		x, y := lpX(lp), lpY(lp)
		for _, s := range sectors {
			if s.rc.contains(x, y) && s.alarm.contains(x, y) && !standby[s.bed] {
				// double-click a red/yellow alarm message: the strip of the
				// rhythm that triggered it (and acknowledge it)
				if _, lvl, _ := shownAlarm(s.bed); IsPatientAlarm(lvl) {
					openAlarmStrip(s.bed)
					silenceBed(s.bed)
					continue
				}
			}
			if s.rc.contains(x, y) {
				onBtn := s.lead.contains(x, y) || s.nbp.contains(x, y)
				for _, b := range s.btns {
					onBtn = onBtn || b.rc.contains(x, y)
				}
				if !onBtn && !standby[s.bed] {
					silenceBed(s.bed)
				}
			}
		}
		return 0
	case WM_COMMAND:
		id := loword(wp)
		switch {
		case id == idExit:
			call(pDestroyWindow, hwnd)
		case id == idHelp:
			messageBox(hwnd, helpText, "How to use", 0x40)
		case id == idResetAll:
			t := now()
			for _, p := range patients {
				c := p.Cfg
				c.Rhythm, c.Rate, c.Ectopy = RLeadsOff, 0, EctNone
				p.Apply(c, t)
			}
		case id == idOdds:
			openOdds()
		case id == idRandom || id == idERMix || id == idMedSurg || id == idStepdown || id == idICU:
			t := now()
			for _, p := range patients {
				var c Config
				if id == idRandom {
					c = RandomConfig(scenRng, p.Cfg.Name)
				} else {
					unit := UnitED
					if id == idMedSurg {
						unit = UnitMedSurg
					} else if id == idStepdown {
						unit = UnitStepdown
					} else if id == idICU {
						unit = UnitICU
					}
					c = UnitConfig(scenRng, p.Cfg.Name, unit)
				}
				c.Lead = p.Cfg.Lead // keep the chosen lead
				p.Apply(c, t)
			}
			if setupHwnd != 0 {
				closeSetup()
			}
		case id == idAllOn || id == idAllOff:
			for i := range standby {
				standby[i] = id == idAllOff
			}
		case id == idAlarms:
			showAlarms = !showAlarms
			updateChecks()
		case id == idLimits:
			openLimits(-1)
		case id == idSilAll:
			for i := 0; i < maxBeds; i++ {
				silenceBed(i)
			}
		case id >= idLatchBase && id < idLatchBase+len(LatchCombos):
			VisualLatch, AudibleLatch = LatchCombos[id-idLatchBase][0], LatchCombos[id-idLatchBase][1]
			updateChecks()
			t := now()
			for _, p := range patients {
				p.computeHR(t)
			}
			updateAlarmState()
		case id == idQuiz:
			openQuiz()
		case id == idEvents:
			openEventList()
		case id == idHRStd:
			HRStandardYellow = !HRStandardYellow
			updateChecks()
			t := now()
			for _, p := range patients {
				p.computeHR(t)
			}
			updateAlarmState()
		case id == idNumAlarm:
			AlarmNumeric = !AlarmNumeric
			updateChecks()
			t := now()
			for _, p := range patients {
				p.computeHR(t)
			}
		case id == idSound:
			soundOn = !soundOn
			updateChecks()
			updateAlarmState()
		case id >= idVolBase && id < idVolBase+3:
			volumeIdx = id - idVolBase
			setAlarmSound(sndNone)
			buildWavs()
			updateChecks()
			updateAlarmState()
		case id == idRhyLbl:
			showRhythm = !showRhythm
			updateChecks()
		case id == idFPS:
			showFPS = !showFPS
			updateChecks()
		case id >= idSpeedBase && id < idSpeedBase+len(sweepSpeeds):
			sweepSpeed = sweepSpeeds[id-idSpeedBase]
			updateChecks()
		case id >= idColsBase && id <= idColsBase+4:
			colsSet = id - idColsBase
			updateChecks()
		case id >= idSetupBase && id < idSetupBase+maxBeds:
			openSetup(id - idSetupBase)
		case id > idBedsBase && id <= idBedsBase+maxBeds:
			nBeds = id - idBedsBase
			updateChecks()
		}
		return 0
	case WM_DESTROY:
		setAlarmSound(sndNone)
		call(pPostQuitMessage, 0)
		return 0
	}
	return call(pDefWindowProcW, hwnd, msg, wp, lp)
}

// ---- sweep speed (real-world mm/s on the physical screen) ----

var (
	sweepSpeed = 25.0 // ECG and SpO2, mm/s (Philips default)
	pxPerMM    = 96 / 25.4
	lastCal    float64
)

// respSpeed: respiration sweeps at a quarter of the ECG speed (6.25 mm/s at 25).
func respSpeed() float64 { return sweepSpeed / 4 }

// calibrate reads the monitor's physical pixel density so 25 mm/s on screen
// really is 25 mm per second. Rechecked every few seconds (window may move
// to another monitor).
func calibrate(hwnd uintptr) {
	if t := now(); t-lastCal < 3 {
		return
	} else {
		lastCal = t
	}
	defer func() { // keep within realistic monitor densities (56-406 dpi)
		if pxPerMM < 2.2 || pxPerMM > 16 {
			pxPerMM = 96 * dpiScale / 25.4
		}
	}()
	if d := rawDPI(hwnd); d >= 50 && d <= 700 {
		pxPerMM = d / 25.4
		return
	}
	dc := call(pGetDC, 0)
	mm := float64(call(pGetDeviceCaps, dc, 4)) // HORZSIZE
	px := float64(call(pGetDeviceCaps, dc, 8)) // HORZRES
	call(pReleaseDC, 0, dc)
	if mm > 100 && px > 0 && px/mm > 2 && px/mm < 30 {
		pxPerMM = px / mm
		return
	}
	pxPerMM = 96 * dpiScale / 25.4
}

// rawDPI returns the physical DPI of the monitor showing hwnd (0 if unknown).
func rawDPI(hwnd uintptr) float64 {
	if pGetDpiForMonitor.Find() != nil {
		return 0
	}
	mon := call(pMonitorFromWindow, hwnd, 2) // MONITOR_DEFAULTTONEAREST
	var x, y uint32
	if call(pGetDpiForMonitor, mon, 2, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y))) != 0 { // MDT_RAW_DPI
		return 0
	}
	return float64(x)
}

// sweepWindow returns how many seconds fit across a trace of width w pixels
// at the given speed, and the erase-gap duration.
func sweepWindow(w, speed float64) (T, gap float64) {
	pps := pxPerMM * speed // pixels per second
	T = w / pps
	if T > 62 { // ring buffer holds 64 s
		T = 62
	}
	if T < 1 {
		T = 1
	}
	gap = float64(S(12)) / pps
	return
}

// drawSweep renders the ECG as a sweeping trace with an erase bar, plus white
// pacer spikes from the top of the waveform area.
func drawSweep(dc uintptr, p *Patient, r RECT) { drawSweepLead(dc, p, r, p.Cfg.LeadName()) }

// drawSweepLead renders one ECG lead (the displayed lead or an ExtraLead).
func drawSweepLead(dc uintptr, p *Patient, r RECT, lead string) {
	wh := float64(r.Bottom - r.Top)
	yb := float64(r.Top) + wh*0.60
	scale := wh * 0.34
	get := func(k int64) float64 { v, _ := p.LeadAt(lead, k); return float64(v) }
	ys, T, gap := sweepTrace(dc, p, r, func(k int64) float64 { return yb - get(k)*scale }, cGreen, sweepSpeed)
	if ys == nil {
		return
	}
	x0 := r.Left + S(6)
	W := float64(r.Right - S(6) - x0)
	last := p.Last()
	t1 := p.TimeOf(last)
	t0 := t1 - (T - gap)
	wp := pen(rgb(255, 255, 255), int32(math.Max(1, math.Round(1.5*dpiScale))))
	for _, tp := range p.PacesBetween(t0, t1) {
		x := x0 + int32(math.Mod(tp, T)/T*W)
		k := last - int64((t1-tp)*FS)
		line(dc, x, r.Top+S(2), x, int32(ys(k)), wp)
	}
}

// drawPleth renders the SpO2 plethysmogram sweep (same speed as the ECG).
func drawPleth(dc uintptr, p *Patient, r RECT) {
	wh := float64(r.Bottom - r.Top)
	yb := float64(r.Bottom) - wh*0.15
	scale := wh * 0.58
	sweepTrace(dc, p, r, func(k int64) float64 { return yb - float64(p.PlethAt(k))*scale }, cCyan, sweepSpeed)
}

// drawResp renders the impedance respiration sweep (purple, slower speed).
func drawResp(dc uintptr, p *Patient, r RECT) {
	wh := float64(r.Bottom - r.Top)
	yb := float64(r.Bottom) - wh*0.18
	scale := wh * 0.6
	sweepTrace(dc, p, r, func(k int64) float64 { return yb - float64(p.RespAt(k))*scale }, cPurple, respSpeed())
}

// sweepPts is reused for every trace to avoid per-frame allocations.
var sweepPts = make([]POINT, 0, 8192)

// sweepTrace draws one sweeping trace at speed mm/s; yOf maps sample index to
// screen y. Samples are reduced to at most two points (min and max, in the
// order they occur) per pixel column, so drawing cost depends only on the
// trace width, never on how many seconds are shown. Narrow spikes survive.
// Returns a clamped y lookup for overlays (nil if nothing drawn) plus the
// sweep window and gap durations.
func sweepTrace(dc uintptr, p *Patient, r RECT, yOf func(int64) float64, c uint32, speed float64) (func(int64) float64, float64, float64) {
	x0 := r.Left + S(6)
	W := float64(r.Right - S(6) - x0)
	if W < 10 {
		return nil, 0, 0
	}
	T, gap := sweepWindow(W, speed)
	top, bot := float64(r.Top+S(3)), float64(r.Bottom-S(3))
	clampY := func(k int64) float64 { return math.Max(top, math.Min(bot, yOf(k))) }
	thick := int32(math.Max(1, math.Round(1.6*dpiScale)))
	last := p.Last()
	first := last - int64((T-gap)*FS)

	pts := sweepPts[:0]
	col := int32(math.MinInt32)
	var cMin, cMax float64
	var iMin, iMax, n int // when the min / max occurred within the column
	flushCol := func() {
		switch {
		case n == 0:
		case cMin == cMax:
			pts = append(pts, POINT{col, int32(cMin)})
		case iMin < iMax:
			pts = append(pts, POINT{col, int32(cMin)}, POINT{col, int32(cMax)})
		default:
			pts = append(pts, POINT{col, int32(cMax)}, POINT{col, int32(cMin)})
		}
	}
	for k := first; k <= last; k++ {
		x := x0 + int32(math.Mod(p.TimeOf(k), T)/T*W)
		y := clampY(k)
		if x != col {
			flushCol()
			if x < col { // wrapped to the left edge: finish this segment
				thickPolyline(dc, pts, c, thick)
				pts = pts[:0]
			}
			col, cMin, cMax, iMin, iMax, n = x, y, y, 0, 0, 0
		}
		if y < cMin {
			cMin, iMin = y, n
		}
		if y > cMax {
			cMax, iMax = y, n
		}
		n++
	}
	flushCol()
	thickPolyline(dc, pts, c, thick)
	sweepPts = pts[:0]
	return clampY, T, gap
}

// ---------------- 10-second freeze-frame strip ----------------

func openStrip(bed int) {
	p := patients[bed]
	s := &Strip{
		data: p.Snapshot(10),
		bed:  bed,
		name: p.Cfg.Name,
		when: time.Now().Format("02 Jan 2006  15:04:05"),
		lead: p.Cfg.LeadName(),
	}
	if p.HRValid {
		s.hr = fmt.Sprintf("HR %d bpm", p.HR)
	} else {
		s.hr = "HR -?-"
	}
	if showAlarms {
		s.alarm, s.lvl, _ = shownAlarm(bed)
	}
	switch p.Cfg.SpO2On {
	case SpO2Monitored:
		if p.SpO2Valid {
			s.spo2 = fmt.Sprintf("SpO2 %d%%", p.SpO2Disp)
		} else {
			s.spo2 = "SpO2 -?-"
		}
	case SpO2SensorOff:
		s.spo2 = "SpO2 -?-"
	}
	t1 := p.TimeOf(p.Last())
	t0 := t1 - 10 + 1/FS
	for _, tp := range p.PacesBetween(t0, t1) {
		s.paces = append(s.paces, tp-t0)
	}
	s.status = strings.TrimSpace(p.RhythmStatus + "  " + p.EctopicStatus)
	showStrip(s, fmt.Sprintf("%s - 10 second strip - %s", bedLabel(bed), s.when))
}

// showStrip opens a strip window (live freeze-frame or saved event).
func showStrip(s *Strip, title string) {
	sw := float64(call(pGetSystemMetrics, 0))
	w := int32(math.Min(sw*0.92, 1400*dpiScale))
	h := int32(float64(w)*0.36) + stripHeader(s) - S(58) + stripPanelH()
	hwnd := call(pCreateWindowExW, 0, uintptr(unsafe.Pointer(wstr("TeleSimStrip"))), uintptr(unsafe.Pointer(wstr(title))), WS_OVERLAPPEDWINDOW|WS_VISIBLE|0x02000000, // WS_CLIPCHILDREN
		CW_USEDEFAULT, CW_USEDEFAULT, uintptr(w), uintptr(h), mainHwnd, 0, hInst, 0)
	strips[hwnd] = s
	stripButtons(hwnd, s)
	call(pInvalidateRect, hwnd, 0, 0)
}

func stripProc(hwnd, msg, wp, lp uintptr) uintptr {
	if s, ok := strips[hwnd]; ok && stripTool(hwnd, s, uint32(msg), wp, lp) {
		return 0
	}
	switch uint32(msg) {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc := call(pBeginPaint, hwnd, uintptr(unsafe.Pointer(&ps)))
		var rc RECT
		call(pGetClientRect, hwnd, uintptr(unsafe.Pointer(&rc)))
		if s, ok := strips[hwnd]; ok && rc.Right > 0 && rc.Bottom > 0 {
			s.buf.ensure(hdc, rc.Right, rc.Bottom)
			drawStrip(s.buf.dc, s, rc.Right, rc.Bottom)
			call(pBitBlt, hdc, 0, 0, uintptr(rc.Right), uintptr(rc.Bottom), s.buf.dc, 0, 0, SRCCOPY)
		}
		call(pEndPaint, hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_KEYDOWN:
		if wp == VK_ESCAPE {
			call(pDestroyWindow, hwnd)
		}
		return 0
	case WM_DESTROY:
		if s, ok := strips[hwnd]; ok {
			s.buf.free()
			delete(strips, hwnd)
		}
		return 0
	}
	return call(pDefWindowProcW, hwnd, msg, wp, lp)
}

func drawStrip(dc uintptr, s *Strip, w, h int32) {
	black := rgb(0, 0, 0)
	fill(dc, 0, 0, w, h, rgb(255, 255, 255))
	m := S(16)
	headH := stripHeader(s)

	title := bedLabel(s.bed)
	if s.name != "" {
		title += "    " + s.name
	}
	if s.quiz != nil {
		quizHeader(dc, s, w)
	} else {
		text(dc, title, m, S(6), w-m, S(30), black, font(S(19), true, "Segoe UI"), DT_SINGLELINE)
	}
	hr := s.hr
	if s.spo2 != "" {
		hr += "     " + s.spo2
	}
	info := s.when + "     " + hr + "     Lead " + s.lead + "     25 mm/s     10 mm/mV     10 s freeze-frame"
	if s.quiz == nil {
		text(dc, info, m, S(30), w-m, S(52), rgb(40, 40, 40), font(S(14), false, "Segoe UI"), DT_SINGLELINE|DT_END_ELLIPSIS)
	}
	if s.alarm != "" {
		c := rgb(200, 0, 0)
		if s.lvl == 4 {
			c = rgb(0, 120, 160)
		} else if s.lvl < 3 {
			c = rgb(150, 110, 0)
		}
		text(dc, s.alarm, w/2, S(6), w-m, S(30), c, font(S(18), true, "Segoe UI"), DT_SINGLELINE|DT_RIGHT)
	}
	l3 := ""
	if s.isAlarm {
		l3 = "Alarm strip: " + strings.TrimSpace(s.alarm) + " at " + s.at.Format("15:04:05") + "  (6 s before to 4 s after onset)"
	}
	if s.status != "" {
		if l3 != "" {
			l3 += "     "
		}
		l3 += "Monitor: " + s.status
	}
	text(dc, l3, m, S(54), w/2-S(8), S(76), rgb(120, 0, 0), font(S(14), true, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	h -= stripPanelH() // the event-log panel sits below the grid

	gl := float64(m)
	pxmm := float64(w-2*m) / 250.0
	if pxmm <= 0 {
		return
	}
	gt := float64(headH + S(10))
	avail := float64(h-m) - gt
	mmH := math.Floor(avail/pxmm/5) * 5
	if mmH < 20 {
		mmH = 20
	}
	gr := gl + 250*pxmm
	gb := gt + mmH*pxmm

	minor := pen(rgb(250, 208, 208), 1)
	major := pen(rgb(232, 135, 135), 1)
	if pxmm >= 2.5 {
		for i := 0; i <= 250; i++ {
			if i%5 != 0 {
				x := int32(gl + float64(i)*pxmm)
				line(dc, x, int32(gt), x, int32(gb), minor)
			}
		}
		for j := 0; j <= int(mmH); j++ {
			if j%5 != 0 {
				y := int32(gt + float64(j)*pxmm)
				line(dc, int32(gl), y, int32(gr), y, minor)
			}
		}
	}
	for i := 0; i <= 250; i += 5 {
		x := int32(gl + float64(i)*pxmm)
		line(dc, x, int32(gt), x, int32(gb), major)
	}
	for j := 0; j <= int(mmH); j += 5 {
		y := int32(gt + float64(j)*pxmm)
		line(dc, int32(gl), y, int32(gr), y, major)
	}
	// 1-second tick marks above the grid
	tick := pen(rgb(60, 60, 60), S(1.5))
	for sec := 0; sec <= 10; sec++ {
		x := int32(gl + float64(sec)*25*pxmm)
		line(dc, x, int32(gt)-S(7), x, int32(gt), tick)
		if sec < 10 {
			text(dc, strconv.Itoa(sec)+" s", x+S(3), int32(gt)-S(18), x+S(60), int32(gt)-S(1), rgb(110, 110, 110), font(S(11), false, "Segoe UI"), DT_SINGLELINE)
		}
	}

	yb := gt + math.Round(mmH*0.55)*pxmm
	pts := make([]POINT, len(s.data))
	for i, v := range s.data {
		x := gl + float64(i)/FS*25*pxmm
		y := yb - float64(v)*10*pxmm
		y = math.Max(gt, math.Min(gb, y))
		pts[i] = POINT{int32(math.Round(x)), int32(math.Round(y))}
	}
	polyline(dc, pts, pen(black, int32(math.Max(1, math.Round(1.7*dpiScale)))))
	s.cal.gl, s.cal.pxmm, s.cal.gt, s.cal.gb, s.cal.yb = gl, pxmm, gt, gb, yb
	defer drawStripTools(dc, s, w)
	// pacer spikes: thin dark marker from the top of the grid down to the trace
	sp := pen(rgb(30, 30, 30), int32(math.Max(2, math.Round(2.4*dpiScale)))) // pacer spikes: a little wider
	for _, tp := range s.paces {
		i := int(tp * FS)
		if i < 0 || i >= len(pts) {
			continue
		}
		x := int32(math.Round(gl + tp*25*pxmm))
		line(dc, x, int32(gt), x, pts[i].Y, sp)
	}
}
