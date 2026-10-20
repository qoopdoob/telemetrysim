# TeleSim — Central Telemetry Station Simulator
 
A vibe-coded cardiac monitor training simulator for Windows written in Go that reproduces the look and behavior of a hospital central telemetry station, specifically a Philips IntelliVue system. The program features a board of patient sectors with live ECG, pleth, respiration, arterial pressure and capnography waveforms, numerics, alarm messages and alarm tones. Every rhythm, rate, waveform detail and alarm is controllable, so it can be used for rhythm-recognition practice, alarm-response drills and teaching for medical professionals.
 
---
 
## Contents
 
- [Quick start](#quick-start)
- [The screen](#the-screen)
- [Working with a patient](#working-with-a-patient)
- [Rhythms](#rhythms)
- [Monitored parameters](#monitored-parameters)
- [Alarms](#alarms)
- [Scenario presets](#scenario-presets)
- [Menus](#menus)
- [Building from source](#building-from-source)
- [Project layout](#project-layout)
- [Testing](#testing)
- [Notes and limitations](#notes-and-limitations)
---
 
## Quick start
 
1. Copy `TelemetrySim.exe` to any folder on a 64-bit Windows 10/11 PC. Nothing needs to be installed.
2. Run it. Windows SmartScreen may warn that the program is unsigned; choose **More info → Run anyway**.
3. All beds start in **Standby**. Either:
   - hover over a bed and press **Activate Bed**, then choose a rhythm, or
   - fill the whole board at once from the **Patients** menu (e.g. *Simulate typical ER telemetry board*).
The default board shows 12 patients in 3 columns.
 
---
 
## The screen
 
| Area | What it shows |
|---|---|
| **Top bar** | Station name, *Silence All* button (while anything is sounding), sound on/off, sweep speed, optional frame rate, date and time |
| **Patient sectors** | Bed label, patient name (or *Not Admitted*), alarm message box, waveforms on the left, numerics on the right |
| **Expanded view** | One patient docked full-width at the bottom of the board with every wave and measurement |
| **Bottom bar** | Reminder of the mouse controls |
 
### Inside a sector
 
- **ECG** (green) with a clickable lead label, a 1 mV calibration bar and a rhythm/status label (e.g. *Sinus Tach*, *Atrial Fib*, *Irregular HR*). Paced beats show a white pacer spike from the top of the wave area.
- **ABP** (red, optional) with 0 / 150 mmHg scale lines.
- **Pleth** (cyan) from the pulse oximeter.
- **CO₂** (yellow, optional) capnogram.
- **Resp** (purple, optional) impedance respiration wave.
- **Numerics**: label, alarm limits and value — HR, ABP, NBP, RR, SpO₂, Pulse, etCO₂. With more than three numerics they form two columns; the first column lines each number up with its own waveform (HR beside the ECG, SpO₂ beside the Pleth, …) and the second holds the rest (RR above NBP, Pulse, …).
- A sector turns **blue** while it has a new, unacknowledged alarm.
### Sweep speed
 
Waves move at real physical speeds on your monitor — ECG, Pleth and ABP at **25 mm/s**, Resp and CO₂ at **6.25 mm/s** — calibrated from the monitor's reported size. Wider sectors therefore show more seconds, as on a real station. 12.5 and 50 mm/s are available in the View menu.
 
---
 
## Working with a patient
 
### Mouse controls
 
| Action | Result |
|---|---|
| Hover over a sector | Shows its buttons along the bottom edge and an orange frame |
| **Double-click** a sector | Silences (acknowledges) that patient's alarm |
| Click the **lead label** (e.g. "II") | Choose lead I, II, III, aVR, aVL, aVF, V1–V6 |
| Click the **NBP title** ("NBP 17:32 Auto 15 min") | Choose Auto 1 min … 8 hr, Manual, Measure NBP now, Turn NBP off |
 
### Hover buttons
 
| Button | Opens |
|---|---|
| Strip icon | A **10-second freeze-frame** printed black-on-white on ECG paper (25 mm/s, 10 mm/mV). Several can be open; *Esc* closes one |
| **Adjust Rhythm** | Patient Setup (below) |
| **Fine Tune** | Waveform fine-tuning (below) |
| **Alarm Limits** | This patient's own alarm limits |
| Bell icon | Silence this patient's alarm (shown while it is sounding) |
| Window icon | **Expand** the patient across the bottom of the board; press again to close |
 
A small cyan dot on *Fine Tune* means the waveform has been hand-tuned; a yellow dot on *Alarm Limits* means the bed has its own limits.
 
### Patient Setup (*Adjust Rhythm*)
 
**Left column — cardiac and respiration**
 
- Patient name, rhythm, heart rate (or flutter-wave rate / pacer rate / pulse rate, depending on the rhythm)
- Atrial flutter conduction: Auto, 2:1, 3:1, 4:1, 5:1, variable
- Pacing mode: Fixed (100 % paced) or Demand, with the patient's intrinsic rate
- Ectopy: PVCs (unifocal / multifocal), PACs, PJCs — occasional, bigeminy, trigeminy, quadrigeminy, couplets or runs of 3, with a frequency per minute
- SpO₂: not monitored / monitored at a target % / sensor off
- Artifact: muscle tremor, 60 Hz interference, wandering baseline, patient movement, loose lead — mild, moderate or severe
- Respiration: breath rate (0 = apnea), *Rate* and *Wave* checkboxes (the wave requires the rate)
- Cardiac (ECG) monitoring on/off, Standby
**Right column — blood pressure and CO₂**
 
- NBP: Off / Manual / Auto with interval, the patient's true BP, *Measure NBP now*
- ABP (arterial line): on/off and systolic/diastolic
- EtCO₂ (capnography): on/off and end-tidal value
*Apply* changes the patient live; *OK* applies and closes.
 
### Fine Tune
 
Adjusts the patient's own waveform live: **PR, QRS and QT intervals** (0 = automatic), **P, QRS and T amplitudes** (negative T = inverted) and the **ST segment** in mm (+ elevation / − depression). *Reset* returns to a standard waveform; *Cancel* undoes anything applied.
 
---
 
## Rhythms
 
| Group | Rhythms |
|---|---|
| Baseline | Leads Off, Asystole |
| Sinus | Sinus rhythm (brady / normal / tachy by rate), Sinus arrhythmia |
| Atrial | Atrial fibrillation, Atrial flutter (any conduction ratio), SVT |
| Junctional | Junctional rhythm (set rate for accelerated / tachy) |
| AV block | 1st degree, 2nd degree type I (Wenckebach), 2nd degree type II, 3rd degree |
| Ventricular | Idioventricular, Monomorphic VT, Torsades de pointes, VF (coarse / fine) |
| Paced | Ventricular (VVI), Atrial (AAI), AV / dual-chamber (DDD) — fixed or demand |
 
The physiology is consistent across channels: PVCs give compensatory pauses and weak pulses on the Pleth and ABP; VF and asystole make the Pleth and ABP go flat; SpO₂ and EtCO₂ keep working with ECG leads off. ED-board patients also get natural per-patient ECG variation (their own intervals, amplitudes and ST position, occasionally a bundle branch block, long QT, T inversion or ST change) plus beat-to-beat variability. Breathing varies naturally for every patient (drifting rate and depth, occasional sighs and shallow breaths).
 
---
 
## Monitored parameters
 
| Parameter | Display | Alarms |
|---|---|---|
| **HR** | ECG-derived heart rate | HR High / Low, ***xTachy / ***xBrady (panic), ***Asystole, ***VFib/VTach, ***VTach, **PVCs/min High |
| **ST / QT** | ST-*lead* (mm) and QTc/QT in the expanded view | **ST-*lead* above/below limits, **QTc High |
| **SpO₂ / Pulse** | Saturation and pulse rate from the pleth | **SpO2 Low, ***Desat (panic), SpO2 Non-Pulsatile, SpO2 Sensor Off; Pulse High/Low and xTachy/xBrady when there is no usable ECG |
| **RR** | Impedance respiration rate (and optional wave) | **RR High / Low, ***Apnea (off only when both rate and wave are hidden) |
| **NBP** | `sys/dia (mean)` with time and mode, e.g. *NBP 17:32 Auto 15 min*; live cuff pressure during a ~25 s measurement | **NBPs High / Low, NBP Measure Failed (no pulse) |
| **ABP** | Arterial waveform with dicrotic notch; `sys/dia (mean)` measured from the wave | **ABPs High / Low, ABP Non-Pulsatile |
| **etCO₂ / awRR** | Capnogram, end-tidal CO₂ and airway resp rate | **etCO2 High / Low, ***Apnea |
 
NBP auto intervals: 1, 3, 5, 10, 15, 30 min, 1, 4, 8 hr.
 
---
 
## Alarms
 
- **Priorities**: red / panic (`***`), yellow (`**`), technical / INOP (cyan, e.g. *ECG Leads Off*). Short-yellow status messages such as *Irregular HR* (used for irregular rhythms other than AF) appear as white text on the ECG and do not sound.
- **Message boxes** show the value and the limit crossed, e.g. `***xBrady 35 <40`.
- **Sound**: the station plays the tone of the highest-priority unacknowledged alarm on the board. Tones were synthesized to match recordings of a real central station (red: two-tone chime every second; yellow: lower chime every 2 s; technical: short low tone every 2 s).
- **Acknowledging** (double-click, bell button or *Silence All*): the sector returns to black and the message is **frozen** with a ✓. It stays paused — even if the value changes or the alarm drops to a same- or lower-priority alarm — until the condition has been clear for 5 s, or a **higher-priority** alarm appears (e.g. yellow → red), which alarms anew.
- **Limits**: *Alarms → Alarm limits* sets the station-wide defaults; each patient's *Alarm Limits* button can override them. Limits cover HR (incl. panic), SpO₂ (incl. desat), RR and apnea time, ST high/low, QTc, NBP and ABP systolic, and etCO₂.
---
 
## Scenario presets
 
*Patients* menu:
 
| Preset | Character |
|---|---|
| **Randomize** | Any rhythm, including lethal ones |
| **Typical ER telemetry board** | Mostly sinus rhythms, AF, some paced and block rhythms, auto NBP on most |
| **Med-surg telemetry floor** | Older patients: more AF, pacemakers and sinus brady; fewer with SpO₂; no NBP on the monitor; more motion artifact |
| **Step-down unit** | Sicker: more sinus tach, AF with RVR, flutter, SVT, blocks; SpO₂ and NBP on most (mostly hourly); a few arterial lines |
| **ICU** | Sickest mix, SpO₂ on nearly everyone, many arterial lines and capnography |
 
None of the unit presets include lethal rhythms — add them yourself for drills. **Scenario odds…** sets, per unit, the percentage of patients with SpO₂, auto NBP, an arterial line, capnography and artifact.
 
---
 
## Menus
 
| Menu | Items |
|---|---|
| **File** | Exit |
| **Patients** | Set up any bed, Activate all / Standby all, scenario presets, Scenario odds, Reset all to Leads Off |
| **View** | Number of sectors (4–16), columns (Auto, 1–4), sweep speed, rhythm labels, frame rate, alarm messages on/off (turn off for recognition quizzes) |
| **Alarms** | Silence all, station-wide alarm limits, sound on/off, volume |
| **Help** | How to use |
 
---
 
## Building from source
 
TeleSim is written in **Go** using only the standard library: the Windows interface talks directly to the Win32 API (`user32`, `gdi32`, `winmm`, `shcore`) via `syscall`, so there are no third-party dependencies and no cgo.
 
Requirements: Go 1.21 or newer (developed with Go 1.24). It can be built on any OS:
 
```sh
# Windows executable (GUI subsystem, stripped)
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui -s -w" -o TelemetrySim.exe .
```
 
On Windows PowerShell:
 
```powershell
$env:GOOS="windows"; $env:GOARCH="amd64"
go build -trimpath -ldflags "-H windowsgui -s -w" -o TelemetrySim.exe .
```
 
Running `go run .` on Linux or macOS does not open the GUI; it runs a preview tool (`preview_other.go`) that renders test strips of every rhythm to PNG files in `./preview`.
 
---
 
## Project layout
 
| File | Purpose |
|---|---|
| `ecg.go` | Core simulation engine: rhythms, beat scheduling, ectopy, pacing, ECG leads and morphology, artifact, pleth, respiration, HR/SpO₂/RR measurement and alarm evaluation |
| `vitals.go` | ABP, NBP (cuff cycle and auto intervals) and capnography models |
| `scenarios.go` | Randomize and care-unit presets, natural per-patient variation, scenario odds |
| `alarmsynth.go` | Alarm tone synthesis (WAV generation) |
| `main_windows.go` | Program entry, menus, message loop, sweep drawing, 10-second strip window, help text |
| `sector_windows.go` | Board layout, patient sectors, header, alarm boxes, hover buttons, mouse hit-testing |
| `expanded_windows.go` | Expanded full-width patient view |
| `params_windows.go` | Shared drawing of numerics (incl. BP), ABP and CO₂ waves |
| `setup_windows.go` | Patient Setup dialog |
| `tune_windows.go` | Fine Tune dialog |
| `limits_windows.go` | Alarm Limits dialog (station-wide and per patient) |
| `odds_windows.go` | Scenario odds dialog |
| `sound_windows.go` | Alarm sound playback and priority |
| `crashlog_windows.go` | Writes `TelemetrySim-crash.log` next to the .exe if the program crashes |
| `win32.go` | Minimal Win32 bindings and GDI helpers |
| `preview_other.go` | Non-Windows preview renderer |
| `rate_test.go`, `soak_test.go` | Tests |
 
The engine files (`ecg.go`, `vitals.go`, `scenarios.go`, `alarmsynth.go`) are platform-independent; everything with the `_windows.go` suffix is the Win32 front end.
 
---
 
## Testing
 
```sh
go test ./...
```
 
- **`rate_test.go`** steps every rhythm through its full allowed rate range and checks the simulated rate matches the setting.
- **`soak_test.go`** runs dozens of patients across all rhythms and presets for 25 simulated minutes, reconfiguring them as it goes, and checks for unbounded memory growth and invalid samples.
---
 
## Notes and limitations
 
- Settings (patients, limits, odds) are not saved between sessions.
- Physical sweep-speed calibration relies on the monitor size Windows reports; some TVs, projectors and remote-desktop sessions report it incorrectly.
- Alarm tones approximate a real central station; they were synthesized from analysis of recordings, not copied.
- Waveforms are synthetic models intended to look and behave realistically for teaching; they are not derived from patient data.
- If the program ever crashes, send the `TelemetrySim-crash.log` file created next to the .exe.
**For education and training only. Not a medical device.**
 
