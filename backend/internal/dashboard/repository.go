package dashboard

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"hr-management/backend/internal/calculation"
	"hr-management/backend/internal/clock"
	"hr-management/backend/internal/employee"
)

// Summary is the dashboard's headline figures. It is the calculation engine's
// own day summary, not a separate computation: the non-negotiable architecture
// rule is that everything downstream is a view over the engine's output, and
// this used to break it by reading raw events directly.
type Summary = calculation.DaySummary

type TrendPoint struct {
	Date       string `json:"date"`
	Present    int    `json:"present"`
	LateComing int    `json:"late_coming"`
	HalfDay    int    `json:"half_day"`
}

type Holiday struct {
	Date string `json:"date"`
	Name string `json:"name"`
}

type Birthday struct {
	EmployeeID     int64  `json:"employee_id"`
	FullName       string `json:"full_name"`
	DepartmentName string `json:"department_name,omitempty"`
	DateOfBirth    string `json:"date_of_birth"`
	// DaysUntil is 0 today, 1 tomorrow, 2 the day after — the only values this
	// ever returns, since the query below only looks that far ahead.
	DaysUntil int `json:"days_until"`
}

type Repository struct {
	db   *pgxpool.Pool
	calc *calculation.Repository
	emp  *employee.Repository
}

func NewRepository(db *pgxpool.Pool, calc *calculation.Repository, emp *employee.Repository) *Repository {
	return &Repository{db: db, calc: calc, emp: emp}
}

// DayDetailEntry is one employee's engine-computed result for a single date —
// the per-employee counterpart to Summary's aggregate counts, used to drill a
// dashboard stat card (e.g. "12 absent") down into who those people are.
type DayDetailEntry struct {
	EmployeeID        int64      `json:"employee_id"`
	EmployeeCode      string     `json:"employee_code"`
	FullName          string     `json:"full_name"`
	DepartmentName    *string    `json:"department_name"`
	DesignationTitle  *string    `json:"designation_title"`
	Status            string     `json:"status"`
	FirstIn           *time.Time `json:"first_in"`
	LastOut           *time.Time `json:"last_out"`
	WorkingMinutes    int        `json:"working_minutes"`
	LateMinutes       int        `json:"late_minutes"`
	EarlyLeaveMinutes int        `json:"early_leave_minutes"`
}

// DayDetail lists every employee whose computed status on date is in wanted
// (or every employee, if wanted is nil) — the same ListDate the dashboard
// summary aggregates, just returned per-employee instead of counted.
func (r *Repository) DayDetail(ctx context.Context, date string, wanted map[calculation.DayStatus]bool) ([]DayDetailEntry, error) {
	results, err := r.calc.ListDate(ctx, date)
	if err != nil {
		return nil, err
	}

	employees, err := r.emp.List(ctx, employee.ListFilters{})
	if err != nil {
		return nil, err
	}

	out := make([]DayDetailEntry, 0, len(results))
	for _, e := range employees {
		result, ok := results[e.ID]
		if !ok || (wanted != nil && !wanted[result.Status]) {
			continue
		}
		// EarlyLeaveMinutes is informational at the engine level (see
		// engine.go) and stays populated even from a trailing unpaired IN —
		// e.g. someone back at their desk after lunch, most recently scanned
		// IN, not actually gone for the day. The "Early Leave Today" card
		// means people who have genuinely left, so it's withheld here unless
		// LastOut is set (only true once the most recent scan is a real OUT).
		earlyLeaveMinutes := result.EarlyLeaveMinutes
		if result.LastOut == nil {
			earlyLeaveMinutes = 0
		}
		out = append(out, DayDetailEntry{
			EmployeeID:        e.ID,
			EmployeeCode:      e.EmployeeCode,
			FullName:          e.FullName,
			DepartmentName:    e.DepartmentName,
			DesignationTitle:  e.DesignationTitle,
			Status:            string(result.Status),
			FirstIn:           result.FirstIn,
			LastOut:           result.LastOut,
			WorkingMinutes:    result.WorkingMinutes,
			LateMinutes:       result.LateMinutes,
			EarlyLeaveMinutes: earlyLeaveMinutes,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].FullName < out[j].FullName })
	return out, nil
}

// Summary reports today through the calculation engine, so the dashboard, an
// employee's own attendance page and any future report all agree by
// construction. Nothing is persisted — this is a read.
func (r *Repository) Summary(ctx context.Context) (*Summary, error) {
	return r.calc.SummarizeDate(ctx, clock.Now().Format("2006-01-02"))
}

// Trend returns the last 7 days of Present/Late Coming/Half Day counts, one
// point per day, straight from the same engine-backed SummarizeDate every
// other attendance figure in the app reads — so this chart can never
// disagree with what the dashboard's own stat cards showed on those days.
func (r *Repository) Trend(ctx context.Context) ([]TrendPoint, error) {
	out := make([]TrendPoint, 0, 7)
	for i := 6; i >= 0; i-- {
		date := clock.Now().AddDate(0, 0, -i).Format("2006-01-02")
		summary, err := r.calc.SummarizeDate(ctx, date)
		if err != nil {
			return nil, err
		}
		out = append(out, TrendPoint{
			Date:       date,
			Present:    summary.Present,
			LateComing: summary.Late,
			HalfDay:    summary.HalfDay,
		})
	}
	return out, nil
}

// UpcomingHolidays lists the next N company holidays from today onward, read
// straight from the same table the calculation engine checks for HOLIDAY
// status — so this list and a day's actual status can never disagree.
func (r *Repository) UpcomingHolidays(ctx context.Context, limit int) ([]Holiday, error) {
	rows, err := r.db.Query(ctx, `
		SELECT holiday_date::text, name
		FROM holidays
		WHERE holiday_date >= CURRENT_DATE
		ORDER BY holiday_date
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Holiday{}
	for rows.Next() {
		var h Holiday
		if err := rows.Scan(&h.Date, &h.Name); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// UpcomingBirthdays returns active employees whose next birthday falls today,
// tomorrow, or the day after — computed fresh on every read rather than a
// stored reminder, the same lazy-sweep philosophy employee.Repository's
// resignExpiredNotices already uses: always correct for "today" by
// construction, with no scheduler needed to keep it that way.
func (r *Repository) UpcomingBirthdays(ctx context.Context) ([]Birthday, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.id, e.full_name, d.name, e.date_of_birth::text
		FROM employees e
		LEFT JOIN departments d ON d.id = e.department_id
		WHERE e.date_of_birth IS NOT NULL
		  AND e.status NOT IN ('RESIGNED', 'TERMINATED')
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := clock.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	out := []Birthday{}
	for rows.Next() {
		var b Birthday
		var deptName *string
		if err := rows.Scan(&b.EmployeeID, &b.FullName, &deptName, &b.DateOfBirth); err != nil {
			return nil, err
		}
		if deptName != nil {
			b.DepartmentName = *deptName
		}
		born, err := time.Parse("2006-01-02", b.DateOfBirth)
		if err != nil {
			continue
		}
		next := nextBirthdayOccurrence(today, born.Month(), born.Day())
		days := int(next.Sub(today).Hours() / 24)
		if days > 2 {
			continue
		}
		b.DaysUntil = days
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DaysUntil < out[j].DaysUntil })
	return out, rows.Err()
}

// nextBirthdayOccurrence returns the next date on or after today that falls
// on month/day — this year's if it hasn't passed yet, next year's otherwise.
// A Feb 29 birthday clamps to Feb 28 in a year that isn't a leap year rather
// than landing on a date that doesn't exist.
func nextBirthdayOccurrence(today time.Time, month time.Month, day int) time.Time {
	occ := birthdayInYear(today.Year(), month, day, today.Location())
	if occ.Before(today) {
		occ = birthdayInYear(today.Year()+1, month, day, today.Location())
	}
	return occ
}

func birthdayInYear(year int, month time.Month, day int, loc *time.Location) time.Time {
	if month == time.February && day == 29 && !isLeapYear(year) {
		day = 28
	}
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}
