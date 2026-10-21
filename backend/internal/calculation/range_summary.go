package calculation

import (
	"context"
	"time"

	"hr-management/backend/internal/clock"
)

// overtimeEligibleShifts is the allowlist for OvertimeDays — the internship
// shift has no overtime rule, so a stay-late day only counts for someone on
// one of the two regular shifts.
var overtimeEligibleShifts = map[string]bool{
	"General Shift": true,
	"10 AM Shift":   true,
}

// EmployeeRangeResult accumulates one employee's totals across a reported
// date range — the same per-day rules CalculateDay already applies, just
// summed instead of shown one day at a time.
type EmployeeRangeResult struct {
	EmployeeID    int64
	PresentDays   int
	AbsentDays    int
	HalfDays      int
	OnLeaveDays   int
	HolidayDays   int
	WeeklyOffDays int
	LateCount     int
	// LateStreakHalfDays counts days the engine turned into a Half Day purely
	// for being the 3rd/6th/9th... late arrival this month (ExLateStreakHalfDay)
	// — a subset of HalfDays, tracked separately so reports.Handlers can undo
	// this specific penalty and recompute it against the overtime-adjusted
	// late count without also touching half-days caused by something else
	// (e.g. arriving past the hard cutoff).
	LateStreakHalfDays int
	EarlyLeaveCount    int
	MissingOutCount    int
	// LunchOTCount counts days whose LUNCH-tagged break ran longer than
	// policy.LunchTimeLimitMinutes (ExLunchOT) — independent of shift, so
	// unlike OvertimeDays it's tallied in add() below, not in the per-day
	// loop further down. LunchOTMinutesSum is the sum of how far over the
	// limit each of those days ran — LunchOTMinutesSum / LunchOTCount is the
	// average overage analytics.Handlers ranks the Lunch OT leaderboard by.
	LunchOTCount      int
	LunchOTMinutesSum int
	WorkingMinutes    int
	OvertimeMinutes   int
	// OvertimeDays counts days the employee scanned OUT at least
	// StayLatePassThresholdMinutes after their shift's end time — a simple
	// "did they stay late today" day count for reporting, separate from
	// OvertimeMinutes (which measures worked time against the shift's minimum
	// hours and can register minutes on an ordinary full day). Only ever set
	// for days with a real recorded OUT scan — a historical/imported day has
	// no punch times to check, so it never counts here.
	OvertimeDays int
	// ScheduledDays is present+absent+half-day — the days the employee was
	// actually expected to show up, i.e. the denominator for an attendance
	// percentage. Leave/holiday/weekly-off days are excused, not missed, so
	// they don't belong on either side of that ratio.
	ScheduledDays int
	// PaidLeaveDays/UnpaidLeaveDays/CompOffLeaveDays split OnLeaveDays by the
	// leave request's own type (see leave.ValidLeaveTypes) — a day only ever
	// lands in one of these, the same day it counts toward OnLeaveDays, so
	// all three always sum to it. CompOffLeaveDays backs reports.CompOffUsed
	// directly — an actual count of approved COMPOFF-type leave days, not a
	// projection of what the current balance could theoretically cover.
	PaidLeaveDays    int
	UnpaidLeaveDays  int
	CompOffLeaveDays int

	// OfficeInMinutesSum/OfficeInDays back the average arrival time leaderboard
	// (analytics.Handlers.GetPerformance) — summed rather than stored as a list
	// so a range of any length costs one int addition per day instead of
	// growing memory with it. Only days with a genuine FirstIn count (a
	// holiday/leave/weekly-off day has none), so the average is over days the
	// employee actually showed up, not the whole range.
	OfficeInMinutesSum int
	OfficeInDays       int

	// OfficeOutMinutesSum/OfficeOutDays mirror OfficeInMinutesSum/OfficeInDays
	// exactly, but for the day's last genuine OUT scan — back the Employee
	// Performance module's "Highest Working Time" leaderboard
	// (analytics.Handlers), which measures average working time as the gap
	// between the average Office Out time and the average Office In time,
	// rather than summing each day's own worked minutes. Only a day with a
	// genuine LastOut counts (an in-progress day with no final OUT yet, or a
	// holiday/leave/weekly-off day, has none).
	OfficeOutMinutesSum int
	OfficeOutDays       int
}

// isToday tells add whether res is for the current calendar day — a
// trailing unpaired IN (ExMissingOut) on today just means "hasn't scanned
// out yet," expected for anyone still at work, not a gap for HR to fill. The
// same exception on a day that's already over is a real problem.
func (r *EmployeeRangeResult) add(res DailyResult, isToday bool) {
	switch res.Status {
	case StatusPresent:
		r.PresentDays++
		r.ScheduledDays++
	case StatusAbsent:
		r.AbsentDays++
		r.ScheduledDays++
	case StatusHalfDay:
		r.HalfDays++
		r.ScheduledDays++
	case StatusOnLeave:
		r.OnLeaveDays++
	case StatusHoliday:
		r.HolidayDays++
	case StatusWeeklyOff:
		r.WeeklyOffDays++
	}
	if res.LateMinutes > 0 {
		r.LateCount++
	}
	// EarlyLeaveMinutes stays populated from a trailing unpaired IN (still
	// checked in, not actually gone for the day — see engine.go) — counting
	// it here requires a genuine LastOut too, the same gate DaySummary.add
	// already applies, so an employee mid-shift today isn't wrongly counted
	// as having left early just because shift end hasn't arrived yet.
	if res.EarlyLeaveMinutes > 0 && res.LastOut != nil {
		r.EarlyLeaveCount++
	}
	if !isToday {
		for _, e := range res.Exceptions {
			if e == ExMissingOut {
				r.MissingOutCount++
				break
			}
		}
	}
	for _, e := range res.Exceptions {
		if e == ExLateStreakHalfDay {
			r.LateStreakHalfDays++
			break
		}
	}
	if res.LunchOTMinutes > 0 {
		r.LunchOTCount++
		r.LunchOTMinutesSum += res.LunchOTMinutes
	}
	r.WorkingMinutes += res.WorkingMinutes
	r.OvertimeMinutes += res.OvertimeMinutes
	if res.FirstIn != nil {
		r.OfficeInMinutesSum += res.FirstIn.Hour()*60 + res.FirstIn.Minute()
		r.OfficeInDays++
	}
	if res.LastOut != nil {
		r.OfficeOutMinutesSum += res.LastOut.Hour()*60 + res.LastOut.Minute()
		r.OfficeOutDays++
	}
}

// rangeLookbackDays matches findPreviousWorkDayLastOut's own window — the
// stay-late pass can have been earned up to that many days before the day
// being calculated, which may fall before the reported range even starts.
const rangeLookbackDays = 14

// SummarizeRange runs the engine for every active employee across [from,to],
// batched the same way SummarizeDate batches a single day: everything the
// engine needs — events (plus a lookback buffer), holidays, approved leave,
// and each employee's late-streak count carried in from before the range —
// is fetched in a fixed number of queries, then the pure CalculateDay
// function runs in memory once per employee-day. Nothing is written; like
// CalculateOnly, this is a read and must never create ledger rows as a side
// effect of someone viewing a report.
// The second return value is each employee's day-by-day result across
// [from,to] (date -> DailyResult) — the exact per-day results the totals are
// summed from, exposed as-is for a caller that needs the daily breakdown
// (e.g. a date-wise attendance grid, or a per-employee daily punch view), not
// just the aggregate counts. A date with no entry means the employee wasn't
// active yet (before joining).
//
// employeeID, if non-nil, scopes the whole run to just that one employee —
// used by a single-employee report (e.g. the Reports page's employee filter,
// or its per-employee daily drill-down) so it isn't paying to compute every
// other employee's range too. nil means everyone, the original behavior.
func (r *Repository) SummarizeRange(ctx context.Context, from, to string, employeeID *int64) (map[int64]*EmployeeRangeResult, map[int64]map[string]DailyResult, error) {
	return r.summarizeRange(ctx, from, to, employeeID, summarizeRangeOpts{preferStoredRows: true, skipBeforeJoining: true})
}

// summarizeRangeOpts controls two behaviors that are correct for
// SummarizeRange's own callers (company-wide reports/analytics, where
// trusting a stored/historical row and skipping days before an employee's
// joining date are both deliberate) but wrong for CalculateOnly below, which
// has always promised an employee viewing their own attendance a fresh live
// recalculation for every requested day — the same guarantee buildDay's old
// per-day loop gave it. Verified empirically (byte-for-byte diff against the
// old per-day loop over real data) that preferStoredRows:true silently
// returns stale/incomplete historical rows for CalculateOnly's purposes, and
// skipBeforeJoining:true silently omits days buildDay would have computed —
// hence CalculateOnly passing false for both instead of just reusing
// SummarizeRange's defaults.
type summarizeRangeOpts struct {
	preferStoredRows  bool
	skipBeforeJoining bool
	// liveStreakCounts re-derives the monthly Late Streak / Lunch OT Streak
	// count fresh from attendance_calculations for every single day — the
	// same two queries buildDay always ran — instead of snapshotting the
	// count once at range start and carrying it forward in memory across the
	// pass. The two can disagree whenever the persisted history has a gap
	// (a day nobody has recalculated since, or one that predates a rule
	// change) — verified empirically — so CalculateOnly turns this on to stay
	// byte-for-byte identical to buildDay. SummarizeRange's own callers
	// (company-wide reports/analytics) leave it off: the in-memory count is
	// both cheaper and, for a whole-company batch, the one actually trusted
	// today.
	liveStreakCounts bool
	// holidayNeverOverridesInactive stops the "a company holiday always
	// wins" override below from promoting a genuinely-Inactive day (not yet
	// joined, or already resigned) to Holiday. buildDay never needed this
	// guard — CalculateDay's own EmployeeActiveToday check returns Inactive
	// before it even looks at IsHoliday — so CalculateOnly turns this on to
	// match that. Off for SummarizeRange's own callers: the override's
	// purpose is fixing up a stored/imported row that predates a holiday
	// being added, and changing that today is out of scope here.
	holidayNeverOverridesInactive bool
}

func (r *Repository) summarizeRange(ctx context.Context, from, to string, employeeID *int64, opts summarizeRangeOpts) (map[int64]*EmployeeRangeResult, map[int64]map[string]DailyResult, error) {
	policy := CurrentPolicySettings()
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil, nil, err
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return nil, nil, err
	}

	// activeEmployeesWithShifts excludes RESIGNED/TERMINATED outright, which is
	// right for "who's active today" but wrong for a historical report — someone
	// who resigned last month still worked days within an earlier range. This
	// range-scoped variant includes anyone who could have been active for any
	// part of [from,to].
	employees, err := r.employeesForRange(ctx, from, employeeID)
	if err != nil {
		return nil, nil, err
	}

	bufferStart := start.AddDate(0, 0, -rangeLookbackDays)

	eventsByEmployeeDay, err := r.eventsForRange(ctx, bufferStart.Format("2006-01-02"), to)
	if err != nil {
		return nil, nil, err
	}

	// Historical/imported periods are stored directly rather than reconstructed
	// from punch events (there may be none) — when a stored row exists for an
	// employee-day, it's used as-is instead of live-computing, the same way
	// GetStored/GetHistory already trust this table as authoritative. Skipped
	// entirely when the caller wants a guaranteed-fresh recomputation instead
	// (see summarizeRangeOpts).
	var stored map[int64]map[string]DailyResult
	if opts.preferStoredRows {
		stored, err = r.storedResultsForRange(ctx, from, to)
		if err != nil {
			return nil, nil, err
		}
	}

	holidays, err := r.holidaysInRange(ctx, bufferStart.Format("2006-01-02"), to)
	if err != nil {
		return nil, nil, err
	}

	onLeave, err := r.employeesOnApprovedLeaveRange(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}

	sandwichLeave, err := r.employeesOnSandwichLeaveRange(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}

	absenceOverrides, err := r.employeesWithAbsenceOverrideRange(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}

	// Whatever late-streak count each employee already has on record for the
	// days between this month's start and the range's own start — the range
	// loop below then carries this forward day by day using its own
	// in-memory results, the same way CalculateAndStore's writes would.
	lateStreakBase, err := r.lateStreaksThisMonth(ctx, start)
	if err != nil {
		return nil, nil, err
	}
	lateStreak := map[int64]int{}
	for id, n := range lateStreakBase {
		lateStreak[id] = n
	}

	// Same carry-forward, for Lunch OT's own monthly strike count.
	lunchOTStreakBase, err := r.lunchOTStreaksThisMonth(ctx, start)
	if err != nil {
		return nil, nil, err
	}
	lunchOTStreak := map[int64]int{}
	for id, n := range lunchOTStreakBase {
		lunchOTStreak[id] = n
	}

	out := map[int64]*EmployeeRangeResult{}
	dailyResults := map[int64]map[string]DailyResult{}
	for _, e := range employees {
		out[e.id] = &EmployeeRangeResult{EmployeeID: e.id}
		dailyResults[e.id] = map[string]DailyResult{}
	}

	today := clock.Now().Format("2006-01-02")
	currentMonth := start.Month()
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Month() != currentMonth {
			currentMonth = d.Month()
			for id := range lateStreak {
				lateStreak[id] = 0
			}
			for id := range lunchOTStreak {
				lunchOTStreak[id] = 0
			}
		}
		date := d.Format("2006-01-02")
		isWeeklyOff := d.Weekday() == time.Sunday
		isHoliday := holidays[date]

		for _, e := range employees {
			if opts.skipBeforeJoining && d.Before(e.joiningDate) {
				continue // not employed yet — not scheduled, not absent, just not there
			}

			resolvedShift := e.shift.ResolvedFor(d)

			var result DailyResult
			if storedResult, ok := stored[e.id][date]; opts.preferStoredRows && ok {
				result = storedResult
			} else {
				ctxDay := DayContext{
					EmployeeActiveToday: (&employeeContext{
						Status:         e.status,
						LastWorkingDay: e.lastWorkingDay,
					}).isActiveOn(d),
					Shift:  resolvedShift,
					Policy: policy,
				}

				var events []RawEvent
				if ctxDay.EmployeeActiveToday {
					ctxDay.IsHoliday = isHoliday
					ctxDay.IsWeeklyOff = isWeeklyOff
					ctxDay.HasApprovedLeave = onLeave[e.id][date] != ""
					ctxDay.IsSandwichedLeave = sandwichLeave[e.id][date]
					ctxDay.IsExcusedAbsence = absenceOverrides[e.id][date]
					if !ctxDay.IsHoliday && !ctxDay.IsWeeklyOff && !ctxDay.HasApprovedLeave {
						if opts.liveStreakCounts {
							// The in-memory lateStreak/lunchOTStreak maps carry
							// forward whatever THIS pass itself computed for
							// earlier days in the range — correct and fast for
							// SummarizeRange's own callers, but not what
							// buildDay ever did: it re-derives each count fresh
							// from whatever's actually persisted in
							// attendance_calculations, for each day
							// independently. Where stored history has gaps
							// (never recalculated, or predates a rule change),
							// those two can disagree — verified empirically —
							// so CalculateOnly asks for the real per-day query
							// instead, same as buildDay always has.
							var err2 error
							ctxDay.LateStreakSoFarThisMonth, err2 = r.lateStreakSoFarThisMonth(ctx, e.id, d)
							if err2 != nil {
								return nil, nil, err2
							}
							ctxDay.LunchOTStreakSoFarThisMonth, err2 = r.lunchOTStreakSoFarThisMonth(ctx, e.id, d)
							if err2 != nil {
								return nil, nil, err2
							}
						} else {
							ctxDay.LateStreakSoFarThisMonth = lateStreak[e.id]
							ctxDay.LunchOTStreakSoFarThisMonth = lunchOTStreak[e.id]
						}
						ctxDay.PreviousWorkDayLastOut = previousWorkDayLastOutFromMap(eventsByEmployeeDay, holidays, e.id, d)
					}
					events = eventsByEmployeeDay[e.id][date]
				}

				result = CalculateDay(events, ctxDay)
			}

			// A company holiday always wins, the same way CalculateDay itself
			// treats it (see engine.go's rule 2) — including over a stored/
			// imported row. A legacy import predates a holiday added to the
			// calendar afterward has no way to have known about it, so without
			// this a historical PRESENT would silently outrank a real holiday.
			// Sandwich Leave is the one deliberate exception (engine.go's rule
			// 1b already outranks Holiday for the same reason) — a stored row
			// predates sandwich leave existing as a concept entirely, so it
			// can't have already accounted for it either.
			//
			// Status != StatusInactive guards the one case buildDay itself
			// never needed to think about: CalculateDay's own rule 1 returns
			// Inactive before it ever looks at IsHoliday, so a not-yet-joined
			// or already-resigned day is never "corrected" into a Holiday by
			// buildDay — verified empirically against real data. Left
			// unguarded, this override would silently disagree with that for
			// CalculateOnly specifically; kept unconditional for every other
			// caller (Reports/Analytics/GetDailyBreakdown already rely on
			// today's exact behavior here, out of scope for this change).
			if isHoliday && result.Status != StatusHoliday && !sandwichLeave[e.id][date] &&
				!(opts.holidayNeverOverridesInactive && result.Status == StatusInactive) {
				result.Status = StatusHoliday
			}

			out[e.id].add(result, date == today)
			dailyResults[e.id][date] = result

			if result.Status == StatusOnLeave {
				switch onLeave[e.id][date] {
				case "PAID":
					out[e.id].PaidLeaveDays++
				case "UNPAID":
					out[e.id].UnpaidLeaveDays++
				case "COMPOFF":
					out[e.id].CompOffLeaveDays++
				}
			}

			if resolvedShift != nil && overtimeEligibleShifts[resolvedShift.Name] && result.LastOut != nil {
				shiftEnd := onSameDayAt(*result.LastOut, resolvedShift.EndTime)
				cutoff := shiftEnd.Add(time.Duration(policy.StayLatePassThresholdMinutes) * time.Minute)
				if !result.LastOut.Before(cutoff) {
					out[e.id].OvertimeDays++
				}
			}

			for _, ex := range result.Exceptions {
				if ex == ExLateStreakCounted || ex == ExLateStreakHalfDay {
					lateStreak[e.id]++
					break
				}
			}
			for _, ex := range result.Exceptions {
				if ex == ExLunchOTStreakCounted || ex == ExLunchOTStreakPenalty {
					lunchOTStreak[e.id]++
					break
				}
			}
		}
	}

	return out, dailyResults, nil
}

// previousWorkDayLastOutFromMap mirrors findPreviousWorkDayLastOut, but reads
// from the range's own pre-fetched event map instead of issuing a query per
// candidate day.
func previousWorkDayLastOutFromMap(
	eventsByEmployeeDay map[int64]map[string][]RawEvent,
	holidays map[string]bool,
	employeeID int64,
	date time.Time,
) *time.Time {
	for i := 1; i <= rangeLookbackDays; i++ {
		candidate := date.AddDate(0, 0, -i)
		if candidate.Weekday() == time.Sunday {
			continue
		}
		day := candidate.Format("2006-01-02")
		if holidays[day] {
			continue
		}
		events := eventsByEmployeeDay[employeeID][day]
		if len(events) == 0 {
			return nil
		}
		for j := len(events) - 1; j >= 0; j-- {
			if events[j].Direction == "OUT" {
				last := events[j].EventTime
				return &last
			}
		}
		return nil
	}
	return nil
}

// rangeEmployeeRow is employeeRow plus joiningDate — needed here (and not in
// the single-date SummarizeDate) because a report can span months before an
// employee's first day, which a single "today" calculation never has to
// consider.
type rangeEmployeeRow struct {
	id             int64
	status         string
	lastWorkingDay *time.Time
	joiningDate    time.Time
	shift          *ShiftInfo
}

// employeesForRange is activeEmployeesWithShifts generalized to include
// anyone who could have been active during any part of [from,∞) — a resigned
// employee is still worth reporting on for the months before they left.
// employeeID, if non-nil, narrows this to just that one employee (see
// SummarizeRange).
func (r *Repository) employeesForRange(ctx context.Context, from string, employeeID *int64) ([]rangeEmployeeRow, error) {
	query := `SELECT e.id, e.status, e.last_working_day, e.joining_date,
		        s.name, s.start_time, s.end_time, s.grace_period_minutes,
		        s.saturday_start_time, s.saturday_end_time, s.saturday_grace_period_minutes
		 FROM employees e
		 LEFT JOIN shifts s ON s.id = e.shift_id
		 WHERE (e.status NOT IN ('RESIGNED', 'TERMINATED') OR e.last_working_day >= $1::date)`
	args := []any{from}
	if employeeID != nil {
		query += " AND e.id = $2"
		args = append(args, *employeeID)
	}
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []rangeEmployeeRow
	for rows.Next() {
		var e rangeEmployeeRow
		var name *string
		var start, end *time.Time
		var grace *int
		var satStart, satEnd *time.Time
		var satGrace *int
		if err := rows.Scan(&e.id, &e.status, &e.lastWorkingDay, &e.joiningDate, &name, &start, &end, &grace,
			&satStart, &satEnd, &satGrace); err != nil {
			return nil, err
		}
		if start != nil && end != nil && grace != nil {
			e.shift = &ShiftInfo{
				Name: *name, StartTime: *start, EndTime: *end, GracePeriodMinutes: *grace,
				SaturdayStartTime: satStart, SaturdayEndTime: satEnd,
				SaturdayGracePeriodMinutes: satGrace,
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// storedResultsForRange batch-loads whatever's already persisted in
// attendance_calculations for [from,to] — historical/imported data that has
// no punch events to recompute from, and a real day's cached result if one
// happens to already exist. Either way, a stored row is trusted as-is rather
// than recomputed.
func (r *Repository) storedResultsForRange(ctx context.Context, from, to string) (map[int64]map[string]DailyResult, error) {
	rows, err := r.db.Query(ctx,
		`SELECT employee_id, calc_date::text, status, working_minutes,
		        late_minutes, early_leave_minutes, overtime_minutes, exceptions,
		        first_in, last_out, calculation_version, compoff_earned, compoff_half_day
		 FROM attendance_calculations
		 WHERE calc_date BETWEEN $1::date AND $2::date`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]map[string]DailyResult{}
	for rows.Next() {
		var employeeID int64
		var date string
		var res DailyResult
		var exceptions []string
		if err := rows.Scan(&employeeID, &date, &res.Status, &res.WorkingMinutes,
			&res.LateMinutes, &res.EarlyLeaveMinutes, &res.OvertimeMinutes, &exceptions,
			&res.FirstIn, &res.LastOut, &res.CalculationVersion, &res.CompOffEarned, &res.CompOffHalfDay); err != nil {
			return nil, err
		}
		res.Exceptions = make([]ExceptionCode, len(exceptions))
		for i, e := range exceptions {
			res.Exceptions[i] = ExceptionCode(e)
		}
		if out[employeeID] == nil {
			out[employeeID] = map[string]DailyResult{}
		}
		out[employeeID][date] = res
	}
	return out, rows.Err()
}

// PunchesInRange is the raw-punch counterpart to SummarizeRange's computed
// totals: every recorded event for every employee in [from,to], grouped by
// employee then by date — for a report that wants to show what was actually
// scanned (office in/out, a lunch break) rather than the engine's derived
// status. A thin, exported wrapper around eventsForRange, so there's still
// only one place that reads attendance_events for a date span.
func (r *Repository) PunchesInRange(ctx context.Context, from, to string) (map[int64]map[string][]RawEvent, error) {
	return r.eventsForRange(ctx, from, to)
}

// eventsForRange is eventsForDate generalized to a span of dates, grouped by
// employee and then by day so both "today's events" and "the previous work
// day's last OUT" can be read from one fetch.
func (r *Repository) eventsForRange(ctx context.Context, from, to string) (map[int64]map[string][]RawEvent, error) {
	rows, err := r.db.Query(ctx,
		// WHERE is rewritten sargable (plain event_time range, no cast) —
		// see getEvents in repository.go for why the cast form can't use
		// idx_attendance_events_time. SELECT still needs the ::date cast for
		// the day-bucket output column.
		`SELECT employee_id, event_time::date::text AS event_day, direction, event_time, confidence, source,
		        COALESCE(out_reason::text, ''), note
		 FROM attendance_events
		 WHERE event_time >= $1::date AND event_time < $2::date + interval '1 day'
		 ORDER BY employee_id, event_time ASC`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]map[string][]RawEvent{}
	for rows.Next() {
		var employeeID int64
		var day string
		var e RawEvent
		if err := rows.Scan(&employeeID, &day, &e.Direction, &e.EventTime, &e.Confidence, &e.Source, &e.OutReason, &e.Note); err != nil {
			return nil, err
		}
		if out[employeeID] == nil {
			out[employeeID] = map[string][]RawEvent{}
		}
		out[employeeID][day] = append(out[employeeID][day], e)
	}
	return out, rows.Err()
}

// HolidayNamesInRange is holidaysInRange's display companion — the holiday's
// own name, keyed by date, for a caller that needs to say which holiday a
// HOLIDAY-status day actually was (e.g. reports.Handlers.EmployeeDailyBreakdown),
// not just that it was one.
func (r *Repository) HolidayNamesInRange(ctx context.Context, from, to string) (map[string]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT holiday_date::text, name FROM holidays WHERE holiday_date BETWEEN $1::date AND $2::date`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var d, name string
		if err := rows.Scan(&d, &name); err != nil {
			return nil, err
		}
		out[d] = name
	}
	return out, rows.Err()
}

// holidaysInRange is isHoliday generalized to a span of dates.
func (r *Repository) holidaysInRange(ctx context.Context, from, to string) (map[string]bool, error) {
	rows, err := r.db.Query(ctx,
		`SELECT holiday_date::text FROM holidays WHERE holiday_date BETWEEN $1::date AND $2::date`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out[d] = true
	}
	return out, rows.Err()
}

// employeesOnApprovedLeaveRange always returns empty — this duplicate has no
// Leave Management module (no leave_requests table). Kept as a function so
// every call site's shape (including the per-employee leave_type value other
// callers used to switch on) is unchanged from the original.
func (r *Repository) employeesOnApprovedLeaveRange(ctx context.Context, from, to string) (map[int64]map[string]string, error) {
	return map[int64]map[string]string{}, nil
}

// employeesOnSandwichLeaveRange always returns empty — see employeesOnApprovedLeaveRange.
func (r *Repository) employeesOnSandwichLeaveRange(ctx context.Context, from, to string) (map[int64]map[string]bool, error) {
	return map[int64]map[string]bool{}, nil
}

// employeesWithAbsenceOverrideRange is the batch (all-employees, date-range)
// form of hasAbsenceOverride — see absence_overrides.
func (r *Repository) employeesWithAbsenceOverrideRange(ctx context.Context, from, to string) (map[int64]map[string]bool, error) {
	rows, err := r.db.Query(ctx,
		`SELECT employee_id, override_date::text FROM absence_overrides
		 WHERE override_date BETWEEN $1::date AND $2::date`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]map[string]bool{}
	for rows.Next() {
		var id int64
		var day string
		if err := rows.Scan(&id, &day); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[string]bool{}
		}
		out[id][day] = true
	}
	return out, rows.Err()
}
