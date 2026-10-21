package calculation

import "testing"

func countsTowardStreak(r DailyResult) bool {
	return hasException(r.Exceptions, ExLateStreakCounted) || hasException(r.Exceptions, ExLateStreakHalfDay)
}

// With two lates already this month, a third late arrival inside the late
// window is the one that costs a half day.
func TestThirdLateInsideWindowBecomesHalfDay(t *testing.T) {
	ctx := baseCtx()
	ctx.LateStreakSoFarThisMonth = 2

	r := CalculateDay([]RawEvent{ev("IN", "09:45", 0.9), ev("OUT", "19:00", 0.9)}, ctx)

	if r.Status != StatusHalfDay {
		t.Errorf("status = %s, want HALF_DAY on the third late arrival", r.Status)
	}
	if !hasException(r.Exceptions, ExLateStreakHalfDay) {
		t.Errorf("expected LATE_STREAK_HALF_DAY, got %v", r.Exceptions)
	}
	if !countsTowardStreak(r) {
		t.Error("the third late arrival must itself count toward the streak")
	}
}

// Arriving past the cutoff already costs a half day on its own, so it must not
// also burn a strike — the tally stays where it was and keeps watching the
// following days for a third late arrival. Nor is it marked late: past the
// cutoff the day is a half day instead of a late arrival, not as well as one.
func TestPastCutoffDoesNotBurnAStrike(t *testing.T) {
	ctx := baseCtx()
	ctx.LateStreakSoFarThisMonth = 2

	r := CalculateDay([]RawEvent{ev("IN", "10:35", 0.9), ev("OUT", "19:00", 0.9)}, ctx)

	if r.Status != StatusHalfDay {
		t.Errorf("status = %s, want HALF_DAY past the cutoff", r.Status)
	}
	if !hasException(r.Exceptions, ExLateArrivalHalfDay) {
		t.Errorf("expected LATE_ARRIVAL_HALF_DAY, got %v", r.Exceptions)
	}
	if countsTowardStreak(r) {
		t.Errorf("a past-cutoff day must not count toward the three strikes, got %v", r.Exceptions)
	}
	if r.LateMinutes != 0 {
		t.Errorf("late minutes = %d, want 0 — a half day is not also a late arrival", r.LateMinutes)
	}
}

// The same past-cutoff day must leave a clean tally alone too, so the strike
// really does carry forward rather than being silently consumed.
func TestPastCutoffLeavesTallyUntouched(t *testing.T) {
	ctx := baseCtx()
	ctx.LateStreakSoFarThisMonth = 0

	r := CalculateDay([]RawEvent{ev("IN", "11:00", 0.9), ev("OUT", "19:00", 0.9)}, ctx)

	if countsTowardStreak(r) {
		t.Errorf("past-cutoff day should not start the tally, got %v", r.Exceptions)
	}
}
