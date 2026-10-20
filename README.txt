Central Telemetry Station Simulator (training use only - not a medical device)

Run TelemetrySim.exe (no install needed, Windows 10/11 64-bit).
Windows SmartScreen may warn because the program is unsigned: More info -> Run anyway.

- All beds start in Standby. Click a standby sector and press Activate to bring it
  online (starts with ECG leads off unless you pick a rhythm). Tick "Standby" in
  setup to pause a bed again; Patients menu can activate/standby all at once.
- Expand button (rightmost): docks the patient full-width at the bottom with
  3 ECG leads, Pleth, Resp and all measurements.
- Hover over a patient for buttons (strip, Adjust Rhythm, Fine Tune, Alarm
  Limits, silence). Double-click a patient to silence its alarm. New alarms
  turn the sector blue.
- Click a patient sector (or Patients menu) to set rhythm, rate, ectopy type,
  pattern and frequency. "Apply" changes the rhythm live.
- Patients menu: randomize every bed's rhythm, or simulate a typical ER
  telemetry board (realistic mix, no lethal rhythms).
- Atrial flutter: choose 2:1, 3:1, 4:1, 5:1 or variable conduction; the rate box
  then sets the flutter-wave rate (ventricular rate = flutter rate / ratio).
- SpO2: choose Monitored + target % in setup for a cyan pleth wave and SpO2
  numeric (Sensor off shows -?-). Pleth follows actual beats.
- Paced beats show a white pacer spike from the top of the ECG area.
- Grey strip icon (top-right of each sector) opens a 10 s black-on-white
  freeze-frame on ECG paper (25 mm/s, 10 mm/mV). Esc closes it.
- Alarms: red alarms flash the sector red; the station sounds the highest
  unsilenced alarm. Each sector has a Silence button; top bar has Silence All.
  Alarms menu: sound on/off, volume.
- Pacemakers: VVI, AAI, DDD; Fixed (100% paced) or Demand (paces only below
  the set rate; set the patient's intrinsic rate).
- Alarms > Alarm limits: yellow limits plus extreme PANIC limits (red alarm
  tone, flashing red header).
- Artifact (Patient Setup): tremor, 60 Hz, wandering baseline, movement,
  loose lead; mild/moderate/severe.
- Tune button in each sector: PR/QRS/QT, P/QRS/T amplitude, ST elevation.
- Click the lead label on an ECG to choose I, II, III, aVR, aVL, aVF, V1-V6.
- Respiration: Rate / Wave checkboxes (wave needs rate); unticked items are
  removed; resp alarms off only when both are off. Breath rate 0 = apnea.
- Limits button per sector: that patient's own alarm limits.
- Default view: 12 patients in 3 columns. ER board includes some artifact.
- Monitoring toggles: Cardiac / SpO2 / Resp each on/off; hidden = no alarms.
- Sweep speed: real-world 25 mm/s (ECG, SpO2) and 6.25 mm/s (resp),
  calibrated to your monitor; 12.5 / 50 mm/s in the View menu.
- Pulse-ox-only patients: untick Cardiac (or Leads Off) and set Pulse rate;
  a cyan Pulse numeric replaces HR and HR limits apply to the pulse.
- ED board: natural per-patient variation (intervals, amplitudes, ST) and
  beat-to-beat variability; see/edit via Tune.
- ST/QTc measurement and alarms; NBP (manual/auto, cuff cycle), optional ABP
  arterial line and EtCO2 capnography with alarms (limits in Alarm Limits).
- Presets: ER board, med-surg tele floor, step-down unit, ICU; Patients > Scenario
  odds adjusts the % with SpO2, NBP, ABP, EtCO2 and artifact per unit.
- Click the NBP title on a patient to pick the interval (Auto 1 min-8 hr),
  Manual, Measure now or Off.
- View menu: columns (Auto or 1-4), 4/6/8/12/16 sectors; alarm messages on/off.

Rebuild from source (Go 1.21+, any OS):
  GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w" -o TelemetrySim.exe .
