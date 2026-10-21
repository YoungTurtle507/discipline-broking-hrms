// Package calculation implements the attendance calculation engine: turning
// raw IN/OUT events into a daily result (working minutes, late/early,
// half-day, overtime, absence) per the company's rule priority.
package calculation

import "time"

type ExceptionCode string

const (
	ExMissingOut           ExceptionCode = "MISSING_OUT"
	ExDuplicateIn          ExceptionCode = "DUPLICATE_IN"
	ExDuplicateOut         ExceptionCode = "DUPLICATE_OUT"
	ExInvalidSequence      ExceptionCode = "INVALID_SEQUENCE"
	ExLowConfidence        ExceptionCode = "LOW_CONFIDENCE"
	ExLateStreakCounted    ExceptionCode = "LATE_STREAK_COUNTED"     // marker: this lateness counts toward the monthly 3-strike total
	ExLateStreakHalfDay    ExceptionCode = "LATE_STREAK_HALF_DAY"    // 3rd late day this month
	ExLateArrivalHalfDay   ExceptionCode = "LATE_ARRIVAL_HALF_DAY"   // arrived past the hard half-day cutoff
	ExLatePassUsed         ExceptionCode = "LATE_PASS_USED"          // excused via yesterday's stay-late pass
	ExUnapprovedAbsence    ExceptionCode = "UNAPPROVED_ABSENCE"      // absent with no approved leave covering it
	ExDeclaredHalfDay      ExceptionCode = "DECLARED_HALF_DAY"       // employee chose Half Day when marking out for lunch
	ExLunchOT              ExceptionCode = "LUNCH_OT"                // lunch break ran longer than the configured limit
	ExLunchOTStreakCounted ExceptionCode = "LUNCH_OT_STREAK_COUNTED" // marker: this Lunch OT day counts toward the monthly strike total
	ExLunchOTStreakPenalty ExceptionCode = "LUNCH_OT_STREAK_PENALTY" // Nth Lunch OT day this month — triggers the leave-balance penalty
)

type DayStatus string

const (
	StatusPresent   DayStatus = "PRESENT"
	StatusAbsent    DayStatus = "ABSENT"
	StatusHalfDay   DayStatus = "HALF_DAY"
	StatusHoliday   DayStatus = "HOLIDAY"
	StatusWeeklyOff DayStatus = "WEEKLY_OFF"
	StatusOnLeave   DayStatus = "ON_LEAVE"
	StatusInactive  DayStatus = "INACTIVE"
)

// EngineVersion is stamped on every result so historical calculations stay
// interpretable if these rules change later.
const EngineVersion = "v2"

// LowConfidenceThreshold flags a match for review even though it cleared the
// accept threshold (faceregistration.MatchThreshold) — a wider safety margin
// than "did it match at all".
const LowConfidenceThreshold = 0.5

// HalfDayCutoffHour/HalfDayCutoffMinute, AbsentCutoffHour/AbsentCutoffMinute,
// StayLatePassThresholdMinutes, and LateStreakLimit all used to be hardcoded
// constants here — they're now admin-configurable (see policy.go) and read
// off ctx.Policy inside CalculateDay instead. DefaultPolicySettings carries
// forward the exact same values this engine always used (10:31 AM half-day
// cutoff, 2:36 PM absent cutoff, a 30-minute stay-late pass, every 3rd late
// arrival a strike), so nothing changes until a Super Admin actually edits
// one from Organization.

type ShiftInfo struct {
	// Name is only populated where a caller needs to key a rule off which
	// shift this is (see range_summary.go's overtime eligibility) — the core
	// per-day rules below are all driven by the fields below it, not by name.
	Name               string
	StartTime          time.Time // time-of-day only; date component ignored
	EndTime            time.Time
	GracePeriodMinutes int

	// Saturday* optionally overrides the field above for a Saturday — the
	// office runs shorter hours that day than Monday-Friday. A nil field
	// means "same as the weekday value" for that one field specifically.
	// Never read directly outside ResolvedFor: every caller that builds a
	// DayContext for a specific date must resolve against that date first.
	SaturdayStartTime          *time.Time
	SaturdayEndTime            *time.Time
	SaturdayGracePeriodMinutes *int
}

// ResolvedFor returns the shift timing actually in effect on date — the
// Saturday override for whichever fields it sets, when date is a Saturday,
// otherwise the regular weekday timing untouched. Safe to call on a nil
// receiver (returns nil) so callers with no shift at all don't need a guard.
func (s *ShiftInfo) ResolvedFor(date time.Time) *ShiftInfo {
	if s == nil || date.Weekday() != time.Saturday {
		return s
	}
	resolved := *s
	if s.SaturdayStartTime != nil {
		resolved.StartTime = *s.SaturdayStartTime
	}
	if s.SaturdayEndTime != nil {
		resolved.EndTime = *s.SaturdayEndTime
	}
	if s.SaturdayGracePeriodMinutes != nil {
		resolved.GracePeriodMinutes = *s.SaturdayGracePeriodMinutes
	}
	return &resolved
}

type RawEvent struct {
	Direction  string // "IN" or "OUT"
	EventTime  time.Time
	Confidence float64
	// Source is "FACE" for a camera scan, "MANUAL" for an entry an admin made
	// because the employee forgot to scan, or "SELF" for an employee
	// self-reporting their own attendance. Neither MANUAL nor SELF carries a
	// face-match confidence, so both are exempt from the confidence check below.
	Source string
	// OutReason carries why this event was flagged, if at all. On an OUT
	// scanned before the employee's own shift end, the kiosk requires one:
	// "HALF_DAY" (leaving for the rest of the day), "LUNCH" or "PERSONAL"
	// (expected back today), or "EARLY_LEAVE" (final checkout for the day,
	// informational only). A manual correction can also set one on an IN:
	// "LATE_COMING" (forces late-arrival classification), "HALF_DAY" (forces
	// the day to Half Day, same as on an OUT), or "LUNCH"/"PERSONAL" (paired
	// with the OUT that started the break, display-only). Empty otherwise.
	OutReason string
	// Note is the free-text reason attached to this event, if any (e.g. "Doctor's
	// appointment", or the justification typed on a manual correction). Not
	// read by the engine itself — display-only, e.g. reports.DailyTimesheet's
	// Reason column.
	Note string
}

// SourceManual marks an event entered by an admin rather than matched by the
// face pipeline. SourceSelf marks an employee self-reporting their own
// attendance with no face pipeline at all (see attendance.SourceSelf).
// SourceAutoClosed marked a synthetic OUT the system used to insert itself
// when a day ended with no scan-out at all. That auto-close mechanism has
// been removed — such a day is left flagged MISSING_OUT for HR to fix by
// hand instead — but the constant and this exclusion stay so any
// already-inserted synthetic events from before the removal keep behaving
// the same way if that day is ever recalculated. All three carry confidence
// 0 by construction, not because any match was weak.
const (
	SourceManual     = "MANUAL"
	SourceSelf       = "SELF"
	SourceAutoClosed = "AUTO_CLOSED"
)

type DailyResult struct {
	Status            DayStatus
	FirstIn           *time.Time
	LastOut           *time.Time
	WorkingMinutes    int
	LateMinutes       int
	EarlyLeaveMinutes int
	OvertimeMinutes   int
	// LunchOTMinutes is how far the LUNCH-tagged break (see PunchTimes) ran
	// past policy.LunchTimeLimitMinutes — 0 when there was no LUNCH break, or
	// it stayed within the limit.
	LunchOTMinutes int
	// LunchOTStreakPenalty is true when today is the LunchOTStreakLimit-th
	// Lunch OT day this month (see policy.LunchOTStreakLimit) — the signal
	// repository.go's CalculateAndStore reads to sync the fixed
	// policy.LunchOTPenaltyDays ledger deduction, the same way
	// UnapprovedAbsence drives the Uninformed Leave penalty below. Never
	// changes Status — a long lunch doesn't make the rest of the day a Half
	// Day on its own.
	LunchOTStreakPenalty bool
	Exceptions           []ExceptionCode
	CalculationVersion   string
	CompOffEarned        bool // worked on a day that was actually a scheduled holiday
	// CompOffHalfDay is true when CompOffEarned is true and the employee also
	// declared Half Day (out_reason="HALF_DAY") that day — the same
	// declared-choice-at-scan-out signal Half Day uses every other day, not
	// inferred from hours worked. Halves the Comp Off credit to 0.5 instead
	// of a full 1; always false when CompOffEarned is false.
	CompOffHalfDay    bool
	UnapprovedAbsence bool // absent with no approved leave covering it — triggers a leave-balance penalty
	// AbsenceExcused echoes back DayContext.IsExcusedAbsence — true only when
	// that override actually changed this day's outcome (an otherwise-Absent
	// day), so a caller can tell "On Leave because of a real leave request"
	// apart from "On Leave because a Super Admin manually excused it" without
	// a second database round-trip.
	AbsenceExcused bool
}

// DayContext is everything CalculateDay needs to know about one employee-day
// beyond the raw events themselves.
type DayContext struct {
	Shift               *ShiftInfo
	EmployeeActiveToday bool
	IsHoliday           bool
	IsWeeklyOff         bool
	HasApprovedLeave    bool
	// IsSandwichedLeave is true when this day is a Holiday/Weekly Off that a
	// Sandwich Leave policy has turned into a leave day (see
	// leave.Repository.applySandwichLeave) — deliberately checked before
	// IsHoliday/IsWeeklyOff below, unlike HasApprovedLeave, so this specific
	// case overrides them while an ordinary leave request that merely happens
	// to include a holiday still lets the holiday win.
	IsSandwichedLeave bool
	// IsExcusedAbsence is true when a Super Admin has manually excused this
	// specific day (see absence_overrides / leave.Repository or
	// attendance.Repository's override lookup) — e.g. the employee actually
	// called in sick but never filed a leave request. Deliberately treated
	// identically to HasApprovedLeave (StatusOnLeave, no penalty) rather than
	// as a separate status, so the override ripples through every consumer
	// of this engine's output — the leave-balance penalty, Reports'
	// "Uninformed Leave" column, and attendance-percentage math — instead of
	// only being suppressed on one screen. Only takes effect on a day that
	// would otherwise resolve to Absent (checked at each Absent site below,
	// not as an early return like Sandwich Leave/Approved Leave) — it has no
	// effect on a day the employee actually attended.
	IsExcusedAbsence         bool
	LateStreakSoFarThisMonth int // count of late days already counted this month, before today
	// LunchOTStreakSoFarThisMonth is Lunch OT's own equivalent of
	// LateStreakSoFarThisMonth — count of Lunch OT days already counted this
	// month, before today.
	LunchOTStreakSoFarThisMonth int
	PreviousWorkDayLastOut      *time.Time // last event time of the most recent prior working day, if any
	// Policy carries the admin-configurable thresholds (late streak limit,
	// half-day/absent cutoff times, stay-late pass minutes) — see policy.go.
	// Left unset (the zero value), CalculateDay falls back to this engine's
	// own long-standing defaults via resolvePolicy, so every caller that has
	// no reason to care about this — nearly every existing test — behaves
	// exactly as before this field existed.
	Policy PolicySettings
}

type session struct{ in, out time.Time }

// pairEvents walks events in time order, pairing IN→OUT into sessions.
// Consecutive INs or OUTs are duplicates and get flagged rather than
// silently dropped; a trailing IN with no OUT is MISSING_OUT.
func pairEvents(events []RawEvent) ([]session, []ExceptionCode) {
	var sessions []session
	var exceptions []ExceptionCode
	var pendingIn *time.Time

	if len(events) > 0 && events[0].Direction != "IN" {
		exceptions = append(exceptions, ExInvalidSequence)
	}

	for _, e := range events {
		if e.Direction == "IN" {
			if pendingIn != nil {
				exceptions = append(exceptions, ExDuplicateIn)
				continue
			}
			t := e.EventTime
			pendingIn = &t
			continue
		}
		// OUT
		if pendingIn == nil {
			exceptions = append(exceptions, ExDuplicateOut)
			continue
		}
		sessions = append(sessions, session{in: *pendingIn, out: e.EventTime})
		pendingIn = nil
	}

	if pendingIn != nil {
		exceptions = append(exceptions, ExMissingOut)
	}

	return sessions, exceptions
}

func onSameDayAt(reference time.Time, clock time.Time) time.Time {
	return time.Date(
		reference.Year(), reference.Month(), reference.Day(),
		clock.Hour(), clock.Minute(), clock.Second(), 0,
		reference.Location(),
	)
}

func sumSessionMinutes(sessions []session) int {
	total := 0
	for _, s := range sessions {
		total += int(s.out.Sub(s.in).Minutes())
	}
	return total
}

// CalculateDay runs the full rule-priority chain for one employee-day.
func CalculateDay(events []RawEvent, ctx DayContext) DailyResult {
	result := DailyResult{CalculationVersion: EngineVersion}
	policy := resolvePolicy(ctx.Policy)

	// 1. Employee Status
	if !ctx.EmployeeActiveToday {
		result.Status = StatusInactive
		return result
	}

	// 1b. Sandwich Leave — a Holiday/Weekly Off that Sandwich Leave has turned
	// into a leave day takes priority over both, unlike an ordinary leave
	// request (rule 4) that merely happens to include one.
	if ctx.IsSandwichedLeave {
		result.Status = StatusOnLeave
		return result
	}

	// 2. Holiday — still credit any work actually done (compoff), but the
	// day's status is Holiday regardless. A declared Half Day on the holiday
	// (the same out_reason="HALF_DAY" signal used every other day) halves
	// the credit to 0.5 instead of a full 1 — a declared choice at
	// scan-out, same convention as the rest of the app, never inferred from
	// hours worked.
	if ctx.IsHoliday {
		sessions, _ := pairEvents(events)
		worked := sumSessionMinutes(sessions)
		if worked > 0 {
			result.WorkingMinutes = worked
			result.CompOffEarned = true
			for _, e := range events {
				if e.OutReason == "HALF_DAY" {
					result.CompOffHalfDay = true
					break
				}
			}
		}
		result.Status = StatusHoliday
		return result
	}

	// 3. Weekly Off
	if ctx.IsWeeklyOff {
		sessions, _ := pairEvents(events)
		result.WorkingMinutes = sumSessionMinutes(sessions)
		result.Status = StatusWeeklyOff
		return result
	}

	// 4. Approved Leave
	if ctx.HasApprovedLeave {
		result.Status = StatusOnLeave
		return result
	}

	// 5. Raw Events
	if len(events) == 0 {
		if ctx.IsExcusedAbsence {
			result.Status = StatusOnLeave
			result.AbsenceExcused = true
			return result
		}
		result.Status = StatusAbsent
		result.UnapprovedAbsence = true
		return result
	}

	// Low confidence flags a face match that cleared the accept threshold but
	// deserves a second look. A manual entry, a self-reported check-in, or a
	// system auto-close never went through the face pipeline at all, so its
	// zero confidence means "not applicable", not "poor match" — flagging it
	// would bury real low-confidence scans in noise.
	for _, e := range events {
		if e.Source != SourceManual && e.Source != SourceSelf && e.Source != SourceAutoClosed && e.Confidence < LowConfidenceThreshold {
			result.Exceptions = append(result.Exceptions, ExLowConfidence)
			break
		}
	}

	// 6. Event Pairing
	sessions, pairingExceptions := pairEvents(events)
	result.Exceptions = append(result.Exceptions, pairingExceptions...)

	first := events[0].EventTime
	result.FirstIn = &first
	// last is the raw last event's time, used below for an informational
	// early-leave estimate regardless of its direction (see 9. Early Leave).
	// result.LastOut is a separate, stricter concept: genuinely the last OUT
	// scan, not just the last event — if the most recent scan is actually an
	// IN (back from lunch, or back at work with no final checkout yet today),
	// there is no "last out" to report yet, and showing that IN time as
	// though it were an out would be exactly backwards.
	last := events[len(events)-1].EventTime
	if events[len(events)-1].Direction == "OUT" {
		result.LastOut = &last
	}

	// 7b. Absent Cutoff — first scan in at or after 14:36 voids the day
	// entirely, overriding everything else the events would otherwise mean
	// (late arrival, half day, overtime). Checked before any shift-relative
	// math since it applies the same regardless of shift. Gated on the first
	// event actually being an IN — a stray OUT with no matching scan-in is a
	// data-quality issue, not a late arrival, and is handled below instead.
	if events[0].Direction == "IN" {
		absentCutoff := time.Date(first.Year(), first.Month(), first.Day(), policy.AbsentCutoffHour, policy.AbsentCutoffMinute, 0, 0, first.Location())
		if !first.Before(absentCutoff) {
			if ctx.IsExcusedAbsence {
				result.Status = StatusOnLeave
				result.AbsenceExcused = true
				return result
			}
			result.Status = StatusAbsent
			result.UnapprovedAbsence = true
			return result
		}
	}

	shift := ctx.Shift
	isLateArrival := false
	pastHalfDayCutoff := false
	var graceDeadline time.Time

	// 8. Late Arrival — computed before the pairing is judged usable, because
	// it depends only on the first scan. A late arrival is still late even if
	// the person forgot to scan out.
	//
	// Late and half day are separate outcomes, never both. Arriving in the
	// window between the grace deadline and the cutoff is late; arriving at or
	// after the cutoff is a half day and is not additionally marked late.
	//
	// Both boundaries are inclusive: the grace period covers the first
	// GracePeriodMinutes from the shift's own start, so on a 09:00 shift with
	// 15 minutes' grace you are late at 09:15:00 exactly. The half-day cutoff
	// itself doesn't move with the shift, though: 10:31:00 is a half day
	// whether you're on the 9:00 shift or the 10:00 one.
	if shift != nil {
		shiftStart := onSameDayAt(first, shift.StartTime)
		graceDeadline = shiftStart.Add(time.Duration(shift.GracePeriodMinutes) * time.Minute)
		halfDayCutoff := time.Date(first.Year(), first.Month(), first.Day(), policy.HalfDayCutoffHour, policy.HalfDayCutoffMinute, 0, 0, first.Location())
		// A guard for a shift starting late enough that its own grace period
		// wouldn't even end until after the fixed cutoff (not true for the
		// 9:00/10:00 shifts today, but a later shift added down the line
		// shouldn't be able to mark someone half-day before their grace period
		// is even over).
		if halfDayCutoff.Before(graceDeadline) {
			halfDayCutoff = graceDeadline
		}

		switch {
		case !first.Before(halfDayCutoff):
			pastHalfDayCutoff = true
		case !first.Before(graceDeadline):
			isLateArrival = true
			result.LateMinutes = int(first.Sub(shiftStart).Minutes())
		}

		// A "Late Coming" reason recorded on the arrival scan — only possible
		// today via a manual correction (see attendance.RecordManual) — forces
		// late-arrival classification even if the recorded time itself reads
		// as on-time: HR is asserting something the clock alone doesn't show.
		// Never downgrades an already-worse outcome.
		if !pastHalfDayCutoff && !isLateArrival && events[0].OutReason == "LATE_COMING" {
			isLateArrival = true
			result.LateMinutes = int(first.Sub(shiftStart).Minutes())
			if result.LateMinutes < 0 {
				result.LateMinutes = 0
			}
		}
	}

	// Events exist but none pair into a complete session — almost always a
	// scan-in with no scan-out. The person was demonstrably here (a face
	// matched to create the event), so this is a record that needs correcting,
	// not an absence. The spec is explicit that MISSING_OUT should be flagged
	// "and allow HR/admin to resolve it"; treating it as an unapproved absence
	// would silently dock two days of leave from someone who forgot to tap out,
	// and would show a present employee as ABSENT on their own dashboard.
	// Working minutes stay 0 until the missing event is corrected, but the
	// arrival time is known, so lateness still counts.
	if len(sessions) == 0 {
		result.Status = StatusPresent
		if pastHalfDayCutoff {
			// This rule keys off arrival time alone, so it applies regardless
			// of whether the day's hours are known.
			result.Status = StatusHalfDay
			result.Exceptions = append(result.Exceptions, ExLateArrivalHalfDay)
		}
		return result
	}

	// 7. Working Hours
	totalMinutes := sumSessionMinutes(sessions)
	result.WorkingMinutes = totalMinutes

	// Was yesterday's stay-late pass earned, and is it usable today?
	hasLatePass := false
	if shift != nil && ctx.PreviousWorkDayLastOut != nil {
		prevShiftEnd := onSameDayAt(*ctx.PreviousWorkDayLastOut, shift.EndTime)
		passDeadline := prevShiftEnd.Add(time.Duration(policy.StayLatePassThresholdMinutes) * time.Minute)
		if ctx.PreviousWorkDayLastOut.After(passDeadline) {
			hasLatePass = true
		}
	}

	// 9. Early Leave — purely informational now: how many minutes before the
	// grace-adjusted shift end the last scan was, whatever its direction.
	// EndOfShiftGraceMinutes before shift end still counts as leaving on
	// time — the same cutoff IsBeforeShiftEnd uses to decide whether the
	// kiosk even asks for a reason — so a departure inside that window never
	// shows as an early leave at all, not even informationally. There's no
	// automatic escalation to Half Day from the clock alone anymore — every
	// OUT before that cutoff goes through the kiosk's reason popup (or a
	// manual correction's own reason selector), so an early departure that's
	// meant to be a Half Day is always a declared one (see 10b below), never
	// inferred from a time window.
	if shift != nil {
		shiftEnd := onSameDayAt(last, shift.EndTime)
		graceAdjustedEnd := shiftEnd.Add(-time.Duration(policy.EndOfShiftGraceMinutes) * time.Minute)
		if last.Before(graceAdjustedEnd) {
			result.EarlyLeaveMinutes = int(graceAdjustedEnd.Sub(last).Minutes())
		}
	}

	// 9b. Lunch OT — a LUNCH-tagged break (never Personal or any other reason)
	// that ran longer than policy.LunchTimeLimitMinutes. Purely informational,
	// same as Early Leave/Overtime: it never changes Status. Reuses PunchTimes
	// rather than re-walking events, so this always agrees with the lunch
	// duration already shown on the Daily Timesheet and the employee's own
	// attendance page.
	_, _, _, _, lunchMinutes := PunchTimes(events)
	if lunchMinutes > policy.LunchTimeLimitMinutes {
		result.LunchOTMinutes = lunchMinutes - policy.LunchTimeLimitMinutes
		result.Exceptions = append(result.Exceptions, ExLunchOT)

		// 9c. Lunch OT Streak — every LunchOTStreakLimit-th Lunch OT day this
		// month is a strike, same every-Nth-one convention as the Late Streak
		// above (the 3rd is a strike, the 4th and 5th aren't, the 6th is
		// again). Unlike Late Streak, a strike here never touches Status —
		// it only flags LunchOTStreakPenalty so repository.go can sync a
		// fixed policy.LunchOTPenaltyDays leave-balance deduction.
		result.Exceptions = append(result.Exceptions, ExLunchOTStreakCounted)
		if (ctx.LunchOTStreakSoFarThisMonth+1)%policy.LunchOTStreakLimit == 0 {
			result.LunchOTStreakPenalty = true
			result.Exceptions = append(result.Exceptions, ExLunchOTStreakPenalty)
		}
	}

	// 10. Half Day
	switch {
	case pastHalfDayCutoff:
		result.Status = StatusHalfDay
		result.Exceptions = append(result.Exceptions, ExLateArrivalHalfDay)
	case isLateArrival && hasLatePass:
		// Excused: doesn't count toward the monthly streak, doesn't force
		// half day.
		result.Exceptions = append(result.Exceptions, ExLatePassUsed)
		result.Status = StatusPresent
	case isLateArrival:
		result.Exceptions = append(result.Exceptions, ExLateStreakCounted)
		// Every LateStreakLimit-th late arrival this month is the strike, not
		// every one from the limit onward — the 3rd is a half day, the 4th and
		// 5th are ordinary lates, the 6th is a half day again, and so on.
		if (ctx.LateStreakSoFarThisMonth+1)%policy.LateStreakLimit == 0 {
			result.Status = StatusHalfDay
			result.Exceptions = append(result.Exceptions, ExLateStreakHalfDay)
		} else {
			result.Status = StatusPresent
		}
	default:
		result.Status = StatusPresent
	}

	// 10b. Declared Half Day — either the employee said so themselves when
	// marking out, or HR flagged it while manually correcting an IN/OUT — so
	// it holds regardless of hours worked, overriding whatever the
	// hours-based rules above decided.
	for _, e := range events {
		if e.OutReason == "HALF_DAY" {
			result.Status = StatusHalfDay
			result.Exceptions = append(result.Exceptions, ExDeclaredHalfDay)
			break
		}
	}

	// 11. Overtime — not a policy every shift has (see overtimeEligibleShifts);
	// the internship shift simply doesn't earn it. Measured the same way
	// EmployeeRangeResult.OvertimeDays counts a day as overtime: minutes past
	// shift end plus the stay-late threshold (StayLatePassThresholdMinutes,
	// default 30) — not against the shift's minimum working hours at all.
	// Someone marking out at 7:40 PM on a shift ending 7:00 PM, with the
	// default 30-minute threshold, earns 10 overtime minutes (7:40 minus
	// 7:30), not 40. Requires a genuine final OUT — a trailing unpaired IN
	// has no real checkout time to measure lateness from.
	if shift != nil && overtimeEligibleShifts[shift.Name] && result.LastOut != nil {
		shiftEnd := onSameDayAt(*result.LastOut, shift.EndTime)
		cutoff := shiftEnd.Add(time.Duration(policy.StayLatePassThresholdMinutes) * time.Minute)
		if result.LastOut.After(cutoff) {
			result.OvertimeMinutes = int(result.LastOut.Sub(cutoff).Minutes())
		}
	}

	// 12. Absence is handled above wherever no working session exists.

	return result
}
