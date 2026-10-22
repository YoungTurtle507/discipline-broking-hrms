package leave

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("leave request not found")
var ErrAlreadyDecided = errors.New("leave request has already been decided")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// requestColumns is the standard column list for a Request row, joined
// against employees/users for display names — kept as one constant so
// Create/List/GetByID can never quietly drift apart on which columns they read.
const requestColumns = `lr.id, lr.employee_id, e.full_name, lr.leave_type, lr.start_date::text, lr.end_date::text,
	lr.reason, lr.status, lr.requested_at, lr.decided_by, du.full_name, lr.decided_at, lr.document_path`

const requestJoins = `FROM leave_requests lr
	JOIN employees e ON e.id = lr.employee_id
	LEFT JOIN users du ON du.id = lr.decided_by`

func scanRequest(row pgx.Row) (Request, error) {
	var req Request
	err := row.Scan(&req.ID, &req.EmployeeID, &req.EmployeeName, &req.LeaveType, &req.StartDate, &req.EndDate,
		&req.Reason, &req.Status, &req.RequestedAt, &req.DecidedBy, &req.DecidedByName, &req.DecidedAt, &req.DocumentPath)
	req.HasDocument = req.DocumentPath != nil
	return req, err
}

func (r *Repository) Create(ctx context.Context, employeeID int64, leaveType, startDate, endDate, reason string) (*Request, error) {
	var id int64
	err := r.db.QueryRow(ctx,
		`INSERT INTO leave_requests (employee_id, leave_type, start_date, end_date, reason, document_required)
		 VALUES ($1, $2, $3::date, $4::date, $5, false)
		 RETURNING id`,
		employeeID, leaveType, startDate, endDate, reason,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) List(ctx context.Context, employeeID *int64, status string) ([]Request, error) {
	query := `SELECT ` + requestColumns + ` ` + requestJoins + ` WHERE 1=1`
	args := []any{}
	argN := 1
	if employeeID != nil {
		query += argClause(" AND lr.employee_id = $", argN)
		args = append(args, *employeeID)
		argN++
	}
	if status != "" {
		query += argClause(" AND lr.status = $", argN)
		args = append(args, status)
		argN++
	}
	query += " ORDER BY lr.requested_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Request{}
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*Request, error) {
	req, err := scanRequest(r.db.QueryRow(ctx, `SELECT `+requestColumns+` `+requestJoins+` WHERE lr.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &req, nil
}

// SetDocumentPath records where an uploaded supporting document was saved.
func (r *Repository) SetDocumentPath(ctx context.Context, requestID int64, path string) error {
	tag, err := r.db.Exec(ctx, `UPDATE leave_requests SET document_path = $2 WHERE id = $1`, requestID, path)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func argClause(prefix string, n int) string {
	digits := "0123456789"
	s := prefix
	if n < 10 {
		s += string(digits[n])
	} else {
		s += string(digits[n/10]) + string(digits[n%10])
	}
	return s
}

// Decide approves or rejects a pending request. Approving writes one ledger
// deduction per day in the range for PAID/COMPOFF leave (UNPAID draws down
// nothing but is still logged the same way for a consistent audit trail) —
// the calculation engine itself only checks "is there an approved request
// covering this date" (see calculation.Repository.hasApprovedLeave), it
// never reads the ledger; this just keeps leave_ledger consistent with the
// schema's own intent for whenever balance reporting is built.
func (r *Repository) Decide(ctx context.Context, requestID int64, decision string, decidedBy int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var req Request
	err = tx.QueryRow(ctx,
		`SELECT id, employee_id, leave_type, start_date::text, end_date::text, status FROM leave_requests WHERE id = $1 FOR UPDATE`,
		requestID,
	).Scan(&req.ID, &req.EmployeeID, &req.LeaveType, &req.StartDate, &req.EndDate, &req.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if req.Status != "PENDING" {
		return ErrAlreadyDecided
	}

	if _, err := tx.Exec(ctx,
		`UPDATE leave_requests SET status = $2, decided_by = $3, decided_at = now() WHERE id = $1`,
		requestID, decision, decidedBy,
	); err != nil {
		return err
	}

	if decision == "APPROVED" {
		start, err := time.Parse("2006-01-02", req.StartDate)
		if err != nil {
			return err
		}
		end, err := time.Parse("2006-01-02", req.EndDate)
		if err != nil {
			return err
		}
		reason := "APPROVED_LEAVE"
		if req.LeaveType == "COMPOFF" {
			reason = "COMPOFF_USED"
		}
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			if _, err := tx.Exec(ctx,
				`INSERT INTO leave_ledger (employee_id, leave_type, entry_date, amount, reason, related_leave_request_id)
				 VALUES ($1, $2, $3::date, -1, $4, $5)
				 ON CONFLICT (employee_id, leave_type, entry_date, reason) DO NOTHING`,
				req.EmployeeID, req.LeaveType, d.Format("2006-01-02"), reason, requestID,
			); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}
