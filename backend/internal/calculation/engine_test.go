package calculation

import (
	"testing"
	"time"
)

func mustTime(hhmm string) time.Time {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		panic(err)
	}
	return t
}

func ev(direction, hhmm string, confidence float64) RawEvent {
	return evOnDay(15, direction, hhmm, confidence)
}

func evOnDay(day int, direction, hhmm string, confidence float64) RawEvent {
	clock := mustTime(hhmm)
	return RawEvent{
		Direction:  direction,
		EventTime:  time.Date(2026, 9, day, clock.Hour(), clock.Minute(), 0, 0, time.UTC),
		Confidence: confidence,
	}
}

// withReason returns e with OutReason set — a small builder so a test can
// attach a manually-declared reason (e.g. "LATE_COMING" on an IN) without a
// separate literal RawEvent construction.
func withReason(e RawEvent, reason string) RawEvent {
	e.OutReason = reason
	return e
}

// Shift span (09:00-17:00) — this system has no separate lunch-break fields,
// so a shift's full span working straight through is the zero-overtime
// baseline.
func standardShift() *ShiftInfo {
	return &ShiftInfo{
		Name:               "General Shift",
		StartTime:          mustTime("09:00"),
		EndTime:            mustTime("17:00"),
		GracePeriodMinutes: 15,
	}
}

func baseCtx() DayContext {
	return DayContext{Shift: standardShift(), EmployeeActiveToday: true}
}

func hasException(exceptions []ExceptionCode, code ExceptionCode) bool {
	for _, e := range exceptions {
		if e == code {
			return true
		}
	}
	return false
}

func TestNormalDay(t *testing.T) {
	events := []RawEvent{ev("IN", "09:00", 0.9), ev("OUT", "17:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.Status != StatusPresent {
		t.Errorf("status = %s, want PRESENT", r.Status)
	}
	if r.WorkingMinutes != 8*60 {
		t.Errorf("working minutes = %d, want %d", r.WorkingMinutes, 8*60)
	}
	if r.LateMinutes != 0 || r.EarlyLeaveMinutes != 0 || r.OvertimeMinutes != 0 {
		t.Errorf("expected no late/early/overtime, got late=%d early=%d overtime=%d", r.LateMinutes, r.EarlyLeaveMinutes, r.OvertimeMinutes)
	}
	if len(r.Exceptions) != 0 {
		t.Errorf("expected no exceptions, got %v", r.Exceptions)
	}
}

func TestWithinGracePeriodIsNotLate(t *testing.T) {
	events := []RawEvent{ev("IN", "09:10", 0.9), ev("OUT", "17:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.LateMinutes != 0 {
		t.Errorf("late minutes = %d, want 0 (within 15min grace)", r.LateMinutes)
	}
}

func TestLateArrivalFirstTimeThisMonthIsJustLate(t *testing.T) {
	// First late day this month (streak count = 0 before today): late, but
	// not yet a half day.
	events := []RawEvent{ev("IN", "09:30", 0.9), ev("OUT", "17:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.LateMinutes != 30 {
		t.Errorf("late minutes = %d, want 30", r.LateMinutes)
	}
	if r.Status != StatusPresent {
		t.Errorf("status = %s, want PRESENT (1st late day shouldn't be half day)", r.Status)
	}
	if !hasException(r.Exceptions, ExLateStreakCounted) {
		t.Errorf("expected LATE_STREAK_COUNTED, got %v", r.Exceptions)
	}
}

func TestThirdLateArrivalThisMonthIsHalfDay(t *testing.T) {
	ctx := baseCtx()
	ctx.LateStreakSoFarThisMonth = 2 // this is the 3rd late day
	events := []RawEvent{ev("IN", "09:30", 0.9), ev("OUT", "17:00", 0.9)}
	r := CalculateDay(events, ctx)

	if r.Status != StatusHalfDay {
		t.Errorf("status = %s, want HALF_DAY (3rd late day this month)", r.Status)
	}
	if !hasException(r.Exceptions, ExLateStreakHalfDay) {
		t.Errorf("expected LATE_STREAK_HALF_DAY, got %v", r.Exceptions)
	}
}

// The strike fires every 3rd late arrival, not the 3rd and every one after
// it — the 4th and 5th are ordinary lates, the 6th is a half day again.
func TestLateStreakStrikeRepeatsEveryThirdLate(t *testing.T) {
	events := []RawEvent{ev("IN", "09:30", 0.9), ev("OUT", "17:00", 0.9)}

	cases := []struct {
		streakSoFar int
		wantHalfDay bool
	}{
		{streakSoFar: 2, wantHalfDay: true},  // 3rd late
		{streakSoFar: 3, wantHalfDay: false}, // 4th late
		{streakSoFar: 4, wantHalfDay: false}, // 5th late
		{streakSoFar: 5, wantHalfDay: true},  // 6th late
	}

	for _, c := range cases {
		ctx := baseCtx()
		ctx.LateStreakSoFarThisMonth = c.streakSoFar
		r := CalculateDay(events, ctx)

		gotHalfDay := r.Status == StatusHalfDay
		if gotHalfDay != c.wantHalfDay {
			t.Errorf("streak so far = %d: status = %s, want half day = %v", c.streakSoFar, r.Status, c.wantHalfDay)
		}
		if hasException(r.Exceptions, ExLateStreakHalfDay) != c.wantHalfDay {
			t.Errorf("streak so far = %d: LATE_STREAK_HALF_DAY present = %v, want %v",
				c.streakSoFar, hasException(r.Exceptions, ExLateStreakHalfDay), c.wantHalfDay)
		}
	}
}

func TestArrivalPastHalfDayCutoffIsAlwaysHalfDay(t *testing.T) {
	// Company rule: arriving at or after 10:31 is an automatic half day.
	events := []RawEvent{ev("IN", "10:31", 0.9), ev("OUT", "17:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.Status != StatusHalfDay {
		t.Errorf("status = %s, want HALF_DAY (past 10:30 cutoff)", r.Status)
	}
	if !hasException(r.Exceptions, ExLateArrivalHalfDay) {
		t.Errorf("expected LATE_ARRIVAL_HALF_DAY, got %v", r.Exceptions)
	}
}

func TestFirstInAtOrAfter236PMIsAbsent(t *testing.T) {
	// Even a full day's work afterward doesn't rescue it — the cutoff is
	// unconditional once tripped.
	events := []RawEvent{ev("IN", "14:36", 0.9), ev("OUT", "20:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.Status != StatusAbsent {
		t.Errorf("status = %s, want ABSENT (first IN at/after 14:36)", r.Status)
	}
	if !r.UnapprovedAbsence {
		t.Error("expected UnapprovedAbsence = true so the leave-balance penalty applies")
	}
}

func TestWithinFiveMinuteGraceOfAbsentCutoffIsNotAbsent(t *testing.T) {
	// The cutoff is nominally 2:30 PM with a 5-minute grace — 14:35 is the
	// last minute that's still fine, 14:30 (well within it) obviously is too.
	for _, arrival := range []string{"14:30", "14:35"} {
		events := []RawEvent{ev("IN", arrival, 0.9), ev("OUT", "19:00", 0.9)}
		r := CalculateDay(events, baseCtx())
		if r.Status == StatusAbsent {
			t.Errorf("arrival %s is within the 5-minute grace — should not be ABSENT", arrival)
		}
	}
}

func TestLatePassExcusesLateArrival(t *testing.T) {
	// Stayed until 17:35 yesterday (past 17:00+30min=17:30 threshold) →
	// earns a pass usable today. Arriving 09:45 (late, but before the 10:30
	// hard cutoff) should be excused and NOT counted toward the streak.
	yesterday1735 := time.Date(2026, 9, 14, 17, 35, 0, 0, time.UTC)
	ctx := baseCtx()
	ctx.PreviousWorkDayLastOut = &yesterday1735
	ctx.LateStreakSoFarThisMonth = 2 // would be the 3rd late day if NOT excused

	events := []RawEvent{ev("IN", "09:45", 0.9), ev("OUT", "17:00", 0.9)}
	r := CalculateDay(events, ctx)

	if !hasException(r.Exceptions, ExLatePassUsed) {
		t.Errorf("expected LATE_PASS_USED, got %v", r.Exceptions)
	}
	if hasException(r.Exceptions, ExLateStreakCounted) || hasException(r.Exceptions, ExLateStreakHalfDay) {
		t.Errorf("excused lateness must not count toward the streak, got %v", r.Exceptions)
	}
	if r.Status != StatusPresent {
		t.Errorf("status = %s, want PRESENT (excused via pass)", r.Status)
	}
}

func TestLatePassDoesNotExcusePastHardCutoff(t *testing.T) {
	// Even with a valid pass, arriving past 10:30 is still half day.
	yesterday1735 := time.Date(2026, 9, 14, 17, 35, 0, 0, time.UTC)
	ctx := baseCtx()
	ctx.PreviousWorkDayLastOut = &yesterday1735

	events := []RawEvent{ev("IN", "10:45", 0.9), ev("OUT", "17:00", 0.9)}
	r := CalculateDay(events, ctx)

	if r.Status != StatusHalfDay {
		t.Errorf("status = %s, want HALF_DAY (pass doesn't cover past 10:30)", r.Status)
	}
}

func TestStayingExactlyToPassThresholdDoesNotEarnPass(t *testing.T) {
	// Only 17:30 exactly (not after) — should NOT earn a pass.
	yesterday1730 := time.Date(2026, 9, 14, 17, 30, 0, 0, time.UTC)
	ctx := baseCtx()
	ctx.PreviousWorkDayLastOut = &yesterday1730
	ctx.LateStreakSoFarThisMonth = 2

	events := []RawEvent{ev("IN", "09:45", 0.9), ev("OUT", "17:00", 0.9)}
	r := CalculateDay(events, ctx)

	if hasException(r.Exceptions, ExLatePassUsed) {
		t.Error("staying exactly to the threshold (not past it) should not earn a pass")
	}
	if r.Status != StatusHalfDay {
		t.Errorf("status = %s, want HALF_DAY (3rd late day, no pass earned)", r.Status)
	}
}

func TestEarlyLeave(t *testing.T) {
	events := []RawEvent{ev("IN", "09:00", 0.9), ev("OUT", "16:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	// 60 minutes before shift end, minus the default 10-minute end-of-shift
	// grace period that now also applies to the informational figure, not
	// just whether a reason was required.
	if r.EarlyLeaveMinutes != 50 {
		t.Errorf("early leave minutes = %d, want 50", r.EarlyLeaveMinutes)
	}
}

func TestAbsentWithNoEventsIsUnapproved(t *testing.T) {
	r := CalculateDay(nil, baseCtx())

	if r.Status != StatusAbsent {
		t.Errorf("status = %s, want ABSENT", r.Status)
	}
	if !r.UnapprovedAbsence {
		t.Error("expected UnapprovedAbsence = true when nothing excuses the absence")
	}
}

// A Super Admin's manual override (see absence_overrides) turns an otherwise
// unapproved absence into an ordinary On Leave day — deliberately the same
// status Approved Leave produces, so the leave-balance penalty, Reports'
// "Uninformed Leave" count, and attendance-percentage math all treat it as
// excused too, not just this one check.
func TestExcusedAbsenceWithNoEventsBecomesOnLeave(t *testing.T) {
	ctx := baseCtx()
	ctx.IsExcusedAbsence = true
	r := CalculateDay(nil, ctx)

	if r.Status != StatusOnLeave {
		t.Errorf("status = %s, want ON_LEAVE (excused absence)", r.Status)
	}
	if r.UnapprovedAbsence {
		t.Error("did not expect UnapprovedAbsence once excused — the leave-balance penalty must not apply")
	}
}

func TestExcusedAbsenceAtAbsentCutoffBecomesOnLeave(t *testing.T) {
	ctx := baseCtx()
	ctx.IsExcusedAbsence = true
	events := []RawEvent{ev("IN", "14:36", 0.9), ev("OUT", "20:00", 0.9)}
	r := CalculateDay(events, ctx)

	if r.Status != StatusOnLeave {
		t.Errorf("status = %s, want ON_LEAVE (excused absence, even with a too-late first scan)", r.Status)
	}
	if r.UnapprovedAbsence {
		t.Error("did not expect UnapprovedAbsence once excused")
	}
}

// The override only ever matters on a day that would otherwise be Absent —
// it has no effect on a day the employee actually attended normally.
func TestExcusedAbsenceHasNoEffectOnAnOrdinaryPresentDay(t *testing.T) {
	ctx := baseCtx()
	ctx.IsExcusedAbsence = true
	events := []RawEvent{ev("IN", "09:00", 0.9), ev("OUT", "17:00", 0.9)}
	r := CalculateDay(events, ctx)

	if r.Status != StatusPresent {
		t.Errorf("status = %s, want PRESENT — the override shouldn't change an ordinary attended day", r.Status)
	}
}

func TestMissingInInvalidSequence(t *testing.T) {
	events := []RawEvent{ev("OUT", "13:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if !hasException(r.Exceptions, ExInvalidSequence) {
		t.Errorf("expected INVALID_SEQUENCE exception, got %v", r.Exceptions)
	}
}

func TestMissingOut(t *testing.T) {
	events := []RawEvent{ev("IN", "09:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if !hasException(r.Exceptions, ExMissingOut) {
		t.Errorf("expected MISSING_OUT exception, got %v", r.Exceptions)
	}
	if r.WorkingMinutes != 0 {
		t.Errorf("working minutes = %d, want 0 (no completed session)", r.WorkingMinutes)
	}
}

// Someone who scans in and forgets to scan out must not be recorded as absent
// or penalised — they were demonstrably at work. The day is flagged for HR to
// correct instead.
func TestMissingOutIsNotAnAbsence(t *testing.T) {
	events := []RawEvent{ev("IN", "09:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.Status == StatusAbsent {
		t.Error("a scan-in with no scan-out must not be recorded as ABSENT")
	}
	if r.UnapprovedAbsence {
		t.Error("a scan-in with no scan-out must not trigger the leave penalty")
	}
	if !hasException(r.Exceptions, ExMissingOut) {
		t.Errorf("expected MISSING_OUT so HR can resolve it, got %v", r.Exceptions)
	}
	if r.LastOut != nil {
		t.Errorf("LastOut = %v, want nil — the only scan is an IN, there is no out yet", r.LastOut)
	}
}

// Real case: went out for lunch, then scanned back IN and is still at work —
// the most recent event is an IN, so there is no "last out" for today yet.
// LastOut must not report that return-from-lunch IN as though it were a
// checkout.
func TestLastOutIsNilWhenMostRecentScanIsAnIn(t *testing.T) {
	lunchOut := ev("OUT", "13:48", 0.9)
	lunchOut.OutReason = "LUNCH"
	events := []RawEvent{ev("IN", "10:06", 0.9), lunchOut, ev("IN", "14:25", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.LastOut != nil {
		t.Errorf("LastOut = %v, want nil — back from lunch and still at work, no checkout recorded yet", r.LastOut)
	}
	if r.FirstIn == nil || r.FirstIn.Format("15:04") != "10:06" {
		t.Errorf("FirstIn = %v, want 10:06", r.FirstIn)
	}
}

// An OUT with no preceding IN is equally a broken pair, not an absence.
func TestStrayOutIsNotAnAbsence(t *testing.T) {
	events := []RawEvent{ev("OUT", "18:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.UnapprovedAbsence {
		t.Error("a stray OUT must not trigger the leave penalty")
	}
	if r.Status == StatusAbsent {
		t.Error("a stray OUT means the person was here; not ABSENT")
	}
}

// Forgetting to scan out must not also erase the fact that you turned up late.
func TestMissingOutStillCountsLateArrival(t *testing.T) {
	// baseCtx shift starts 09:00 with a 10-minute grace, so 09:40 is late.
	events := []RawEvent{ev("IN", "09:40", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.LateMinutes != 40 {
		t.Errorf("late minutes = %d, want 40 even with no scan-out", r.LateMinutes)
	}
	if !hasException(r.Exceptions, ExMissingOut) {
		t.Errorf("expected MISSING_OUT, got %v", r.Exceptions)
	}
}

// The hard half-day cutoff keys off arrival time alone, so it still applies
// when the day's hours are unknown.
func TestMissingOutPastHalfDayCutoffIsHalfDay(t *testing.T) {
	events := []RawEvent{ev("IN", "11:30", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.Status != StatusHalfDay {
		t.Errorf("status = %s, want HALF_DAY for an arrival past the cutoff", r.Status)
	}
	if r.UnapprovedAbsence {
		t.Error("still must not trigger the leave penalty")
	}
}

// The grace period covers its full span, so the first late minute is the one
// the deadline lands on: 09:14 is fine, 09:15 is late.
func TestGraceBoundaryIsInclusive(t *testing.T) {
	onTime := CalculateDay([]RawEvent{ev("IN", "09:14", 0.9), ev("OUT", "17:00", 0.9)}, baseCtx())
	if onTime.LateMinutes != 0 {
		t.Errorf("09:14 late minutes = %d, want 0 — still inside the 15-minute grace", onTime.LateMinutes)
	}

	late := CalculateDay([]RawEvent{ev("IN", "09:15", 0.9), ev("OUT", "17:00", 0.9)}, baseCtx())
	if late.LateMinutes == 0 {
		t.Error("09:15 should be late — the grace period has run out")
	}
}

// 10:30 is still a late-but-full day; 10:31 is the automatic half day.
func TestHalfDayCutoffBoundaryIsInclusive(t *testing.T) {
	justInside := CalculateDay([]RawEvent{ev("IN", "10:30", 0.9), ev("OUT", "19:00", 0.9)}, baseCtx())
	if hasException(justInside.Exceptions, ExLateArrivalHalfDay) {
		t.Errorf("10:30 must not trip the cutoff, got %v", justInside.Exceptions)
	}

	past := CalculateDay([]RawEvent{ev("IN", "10:31", 0.9), ev("OUT", "19:00", 0.9)}, baseCtx())
	if past.Status != StatusHalfDay {
		t.Errorf("10:31 status = %s, want HALF_DAY", past.Status)
	}
}

// The half-day cutoff is a fixed company-wide clock time, not measured from
// each shift's own start — an employee on the 10:00 AM shift trips it at the
// same 10:31 as one on the 9:00 AM shift, not at 11:31.
func TestHalfDayCutoffIsFixedAcrossShifts(t *testing.T) {
	tenAMShift := &ShiftInfo{StartTime: mustTime("10:00"), EndTime: mustTime("19:00"), GracePeriodMinutes: 15}
	ctx := DayContext{Shift: tenAMShift, EmployeeActiveToday: true}

	onTime := CalculateDay([]RawEvent{ev("IN", "10:20", 0.9), ev("OUT", "19:00", 0.9)}, ctx)
	if onTime.Status == StatusHalfDay {
		t.Error("10:20 is within this shift's own grace period — should not be HALF_DAY")
	}

	past := CalculateDay([]RawEvent{ev("IN", "10:35", 0.9), ev("OUT", "19:00", 0.9)}, ctx)
	if past.Status != StatusHalfDay {
		t.Errorf("10:35 status = %s, want HALF_DAY (past the fixed 10:31 cutoff, even on a 10 AM shift)", past.Status)
	}
}

// Guard: a hypothetical shift starting late enough that its own grace period
// wouldn't end until after the fixed cutoff must not mark someone half-day
// before their grace period is even over.
func TestHalfDayCutoffNeverPrecedesOwnGracePeriod(t *testing.T) {
	lateShift := &ShiftInfo{StartTime: mustTime("10:30"), EndTime: mustTime("19:30"), GracePeriodMinutes: 15}
	ctx := DayContext{Shift: lateShift, EmployeeActiveToday: true}

	// Grace deadline is 10:45, after the nominal fixed cutoff of 10:31.
	withinOwnGrace := CalculateDay([]RawEvent{ev("IN", "10:40", 0.9), ev("OUT", "19:30", 0.9)}, ctx)
	if withinOwnGrace.Status == StatusHalfDay {
		t.Error("10:40 is within this shift's own grace period (ends 10:45) — should not be HALF_DAY")
	}
}

// A pass excuses the arrival from the monthly streak but does not erase the
// lateness itself — the day still reports late minutes.
func TestLatePassStillRecordsLateMinutes(t *testing.T) {
	ctx := baseCtx()
	prev := evOnDay(14, "OUT", "17:45", 0.9).EventTime
	ctx.PreviousWorkDayLastOut = &prev

	r := CalculateDay([]RawEvent{ev("IN", "09:45", 0.9), ev("OUT", "17:00", 0.9)}, ctx)

	if !hasException(r.Exceptions, ExLatePassUsed) {
		t.Fatalf("expected LATE_PASS_USED, got %v", r.Exceptions)
	}
	if r.LateMinutes == 0 {
		t.Error("an excused arrival is still recorded as late")
	}
	if hasException(r.Exceptions, ExLateStreakCounted) {
		t.Error("an excused arrival must not count toward the 3-strike total")
	}
}

// A scan the admin entered by hand carries no face-match confidence. That must
// not be read as a poor match, or every correction would raise a false flag and
// bury the genuine low-confidence scans.
func TestManualEntryIsNotLowConfidence(t *testing.T) {
	in := ev("IN", "09:00", 0.9)
	out := ev("OUT", "17:00", 0)
	out.Source = SourceManual

	r := CalculateDay([]RawEvent{in, out}, baseCtx())

	if hasException(r.Exceptions, ExLowConfidence) {
		t.Errorf("a manual entry must not raise LOW_CONFIDENCE, got %v", r.Exceptions)
	}
	if r.Status != StatusPresent {
		t.Errorf("status = %s, want PRESENT once the missing scan is filled in", r.Status)
	}
	if r.WorkingMinutes != 8*60 {
		t.Errorf("working minutes = %d, want 480", r.WorkingMinutes)
	}
}

// A self-reported check-in (no camera involved at all) carries confidence 0
// by construction, same as a manual entry — it must not be flagged either.
func TestSelfCheckinIsNotLowConfidence(t *testing.T) {
	in := ev("IN", "09:00", 0)
	in.Source = SourceSelf
	out := ev("OUT", "17:00", 0)
	out.Source = SourceSelf

	r := CalculateDay([]RawEvent{in, out}, baseCtx())

	if hasException(r.Exceptions, ExLowConfidence) {
		t.Errorf("a self-reported check-in must not raise LOW_CONFIDENCE, got %v", r.Exceptions)
	}
}

// A genuine face scan below the threshold still gets flagged.
func TestLowConfidenceFaceScanStillFlagged(t *testing.T) {
	r := CalculateDay([]RawEvent{ev("IN", "09:00", 0.4), ev("OUT", "17:00", 0.9)}, baseCtx())
	if !hasException(r.Exceptions, ExLowConfidence) {
		t.Errorf("expected LOW_CONFIDENCE for a weak face match, got %v", r.Exceptions)
	}
}

func TestDuplicateIn(t *testing.T) {
	events := []RawEvent{ev("IN", "09:00", 0.9), ev("IN", "09:05", 0.9), ev("OUT", "17:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if !hasException(r.Exceptions, ExDuplicateIn) {
		t.Errorf("expected DUPLICATE_IN exception, got %v", r.Exceptions)
	}
	if r.WorkingMinutes != 8*60 {
		t.Errorf("working minutes = %d, want %d despite duplicate IN", r.WorkingMinutes, 8*60)
	}
}

func TestDuplicateOut(t *testing.T) {
	events := []RawEvent{ev("IN", "09:00", 0.9), ev("OUT", "17:00", 0.9), ev("OUT", "17:05", 0.9)}
	r := CalculateDay(events, baseCtx())

	if !hasException(r.Exceptions, ExDuplicateOut) {
		t.Errorf("expected DUPLICATE_OUT exception, got %v", r.Exceptions)
	}
}

func TestMultipleSessions(t *testing.T) {
	events := []RawEvent{
		ev("IN", "09:00", 0.9), ev("OUT", "13:00", 0.9),
		ev("IN", "14:00", 0.9), ev("OUT", "18:00", 0.9),
	}
	r := CalculateDay(events, baseCtx())

	if r.WorkingMinutes != 8*60 {
		t.Errorf("working minutes = %d, want %d across two sessions", r.WorkingMinutes, 8*60)
	}
}

// Default LunchTimeLimitMinutes is 60 (see DefaultPolicySettings) — a lunch
// break longer than that flags LunchOTMinutes/ExLunchOT, purely
// informational like Early Leave/Overtime: it never changes Status.
func TestLunchOTOverLimit(t *testing.T) {
	lunchOut := ev("OUT", "13:00", 0.9)
	lunchOut.OutReason = "LUNCH"
	events := []RawEvent{ev("IN", "09:00", 0.9), lunchOut, ev("IN", "14:15", 0.9), ev("OUT", "19:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.LunchOTMinutes != 15 {
		t.Errorf("lunch OT minutes = %d, want 15 (75 minute lunch, 60 minute limit)", r.LunchOTMinutes)
	}
	if !hasException(r.Exceptions, ExLunchOT) {
		t.Error("expected LUNCH_OT exception for a 75 minute lunch break")
	}
	if r.Status != StatusPresent {
		t.Errorf("status = %s, want PRESENT (lunch OT is informational, never forces a status)", r.Status)
	}
}

func TestLunchWithinLimitIsNotLunchOT(t *testing.T) {
	lunchOut := ev("OUT", "13:00", 0.9)
	lunchOut.OutReason = "LUNCH"
	events := []RawEvent{ev("IN", "09:00", 0.9), lunchOut, ev("IN", "13:45", 0.9), ev("OUT", "19:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.LunchOTMinutes != 0 {
		t.Errorf("lunch OT minutes = %d, want 0 (45 minute lunch, under the 60 minute limit)", r.LunchOTMinutes)
	}
	if hasException(r.Exceptions, ExLunchOT) {
		t.Error("did not expect LUNCH_OT for a 45 minute lunch break")
	}
}

// A Personal break, however long, is never Lunch OT — only a LUNCH-tagged
// break counts (see PunchTimes).
func TestPersonalBreakIsNeverLunchOT(t *testing.T) {
	personalOut := ev("OUT", "13:00", 0.9)
	personalOut.OutReason = "PERSONAL"
	events := []RawEvent{ev("IN", "09:00", 0.9), personalOut, ev("IN", "14:30", 0.9), ev("OUT", "19:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if hasException(r.Exceptions, ExLunchOT) {
		t.Error("did not expect LUNCH_OT for a 90 minute PERSONAL break")
	}
}

func TestOvertime(t *testing.T) {
	events := []RawEvent{ev("IN", "09:00", 0.9), ev("OUT", "20:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	// Measured from shift end (17:00) plus the default 30-minute stay-late
	// threshold (17:30), not against the shift's minimum hours: 20:00 minus
	// 17:30 = 150 minutes.
	if r.OvertimeMinutes != 150 {
		t.Errorf("overtime minutes = %d, want 150", r.OvertimeMinutes)
	}
}

// The exact scenario the rule is meant to match: a shift ending 17:00, the
// default 30-minute threshold, marking out 10 minutes past that (17:40) —
// not 40 minutes past shift end itself.
func TestOvertimeMeasuredFromStayLateThresholdNotShiftEnd(t *testing.T) {
	events := []RawEvent{ev("IN", "09:00", 0.9), ev("OUT", "17:40", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.OvertimeMinutes != 10 {
		t.Errorf("overtime minutes = %d, want 10", r.OvertimeMinutes)
	}
}

// Staying exactly up to the threshold (17:30) is not yet overtime — only
// past it counts, matching OvertimeDays' own strict "after" comparison.
func TestOvertimeNoneAtExactThreshold(t *testing.T) {
	events := []RawEvent{ev("IN", "09:00", 0.9), ev("OUT", "17:30", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.OvertimeMinutes != 0 {
		t.Errorf("overtime minutes = %d, want 0 (exactly at the threshold, not past it)", r.OvertimeMinutes)
	}
}

// A trailing unpaired IN has no real checkout time — must not be read as
// overtime, same safety gate EarlyLeaveMinutes/OvertimeDays already apply.
func TestOvertimeNotEscalatedWithoutGenuineOut(t *testing.T) {
	events := []RawEvent{
		ev("IN", "09:00", 0.9),
		ev("OUT", "13:00", 0.9),
		ev("IN", "17:45", 0.9), // back at their desk, never scanned out again
	}
	r := CalculateDay(events, baseCtx())

	if r.OvertimeMinutes != 0 {
		t.Errorf("overtime minutes = %d, want 0 — no confirmed departure time", r.OvertimeMinutes)
	}
}

func TestDeclaredHalfDayOverridesFullHours(t *testing.T) {
	// Worked a full day's hours, but declared Half Day when marking out for
	// lunch — that declaration holds regardless of the hours actually worked.
	lunchOut := ev("OUT", "13:00", 0.9)
	lunchOut.OutReason = "HALF_DAY"
	events := []RawEvent{ev("IN", "09:00", 0.9), lunchOut, ev("IN", "13:30", 0.9), ev("OUT", "19:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.Status != StatusHalfDay {
		t.Errorf("status = %s, want HALF_DAY (declared at lunch)", r.Status)
	}
	if !hasException(r.Exceptions, ExDeclaredHalfDay) {
		t.Errorf("expected DECLARED_HALF_DAY, got %v", r.Exceptions)
	}
}

func TestBreakReasonDoesNotForceHalfDay(t *testing.T) {
	// A plain lunch break: multi-session pairing already excludes the gap
	// from worked minutes, so this should calculate exactly like any other
	// full day with a midday break — no forced half day.
	lunchOut := ev("OUT", "13:00", 0.9)
	lunchOut.OutReason = "LUNCH"
	events := []RawEvent{ev("IN", "09:00", 0.9), lunchOut, ev("IN", "13:30", 0.9), ev("OUT", "19:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if r.Status != StatusPresent {
		t.Errorf("status = %s, want PRESENT (lunch break, not a declared half day)", r.Status)
	}
	if hasException(r.Exceptions, ExDeclaredHalfDay) {
		t.Error("did not expect DECLARED_HALF_DAY for a LUNCH reason")
	}
}

// EARLY_LEAVE is a real final checkout for the day, but unlike HALF_DAY it
// never forces the status — leaving within the ordinary early-leave window
// (shift ends 17:00, so 16:00 is inside it) still ends up PRESENT, same as an
// unreasoned departure at that time already would.
func TestEarlyLeaveReasonDoesNotForceHalfDay(t *testing.T) {
	out := ev("OUT", "16:00", 0.9)
	out.OutReason = "EARLY_LEAVE"
	events := []RawEvent{ev("IN", "09:00", 0.9), out}
	r := CalculateDay(events, baseCtx())

	if r.Status != StatusPresent {
		t.Errorf("status = %s, want PRESENT (an ordinary early departure, not a declared half day)", r.Status)
	}
	if hasException(r.Exceptions, ExDeclaredHalfDay) {
		t.Error("did not expect DECLARED_HALF_DAY for an EARLY_LEAVE reason")
	}
	if r.EarlyLeaveMinutes == 0 {
		t.Error("expected EarlyLeaveMinutes > 0 for leaving an hour before shift end")
	}
}

// Even a very early EARLY_LEAVE departure (shift ends 17:00, this leaves at
// 12:00) doesn't force Half Day on its own — there's no hours-based escalation
// left in the engine; only a declared HALF_DAY reason (see 10b) or the
// arrival-time rules can produce that status.
func TestEarlyLeaveReasonNeverForcesHalfDayRegardlessOfHowEarly(t *testing.T) {
	out := ev("OUT", "12:00", 0.9)
	out.OutReason = "EARLY_LEAVE"
	events := []RawEvent{ev("IN", "09:00", 0.9), out}
	r := CalculateDay(events, baseCtx())

	if r.Status != StatusPresent {
		t.Errorf("status = %s, want PRESENT (EARLY_LEAVE never forces half day by itself)", r.Status)
	}
	if r.EarlyLeaveMinutes == 0 {
		t.Error("expected EarlyLeaveMinutes > 0 for leaving five hours before shift end")
	}
}

func TestLowConfidenceFlagged(t *testing.T) {
	events := []RawEvent{ev("IN", "09:00", 0.4), ev("OUT", "17:00", 0.9)}
	r := CalculateDay(events, baseCtx())

	if !hasException(r.Exceptions, ExLowConfidence) {
		t.Errorf("expected LOW_CONFIDENCE exception, got %v", r.Exceptions)
	}
	if r.Status != StatusPresent {
		t.Errorf("status = %s, want PRESENT despite low-confidence flag", r.Status)
	}
}

func TestInactiveEmployeeSkipsCalculation(t *testing.T) {
	ctx := baseCtx()
	ctx.EmployeeActiveToday = false
	events := []RawEvent{ev("IN", "09:00", 0.9), ev("OUT", "17:00", 0.9)}
	r := CalculateDay(events, ctx)

	if r.Status != StatusInactive {
		t.Errorf("status = %s, want INACTIVE", r.Status)
	}
	if r.WorkingMinutes != 0 {
		t.Errorf("working minutes = %d, want 0 for inactive employee", r.WorkingMinutes)
	}
}

func TestNoShiftAssignedStillCalculatesPresence(t *testing.T) {
	ctx := DayContext{EmployeeActiveToday: true}
	events := []RawEvent{ev("IN", "09:00", 0.9), ev("OUT", "17:00", 0.9)}
	r := CalculateDay(events, ctx)

	if r.Status != StatusPresent {
		t.Errorf("status = %s, want PRESENT even with no shift assigned", r.Status)
	}
	if r.LateMinutes != 0 || r.EarlyLeaveMinutes != 0 || r.OvertimeMinutes != 0 {
		t.Errorf("expected no late/early/overtime without a shift to compare against, got late=%d early=%d overtime=%d", r.LateMinutes, r.EarlyLeaveMinutes, r.OvertimeMinutes)
	}
}

func TestHoliday(t *testing.T) {
	ctx := baseCtx()
	ctx.IsHoliday = true
	r := CalculateDay(nil, ctx)

	if r.Status != StatusHoliday {
		t.Errorf("status = %s, want HOLIDAY", r.Status)
	}
	if r.CompOffEarned {
		t.Error("no events on a holiday shouldn't earn compoff")
	}
}

func TestWorkingOnHolidayEarnsCompOff(t *testing.T) {
	ctx := baseCtx()
	ctx.IsHoliday = true
	events := []RawEvent{ev("IN", "10:00", 0.9), ev("OUT", "14:00", 0.9)}
	r := CalculateDay(events, ctx)

	if r.Status != StatusHoliday {
		t.Errorf("status = %s, want HOLIDAY (status stays Holiday even if worked)", r.Status)
	}
	if !r.CompOffEarned {
		t.Error("expected CompOffEarned = true after working on a holiday")
	}
	if r.WorkingMinutes != 4*60 {
		t.Errorf("working minutes = %d, want %d", r.WorkingMinutes, 4*60)
	}
	if r.CompOffHalfDay {
		t.Error("no Half Day declared — CompOffHalfDay should stay false")
	}
}

func TestDeclaredHalfDayOnHolidayHalvesCompOff(t *testing.T) {
	// Worked part of a holiday, then declared Half Day when marking out —
	// same declared-choice-at-scan-out signal Half Day uses every other day.
	ctx := baseCtx()
	ctx.IsHoliday = true
	out := ev("OUT", "14:00", 0.9)
	out.OutReason = "HALF_DAY"
	events := []RawEvent{ev("IN", "10:00", 0.9), out}
	r := CalculateDay(events, ctx)

	if r.Status != StatusHoliday {
		t.Errorf("status = %s, want HOLIDAY (status stays Holiday even if worked)", r.Status)
	}
	if !r.CompOffEarned {
		t.Error("expected CompOffEarned = true after working on a holiday")
	}
	if !r.CompOffHalfDay {
		t.Error("expected CompOffHalfDay = true after declaring Half Day on a holiday")
	}
}

func TestWeeklyOff(t *testing.T) {
	ctx := baseCtx()
	ctx.IsWeeklyOff = true
	r := CalculateDay(nil, ctx)

	if r.Status != StatusWeeklyOff {
		t.Errorf("status = %s, want WEEKLY_OFF", r.Status)
	}
}

func TestApprovedLeave(t *testing.T) {
	ctx := baseCtx()
	ctx.HasApprovedLeave = true
	r := CalculateDay(nil, ctx)

	if r.Status != StatusOnLeave {
		t.Errorf("status = %s, want ON_LEAVE", r.Status)
	}
	if r.UnapprovedAbsence {
		t.Error("approved leave must not be flagged as an unapproved absence")
	}
}

func TestCalculationVersionStamped(t *testing.T) {
	r := CalculateDay(nil, baseCtx())
	if r.CalculationVersion != EngineVersion {
		t.Errorf("calculation version = %q, want %q", r.CalculationVersion, EngineVersion)
	}
}

// Overtime is a General Shift / 10 AM Shift policy — the internship shift has
// no such rule, so staying late earns it nothing.
func TestInternshipShiftEarnsNoOvertime(t *testing.T) {
	internShift := &ShiftInfo{Name: "Intern Shift", StartTime: mustTime("09:00"), EndTime: mustTime("17:00"), GracePeriodMinutes: 15}
	ctx := DayContext{Shift: internShift, EmployeeActiveToday: true}

	r := CalculateDay([]RawEvent{ev("IN", "09:00", 0.9), ev("OUT", "20:00", 0.9)}, ctx)
	if r.OvertimeMinutes != 0 {
		t.Errorf("overtime minutes = %d, want 0 on the internship shift", r.OvertimeMinutes)
	}
}

// 2026-09-19 is a Saturday, 2026-09-21 the following Monday — used below to
// exercise ShiftInfo.ResolvedFor against real weekday values rather than
// asserting on time.Weekday() math.
func mustDate(iso string) time.Time {
	d, err := time.Parse("2006-01-02", iso)
	if err != nil {
		panic(err)
	}
	return d
}

func shiftWithSaturdayOverride() *ShiftInfo {
	satStart := mustTime("09:30")
	satEnd := mustTime("18:30")
	return &ShiftInfo{
		Name: "General Shift", StartTime: mustTime("09:00"), EndTime: mustTime("19:00"),
		GracePeriodMinutes: 15,
		SaturdayStartTime:  &satStart, SaturdayEndTime: &satEnd,
	}
}

func TestResolvedForUsesSaturdayOverrideOnSaturday(t *testing.T) {
	resolved := shiftWithSaturdayOverride().ResolvedFor(mustDate("2026-09-19"))
	if got := resolved.StartTime.Format("15:04"); got != "09:30" {
		t.Errorf("Saturday start = %s, want 09:30", got)
	}
	if got := resolved.EndTime.Format("15:04"); got != "18:30" {
		t.Errorf("Saturday end = %s, want 18:30", got)
	}
}

func TestResolvedForKeepsWeekdayTimingOnAWeekday(t *testing.T) {
	resolved := shiftWithSaturdayOverride().ResolvedFor(mustDate("2026-09-21"))
	if got := resolved.StartTime.Format("15:04"); got != "09:00" {
		t.Errorf("Monday start = %s, want 09:00 (weekday timing, untouched)", got)
	}
	if got := resolved.EndTime.Format("15:04"); got != "19:00" {
		t.Errorf("Monday end = %s, want 19:00 (weekday timing, untouched)", got)
	}
}

// An unset Saturday field (e.g. grace left blank while only start and end
// were overridden) falls back to the weekday value for that field
// specifically, rather than the whole override being all-or-nothing.
func TestResolvedForFallsBackFieldByFieldWhenSaturdayValueUnset(t *testing.T) {
	resolved := shiftWithSaturdayOverride().ResolvedFor(mustDate("2026-09-19"))
	if resolved.GracePeriodMinutes != 15 {
		t.Errorf("grace period = %d, want 15 (no Saturday override set, falls back to weekday)", resolved.GracePeriodMinutes)
	}
}

func TestResolvedForOnNilShiftReturnsNil(t *testing.T) {
	var s *ShiftInfo
	if got := s.ResolvedFor(mustDate("2026-09-19")); got != nil {
		t.Errorf("resolved = %v, want nil for a nil shift", got)
	}
}

// End-to-end through CalculateDay: arriving at 09:20 on a Saturday is on time
// against the 09:30 Saturday start (within its 15-minute grace), even though
// it would be late against the 09:00 weekday start.
func TestSaturdayOverrideAppliesThroughCalculateDay(t *testing.T) {
	ctx := DayContext{Shift: shiftWithSaturdayOverride().ResolvedFor(mustDate("2026-09-19")), EmployeeActiveToday: true}
	r := CalculateDay([]RawEvent{ev("IN", "09:20", 0.9), ev("OUT", "18:30", 0.9)}, ctx)
	if r.LateMinutes != 0 {
		t.Errorf("late minutes = %d, want 0 — 09:20 is within the Saturday grace period (09:30 start + 15min)", r.LateMinutes)
	}
}
