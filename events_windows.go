//go:build windows

package main

// Strip tools: virtual calipers on the 10-second strip, and the event log.
//
// Calipers: drag across the strip with the left mouse button to measure a
// time interval (shown in ms and as a rate); drag up/down with the right
// button to measure an amplitude (mV / mm). Drag a caliper line to move it.
//
// Event log: every strip window has measurement boxes (rhythm, HR, PR, QRS,
// QT, QTc, R-R, ST) and a free-text note; "Save to Event Log" stores the
// strip with them - and, for an alarm strip (double-click a red/yellow alarm
// message), the alarm name and time. Events are kept in memory only: they
// are cleared when the program closes. Events > Event Log lists them.

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

// Event is one saved recording.
type Event struct {
	ID         int
	At         time.Time // when the strip was recorded (alarm onset for alarm strips)
	Bed        int
	Name, Lead string
	HR, SpO2   string
	Alarm      string  // the alarm that triggered the strip (or was active at the time)
	FromAlarm  bool    // recorded from an alarm (double-click on the alarm message)
	Onset      float64 // alarm onset, seconds from the strip start
	AlarmLvl   int
	Status     string // rhythm / ectopic status at the time
	Data       []float32
	Paces      []float64
	Meas       [len(eventFields)]string
	Note       string
}

// measurement boxes of an event
var eventFields = [...]struct{ label, short, unit string }{
	{"Rhythm interpretation", "Rhythm", ""},
	{"Heart rate", "HR", "bpm"},
	{"PR interval", "PR", "ms"},
	{"QRS duration", "QRS", "ms"},
	{"QT interval", "QT", "ms"},
	{"QTc", "QTc", "ms"},
	{"R-R interval", "R-R", "ms"},
	{"ST deviation", "ST", "mm"},
}

var (
	events  []*Event // session only: cleared when the program closes
	eventID int
)

// caliper state of a strip window
type calipers struct {
	t        [2]float64 // time calipers (s from strip start)
	tOn      bool
	v        [2]float64 // amplitude calipers (mV)
	vOn      bool
	drag     int // 0 none, 1/2 time line, 3/4 amplitude line
	gl, pxmm float64
	gt, gb   float64
	yb       float64
}

func (c *calipers) xOf(sec float64) float64 { return c.gl + sec*25*c.pxmm }
func (c *calipers) secAt(x float64) float64 { return clamp((x-c.gl)/(25*c.pxmm), 0, 10) }
func (c *calipers) yOf(mv float64) float64  { return c.yb - mv*10*c.pxmm }
func (c *calipers) mvAt(y float64) float64 {
	return (c.yb - clamp(y, c.gt, c.gb)) / (10 * c.pxmm)
}

// IntervalMs is the current time-caliper reading (0 = none).
func (c *calipers) IntervalMs() int {
	if !c.tOn {
		return 0
	}
	return int(math.Round(math.Abs(c.t[1]-c.t[0]) * 1000))
}

// strip panel controls
type stripPanel struct {
	labels  [len(eventFields)]uintptr
	edits   [len(eventFields)]uintptr
	units   [len(eventFields)]uintptr
	noteLbl uintptr
	note    uintptr
	btns    [4]uintptr // insert caliper, clear calipers, save, delete
	status  uintptr
	last    int // index of the measurement box last focused (-1 = none)
}

const (
	ctlStripCal    = 501
	ctlStripClear  = 502
	ctlStripSave   = 503
	ctlStripDelete = 504
	ctlStripEdit   = 520 // + field index
	ctlStripNote   = 540
	enSetFocus     = 0x0100
	wmSize         = 0x0005
)

// stripHeader / stripPanelH are the heights of the header above the grid
// and the event-log panel below it.
func stripHeader(s *Strip) int32 { return S(84) }
func stripPanelH() int32         { return S(124) }

// stripButtons creates the event-log panel of a strip window.
func stripButtons(hwnd uintptr, s *Strip) {
	if s.quiz != nil {
		quizButtons(hwnd, s)
		return
	}
	ctlParent = hwnd
	uiFont = font(S(14), false, "Segoe UI")
	pn := &s.panel
	pn.last = -1
	var cur [len(eventFields)]string
	note := ""
	if s.ev != nil {
		cur, note = s.ev.Meas, s.ev.Note
	}
	for i, f := range eventFields {
		pn.labels[i] = ctl("STATIC", f.short+":", 0, 0, 0, 10, 10, 0)
		pn.edits[i] = ctl("EDIT", cur[i], WS_TABSTOP|ES_AUTOHSCROLL, 0, 0, 10, 10, ctlStripEdit+i)
		pn.units[i] = ctl("STATIC", f.unit, 0, 0, 0, 10, 10, 0)
	}
	pn.noteLbl = ctl("STATIC", "Note:", 0, 0, 0, 10, 10, 0)
	pn.note = ctl("EDIT", note, WS_TABSTOP|0x0004|0x1000|0x00200000|0x0040, 0, 0, 10, 10, ctlStripNote) // ES_MULTILINE|ES_WANTRETURN|WS_VSCROLL|ES_AUTOVSCROLL
	pn.btns[0] = ctl("BUTTON", "← Caliper into box", WS_TABSTOP, 0, 0, 10, 10, ctlStripCal)
	pn.btns[1] = ctl("BUTTON", "Clear Calipers", WS_TABSTOP, 0, 0, 10, 10, ctlStripClear)
	pn.btns[2] = ctl("BUTTON", "Save to Event Log", WS_TABSTOP|BS_DEFPUSHBUTTON, 0, 0, 10, 10, ctlStripSave)
	pn.btns[3] = ctl("BUTTON", "Delete Event", WS_TABSTOP, 0, 0, 10, 10, ctlStripDelete)
	pn.status = ctl("STATIC", "", 0, 0, 0, 10, 10, 0)
	var rc RECT
	call(pGetClientRect, hwnd, uintptr(unsafe.Pointer(&rc)))
	layoutStripPanel(s, rc.Right, rc.Bottom)
	updateStripStatus(s)
}

// layoutStripPanel positions the panel controls along the bottom.
func layoutStripPanel(s *Strip, w, h int32) {
	if s.quiz != nil {
		layoutQuiz(s, w, h)
		return
	}
	pn := &s.panel
	if pn.note == 0 {
		return
	}
	mv := func(hw uintptr, x, y, cw, ch int32) {
		call(pSetWindowPos, hw, 0, uintptr(x), uintptr(y), uintptr(cw), uintptr(ch), 0x0004) // SWP_NOZORDER
	}
	m := S(16)
	y0 := h - stripPanelH() + S(6)
	eh := S(24)
	left := int32(float64(w) * 0.62) // measurement boxes on the left, note on the right
	// row 1: rhythm interpretation
	mv(pn.labels[0], m, y0+S(3), S(52), S(20))
	mv(pn.edits[0], m+S(54), y0, left-m-S(54)-S(10), eh)
	mv(pn.units[0], 0, 0, 0, 0)
	// row 2: the numeric boxes
	n := len(eventFields) - 1
	cw := (left - m - S(10)) / int32(n)
	for i := 1; i <= n; i++ {
		x := m + cw*int32(i-1)
		y := y0 + S(30)
		mv(pn.labels[i], x, y, cw, S(18))
		mv(pn.edits[i], x, y+S(18), cw-S(30), eh)
		mv(pn.units[i], x+cw-S(29), y+S(22), S(29), S(18))
	}
	// row 3: buttons and status
	by := y0 + S(80)
	bw := (left - m - S(10) - S(24)) / 4
	bw = min(bw, S(150))
	for i, b := range pn.btns {
		mv(b, m+(bw+S(8))*int32(i), by, bw, S(28))
	}
	mv(pn.status, left, h-S(24), w-m-left, S(20))
	// note
	mv(pn.noteLbl, left, y0+S(3), S(60), S(20))
	mv(pn.note, left, y0+S(22), w-m-left, stripPanelH()-S(52))
}

func updateStripStatus(s *Strip) {
	if s.quiz != nil {
		return
	}
	msg := "Not in the event log yet"
	if s.ev != nil {
		msg = fmt.Sprintf("Saved in the event log as event #%d", s.ev.ID)
	}
	call(pSetWindowTextW, s.panel.status, uintptr(unsafe.Pointer(wstr(msg))))
	enable(s.panel.btns[3], s.ev != nil)
}

// stripTool handles caliper mouse input and the panel. It returns true when
// it handled the message.
func stripTool(hwnd uintptr, s *Strip, msg uint32, wp, lp uintptr) bool {
	c := &s.cal
	x, y := float64(lpX(lp)), float64(lpY(lp))
	pad := float64(S(4))
	inGrid := c.pxmm > 0 && x >= c.gl-pad && x <= c.gl+250*c.pxmm+pad && y >= c.gt && y <= c.gb
	near := func(a, b float64) bool { return math.Abs(a-b) <= float64(S(6)) }
	switch msg {
	case wmSize:
		layoutStripPanel(s, int32(lp&0xffff), int32((lp>>16)&0xffff))
		call(pInvalidateRect, hwnd, 0, 0)
		return false
	case WM_LBUTTONDOWN:
		if !inGrid {
			return false
		}
		switch {
		case c.tOn && near(x, c.xOf(c.t[0])):
			c.drag = 1
		case c.tOn && near(x, c.xOf(c.t[1])):
			c.drag = 2
		default: // a new pair
			c.t[0], c.t[1], c.tOn, c.drag = c.secAt(x), c.secAt(x), true, 2
		}
		call(pSetCapture, hwnd)
	case WM_RBUTTONDOWN:
		if !inGrid {
			return false
		}
		switch {
		case c.vOn && near(y, c.yOf(c.v[0])):
			c.drag = 3
		case c.vOn && near(y, c.yOf(c.v[1])):
			c.drag = 4
		default:
			c.v[0], c.v[1], c.vOn, c.drag = c.mvAt(y), c.mvAt(y), true, 4
		}
		call(pSetCapture, hwnd)
	case WM_MOUSEMOVE:
		switch c.drag {
		case 1, 2:
			c.t[c.drag-1] = c.secAt(x)
		case 3, 4:
			c.v[c.drag-3] = c.mvAt(y)
		default:
			return true
		}
	case wmLButtonUp, wmRButtonUp:
		if c.drag != 0 {
			c.drag = 0
			call(pReleaseCapture)
		}
		if c.tOn && c.IntervalMs() == 0 {
			c.tOn = false
		}
		if c.vOn && math.Abs(c.v[1]-c.v[0]) < 0.005 {
			c.vOn = false
		}
	case WM_COMMAND:
		id, code := loword(wp), hiword(wp)
		if s.quiz != nil {
			return quizCommand(hwnd, s, id)
		}
		switch {
		case id >= ctlStripEdit && id < ctlStripEdit+len(eventFields):
			if code == enSetFocus {
				s.panel.last = id - ctlStripEdit
			}
			return true
		case id == ctlStripNote:
			return true
		case id == ctlStripCal:
			insertCaliper(hwnd, s)
		case id == ctlStripClear:
			c.tOn, c.vOn = false, false
		case id == ctlStripSave:
			saveEvent(s)
			updateStripStatus(s)
		case id == ctlStripDelete:
			if s.ev != nil {
				deleteEvent(s.ev)
			}
		default:
			return false
		}
	default:
		return false
	}
	call(pInvalidateRect, hwnd, 0, 0)
	return true
}

// insertCaliper copies the caliper reading into the measurement box last
// selected (as a rate for HR, in mm for ST).
func insertCaliper(hwnd uintptr, s *Strip) {
	c := &s.cal
	i := s.panel.last
	if i <= 0 {
		messageBox(hwnd, "Click into a measurement box (HR, PR, QRS, ...) first, then press this button.", "Calipers", 0x40)
		return
	}
	var v string
	switch eventFields[i].unit {
	case "ms":
		if ms := c.IntervalMs(); ms > 0 {
			v = strconv.Itoa(ms)
		}
	case "bpm":
		if ms := c.IntervalMs(); ms > 0 {
			v = strconv.Itoa(int(math.Round(60000 / float64(ms))))
		}
	case "mm":
		if c.vOn {
			v = strconv.FormatFloat(math.Round(math.Abs(c.v[1]-c.v[0])*100)/10, 'f', 1, 64)
		}
	}
	if v == "" {
		messageBox(hwnd, "Place the calipers on the strip first (left drag for an interval, right drag for an amplitude).", "Calipers", 0x40)
		return
	}
	call(pSetWindowTextW, s.panel.edits[i], uintptr(unsafe.Pointer(wstr(v))))
}

// drawStripTools draws the calipers and their readout.
func drawStripTools(dc uintptr, s *Strip, w int32) {
	c := &s.cal
	if s.isAlarm && s.onset > 0 && s.onset < 10 { // when the alarm fired
		x := int32(c.xOf(s.onset))
		red := pen(rgb(210, 0, 0), int32(math.Max(1, math.Round(1.4*dpiScale))))
		for y := int32(c.gt); y < int32(c.gb); y += S(10) {
			line(dc, x, y, x, min(y+S(5), int32(c.gb)), red)
		}
		lf := font(S(12), true, "Segoe UI")
		lbl := "▼ alarm"
		tw := textWidth(dc, lbl, lf)
		fill(dc, x-tw/2-S(3), int32(c.gb)-S(18), x+tw/2+S(3), int32(c.gb)-S(2), rgb(255, 255, 255))
		text(dc, lbl, x-tw/2-S(3), int32(c.gb)-S(18), x+tw/2+S(3), int32(c.gb)-S(2), rgb(210, 0, 0), lf, DT_SINGLELINE|DT_CENTER|DT_VCENTER)
	}
	blue := rgb(0, 70, 200)
	cp := pen(blue, int32(math.Max(2, math.Round(2.4*dpiScale)))) // caliper lines: a little heavier
	rf := font(S(14), true, "Segoe UI")
	var read []string
	if c.tOn {
		x0, x1 := int32(c.xOf(c.t[0])), int32(c.xOf(c.t[1]))
		for _, x := range []int32{x0, x1} {
			line(dc, x, int32(c.gt), x, int32(c.gb), cp)
		}
		ym := int32(c.gt) + S(14)
		line(dc, x0, ym, x1, ym, cp)
		ms := c.IntervalMs()
		r := fmt.Sprintf("%d ms", ms)
		if ms > 0 {
			r += fmt.Sprintf("  (%.0f /min)", 60000/float64(ms))
		}
		read = append(read, "↔ "+r)
		lx := min(x0, x1) + (max(x0, x1)-min(x0, x1))/2
		tw := textWidth(dc, r, rf)
		fill(dc, lx-tw/2-S(4), ym+S(2), lx+tw/2+S(4), ym+S(20), rgb(255, 255, 255))
		text(dc, r, lx-tw/2-S(4), ym+S(2), lx+tw/2+S(4), ym+S(20), blue, rf, DT_SINGLELINE|DT_CENTER|DT_VCENTER)
	}
	if c.vOn {
		y0, y1 := int32(c.yOf(c.v[0])), int32(c.yOf(c.v[1]))
		r := int32(c.gl + 250*c.pxmm)
		for _, y := range []int32{y0, y1} {
			line(dc, int32(c.gl), y, r, y, cp)
		}
		d := math.Abs(c.v[1] - c.v[0])
		read = append(read, fmt.Sprintf("↕ %.2f mV (%.1f mm)", d, d*10))
	}
	hint := "Calipers: left-drag = interval, right-drag = amplitude; drag a line to move it"
	if len(read) > 0 {
		hint = "Calipers:   " + strings.Join(read, "      ")
	}
	x0 := S(16)
	if s.alarm != "" {
		x0 = w / 2
	}
	text(dc, hint, x0, S(54), w-S(16), S(76), rgb(0, 60, 170), font(S(14), len(read) > 0, "Segoe UI"), DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
}

// saveEvent stores (or updates) the strip's event from its panel.
func saveEvent(s *Strip) {
	e := s.ev
	if e == nil {
		eventID++
		at := time.Now()
		if !s.at.IsZero() {
			at = s.at
		}
		e = &Event{ID: eventID, At: at, Bed: s.bed, Name: s.name, Lead: s.lead, HR: s.hr, SpO2: s.spo2,
			Alarm: s.alarm, AlarmLvl: s.lvl, Status: s.status, Data: s.data, Paces: s.paces, FromAlarm: s.isAlarm, Onset: s.onset}
		events = append(events, e)
	}
	for i := range eventFields {
		e.Meas[i] = strings.TrimSpace(getText(s.panel.edits[i]))
	}
	// QTc by Bazett when QT and the heart rate (or R-R) are given
	if e.Meas[5] == "" && e.Meas[4] != "" {
		qt, _ := strconv.ParseFloat(e.Meas[4], 64)
		rr, _ := strconv.ParseFloat(e.Meas[6], 64)
		if hr, _ := strconv.ParseFloat(e.Meas[1], 64); rr == 0 && hr > 0 {
			rr = 60000 / hr
		}
		if qt > 0 && rr > 0 {
			e.Meas[5] = strconv.Itoa(int(math.Round(qt / math.Sqrt(rr/1000))))
			call(pSetWindowTextW, s.panel.edits[5], uintptr(unsafe.Pointer(wstr(e.Meas[5]))))
		}
	}
	e.Note = getText(s.panel.note)
	s.ev = e
	refreshEventList()
}

func deleteEvent(e *Event) {
	for i, x := range events {
		if x == e {
			events = append(events[:i], events[i+1:]...)
			break
		}
	}
	for _, st := range strips { // windows showing it are no longer saved
		if st.ev == e {
			st.ev = nil
			updateStripStatus(st)
		}
	}
	refreshEventList()
}

// openAlarmStrip shows the 10-second strip of the rhythm that triggered the
// patient's current red/yellow alarm (6 s before to 4 s after its onset).
func openAlarmStrip(bed int) {
	p := patients[bed]
	text, lvl, _ := shownAlarm(bed)
	var h *HistEntry
	for i := len(p.History) - 1; i >= 0; i-- { // the latest onset of this alarm
		e := &p.History[i]
		if IsPatientAlarm(e.Lvl) && (e.Key == p.AlarmKey || e.Text == text) {
			h = e
			break
		}
	}
	if h == nil {
		openStrip(bed)
		return
	}
	data, paces, t0 := h.Data, h.Paces, h.StripT0
	if data == nil { // less than 4 s ago: the most recent 10 s (includes the onset)
		data, paces, t0 = p.StripAt(p.TimeOf(p.Last()))
	}
	s := &Strip{data: data, bed: bed, name: p.Cfg.Name, at: h.At, when: h.At.Format("02 Jan 2006  15:04:05"),
		lead: p.Cfg.LeadName(), hr: h.HR, alarm: h.Text, lvl: lvl, paces: paces, status: h.Status, isAlarm: true, onset: h.SimT - t0}
	if p.Cfg.SpO2On == SpO2Monitored && p.SpO2Valid {
		s.spo2 = fmt.Sprintf("SpO2 %d%%", p.SpO2Disp)
	}
	showStrip(s, fmt.Sprintf("%s - Alarm strip - %s - %s", bedLabel(bed), strings.TrimSpace(h.Text), s.when))
}

// ---------------- event log list ----------------

var evListHwnd, evListBox uintptr

const (
	ctlEvListOpen   = 701
	ctlEvListDelete = 702
	ctlEvListClose  = 703
	ctlEvListBox    = 704
	lbGetCurSel     = 0x0188
	lbnDblClk       = 2
)

func openEventList() {
	if evListHwnd != 0 {
		call(pSetForegroundWin, evListHwnd)
		return
	}
	uiFont = font(S(15), false, "Segoe UI")
	w, h := S(840), S(480)
	var mr RECT
	call(pGetWindowRect, mainHwnd, uintptr(unsafe.Pointer(&mr)))
	x := mr.Left + (mr.Right-mr.Left-w)/2
	y := mr.Top + (mr.Bottom-mr.Top-h)/2
	evListHwnd = call(pCreateWindowExW, WS_EX_DLGMODALFRAME, uintptr(unsafe.Pointer(wstr("TeleSimEventList"))),
		uintptr(unsafe.Pointer(wstr("Event Log (this session only - cleared when the program closes)"))),
		WS_CAPTION|WS_SYSMENU|WS_POPUP, uintptr(x), uintptr(y), uintptr(w), uintptr(h), mainHwnd, 0, hInst, 0)
	ctlParent = evListHwnd
	evListBox = ctl("LISTBOX", "", WS_TABSTOP|WS_VSCROLL|0x00800000|0x0001, S(12), S(12), w-S(30), h-S(100), ctlEvListBox) // WS_BORDER|LBS_NOTIFY
	ctl("BUTTON", "Open", WS_TABSTOP|BS_DEFPUSHBUTTON, S(12), h-S(80), S(100), S(30), ctlEvListOpen)
	ctl("BUTTON", "Delete", WS_TABSTOP, S(120), h-S(80), S(100), S(30), ctlEvListDelete)
	ctl("BUTTON", "Close", WS_TABSTOP, w-S(130), h-S(80), S(100), S(30), ctlEvListClose)
	refreshEventList()
	call(pShowWindow, evListHwnd, SW_SHOW)
}

// eventRows lists events newest first.
func eventRows() []*Event {
	out := make([]*Event, 0, len(events))
	for i := len(events) - 1; i >= 0; i-- {
		out = append(out, events[i])
	}
	return out
}

func refreshEventList() {
	if evListHwnd == 0 {
		return
	}
	call(pSendMessageW, evListBox, lbResetContent, 0, 0)
	if len(events) == 0 {
		call(pSendMessageW, evListBox, lbAddString, 0, uintptr(unsafe.Pointer(wstr("(no events yet - double-click an alarm, or open a strip, and press Save to Event Log)"))))
		return
	}
	for _, e := range eventRows() {
		name := e.Name
		if name == "" {
			name = "Not Admitted"
		}
		l := fmt.Sprintf("#%d   %s   %s   %s", e.ID, e.At.Format("15:04:05"), bedLabel(e.Bed), name)
		switch {
		case e.FromAlarm:
			l += "   Alarm: " + strings.TrimSpace(e.Alarm)
		case e.Alarm != "":
			l += "   Strip (alarm: " + strings.TrimSpace(e.Alarm) + ")"
		default:
			l += "   Strip"
		}
		if e.Meas[0] != "" {
			l += "   – " + e.Meas[0]
		}
		if n := strings.Fields(strings.ReplaceAll(e.Note, "\r\n", " ")); len(n) > 0 {
			nt := strings.Join(n, " ")
			if len(nt) > 50 {
				nt = nt[:50] + "..."
			}
			l += "   “" + nt + "”"
		}
		call(pSendMessageW, evListBox, lbAddString, 0, uintptr(unsafe.Pointer(wstr(l))))
	}
}

func selectedEvent() *Event {
	i := int(int32(call(pSendMessageW, evListBox, lbGetCurSel, 0, 0)))
	rows := eventRows()
	if i < 0 || i >= len(rows) {
		return nil
	}
	return rows[i]
}

// openEventStrip shows a saved event's strip (it can be measured and edited).
func openEventStrip(e *Event) {
	s := &Strip{data: e.Data, bed: e.Bed, name: e.Name, at: e.At, when: e.At.Format("02 Jan 2006  15:04:05"), lead: e.Lead,
		hr: e.HR, spo2: e.SpO2, alarm: e.Alarm, lvl: e.AlarmLvl, paces: e.Paces, status: e.Status, ev: e, isAlarm: e.FromAlarm, onset: e.Onset}
	showStrip(s, fmt.Sprintf("Event #%d - %s - %s", e.ID, bedLabel(e.Bed), s.when))
}

func eventListProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch uint32(msg) {
	case WM_COMMAND:
		id, code := loword(wp), hiword(wp)
		switch {
		case id == ctlEvListOpen || (id == ctlEvListBox && code == lbnDblClk):
			if e := selectedEvent(); e != nil {
				openEventStrip(e)
			}
		case id == ctlEvListDelete:
			if e := selectedEvent(); e != nil {
				deleteEvent(e)
			}
		case id == ctlEvListClose || id == 2:
			h := evListHwnd
			evListHwnd = 0
			call(pDestroyWindow, h)
		}
		return 0
	case WM_CLOSE:
		h := evListHwnd
		evListHwnd = 0
		call(pDestroyWindow, h)
		return 0
	}
	return call(pDefWindowProcW, hwnd, msg, wp, lp)
}
