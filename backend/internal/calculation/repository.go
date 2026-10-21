package calculation

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type StoredResult struct {
	EmployeeID         int64           `json:"employee_id"`
	Date               string          `json:"date"`
	Status             DayStatus       `json:"status"`
	FirstIn            *time.Time      `json:"first_in"`
	LastOut            *time.Time      `json:"last_out"`
	WorkingMinutes     int             `json:"working_minutes"`
	LateMinutes        int             `json:"late_minutes"`
	EarlyLeaveMinutes  int             `json:"early_leave_minutes"`
	OvertimeMinutes    int             `json:"overtime_minutes"`
	Exceptions         []ExceptionCode `json:"exceptions"`
	CalculationVersion string          `json:"calculation_version"`
	CompOffEarned      bool            `json:"compoff_earned"`
	// CompOffHalfDay is true when CompOffEarned is true and the employee
	// declared Half Day on that holiday — see DailyResult.CompOffHalfDay.
	CompOffHalfDay bool `json:"compoff_half_day"`
	// IsExcusedAbsence is true when a Super Admin's manual override (see
	// absence_overrides) is why this day shows On Leave instead of Absent —
	// lets the frontend tell that apart from a real approved leave request,
	// e.g. to offer "undo the excuse" only where there's actually an
	// override to undo.
	IsExcusedAbsence bool      `json:"is_excused_absence"`
	CalculatedAt     time.Time `json:"calculated_at"`
}

type employeeContext struct {
	Status         string
	ShiftID        *int64
	LastWorkingDay *time.Time
}

func (r *Repository) getEmployeeContext(ctx context.Context, employeeID int64) (*employeeContext, error) {
	var ec employeeContext
	err := r.db.QueryRow(ctx,
		`SELECT status, shift_id, last_working_day FROM employees WHERE id = $1`,
		employeeID,
	).Scan(&ec.Status, &ec.ShiftID, &ec.LastWorkingDay)
	if err != nil {
		return nil, err
	}
	return &ec, nil
}

// isActiveOn decides whether an employee counts as active for a given date.
// Only ACTIVE/ON_NOTICE are unconditionally active. Anyone separated is
// active only up through their last_working_day; if that date was never
// recorded, we have no reliable way to know which past days were valid, so
// we skip them entirely rather than guess.
func (ec *employeeContext) isActiveOn(date time.Time) bool {
	switch ec.Status {
	case "ACTIVE", "ON_NOTICE":
		return true
	default:
		if ec.LastWorkingDay == nil {
			return false
		}
		return !date.After(*ec.LastWorkingDay)
	}
}

// IsBeforeShiftEnd reports whether t falls before the employee's own shift
// end time for t's date (the Saturday override resolved, if one is set) —
// used to decide whether marking OUT needs a reason (see
// attendance.tooSoonToMarkOut's sibling check in the Mark/MarkAuto handlers).
// An employee with no shift assigned always reports false: there's nothing to
// compare against, so no reason is ever required for them.
func (r *Repository) IsBeforeShiftEnd(ctx context.Context, employeeID int64, t time.Time) (bool, error) {
	ec, err := r.getEmployeeContext(ctx, employeeID)
	if err != nil {
		return false, err
	}
	if ec.ShiftID == nil {
		return false, nil
	}
	shift, err := r.getShiftInfo(ctx, *ec.ShiftID)
	if err != nil {
		return false, err
	}
	resolved := shift.ResolvedFor(t)
	shiftEnd := onSameDayAt(t, resolved.EndTime)
	// The last EndOfShiftGraceMinutes before shift end count as a normal
	// checkout too, no reason needed — the mirror image of a shift's own
	// start-of-day grace period, at the other end of the day.
	graceMinutes := CurrentPolicySettings().EndOfShiftGraceMinutes
	cutoff := shiftEnd.Add(-time.Duration(graceMinutes) * time.Minute)
	return t.Before(cutoff), nil
}

func (r *Repository) getShiftInfo(ctx context.Context, shiftID int64) (*ShiftInfo, error) {
	var s ShiftInfo
	err := r.db.QueryRow(ctx,
		`SELECT name, start_time, end_time, grace_period_minutes,
		        saturday_start_time, saturday_end_time, saturday_grace_period_minutes
		 FROM shifts WHERE id = $1`,
		shiftID,
	).Scan(&s.Name, &s.StartTime, &s.EndTime, &s.GracePeriodMinutes,
		&s.SaturdayStartTime, &s.SaturdayEndTime, &s.SaturdayGracePeriodMinutes)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) getEvents(ctx context.Context, employeeID int64, date string) ([]RawEvent, error) {
	rows, err := r.db.Query(ctx,
		// event_time >= $2 AND < $2+1day is the sargable equivalent of
		// event_time::date = $2 — the cast alone can't use
		// idx_attendance_events_time (a date-cast of a timestamptz column
		// isn't IMMUTABLE, so Postgres can't index it directly), this range
		// form can. Both depend on the session's TimeZone the same way the
		// cast did, so they always agree regardless of what that's set to.
		`SELECT direction, event_time, confidence, source, COALESCE(out_reason::text, '') FROM attendance_events
		 WHERE employee_id = $1 AND event_time >= $2::date AND event_time < $2::date + interval '1 day'
		 ORDER BY event_time ASC`,
		employeeID, date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []RawEvent
	for rows.Next() {
		var e RawEvent
		if err := rows.Scan(&e.Direction, &e.EventTime, &e.Confidence, &e.Source, &e.OutReason); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *Repository) isHoliday(ctx context.Context, date string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM holidays WHERE holiday_date = $1::date)`, date).Scan(&exists)
	return exists, err
}

// hasApprovedLeave always reports false — this duplicate has no Leave
// Management module (no leave_requests table at all), so no day is ever
// On Leave via an approved request. Kept as a function (rather than deleted
// and inlined as a literal `false` at every call site) so buildDay's shape
// stays identical to the original, and so re-adding Leave later is a
// one-function change.
func (r *Repository) hasApprovedLeave(ctx context.Context, employeeID int64, date string) (bool, error) {
	return false, nil
}

// hasSandwichLeave always reports false — see hasApprovedLeave.
func (r *Repository) hasSandwichLeave(ctx context.Context, employeeID int64, date string) (bool, error) {
	return false, nil
}

// hasAbsenceOverride reports whether a Super Admin has manually excused this
// employee-day (see absence_overrides — row existence is the flag).
func (r *Repository) hasAbsenceOverride(ctx context.Context, employeeID int64, date string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM absence_overrides WHERE employee_id = $1 AND override_date = $2::date)`,
		employeeID, date,
	).Scan(&exists)
	return exists, err
}

// lateStreakSoFarThisMonth counts late days already recorded this month
// (strictly before the date being calculated) that counted toward the
// streak — excused (pass-covered) lateness never sets this marker, so it's
// naturally excluded.
func (r *Repository) lateStreakSoFarThisMonth(ctx context.Context, employeeID int64, date time.Time) (int, error) {
	monthStart := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM attendance_calculations
		 WHERE employee_id = $1 AND calc_date >= $2::date AND calc_date < $3::date
		 AND exceptions && ARRAY['LATE_STREAK_COUNTED','LATE_STREAK_HALF_DAY']::text[]`,
		employeeID, monthStart.Format("2006-01-02"), date.Format("2006-01-02"),
	).Scan(&count)
	return count, err
}

// lunchOTStreakSoFarThisMonth mirrors lateStreakSoFarThisMonth for Lunch OT's
// own monthly strike count.
func (r *Repository) lunchOTStreakSoFarThisMonth(ctx context.Context, employeeID int64, date time.Time) (int, error) {
	monthStart := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM attendance_calculations
		 WHERE employee_id = $1 AND calc_date >= $2::date AND calc_date < $3::date
		 AND exceptions && ARRAY['LUNCH_OT_STREAK_COUNTED','LUNCH_OT_STREAK_PENALTY']::text[]`,
		employeeID, monthStart.Format("2006-01-02"), date.Format("2006-01-02"),
	).Scan(&count)
	return count, err
}

// findPreviousWorkDayLastOut walks backward from date (skipping Sundays and
// holidays) to find the most recent scheduled working day, then returns that
// day's last OUT scan — nil if the employee had no events that day (nothing to
// have "stayed late" for).
//
// Only an OUT counts. The pass is earned by *marking out* after the threshold,
// so an unpaired IN left behind by someone who forgot to scan out must not
// earn one — otherwise forgetting would be rewarded with a free late arrival.
func (r *Repository) findPreviousWorkDayLastOut(ctx context.Context, employeeID int64, date time.Time) (*time.Time, error) {
	for i := 1; i <= 14; i++ {
		candidate := date.AddDate(0, 0, -i)
		if candidate.Weekday() == time.Sunday {
			continue
		}
		holiday, err := r.isHoliday(ctx, candidate.Format("2006-01-02"))
		if err != nil {
			return nil, err
		}
		if holiday {
			continue
		}
		events, err := r.getEvents(ctx, employeeID, candidate.Format("2006-01-02"))
		if err != nil {
			return nil, err
		}
		if len(events) == 0 {
			return nil, nil
		}
		for i := len(events) - 1; i >= 0; i-- {
			if events[i].Direction == "OUT" {
				last := events[i].EventTime
				return &last, nil
			}
		}
		// Worked that day but never scanned out — no pass until HR corrects it.
		return nil, nil
	}
	return nil, nil
}

// syncAutoLedgerEntry and its leave_ledger writes (Comp Off credit,
// Uninformed Absence penalty, Lunch OT Streak penalty) were removed — this
// duplicate has no Leave Management module to hold a leave balance against.
// See CalculateAndStore.

// buildDay gathers everything the engine needs for one employee-day and runs
// it. Pure read: no writes, no ledger side effects. Shared by CalculateAndStore
// and CalculateOnly so both paths always apply identical rules.
func (r *Repository) buildDay(ctx context.Context, employeeID int64, date string) (DailyResult, error) {
	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return DailyResult{}, err
	}

	ec, err := r.getEmployeeContext(ctx, employeeID)
	if err != nil {
		return DailyResult{}, err
	}

	dayCtx := DayContext{EmployeeActiveToday: ec.isActiveOn(parsedDate), Policy: CurrentPolicySettings()}

	if ec.ShiftID != nil {
		shift, err := r.getShiftInfo(ctx, *ec.ShiftID)
		if err != nil {
			return DailyResult{}, err
		}
		dayCtx.Shift = shift.ResolvedFor(parsedDate)
	}

	var events []RawEvent
	if dayCtx.EmployeeActiveToday {
		dayCtx.IsWeeklyOff = parsedDate.Weekday() == time.Sunday
		dayCtx.IsHoliday, err = r.isHoliday(ctx, date)
		if err != nil {
			return DailyResult{}, err
		}
		dayCtx.HasApprovedLeave, err = r.hasApprovedLeave(ctx, employeeID, date)
		if err != nil {
			return DailyResult{}, err
		}
		dayCtx.IsSandwichedLeave, err = r.hasSandwichLeave(ctx, employeeID, date)
		if err != nil {
			return DailyResult{}, err
		}
		dayCtx.IsExcusedAbsence, err = r.hasAbsenceOverride(ctx, employeeID, date)
		if err != nil {
			return DailyResult{}, err
		}
		if !dayCtx.IsHoliday && !dayCtx.IsWeeklyOff && !dayCtx.HasApprovedLeave {
			dayCtx.LateStreakSoFarThisMonth, err = r.lateStreakSoFarThisMonth(ctx, employeeID, parsedDate)
			if err != nil {
				return DailyResult{}, err
			}
			dayCtx.LunchOTStreakSoFarThisMonth, err = r.lunchOTStreakSoFarThisMonth(ctx, employeeID, parsedDate)
			if err != nil {
				return DailyResult{}, err
			}
			dayCtx.PreviousWorkDayLastOut, err = r.findPreviousWorkDayLastOut(ctx, employeeID, parsedDate)
			if err != nil {
				return DailyResult{}, err
			}
		}
		events, err = r.getEvents(ctx, employeeID, date)
		if err != nil {
			return DailyResult{}, err
		}
	}

	return CalculateDay(events, dayCtx), nil
}

func toStoredResult(employeeID int64, date string, result DailyResult) StoredResult {
	return StoredResult{
		EmployeeID: employeeID, Date: date, Status: result.Status,
		FirstIn: result.FirstIn, LastOut: result.LastOut,
		WorkingMinutes: result.WorkingMinutes, LateMinutes: result.LateMinutes,
		EarlyLeaveMinutes: result.EarlyLeaveMinutes, OvertimeMinutes: result.OvertimeMinutes,
		Exceptions: result.Exceptions, CalculationVersion: result.CalculationVersion,
		CompOffEarned: result.CompOffEarned, CompOffHalfDay: result.CompOffHalfDay, IsExcusedAbsence: result.AbsenceExcused,
		CalculatedAt: time.Now(),
	}
}

// CalculateOnly runs the engine over a date range and returns the results
// without persisting anything. Used by read paths such as an employee viewing
// their own attendance: storing there would write leave-ledger rows (comp-off
// credits, absence penalties) as a side effect of merely looking at a page.
//
// Internally this is summarizeRange (SummarizeRange's internals) scoped to
// one employee with both of its shortcuts turned off — preferStoredRows and
// skipBeforeJoining — rather than a day-by-day loop of buildDay. Turning
// those off matters: verified empirically (byte-for-byte diff against the
// old per-day loop over real data) that leaving them on, as SummarizeRange's
// own callers want, would silently (a) return stale/incomplete historical
// rows instead of a fresh recalculation, and (b) omit days before the
// employee's joining date that buildDay would have computed. With both off,
// this is the same rule set and the same output as before — just a fixed
// number of queries for the whole range instead of up to ~38 per day (shift,
// holiday, leave, streaks, events, plus buildDay's own up-to-14-day lookback
// query chain).
func (r *Repository) CalculateOnly(ctx context.Context, employeeID int64, from, to string) ([]StoredResult, error) {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return nil, err
	}

	_, dailyResults, err := r.summarizeRange(ctx, from, to, &employeeID, summarizeRangeOpts{
		liveStreakCounts:              true,
		holidayNeverOverridesInactive: true,
	})
	if err != nil {
		return nil, err
	}
	results := dailyResults[employeeID]

	if len(results) == 0 {
		// employeesForRange excluded this employee from the batch entirely —
		// e.g. resigned/terminated with a last_working_day before `from` (see
		// its own WHERE clause). Rare for a self-service call (an employee
		// viewing their own attendance), but buildDay never had this
		// exclusion, so fall back to it rather than risk silently returning
		// an empty range for a real request.
		var out []StoredResult
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			date := d.Format("2006-01-02")
			result, err := r.buildDay(ctx, employeeID, date)
			if err != nil {
				return nil, err
			}
			out = append(out, toStoredResult(employeeID, date, result))
		}
		return out, nil
	}

	var out []StoredResult
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		out = append(out, toStoredResult(employeeID, date, results[date]))
	}
	return out, nil
}

// CalculateAndStore runs the engine for one employee-day, upserts the
// result, and syncs the side-effect ledger entries (compoff/penalty) that
// follow from it. date is "YYYY-MM-DD".
func (r *Repository) CalculateAndStore(ctx context.Context, employeeID int64, date string) (*StoredResult, error) {
	result, err := r.buildDay(ctx, employeeID, date)
	if err != nil {
		return nil, err
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO attendance_calculations
			(employee_id, calc_date, status, first_in, last_out, working_minutes,
			 late_minutes, early_leave_minutes, overtime_minutes, exceptions,
			 calculation_version, compoff_earned, compoff_half_day, calculated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, now())
		 ON CONFLICT (employee_id, calc_date) DO UPDATE SET
			status = EXCLUDED.status, first_in = EXCLUDED.first_in, last_out = EXCLUDED.last_out,
			working_minutes = EXCLUDED.working_minutes, late_minutes = EXCLUDED.late_minutes,
			early_leave_minutes = EXCLUDED.early_leave_minutes, overtime_minutes = EXCLUDED.overtime_minutes,
			exceptions = EXCLUDED.exceptions, calculation_version = EXCLUDED.calculation_version,
			compoff_earned = EXCLUDED.compoff_earned, compoff_half_day = EXCLUDED.compoff_half_day, calculated_at = now()`,
		employeeID, date, result.Status, result.FirstIn, result.LastOut, result.WorkingMinutes,
		result.LateMinutes, result.EarlyLeaveMinutes, result.OvertimeMinutes,
		exceptionsToStrings(result.Exceptions), result.CalculationVersion, result.CompOffEarned, result.CompOffHalfDay,
	)
	if err != nil {
		return nil, err
	}

	// No leave_ledger here (see syncAutoLedgerEntry's removal note) — this
	// duplicate has no Leave Management module to hold a balance against. The
	// day-status computation above (Late Streak's 3rd-strike Half Day,
	// Comp Off earned/half, etc.) is unaffected; only the separate
	// leave-balance deduction/credit habit (Comp Off credit, Uninformed
	// Absence penalty, Lunch OT Streak penalty) has nowhere to write to, so
	// it's simply not attempted.
	stored := toStoredResult(employeeID, date, result)
	return &stored, nil
}

// SetAbsenceOverride excuses (row inserted) or un-excuses (row deleted) one
// employee-day's absence, then immediately recalculates and re-stores that
// day — the same way approving a leave request retroactively requires a
// Recalculate to actually clear a stale penalty. Without this, the day's
// attendance_calculations row (and range_summary.go's "use the stored row if
// one exists" shortcut) would keep showing the pre-override outcome until
// something else happened to recalculate it. Who's allowed to call this is
// enforced by the route (Super Admin only), not here.
func (r *Repository) SetAbsenceOverride(ctx context.Context, employeeID int64, date, note string, setBy int64, excuse bool) (*StoredResult, error) {
	if excuse {
		if _, err := r.db.Exec(ctx,
			`INSERT INTO absence_overrides (employee_id, override_date, note, set_by)
			 VALUES ($1, $2::date, $3, $4)
			 ON CONFLICT (employee_id, override_date)
			 DO UPDATE SET note = EXCLUDED.note, set_by = EXCLUDED.set_by, set_at = now()`,
			employeeID, date, note, setBy,
		); err != nil {
			return nil, err
		}
	} else {
		if _, err := r.db.Exec(ctx,
			`DELETE FROM absence_overrides WHERE employee_id = $1 AND override_date = $2::date`,
			employeeID, date,
		); err != nil {
			return nil, err
		}
	}
	return r.CalculateAndStore(ctx, employeeID, date)
}

func exceptionsToStrings(exceptions []ExceptionCode) []string {
	out := make([]string, len(exceptions))
	for i, e := range exceptions {
		out[i] = string(e)
	}
	return out
}

// GetStored returns whatever calculations already exist for an employee in
// [from, to] — it does not calculate anything.
func (r *Repository) GetStored(ctx context.Context, employeeID int64, from, to string) ([]StoredResult, error) {
	rows, err := r.db.Query(ctx,
		`SELECT employee_id, calc_date::text, status, first_in, last_out, working_minutes,
			late_minutes, early_leave_minutes, overtime_minutes, exceptions, calculation_version,
			compoff_earned, compoff_half_day, calculated_at
		 FROM attendance_calculations
		 WHERE employee_id = $1 AND calc_date BETWEEN $2::date AND $3::date
		 ORDER BY calc_date ASC`,
		employeeID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StoredResult
	for rows.Next() {
		var s StoredResult
		var exceptions []string
		if err := rows.Scan(&s.EmployeeID, &s.Date, &s.Status, &s.FirstIn, &s.LastOut, &s.WorkingMinutes,
			&s.LateMinutes, &s.EarlyLeaveMinutes, &s.OvertimeMinutes, &exceptions, &s.CalculationVersion,
			&s.CompOffEarned, &s.CompOffHalfDay, &s.CalculatedAt); err != nil {
			return nil, err
		}
		s.Exceptions = make([]ExceptionCode, len(exceptions))
		for i, e := range exceptions {
			s.Exceptions[i] = ExceptionCode(e)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
