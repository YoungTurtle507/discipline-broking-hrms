package calculation

import (
	"context"
	"time"
)

// DaySummary is the whole-company picture for one date, produced by the same
// engine that produces each employee's own day — so the dashboard can never
// disagree with what an employee sees on their attendance page.
type DaySummary struct {
	Date           string `json:"date"`
	TotalEmployees int    `json:"total_employees"`

	// AtWork is everyone who actually turned up — a full day or a half day.
	// Someone who scanned in at 11:00 is at work, whatever the day is worth to
	// them, so this is what the dashboard's Present figure reports. Present
	// below stays the strict full-day count for anywhere that needs it.
	AtWork          int `json:"at_work"`
	Present         int `json:"present"`
	Absent          int `json:"absent"`
	HalfDay         int `json:"half_day"`
	OnLeave         int `json:"on_leave"`
	Holiday         int `json:"holiday"`
	WeeklyOff       int `json:"weekly_off"`
	Late            int `json:"late"`
	EarlyLeave      int `json:"early_leave"`
	MissingOut      int `json:"missing_out"`
	Overtime        int `json:"overtime"`
	WorkingMinutes  int `json:"working_minutes"`
	OvertimeMinutes int `json:"overtime_minutes"`
}

// lookbackDays matches findPreviousWorkDayLastOut's window: how far back the
// stay-late pass can have been earned.
const lookbackDays = 14

type employeeRow struct {
	id             int64
	status         string
	lastWorkingDay *time.Time
	shift          *ShiftInfo
}

// SummarizeDate runs the engine for every employee on one date.
//
// Deliberately batched: calling CalculateAndStore (or buildDay) per employee
// would issue six or more queries each, so a 500-employee dashboard would fire
// thousands per page load. Everything the engine needs is fetched in a fixed
// number of queries and the pure CalculateDay function is then run in memory,
// so cost is flat in the number of employees.
//
// Nothing is written: the dashboard is a read, and storing here would create
// leave-ledger rows as a side effect of someone looking at a page.
func (r *Repository) SummarizeDate(ctx context.Context, date string) (*DaySummary, error) {
	results, err := r.ListDate(ctx, date)
	if err != nil {
		return nil, err
	}
	summary := &DaySummary{Date: date, TotalEmployees: len(results)}
	for _, result := range results {
		summary.add(result)
	}
	return summary, nil
}

// ListDate runs the engine for every employee on one date and returns each
// employee's own result, keyed by employee id — the per-employee counterpart
// to SummarizeDate's aggregate, used to drill a dashboard stat (e.g.
// "12 absent") down into who those 12 people actually are.
func (r *Repository) ListDate(ctx context.Context, date string) (map[int64]DailyResult, error) {
	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, err
	}

	employees, err := r.activeEmployeesWithShifts(ctx)
	if err != nil {
		return nil, err
	}

	isHoliday, err := r.isHoliday(ctx, date)
	if err != nil {
		return nil, err
	}

	eventsByEmployee, err := r.eventsForDate(ctx, date)
	if err != nil {
		return nil, err
	}

	onLeave, err := r.employeesOnApprovedLeave(ctx, date)
	if err != nil {
		return nil, err
	}

	sandwichLeave, err := r.employeesOnSandwichLeave(ctx, date)
	if err != nil {
		return nil, err
	}

	absenceOverrides, err := r.employeesWithAbsenceOverride(ctx, date)
	if err != nil {
		return nil, err
	}

	lateStreaks, err := r.lateStreaksThisMonth(ctx, parsedDate)
	if err != nil {
		return nil, err
	}

	lunchOTStreaks, err := r.lunchOTStreaksThisMonth(ctx, parsedDate)
	if err != nil {
		return nil, err
	}

	lastOutByDay, err := r.lastEventPerDay(ctx, parsedDate)
	if err != nil {
		return nil, err
	}

	// Which of the preceding days were scheduled working days, resolved once
	// rather than per employee.
	previousWorkDays, err := r.previousWorkDays(ctx, parsedDate)
	if err != nil {
		return nil, err
	}

	isWeeklyOff := parsedDate.Weekday() == time.Sunday
	out := make(map[int64]DailyResult, len(employees))
	policy := CurrentPolicySettings()

	for _, e := range employees {
		ctxDay := DayContext{
			EmployeeActiveToday: (&employeeContext{
				Status:         e.status,
				LastWorkingDay: e.lastWorkingDay,
			}).isActiveOn(parsedDate),
			Shift:  e.shift.ResolvedFor(parsedDate),
			Policy: policy,
		}

		var events []RawEvent
		if ctxDay.EmployeeActiveToday {
			ctxDay.IsHoliday = isHoliday
			ctxDay.IsWeeklyOff = isWeeklyOff
			ctxDay.HasApprovedLeave = onLeave[e.id]
			ctxDay.IsSandwichedLeave = sandwichLeave[e.id]
			ctxDay.IsExcusedAbsence = absenceOverrides[e.id]
			if !ctxDay.IsHoliday && !ctxDay.IsWeeklyOff && !ctxDay.HasApprovedLeave {
				ctxDay.LateStreakSoFarThisMonth = lateStreaks[e.id]
				ctxDay.LunchOTStreakSoFarThisMonth = lunchOTStreaks[e.id]
				ctxDay.PreviousWorkDayLastOut = previousLastOut(previousWorkDays, lastOutByDay, e.id)
			}
			events = eventsByEmployee[e.id]
		}

		out[e.id] = CalculateDay(events, ctxDay)
	}

	return out, nil
}

func (s *DaySummary) add(r DailyResult) {
	switch r.Status {
	case StatusPresent:
		s.Present++
		s.AtWork++
	case StatusAbsent:
		s.Absent++
	case StatusHalfDay:
		s.HalfDay++
		s.AtWork++
	case StatusOnLeave:
		s.OnLeave++
	case StatusHoliday:
		s.Holiday++
	case StatusWeeklyOff:
		s.WeeklyOff++
	}
	if r.LateMinutes > 0 {
		s.Late++
	}
	// EarlyLeaveMinutes stays populated from a trailing unpaired IN (e.g. back
	// from lunch, not actually gone for the day) — see engine.go. This count
	// feeds the "Early Leave Today" card, which means people who have
	// genuinely left, so it requires a real LastOut too.
	if r.EarlyLeaveMinutes > 0 && r.LastOut != nil {
		s.EarlyLeave++
	}
	if r.OvertimeMinutes > 0 {
		s.Overtime++
	}
	for _, e := range r.Exceptions {
		if e == ExMissingOut {
			s.MissingOut++
			break
		}
	}
	s.WorkingMinutes += r.WorkingMinutes
	s.OvertimeMinutes += r.OvertimeMinutes
}

// previousLastOut finds the most recent scheduled working day before the date
// being calculated and returns that employee's last OUT scan on it, mirroring
// findPreviousWorkDayLastOut but against pre-fetched data.
func previousLastOut(workDays []string, lastOutByDay map[int64]map[string]time.Time, employeeID int64) *time.Time {
	byDay, ok := lastOutByDay[employeeID]
	if !ok {
		return nil
	}
	for _, day := range workDays {
		if t, ok := byDay[day]; ok {
			return &t
		}
		// The most recent working day had no events at all — nothing to have
		// stayed late for, same as the per-employee path concludes.
		return nil
	}
	return nil
}

func (r *Repository) activeEmployeesWithShifts(ctx context.Context) ([]employeeRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT e.id, e.status, e.last_working_day,
		        s.name, s.start_time, s.end_time, s.grace_period_minutes,
		        s.saturday_start_time, s.saturday_end_time, s.saturday_grace_period_minutes
		 FROM employees e
		 LEFT JOIN shifts s ON s.id = e.shift_id
		 WHERE e.status NOT IN ('RESIGNED', 'TERMINATED')`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []employeeRow
	for rows.Next() {
		var e employeeRow
		var name *string
		var start, end *time.Time
		var grace *int
		var satStart, satEnd *time.Time
		var satGrace *int
		if err := rows.Scan(&e.id, &e.status, &e.lastWorkingDay, &name, &start, &end, &grace,
			&satStart, &satEnd, &satGrace); err != nil {
			return nil, err
		}
		if start != nil && end != nil && grace != nil {
			e.shift = &ShiftInfo{
				Name:                       *name,
				StartTime:                  *start,
				EndTime:                    *end,
				GracePeriodMinutes:         *grace,
				SaturdayStartTime:          satStart,
				SaturdayEndTime:            satEnd,
				SaturdayGracePeriodMinutes: satGrace,
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) eventsForDate(ctx context.Context, date string) (map[int64][]RawEvent, error) {
	rows, err := r.db.Query(ctx,
		// Sargable range instead of event_time::date = $1 — see getEvents in
		// repository.go for why.
		`SELECT employee_id, direction, event_time, confidence, source, COALESCE(out_reason::text, '')
		 FROM attendance_events
		 WHERE event_time >= $1::date AND event_time < $1::date + interval '1 day'
		 ORDER BY employee_id, event_time ASC`,
		date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64][]RawEvent{}
	for rows.Next() {
		var employeeID int64
		var e RawEvent
		if err := rows.Scan(&employeeID, &e.Direction, &e.EventTime, &e.Confidence, &e.Source, &e.OutReason); err != nil {
			return nil, err
		}
		out[employeeID] = append(out[employeeID], e)
	}
	return out, rows.Err()
}

// employeesOnApprovedLeave is the batch (all-employees, one date) form of
// hasApprovedLeave — see leave_requests.
func (r *Repository) employeesOnApprovedLeave(ctx context.Context, date string) (map[int64]bool, error) {
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT employee_id FROM leave_requests WHERE status = 'APPROVED' AND $1::date BETWEEN start_date AND end_date`,
		date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// employeesOnSandwichLeave always returns empty — the Leave module doesn't
// implement sandwich-leave yet. See employeesOnApprovedLeave.
func (r *Repository) employeesOnSandwichLeave(ctx context.Context, date string) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}

// employeesWithAbsenceOverride is the batch (all-employees, one date) form of
// hasAbsenceOverride — see absence_overrides.
func (r *Repository) employeesWithAbsenceOverride(ctx context.Context, date string) (map[int64]bool, error) {
	rows, err := r.db.Query(ctx, `SELECT employee_id FROM absence_overrides WHERE override_date = $1::date`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func (r *Repository) lateStreaksThisMonth(ctx context.Context, date time.Time) (map[int64]int, error) {
	monthStart := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	rows, err := r.db.Query(ctx,
		`SELECT employee_id, COUNT(*) FROM attendance_calculations
		 WHERE calc_date >= $1::date AND calc_date < $2::date
		   AND exceptions && ARRAY['LATE_STREAK_COUNTED','LATE_STREAK_HALF_DAY']::text[]
		 GROUP BY employee_id`,
		monthStart.Format("2006-01-02"), date.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]int{}
	for rows.Next() {
		var id int64
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		out[id] = count
	}
	return out, rows.Err()
}

// lunchOTStreaksThisMonth mirrors lateStreaksThisMonth for Lunch OT's own
// monthly strike count.
func (r *Repository) lunchOTStreaksThisMonth(ctx context.Context, date time.Time) (map[int64]int, error) {
	monthStart := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	rows, err := r.db.Query(ctx,
		`SELECT employee_id, COUNT(*) FROM attendance_calculations
		 WHERE calc_date >= $1::date AND calc_date < $2::date
		   AND exceptions && ARRAY['LUNCH_OT_STREAK_COUNTED','LUNCH_OT_STREAK_PENALTY']::text[]
		 GROUP BY employee_id`,
		monthStart.Format("2006-01-02"), date.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]int{}
	for rows.Next() {
		var id int64
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		out[id] = count
	}
	return out, rows.Err()
}

// lastEventPerDay returns each employee's last OUT scan on each of the
// preceding lookbackDays days, keyed by date, so the stay-late pass can be
// resolved without a query per employee. Only an OUT counts — see
// findPreviousWorkDayLastOut for why.
func (r *Repository) lastEventPerDay(ctx context.Context, date time.Time) (map[int64]map[string]time.Time, error) {
	from := date.AddDate(0, 0, -lookbackDays).Format("2006-01-02")
	rows, err := r.db.Query(ctx,
		// WHERE is rewritten sargable (plain event_time range, no cast) so it
		// can use idx_attendance_events_time — see getEvents in
		// repository.go for why the cast form can't. SELECT/GROUP BY still
		// need the ::date cast since the actual day bucket is the point of
		// this query; only row-filtering needed to change.
		`SELECT employee_id, event_time::date::text, MAX(event_time)
		 FROM attendance_events
		 WHERE event_time >= $1::date AND event_time < $2::date
		   AND direction = 'OUT'
		 GROUP BY employee_id, event_time::date`,
		from, date.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]map[string]time.Time{}
	for rows.Next() {
		var id int64
		var day string
		var last time.Time
		if err := rows.Scan(&id, &day, &last); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[string]time.Time{}
		}
		out[id][day] = last
	}
	return out, rows.Err()
}

// previousWorkDays lists the scheduled working days before date, most recent
// first: not a Sunday, not a holiday.
func (r *Repository) previousWorkDays(ctx context.Context, date time.Time) ([]string, error) {
	from := date.AddDate(0, 0, -lookbackDays)
	rows, err := r.db.Query(ctx,
		`SELECT holiday_date::text FROM holidays
		 WHERE holiday_date >= $1::date AND holiday_date < $2::date`,
		from.Format("2006-01-02"), date.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	holidays := map[string]bool{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		holidays[d] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []string
	for i := 1; i <= lookbackDays; i++ {
		candidate := date.AddDate(0, 0, -i)
		if candidate.Weekday() == time.Sunday {
			continue
		}
		day := candidate.Format("2006-01-02")
		if holidays[day] {
			continue
		}
		out = append(out, day)
	}
	return out, nil
}
