package calculation

import "testing"

// Walks the exact windows described: 09:15–10:30 is late and counts toward the
// three strikes, a pass excuses the strike but not the lateness, and 10:31+ is a
// half day rather than a late arrival — the two are never both.
func TestStatedRuleWindows(t *testing.T) {
	cases := []struct {
		arrive      string
		streakSoFar int
		withPass    bool
		wantStatus  DayStatus
		wantLate    bool
		wantStrike  bool
	}{
		{"09:14", 0, false, StatusPresent, false, false},
		{"09:15", 0, false, StatusPresent, true, true},
		{"10:30", 0, false, StatusPresent, true, true},
		{"10:31", 0, false, StatusHalfDay, false, false},
		{"09:45", 0, true, StatusPresent, true, false},
		{"10:31", 0, true, StatusHalfDay, false, false},
	}
	for _, c := range cases {
		ctx := baseCtx()
		ctx.LateStreakSoFarThisMonth = c.streakSoFar
		if c.withPass {
			prev := evOnDay(14, "OUT", "19:45", 0.9).EventTime
			ctx.PreviousWorkDayLastOut = &prev
		}
		r := CalculateDay([]RawEvent{ev("IN", c.arrive, 0.9), ev("OUT", "19:00", 0.9)}, ctx)

		if r.Status != c.wantStatus {
			t.Errorf("arrive %s pass=%v: status = %s, want %s", c.arrive, c.withPass, r.Status, c.wantStatus)
		}
		if (r.LateMinutes > 0) != c.wantLate {
			t.Errorf("arrive %s pass=%v: late = %d, wantLate %v", c.arrive, c.withPass, r.LateMinutes, c.wantLate)
		}
		if hasException(r.Exceptions, ExLateStreakCounted) != c.wantStrike {
			t.Errorf("arrive %s pass=%v: strike counted = %v, want %v (%v)", c.arrive, c.withPass,
				!c.wantStrike, c.wantStrike, r.Exceptions)
		}
	}
}

// Early Leave carries no automatic escalation to Half Day anymore — leaving
// before shift end (baseCtx's shift runs 09:00-17:00) only ever produces an
// informational minute count. A Half Day for leaving early now only ever
// comes from an explicitly declared reason (see TestDeclaredHalfDayOverridesFullHours),
// never inferred from the clock alone. 13:00 is picked as the earliest case
// here specifically because it still clears the short-hours-half-day floor
// (4 of the 8 minimum hours) — a case that used to be an automatic half day
// under the old hardcoded window, and now isn't.
func TestEarlyLeaveIsInformationalOnly(t *testing.T) {
	cases := []struct {
		leaveAt       string
		wantEarlyMins int
	}{
		{"17:00", 0},
		{"16:50", 0},   // exactly at the grace-adjusted cutoff (shift end minus the default 10-minute grace) — still on time
		{"16:49", 1},   // one minute inside the cutoff
		{"15:00", 110}, // 110 = 120 minus the 10-minute grace
		{"14:00", 170}, // would have been an automatic half day under the old window
		{"13:00", 230}, // well past it too, still clears the short-hours floor exactly
	}
	for _, c := range cases {
		r := CalculateDay([]RawEvent{ev("IN", "09:00", 0.9), ev("OUT", c.leaveAt, 0.9)}, baseCtx())
		if r.Status != StatusPresent {
			t.Errorf("leave at %s: status = %s, want PRESENT — early leave alone never forces Half Day anymore", c.leaveAt, r.Status)
		}
		if r.EarlyLeaveMinutes != c.wantEarlyMins {
			t.Errorf("leave at %s: early leave minutes = %d, want %d", c.leaveAt, r.EarlyLeaveMinutes, c.wantEarlyMins)
		}
	}
}

// A trailing unpaired IN — a forgotten checkout later in the day, after an
// earlier complete session — must not be read as "left early". "last" here
// is just whichever scan happened to be recorded last, not a real departure
// time, so escalating it to an automatic half day would repeat the same
// mistake MISSING_OUT handling already avoids on the arrival side.
func TestEarlyDepartureNotEscalatedWithoutGenuineOut(t *testing.T) {
	events := []RawEvent{
		ev("IN", "09:00", 0.9),
		ev("OUT", "13:00", 0.9), // one complete 4-hour session
		ev("IN", "14:00", 0.9),  // back from break, never scanned out again
	}
	r := CalculateDay(events, baseCtx())

	if r.Status == StatusHalfDay {
		t.Errorf("status = %s, want anything but HALF_DAY — there is no confirmed departure time", r.Status)
	}
	if !hasException(r.Exceptions, ExMissingOut) {
		t.Error("expected MISSING_OUT to still be flagged for HR to correct")
	}
	// The informational number is unaffected by the safety gate — only the
	// escalation to a penalty is withheld. 170 = 180 minus the default
	// 10-minute end-of-shift grace period.
	if r.EarlyLeaveMinutes != 170 {
		t.Errorf("early leave minutes = %d, want 170 (informational only)", r.EarlyLeaveMinutes)
	}
}

// The departure-side cutoff no longer exists as an automatic rule — arriving
// past the half-day cutoff still forces Half Day on its own (that side is
// untouched), and an early departure alongside it stays purely informational
// rather than adding its own exception.
func TestLateArrivalHalfDayNotDoubledWithEarlyLeave(t *testing.T) {
	r := CalculateDay([]RawEvent{ev("IN", "10:35", 0.9), ev("OUT", "14:00", 0.9)}, baseCtx())

	if r.Status != StatusHalfDay {
		t.Fatalf("status = %s, want HALF_DAY", r.Status)
	}
	if !hasException(r.Exceptions, ExLateArrivalHalfDay) {
		t.Error("expected LATE_ARRIVAL_HALF_DAY")
	}
	if r.EarlyLeaveMinutes != 170 {
		t.Errorf("early leave minutes = %d, want 170 (informational only, no separate exception anymore)", r.EarlyLeaveMinutes)
	}
}

// A manual correction can flag an arrival IN as "Late Coming" even when the
// recorded time itself would otherwise read as on-time — HR is asserting
// something the clock alone doesn't show.
func TestManualLateComingForcesLateArrivalEvenIfOnTime(t *testing.T) {
	events := []RawEvent{
		withReason(ev("IN", "09:05", 0.9), "LATE_COMING"),
		ev("OUT", "17:00", 0.9),
	}
	r := CalculateDay(events, baseCtx())

	if r.LateMinutes <= 0 {
		t.Errorf("late minutes = %d, want > 0 — 09:05 is within grace but was flagged Late Coming", r.LateMinutes)
	}
	if !hasException(r.Exceptions, ExLateStreakCounted) {
		t.Error("expected LATE_STREAK_COUNTED — a forced late arrival still counts toward the monthly streak")
	}
}

// The forced Late Coming reason must never downgrade an arrival that's
// already past the hard half-day cutoff — that outcome stays as-is.
func TestManualLateComingNeverDowngradesHalfDayCutoff(t *testing.T) {
	events := []RawEvent{
		withReason(ev("IN", "10:35", 0.9), "LATE_COMING"),
		ev("OUT", "17:00", 0.9),
	}
	r := CalculateDay(events, baseCtx())

	if r.Status != StatusHalfDay {
		t.Errorf("status = %s, want HALF_DAY — 10:35 is already past the hard cutoff", r.Status)
	}
	if !hasException(r.Exceptions, ExLateArrivalHalfDay) {
		t.Error("expected LATE_ARRIVAL_HALF_DAY, not a late-streak entry")
	}
}

// A manual correction can also declare Half Day on the arrival IN itself, not
// just on an OUT — same override, either direction.
func TestManualHalfDayReasonOnInForcesHalfDay(t *testing.T) {
	events := []RawEvent{
		withReason(ev("IN", "09:00", 0.9), "HALF_DAY"),
		ev("OUT", "17:00", 0.9),
	}
	r := CalculateDay(events, baseCtx())

	if r.Status != StatusHalfDay {
		t.Errorf("status = %s, want HALF_DAY — declared via the arrival IN", r.Status)
	}
	if !hasException(r.Exceptions, ExDeclaredHalfDay) {
		t.Error("expected DECLARED_HALF_DAY")
	}
}
