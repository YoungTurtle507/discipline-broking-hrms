package attendance

import "time"

var ValidDirections = map[string]bool{
	"IN":  true,
	"OUT": true,
}

// ValidOutReasons mirrors the attendance_out_reason enum: why an employee is
// marking OUT before their shift ends (see calculation.Repository.IsBeforeShiftEnd),
// while already marked in. HALF_DAY forces the day's status regardless of
// hours worked; LUNCH and PERSONAL each expect a return later today, so the
// multi-session pairing excludes the gap from worked minutes — the two are
// tracked separately only so the timesheet can say which kind of break it
// was, they behave identically in the calculation; EARLY_LEAVE is a genuine
// final checkout for the day — that OUT becomes the day's office-out time
// directly, same as an unreasoned departure would.
//
// LATE_COMING is a fifth reason, valid only on an IN — not offered by the
// live kiosk (arrival time alone already decides lateness there), only by a
// manual correction (see RecordManual), for when HR knows an entry should
// count as late even though the time being typed in wouldn't otherwise read
// that way. LUNCH and PERSONAL are also offered on an IN — pairing the
// return scan with the same label as the OUT that started it (see
// labelAttendanceEvents on the frontend) rather than only ever being inferred
// from the prior OUT.
var ValidOutReasons = map[string]bool{
	"HALF_DAY":    true,
	"LUNCH":       true,
	"PERSONAL":    true,
	"EARLY_LEAVE": true,
}

// ValidManualReason reports whether reason is one of the options offered for
// direction on a manual entry — Late Coming, Half Day, Lunch or Personal when
// correcting an IN, any of ValidOutReasons when correcting an OUT. An empty
// reason is always valid (no reason selected).
func ValidManualReason(direction, reason string) bool {
	if reason == "" {
		return true
	}
	if direction == "IN" {
		return reason == "LATE_COMING" || reason == "HALF_DAY" || reason == "LUNCH" || reason == "PERSONAL"
	}
	return ValidOutReasons[reason]
}

// NoteRequiredReasons are the declared reasons that must carry a free-text
// explanation — Half Day and Early Leave both need HR to know why, and
// Personal likewise since (unlike Lunch) it isn't self-explanatory. Lunch and
// Late Coming don't require one. Enforced both here (kiosk's mid-day OUT
// prompt and self check-in) and by the frontend controls offering these
// reasons — the frontend just gives an earlier error.
var NoteRequiredReasons = map[string]bool{
	"HALF_DAY":    true,
	"EARLY_LEAVE": true,
	"PERSONAL":    true,
}

type Event struct {
	ID           int64     `json:"id"`
	EmployeeID   int64     `json:"employee_id"`
	EmployeeName string    `json:"employee_name"`
	Direction    string    `json:"direction"`
	EventTime    time.Time `json:"event_time"`
	Confidence   float64   `json:"confidence"`
	ModelName    string    `json:"model_name"`
	ModelVersion string    `json:"model_version"`
	VerifiedBy   *int64    `json:"verified_by"`
	// Source distinguishes a verified face match from an event an admin
	// entered because the employee forgot to scan. Note carries the admin's
	// reason on a manual entry.
	Source string `json:"source"`
	Note   string `json:"note"`
	// OutReason carries why this event was flagged, if at all — see
	// ValidManualReason for which values are valid for which direction. Nil
	// when no reason applies.
	OutReason *string `json:"out_reason"`
}

// SourceFace, SourceManual, SourceSelf, and SourceAutoClosed mirror the
// attendance_source enum. SourceSelf is an employee self-reporting their own
// IN/OUT with no verification at all — only usable where remote_self_checkin
// is enabled on that employee (see employee.Repository.SetRemoteSelfCheckin).
// SourceAutoClosed marked a synthetic OUT the system used to insert itself
// when a day ended with no scan-out at all. The mechanism that generated
// these (Repository.AutoCloseMissingOuts) has been removed — a day with a
// dangling scan-IN and no scan-OUT is now left flagged MISSING_OUT for HR to
// resolve by hand via a manual entry, same as any other unfixable exception.
// The constant remains only so any already-inserted synthetic events from
// before the removal keep being recognised as such.
const (
	SourceFace       = "FACE"
	SourceManual     = "MANUAL"
	SourceSelf       = "SELF"
	SourceAutoClosed = "AUTO_CLOSED"
)
