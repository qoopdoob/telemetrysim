# TeleSim — Central Telemetry Station Simulator

TeleSim is a Windows training simulator modelled on a Philips IntelliVue-style central telemetry station. It shows a live board of patients with ECG, pleth, respiration, arterial pressure and capnography waveforms. Each patient has numerics, alarm messages and alarm tones, and alarms follow the monitor manufacturer's published alarm criteria.

You control every patient's rhythm, rate, ectopy, conduction, vital signs and alarm settings. That makes TeleSim suitable for:

- rhythm-recognition practice;
- alarm-response drills;
- strip measurement and documentation;
- classroom teaching.

> **For education and training only. TeleSim is not a medical device** and must never be used to monitor real patients.

---

## Contents

1. [Quick start](#1-quick-start)
2. [The board](#2-the-board)
3. [Working with a patient](#3-working-with-a-patient)
4. [Strips, calipers and the event log](#4-strips-calipers-and-the-event-log)
5. [Rhythm quiz](#5-rhythm-quiz)
6. [Rhythms and physiology](#6-rhythms-and-physiology)
7. [Monitored parameters](#7-monitored-parameters)
8. [Alarms](#8-alarms)
9. [Scenario presets](#9-scenario-presets)
10. [Menus](#10-menus)
11. [Teaching ideas](#11-teaching-ideas)
12. [Building, project layout and tests](#12-building-project-layout-and-tests)
13. [Notes and limitations](#13-notes-and-limitations)

---

## 1. Quick start

1. Copy `TeleSim.exe` to any folder on a 64-bit Windows 10 or 11 PC. Nothing needs to be installed.
2. Run it. Windows SmartScreen may warn that the program is unsigned; choose **More info → Run anyway**.
3. Every bed starts in **Standby**. To bring patients on:
   - fill the whole board at once from the **Patients** menu (for example *Simulate typical ER telemetry board*), or
   - hover over a single bed, press **Activate Bed** and choose a rhythm.

The default board shows 12 patients in 3 columns.

**A two-minute tour:**

1. Hover over a patient to see its buttons.
2. Click the ECG to open *Setup ECG*, or click the SpO₂ number to open *Setup SpO₂*.
3. Open **Manage Patient** and switch the rhythm to ventricular tachycardia.
4. When the red `*** VTach` alarm appears, double-click the alarm message. This opens the 10-second strip of the rhythm that triggered it.
5. Measure the strip with the calipers and save it to the event log.

---

## 2. The board

| Area | What it shows |
|---|---|
| **Top bar** | Station name, *Silence All* button (while anything is sounding), sound on/off, sweep speed, optional frame rate, date and time |
| **Patient sectors** | Bed label, patient name (or *Not Admitted*), alarm message box, waveforms on the left, numerics on the right |
| **Expanded view** | One patient docked full-width along the bottom of the board, with every wave and measurement |
| **Bottom bar** | Reminder of the mouse controls |

### Inside a sector

- **ECG** (green), with:
  - a clickable lead label;
  - a 1 mV calibration bar;
  - the monitor's **rhythm status** and **ectopic status** messages (for example *Sinus Tachy*, *Vent Bigeminy*, *Irregular HR*, *Pair PVCs*), worked out from the monitor's own beat classification;
  - a white pacer spike for each pace pulse.
- **ABP** (red, optional), with 0 and 150 mmHg scale lines.
- **Pleth** (cyan), from the pulse oximeter.
- **CO₂** (yellow, optional) capnogram.
- **Resp** (purple, optional) impedance respiration wave.
- **Numerics**: label, alarm limits and value for HR, ABP, NBP, RR, SpO₂, Pulse and etCO₂.
  - With more than three numerics they form two columns.
  - The first column lines each number up with its own waveform (HR beside the ECG, SpO₂ beside the Pleth).
  - The second column holds the rest (RR above NBP, Pulse, and so on).
- **Background**: a sector turns **blue** while it has a new, unacknowledged red or yellow alarm. Technical alarms (INOPs) do not change the background.

### Sweep speed

Waves move at real physical speeds on your screen: ECG, Pleth and ABP at **25 mm/s**, Resp and CO₂ at **6.25 mm/s**. The speed is calibrated from the monitor size Windows reports. Wider sectors therefore show more seconds of signal, as on a real station. The **View** menu also offers 12.5 and 50 mm/s.

---

## 3. Working with a patient

### Mouse controls

| Action | Result |
|---|---|
| Hover over a sector | Shows its buttons along the bottom edge and an orange frame |
| Click a **wave or numeric** | Opens that measurement's **Setup window**. PVC/min opens *Setup Arrhythmia* |
| **Double-click** a sector | Silences (acknowledges) that patient's alarm |
| **Double-click** a red or yellow **alarm message** | Opens the 10-second strip of the rhythm that triggered the alarm, and acknowledges it |
| Click the **lead label** (for example "II") | Choose lead I, II, III, aVR, aVL, aVF or V1–V6 |
| Click the **NBP title** ("NBP 17:32 Auto 15 min") | Choose Auto 1 min to 8 hr, Manual, Measure NBP now, or Turn NBP off |

### Hover buttons

The buttons are laid out as at the Information Center:

| Button | Opens |
|---|---|
| Strip icon | A **10-second freeze-frame** on ECG paper, with calipers and the event-log boxes (see [section 4](#4-strips-calipers-and-the-event-log)) |
| **Manage Patient** | The patient's name, rhythm and simulation settings (below) |
| **Review** | The patient's **alarm review**: every alarm, acknowledgement and alarms on/off change, with date and time, newest first |
| **Measurements** | A list of the Setup windows, plus the full alarm limits table |
| Window icon | **Expands** the patient across the bottom of the board; press again to close it |

### Manage Patient

- **Patient name, rhythm and heart rate.** Depending on the rhythm, the rate box sets the flutter-wave rate, pacer rate or pulse rate instead.
- **Atrial flutter conduction**: Auto, 2:1, 3:1, 4:1, 5:1 or variable.
- **Pacing mode**: Fixed (100% paced), Demand (with the patient's own intrinsic rate), or fixed with intermittent **failure to capture** or **failure to pace**.
- **Conduction**: normal, **RBBB** or **LBBB**, for any rhythm with conducted beats.
- **Pulseless**, shown when ventricular tachycardia is selected. The heart produces no cardiac output.
- **Ectopy and events:**
  - **Ectopic beats:** PVCs (unifocal, multifocal or R-on-T), PACs or PJCs.
  - **PVC/PAC pattern:** occasional, bigeminy, trigeminy, quadrigeminy, couplets, triplets, or runs of VT (4–12 beats at about 180/min). Frequency is set per minute.
  - **Sinus-rhythm events:** **PSVT** episodes (set per hour) and **sinus pauses** (set per minute).
- **Electrodes come off now and then**: intermittent Leads Off episodes.
- **Alarms Off**: switches off all alarms for this patient.
- **Standby.**

*Apply* changes the patient live; *OK* applies and closes the window.

### Measurement Setup windows

Each measurement has its own *Setup* window in the style of the monitor's menus:

- one row per setting (`High Limit : 100`);
- a bracket joining each high/low limit pair;
- the alarm symbol on the *Alarms* row.

To use a Setup window:

- Click a row to choose a value from its pop-up list.
- Drag the window by its title bar to move it.
- Close it with the **X** or *Esc*.

Rows marked **(sim)** set the simulated patient's own value, such as the true SpO₂ or blood pressure. Changing a limit gives that patient its own limits. The station-wide defaults stay under *Alarms → Alarm and arrhythmia limits*.

| Window | Settings |
|---|---|
| **Setup ECG** | ECG on/off, primary lead, Alarms on/off, HR high/low, ∆ExtrTachy and ∆ExtrBrady (resulting extreme limits shown), ST high/low, QTc high, artifact type and level (sim), *Fine Tune Waveform…*, *Setup Arrhythmia…* |
| **Setup Arrhythmia** | V-Tach HR and run, vent rhythm run, SVT HR and run, PVCs/min, asystole and pause thresholds, AFib/IHR end delay, *All Yellow Off / On*, and each switchable arrhythmia alarm |
| **Setup SpO₂** | SpO₂ on / sensor off / off, patient SpO₂ (sim), high, low and desat limits, Alarms on/off |
| **Setup Resp** | RR numeric and Resp wave on/off, patient resp rate (sim; 0 = apnea), high/low limits, apnea time, Alarms on/off |
| **Setup NBP** | Mode, repeat time, *Start NBP*, patient systolic and diastolic (sim), NBPs high/low, Alarms on/off |
| **Setup ABP** | ABP on/off, patient systolic and diastolic (sim), ABPs high/low and extreme high/low, Alarms on/off |
| **Setup CO₂** | CO₂ on/off, patient etCO₂ (sim), etCO₂ high/low, Alarms on/off |

When a measurement's alarms are off, its numeric shows a red ✕. Switching off ECG alarms also raises the INOP *ECG/Arrh AlarmsOff*. When all of a patient's alarms are off, an ✕ appears beside the bed label together with *Alarms Off*.

### Fine Tune

Open it from *Setup ECG → Fine Tune Waveform…* to adjust the patient's own waveform live:

- **PR, QRS and QT intervals** (0 = automatic);
- **P, QRS and T amplitudes** (a negative T amplitude inverts the T wave);
- **ST segment** in mm (+ for elevation, − for depression).

*Reset* returns to a standard waveform; *Cancel* undoes anything applied.

### Expanded view

The expanded view shows the patient full-width along the bottom of the board:

- **waves:** three ECG leads, ABP, Pleth, CO₂ and Resp;
- **numerics:** HR, PVC/min, ST, QTc/QT, ABP, NBP, SpO₂, Pulse, RR, etCO₂ and awRR.

Waves and numerics there are clickable just as in a sector.

---

## 4. Strips, calipers and the event log

### 10-second strips

A strip is 10 seconds of the displayed lead, printed black-on-white on ECG paper at 25 mm/s and 10 mm/mV, with pacer spikes marked. There are three ways to open one:

- the **strip icon** on a patient, which freezes the last 10 seconds;
- **double-clicking a red or yellow alarm message**, which opens an **alarm strip**. It covers 6 seconds before to 4 seconds after the alarm's onset, with the onset marked *▼ alarm*. The header names the alarm and its time. Every red or yellow alarm in the *Review* list keeps its own onset strip.
- the **Event Log**, which reopens saved strips.

Several strips can be open at once; *Esc* closes one.

### Calipers

- Drag with the **left** mouse button across the strip to measure a **time interval**. It reads in ms and as a rate per minute.
- Drag with the **right** mouse button up or down to measure an **amplitude**. It reads in mV and mm.
- Drag either caliper line to adjust it. **Clear Calipers** removes them.

The readings are shown on the strip and in the header.

### Event log

Below the grid, every strip window has documentation boxes for:

- *Rhythm* interpretation;
- *HR*, *PR*, *QRS*, *QT*, *QTc*, *R-R* and *ST*;
- a free-text **Note**.

To fill and save them:

1. Click into a box and press **← Caliper into box** to copy the caliper reading into it. It goes in as a rate for HR and in mm for ST.
2. Leave QTc empty if you like; it is calculated with Bazett's formula from QT and the heart rate (or R-R).
3. Press **Save to Event Log** to store the strip with these entries, plus the **alarm name and time** for an alarm strip. Pressing it again updates the event; **Delete Event** removes it.

*Events → Event Log…* lists the session's events, newest first, with time, bed, patient, alarm, interpretation and the start of the note. **Open** shows an event's strip again, with its onset marker and entries, so you can measure or edit it.

The event log is kept **in memory only and is cleared when the program closes**.

---

## 5. Rhythm quiz

*Quiz → Rhythm Quiz (10 strips)…* opens a multiple-choice quiz on ECG paper.

Each question is a new 10-second strip generated by the simulator, with natural beat-to-beat variation, drawn from 31 rhythms:

- **Sinus:** normal sinus rhythm, sinus bradycardia, sinus tachycardia, sinus arrhythmia.
- **Atrial and junctional:** atrial fibrillation, atrial flutter, SVT, junctional rhythm, accelerated junctional rhythm.
- **AV block:** first degree, Wenckebach, Mobitz II, complete heart block.
- **Ventricular:** idioventricular rhythm, VT, torsades de pointes, VF, asystole.
- **Paced:** ventricular, atrial and AV paced rhythms, pacemaker failure to capture.
- **Ectopy:** PVCs, ventricular bigeminy, ventricular trigeminy, PVC couplets, runs of VT, PACs, sinus pause.
- **Conduction:** right and left bundle branch block, shown in lead V1.

Strips that depend on an event (a PVC, a dropped beat, a pause, a run) are chosen so that the event is clearly on the strip.

Each question offers four answers: the correct one, two similar rhythms and one other. **Check Answer** says whether you were right and shows the correct rhythm with its key criteria. **Next Rhythm** moves on. The score is shown at the top. After ten strips the final score appears, and **New Quiz** starts again. Calipers work on quiz strips too, with their own **Clear Calipers** button.

---

## 6. Rhythms and physiology

| Group | Rhythms |
|---|---|
| Baseline | Leads Off, Asystole |
| Sinus | Sinus rhythm (brady / normal / tachy by rate), sinus arrhythmia; optional sinus pauses and PSVT episodes |
| Atrial | Atrial fibrillation, atrial flutter (any conduction ratio), SVT |
| Junctional | Junctional rhythm (set the rate for accelerated or tachy) |
| AV block | 1st degree, 2nd degree type I (Wenckebach), 2nd degree type II (Mobitz II), 3rd degree |
| Ventricular | Idioventricular, monomorphic VT (with or without a pulse), torsades de pointes, VF (coarse or fine) |
| Paced | Ventricular (VVI), atrial (AAI), AV dual-chamber (DDD); fixed, demand, or with intermittent failure to capture or pace |
| Ectopy | PVCs (unifocal, multifocal, R-on-T), PACs, PJCs; bigeminy, trigeminy, quadrigeminy, couplets, triplets, runs of VT |
| Conduction | **RBBB**: rSR′ in V1–V2, broad slurred S in I, aVL and V5–V6, T inversion in V1–V3. **LBBB**: no septal q, broad notched R in I, aVL and V5–V6, deep broad QS in V1–V3, discordant ST-T |

### Consistent physiology across channels

- **PVCs** give compensatory pauses and weak pulses on the Pleth and ABP.
- **V-tach with a pulse** is hypotensive.
- **Non-perfusing rhythms** (asystole, VF, torsades, pulseless VT):
  - the Pleth goes flat;
  - the arterial pressure runs off to about 17/15 (16) mmHg within about 15 seconds;
  - NBP measurements fail and EtCO₂ falls.
- **Return of a perfusing rhythm:** pressures recover over 20–30 seconds, and an NBP taken during recovery reads low.
- **Leads Off:** SpO₂ and EtCO₂ keep working. The heart keeps beating underneath, so the pulse oximeter becomes the heart-rate alarm source.
- **Natural variation:**
  - Patients generated by the unit presets get their own ECG "fingerprint": intervals, amplitudes and ST position, occasionally a bundle branch block, long QT, T inversion or ST change. They also get beat-to-beat variability.
  - Breathing varies naturally for every patient, with drifting rate and depth and occasional sighs and shallow breaths.

---

## 7. Monitored parameters

| Parameter | Display | Alarms |
|---|---|---|
| **HR** | ECG-derived heart rate | *** Asystole, *** Vent Fib/Tach, *** VTach, *** Extreme Brady / Tachy; one-star arrhythmia alarms, including * HR High / Low |
| **PVC/min** | Ectopic ventricular beats in the last minute | * PVCs/min High |
| **ST / QT** | ST-*lead* (mm), and QTc/QT in the expanded view | ** ST-*lead* High / Low, ** QTc High (after 5 minutes) |
| **SpO₂ / Pulse** | Saturation and pulse rate from the pleth | ** SpO₂ High / Low (10 s delay), *** Desat (20 s delay), SpO₂ No Pulse, SpO₂ Sensor Off. With no usable ECG: ** Pulse High / Low, *** Brady / Tachy (Pulse) |
| **RR** | Impedance respiration rate, with optional wave | ** RR High / Low, *** Apnea, Resp Leads Off |
| **NBP** | `sys/dia (mean)` on one line, with time and mode (e.g. *NBP 17:32 Auto 15 min*); live cuff pressure during a ~25 s measurement | ** NBPs High / Low, NBP Measure Failed |
| **ABP** | Arterial waveform with dicrotic notch; `sys/dia (mean)` measured from the wave, also when non-pulsatile | *** ABPs High / Low (extreme limits), ** ABPs High / Low, *** ABP Disconnect |
| **etCO₂ / awRR** | Capnogram, end-tidal CO₂ and airway respiratory rate | ** etCO₂ High / Low, ** awRR High / Low, *** Apnea, CO₂ Alarm Suppress |

NBP auto intervals: 1, 3, 5, 10, 15 and 30 min; 1, 4 and 8 hr.

---

## 8. Alarms

Alarm texts, priorities, latching and detection criteria follow the Philips IntelliVue **MX100/X3** *Instructions for Use* ("Alarms"; "Patient Alarms and INOPs"; "ECG, Arrhythmia, ST and QT Monitoring"), using the adult factory defaults.

### Display and sound

- **Priorities:**
  - red `***`;
  - yellow `**`;
  - one-star yellow arrhythmia alarms `*`;
  - technical alarms (INOPs): cyan, with no stars (for example *ECG Leads Off*).
- **Alarm text** (*Alarms → Alarm text: Enhanced*):
  - **Enhanced** shows the worst value measured during the alarm and the limit: `*** Desat 78 < 80`, `*** xBrady 35<40`, `*** Brady/P 35<40`, `*** Apnea 0:25`, `** SpO₂ 88<90`.
  - **Standard** shows the plain name: `*** Desat`, `*** Extreme Brady`, `** SpO₂ Low`.
  - When several alarms share the top priority, the message alternates every 2 seconds.
- **Sound:** the station plays the tone of the highest-priority alarm sounding anywhere on the board. The tones were synthesized to match recordings of a real central station:
  - red: a two-tone chime every second;
  - yellow: a lower chime every 2 seconds;
  - one-star: the yellow tone for 6 seconds;
  - technical: a short low tone every 2 seconds.

### Acknowledging, latching and switching alarms off

- **Acknowledge** with a double-click on the patient, or with *Silence All*.
  - A latched alarm whose condition has ended clears at once.
  - Otherwise the sector returns to black and the message is **frozen** with a ✓ until the condition has been clear for 5 seconds.
  - A **higher-priority** alarm (for example yellow escalating to red) alarms anew.
- **Latching** (*Alarms → Alarm latching*) offers the six combinations the monitor allows: visual Red&Yellow / Red Only / Off, combined with audible Red&Yellow / Red Only / Off. The default is Red Only / Red Only.

| | Not acknowledged | Acknowledged |
|---|---|---|
| Condition present | Tone and message | Message only |
| Condition ended, non-latching | Indicators stop | Indicators stop |
| Condition ended, visual + audible latching | Tone and message continue | Indicators stop |
| Condition ended, visual latching only | Message continues, tone stops | Indicators stop |

  INOPs never latch. One-star arrhythmia alarms have their own behaviour (below).
- **Alarms Off:**
  - *Manage Patient → Alarms Off* switches off all of a patient's alarms. INOPs still appear, silently. Switching alarms back on also resets the arrhythmia timeouts.
  - Each *Setup* window has *Alarms On/Off* for that measurement alone.

### Arrhythmia analysis

Every beat is labelled **N** (supraventricular), **S** (premature supraventricular), **V** (ventricular) or **P** (paced). The arrhythmia alarms are detected from these labels:

| Alarm | Criterion (default limit) |
|---|---|
| *** Asystole | No QRS for longer than the asystole threshold (4 s) |
| *** Vent Fib/Tach | Fibrillatory waveform (2–10 Hz) for 4 consecutive seconds |
| *** VTach | Run of PVCs ≥ V-Tach run (5) and HR > V-Tach HR (100) |
| *** Extreme Brady / Tachy | HR below HR low − ∆ExtrBrady, or above HR high + ∆ExtrTachy (20 bpm, clamped at 40 / 200) |
| * Non-Sustain VT | Run of PVCs shorter than the V-Tach run, with ventricular rate > V-Tach HR |
| * Vent Rhythm | Dominant rhythm of adjacent Vs longer than the vent rhythm run (14), with rate < V-Tach HR |
| * Run PVCs High | A run of more than 2 PVCs |
| * Pair PVCs | Two ventricular beats between two non-ventricular beats |
| * R-on-T PVCs | At HR < 100: a PVC earlier than ⅓ of the average R-R with a 1.25× compensatory pause, or two such PVCs within 5 minutes |
| * Vent Bigeminy / Trigeminy | Dominant N V N V / N N V pattern |
| * PVCs/min High | PVCs within a minute exceed the limit (10) |
| * Multiform PVCs | Two V shapes, each seen at least twice in the last 300 beats and at least once in the last 60 |
| * Pause | No beat for longer than the pause threshold (2 s) |
| * Missed Beat | No beat for > 1.75 × the averaged R-R (or > 1 s above 120 bpm); non-paced patients only |
| * Pacer Not Capt / * Pacer Not Pacing | Paced patients: no beat for > 1.75 × the averaged R-R, with / without a pace pulse |
| * SVT | Run of SV beats longer than the SVT run (5), with HR > SVT HR (180) |
| * HR High / Low | HR beyond the HR limits |
| * AFIB / * End AFIB | Atrial fibrillation detected / no longer detected for the end delay (60 s) |
| * Irregular HR / * End Irregular HR | Consistently irregular R-R intervals other than AF / no longer detected for the end delay |

- **One-star alarm behaviour:** the yellow lamp and tone are active for **6 seconds**. The message then stays until the condition ends, plus up to **3 minutes**.
- **Alarm chains:** one-star alarms are grouped in four chains. Within each chain, alarms are listed from highest to lowest priority:
  - **PVC:** Non-Sustain VT / Vent Rhythm > Run PVCs > Pair PVCs > R-on-T > Bigeminy > Trigeminy > PVCs/min > Multiform.
  - **Missed Beat:** Pause > Pacer Not Capt / Pacer Not Pacing / Missed Beat.
  - **Arrhy HR:** SVT > HR High/Low.
  - **AFib/HR:** AFIB / End AFIB > Irregular HR / End Irregular HR.

  Only the highest condition in each chain is announced. Across chains, the most recent one is shown.
- **Timeouts:** announcing an alarm starts a timeout. During it, that alarm and those below it in its chain do not re-alarm.
  - *TimeOut 1st* (3 minutes) covers the upper alarms of each chain.
  - *TimeOut 2nd* (10 minutes) covers Bigeminy, Trigeminy, PVCs/min and Multiform.
  - AFIB and Irregular HR have no timeout.
  - When a timeout expires and the condition is still (or again) present, the tone sounds again as a **reminder**, even if the alarm was acknowledged.
- **Acknowledging a one-star alarm** clears its message at once if the condition has stopped; otherwise the message stays until the condition stops.
- **HR High/Low** are short yellow (`* HR High`) by default. *Alarms → HR High/Low as standard yellow* makes them `** HR High`: independent of the chains, with no timeout.
- **Relearn:** the monitor relearns the rhythm when ECG monitoring is switched on, the ECG lead is changed, or a Leads Off lasting more than 60 seconds ends.
  - The ECG shows *Learning ECG*, then *Learning Rhythm*, for about 16 seconds.
  - Meanwhile only Asystole, Vent Fib/Tach and HR alarms are active.
  - Changing the rhythm itself does not trigger a relearn.
- **Individual arrhythmia alarms** can be switched on or off in *Setup Arrhythmia*, along with *All Yellow Off / On*. The INOP *Some ECG AlarmsOff* is shown while any are off.

### Other alarm criteria and INOPs

- **Timing:**
  - ** SpO₂ Low after 10 seconds; *** Desat after 20 seconds;
  - ** QTc High when the limit has been exceeded for more than 5 minutes.
- **ABP:**
  - *** ABPs High / Low beyond the extreme limits (200 / 70);
  - ** ABPs High / Low beyond the normal limits;
  - *** ABP Disconnect when the trace is non-pulsatile with a mean below 10 mmHg.
- **CO₂** alarms are suppressed (*CO₂ Alarm Suppress*) until the first breath is detected.
- *NBP Measure Failed* stays visible until the next measurement starts.
- **INOPs:**
  - ECG: ECG Leads Off; ECG Noisy Signal or Cannot Analyze ECG (severe artifact); ECG/Arrh AlarmsOff; Some ECG AlarmsOff.
  - Other measurements: SpO₂ Sensor Off; SpO₂ No Pulse; Resp Leads Off; NBP Measure Failed; CO₂ Alarm Suppress.

### Limits

- **Station-wide defaults:** *Alarms → Alarm and arrhythmia limits*.
- **Per patient:** a patient's *Setup* windows, or *Measurements → Alarm limits table*.
- **What can be set:**
  - **ECG and arrhythmia:** HR low/high, ∆ExtrBrady and ∆ExtrTachy, V-Tach HR and run, vent rhythm run, SVT HR and run, PVCs/min, asystole and pause thresholds, AFib/IHR end delay, ST high/low, QTc.
  - **Other measurements:** SpO₂ high/low and desat, RR and apnea time, NBPs, ABPs and ABPs extreme, etCO₂.

---

## 9. Scenario presets

The **Patients** menu fills the whole board:

| Preset | Character |
|---|---|
| **Randomize** | Any rhythm, including lethal ones |
| **Typical ER telemetry board** | Mostly sinus rhythms and AF, some paced and block rhythms; auto NBP on most patients; capnography rare |
| **Med-surg telemetry floor** | Older, stable patients: more rate-controlled AF, pacemakers and sinus brady. Unstable rhythms are rare. No SpO₂ or NBP on the monitor; more motion artifact |
| **Step-down unit** | Sicker: more sinus tach, AF with RVR, flutter, SVT and blocks; SpO₂ and NBP on most (mostly hourly); a few arterial lines |
| **ICU** | The sickest mix: SpO₂ on nearly everyone, many arterial lines and capnography |

- **No lethal rhythms in the unit presets.** None of them include lethal rhythms; add those yourself for drills.
- **Scenario odds…** sets, per unit, the percentage of patients with SpO₂, auto NBP, an arterial line, capnography, artifact, and electrodes that come off now and then.
- **Leads off in every preset.** Every preset, and *Randomize*, includes a few patients whose leads are off and a few whose **electrodes come off now and then**:
  - about three times an hour, the ECG and impedance resp show *ECG Leads Off*;
  - most episodes last 10–50 seconds (patient moving, loose electrode); some last 1–4 minutes (off to the bathroom);
  - then the leads reconnect. A Leads Off of more than 60 seconds triggers an arrhythmia relearn.

---

## 10. Menus

| Menu | Items |
|---|---|
| **File** | Exit |
| **Patients** | Set up any bed, Activate all beds, Put all beds in Standby, Randomize, the unit presets, Scenario odds, Reset all beds to Leads Off |
| **View** | Number of sectors (4–16), columns (Auto, 1–4), sweep speed, rhythm labels, frame rate, alarm messages on/off |
| **Alarms** | Silence all, alarm and arrhythmia limits, alarm text (Enhanced / Standard), alarm latching, HR High/Low as standard yellow, sound on/off, volume |
| **Events** | Event Log |
| **Quiz** | Rhythm Quiz (10 strips) |
| **Help** | How to use, About TeleSim |

---

## 11. Teaching ideas

- **Rhythm recognition:** turn off *View → Show alarm messages* and *Show rhythm labels*, load a preset, and ask learners to identify every rhythm on the board. Or use the built-in **Rhythm Quiz**.
- **Strip measurement:** have learners measure PR, QRS and QT with the calipers and record them in the event log. Compare their entries afterwards in *Events → Event Log*.
- **Alarm response:**
  1. Run a med-surg or step-down board.
  2. Change one patient to VT, pulseless VT, VF or asystole.
  3. Watch who notices first, who opens the alarm strip, and how quickly they respond.
- **Alarm fatigue:** use a board with artifact and electrodes coming off. Discuss INOPs, latching and how acknowledgements behave.
- **Hemodynamics:** put an arterial line on a patient and switch between VT with and without a pulse to show the pressure change. Then convert back to sinus rhythm and watch the pressure recover.

---

## 12. Building, project layout and tests

TeleSim is written in **Go** using only the standard library. The Windows interface talks directly to the Win32 API (`user32`, `gdi32`, `winmm`, `shcore`) through `syscall`, so there are no third-party dependencies and no cgo.

It needs Go 1.21 or newer (it was developed with Go 1.24) and can be built on any OS:

```sh
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui -s -w" -o TeleSim.exe .
```

On Windows PowerShell:

```powershell
$env:GOOS="windows"; $env:GOARCH="amd64"
go build -trimpath -ldflags "-H windowsgui -s -w" -o TeleSim.exe .
```

The program icon is already compiled into `rsrc_windows_amd64.syso`, so the commands above produce an .exe with its icon. To redraw or change it (needs Python with Pillow, and LLVM's `llvm-rc` and `llvm-cvtres`):

```sh
python3 tools/mkicon.py TeleSim.ico
cd tools && cp ../TeleSim.ico . && llvm-rc /fo app.res app.rc \
  && llvm-cvtres /machine:x64 /out:../rsrc_windows_amd64.syso app.res && rm app.res TeleSim.ico
```

On Linux or macOS, `go run .` does not open the GUI. It runs a preview tool that renders test strips of every rhythm to PNG files in `./preview`.

| File | Purpose |
|---|---|
| `ecg.go` | Core engine: rhythms, beat scheduling, ectopy, pacing, PSVT and sinus pauses, ECG leads and morphology (including BBB), artifact, pleth, respiration, measurements, alarm evaluation and history |
| `arrhythmia.go` | Beat labelling, arrhythmia detection, alarm chains, timeouts, latching, relearn, rhythm and ectopic status messages |
| `vitals.go` | ABP, NBP (cuff cycle and auto intervals), capnography, and the perfusion model (pressure fall-off and recovery) |
| `scenarios.go` | Randomize and unit presets, natural per-patient variation, scenario odds |
| `quiz.go` | Rhythm quiz question bank and strip generator |
| `alarmsynth.go` | Alarm tone synthesis (WAV generation) |
| `main_windows.go` | Program entry, menus, message loop, sweep drawing, 10-second strip window, help text |
| `sector_windows.go` | Board layout, patient sectors, alarm boxes, hover buttons, mouse hit-testing |
| `expanded_windows.go` | Expanded full-width patient view |
| `params_windows.go` | Numerics (including BP), ABP and CO₂ waves |
| `meas_windows.go` | Measurement Setup windows |
| `setup_windows.go` | Manage Patient dialog |
| `tune_windows.go` | Fine Tune dialog |
| `limits_windows.go` | Alarm limits table (station-wide and per patient) |
| `review_windows.go` | Alarm review window |
| `events_windows.go` | Alarm strips, calipers, event-log panel and Event Log list |
| `quiz_windows.go` | Rhythm quiz window |
| `odds_windows.go` | Scenario odds dialog |
| `sound_windows.go` | Alarm sound playback and priority |
| `about_windows.go` | About window (Help → About TeleSim) |
| `smiley.go` | Vector outlines of the About window's smiley face, traced from `tools/smiley.png` by `tools/tracesmiley.py` |
| `crashlog_windows.go` | Writes `TeleSim-crash.log` next to the .exe if the program crashes |
| `win32.go` | Minimal Win32 bindings and GDI helpers; loads the program icon |
| `TeleSim.ico` | Program icon (16–256 px), drawn by `tools/mkicon.py` |
| `rsrc_windows_amd64.syso` | The icon compiled as a Windows resource; `go build` links it into the .exe automatically |
| `tools/` | `mkicon.py` (draws the icon), `app.rc` (resource script), `smiley.png` and `tracesmiley.py` (the smiley drawing and its tracer) |
| `preview_other.go` | Non-Windows preview renderer |
| `rate_test.go`, `soak_test.go` | Tests |
| `LICENSE` | The GNU General Public License, version 3 |

The engine files (`ecg.go`, `arrhythmia.go`, `vitals.go`, `scenarios.go`, `quiz.go`, `alarmsynth.go`) are platform-independent. Everything with the `_windows.go` suffix is the Win32 front end.

Run the tests with:

```sh
go test ./...
```

- `rate_test.go` steps every rhythm through its full rate range and checks that the simulated rate matches the setting.
- `soak_test.go` runs dozens of patients across all rhythms and presets for 25 simulated minutes, reconfiguring them as it goes. It checks for unbounded memory growth and invalid samples.

---

## 13. Notes and limitations

- Settings (patients, limits, odds) are not saved between sessions, and the event log is cleared when the program closes.
- Physical sweep-speed calibration relies on the monitor size Windows reports. Some TVs, projectors and remote-desktop sessions report it incorrectly.
- Alarm behaviour follows the published IntelliVue documentation, but TeleSim is an independent simulator. It is not affiliated with or endorsed by Philips.
- Alarm tones approximate a real central station. They were synthesized from analysis of recordings, not copied.
- Waveforms are synthetic models designed to look and behave realistically for teaching. They are not derived from patient data.
- If the program ever crashes, send the `TeleSim-crash.log` file created next to the .exe.

---

## License

Vibe-coded by Jeffrey Olvesen.

TeleSim is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, version 3 of the License. It is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the [`LICENSE`](LICENSE) file for the full terms.

**For education and training only. Not a medical device.**
