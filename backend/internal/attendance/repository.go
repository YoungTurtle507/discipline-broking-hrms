package attendance

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hr-management/backend/internal/clock"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// note is the free-text detail alongside outReason (e.g. "Lunch" or
// "Doctor's appointment") — blank unless outReason is set. eventTime is
// supplied by the caller (rather than defaulted to clock.Now() here) so it
// can be the exact same instant already burned into the capture photo's
// stamp — see photo.go. photoPath is the stamped capture photo's path
// relative to STORAGE_PATH, empty for a source with no photo.
func (r *Repository) Create(ctx context.Context, employeeID int64, direction string, eventTime time.Time, confidence float64, modelName, modelVersion string, verifiedBy *int64, outReason *string, note, photoPath string) (*Event, error) {
	var e Event
	err := r.db.QueryRow(ctx,
		`INSERT INTO attendance_events (employee_id, direction, event_time, confidence, model_name, model_version, verified_by, source, out_reason, note, photo_path)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, employee_id, direction, event_time, confidence, model_name, model_version, verified_by, source, note, out_reason, photo_path`,
		employeeID, direction, eventTime, confidence, modelName, modelVersion, verifiedBy, SourcePhotoCapture, outReason, note, photoPath,
	).Scan(&e.ID, &e.EmployeeID, &e.Direction, &e.EventTime, &e.Confidence, &e.ModelName, &e.ModelVersion, &e.VerifiedBy, &e.Source, &e.Note, &e.OutReason, &e.PhotoPath)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRow(ctx, `SELECT full_name FROM employees WHERE id = $1`, employeeID).Scan(&e.EmployeeName)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// PhotoPath looks up one event's stamped capture photo path (nil if it has
// none) — used to serve it back without loading the whole Event.
func (r *Repository) PhotoPath(ctx context.Context, eventID int64) (*string, error) {
	var path *string
	err := r.db.QueryRow(ctx, `SELECT photo_path FROM attendance_events WHERE id = $1`, eventID).Scan(&path)
	return path, err
}

// LogAttempt records every verification attempt during attendance marking —
// matched or not — as an audit trail independent of confirmed events.
func (r *Repository) LogAttempt(ctx context.Context, employeeID int64, direction string, matched bool, confidence float64, reason string, attemptedBy *int64) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO attendance_verification_attempts (employee_id, direction, matched, confidence, reason, attempted_by)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		employeeID, direction, matched, confidence, reason, attemptedBy,
	)
	return err
}

// NextDirection looks at an employee's most recent event today and returns
// the direction that should come next — IN if they haven't checked in yet
// (or their last event was OUT), otherwise OUT — plus that last event's time
// (nil if there wasn't one), so callers can enforce a minimum gap between it
// and the next scan. This is the single source of truth for direction so the
// manual and auto-detect flows can't disagree.
func (r *Repository) NextDirection(ctx context.Context, employeeID int64) (string, *time.Time, error) {
	var last string
	var lastTime time.Time
	err := r.db.QueryRow(ctx,
		`SELECT direction, event_time FROM attendance_events
		 WHERE employee_id = $1 AND event_time::date = $2::date
		 ORDER BY event_time DESC LIMIT 1`,
		employeeID, clock.Now().Format("2006-01-02"),
	).Scan(&last, &lastTime)
	if errors.Is(err, pgx.ErrNoRows) {
		return "IN", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	if last == "IN" {
		return "OUT", &lastTime, nil
	}
	return "IN", &lastTime, nil
}

// ListToday returns every confirmed attendance event from today, most recent
// first — used both for the day's log and for suggesting an employee's next
// direction (their last event today).
func (r *Repository) ListToday(ctx context.Context) ([]Event, error) {
	rows, err := r.db.Query(ctx,
		`SELECT ae.id, ae.employee_id, e.full_name, ae.direction, ae.event_time, ae.confidence, ae.model_name, ae.model_version, ae.verified_by, ae.out_reason, ae.note, ae.photo_path
		 FROM attendance_events ae
		 JOIN employees e ON e.id = ae.employee_id
		 WHERE ae.event_time::date = $1::date
		 ORDER BY ae.event_time DESC`,
		clock.Now().Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.EmployeeID, &e.EmployeeName, &e.Direction, &e.EventTime, &e.Confidence, &e.ModelName, &e.ModelVersion, &e.VerifiedBy, &e.OutReason, &e.Note, &e.PhotoPath); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// RecordManual adds an attendance event an admin entered by hand, typically a
// check-out the employee forgot to scan. It is an additive correction: the
// original events are untouched, this is simply another event in the day's
// sequence, tagged MANUAL with who entered it and why. The calculation engine
// reads it like any other event, so the day recalculates on its own.
//
// eventTime is supplied by the admin rather than defaulted to now(), because
// the point is to record when the person actually left.
func (r *Repository) RecordManual(ctx context.Context, employeeID int64, direction string, eventTime time.Time, note string, enteredBy int64, reason *string) (*Event, error) {
	var e Event
	err := r.db.QueryRow(ctx,
		`INSERT INTO attendance_events
			(employee_id, direction, event_time, confidence, model_name, model_version, verified_by, source, note, out_reason)
		 VALUES ($1, $2, $3, 0, 'manual-entry', '-', $4, 'MANUAL', $5, $6)
		 RETURNING id, employee_id, direction, event_time, confidence, model_name, model_version, verified_by, source, note, out_reason`,
		employeeID, direction, eventTime, enteredBy, note, reason,
	).Scan(&e.ID, &e.EmployeeID, &e.Direction, &e.EventTime, &e.Confidence, &e.ModelName, &e.ModelVersion,
		&e.VerifiedBy, &e.Source, &e.Note, &e.OutReason)
	if err != nil {
		return nil, err
	}

	if err := r.db.QueryRow(ctx, `SELECT full_name FROM employees WHERE id = $1`, employeeID).Scan(&e.EmployeeName); err != nil {
		return nil, err
	}
	return &e, nil
}

// NeighboringEvents returns the employee's event immediately before and
// immediately after t (nil for either if there isn't one) — everything a
// manual entry needs to check that inserting a new event at t still leaves
// IN/OUT strictly alternating, wherever in the employee's history t falls.
func (r *Repository) NeighboringEvents(ctx context.Context, employeeID int64, t time.Time) (prev, next *Event, err error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, employee_id, direction, event_time, confidence, model_name, model_version, verified_by, source, note, out_reason
		 FROM attendance_events WHERE employee_id = $1 AND event_time < $2 ORDER BY event_time DESC LIMIT 1`,
		employeeID, t,
	)
	var p Event
	if scanErr := row.Scan(&p.ID, &p.EmployeeID, &p.Direction, &p.EventTime, &p.Confidence, &p.ModelName, &p.ModelVersion,
		&p.VerifiedBy, &p.Source, &p.Note, &p.OutReason); scanErr == nil {
		prev = &p
	} else if !errors.Is(scanErr, pgx.ErrNoRows) {
		return nil, nil, scanErr
	}

	row = r.db.QueryRow(ctx,
		`SELECT id, employee_id, direction, event_time, confidence, model_name, model_version, verified_by, source, note, out_reason
		 FROM attendance_events WHERE employee_id = $1 AND event_time > $2 ORDER BY event_time ASC LIMIT 1`,
		employeeID, t,
	)
	var n Event
	if scanErr := row.Scan(&n.ID, &n.EmployeeID, &n.Direction, &n.EventTime, &n.Confidence, &n.ModelName, &n.ModelVersion,
		&n.VerifiedBy, &n.Source, &n.Note, &n.OutReason); scanErr == nil {
		next = &n
	} else if !errors.Is(scanErr, pgx.ErrNoRows) {
		return nil, nil, scanErr
	}

	return prev, next, nil
}

// EventsInRange is the Reports page's data source — every event in
// [from, to] (inclusive), optionally narrowed to one employee, newest
// first. employeeID nil means every employee. Powers both the attendance
// report (IN/OUT history) and the captured-photos viewer, which are the
// same underlying data shown two different ways.
func (r *Repository) EventsInRange(ctx context.Context, employeeID *int64, from, to string) ([]Event, error) {
	rows, err := r.db.Query(ctx,
		`SELECT ae.id, ae.employee_id, e.full_name, ae.direction, ae.event_time, ae.confidence,
		        ae.model_name, ae.model_version, ae.verified_by, ae.source, ae.note, ae.out_reason, ae.photo_path
		 FROM attendance_events ae
		 JOIN employees e ON e.id = ae.employee_id
		 WHERE ae.event_time::date BETWEEN $1::date AND $2::date
		   AND ($3::bigint IS NULL OR ae.employee_id = $3)
		 ORDER BY ae.event_time DESC`,
		from, to, employeeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.EmployeeID, &e.EmployeeName, &e.Direction, &e.EventTime, &e.Confidence,
			&e.ModelName, &e.ModelVersion, &e.VerifiedBy, &e.Source, &e.Note, &e.OutReason, &e.PhotoPath); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// EventsOn returns an employee's events for one date, so a caller can show the
// admin what's already recorded before they add anything.
func (r *Repository) EventsOn(ctx context.Context, employeeID int64, date string) ([]Event, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, employee_id, direction, event_time, confidence, model_name, model_version,
		        verified_by, source, note, out_reason
		 FROM attendance_events
		 WHERE employee_id = $1 AND event_time::date = $2::date
		 ORDER BY event_time ASC`,
		employeeID, date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.EmployeeID, &e.Direction, &e.EventTime, &e.Confidence,
			&e.ModelName, &e.ModelVersion, &e.VerifiedBy, &e.Source, &e.Note, &e.OutReason); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// A day with a dangling scan-IN and no scan-OUT used to be auto-closed with a
// synthetic OUT at shift end (Repository.AutoCloseMissingOuts, lazy-swept on
// the notifications poll). That mechanism has been removed — such a day now
// stays flagged MISSING_OUT (calculation.ExMissingOut) until HR adds the
// missed scan by hand through the existing manual-entry flow.
