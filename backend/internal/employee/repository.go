package employee

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound                      = errors.New("employee not found")
	ErrInvalidInput                  = errors.New("invalid input")
	ErrLoginExists                   = errors.New("employee already has a login account")
	ErrDesignationNotFound           = errors.New("designation not found")
	ErrDesignationDepartmentMismatch = errors.New("designation does not belong to the selected department")
	ErrShiftNotFound                 = errors.New("shift not found")
)

const uniqueViolation = "23505"

const selectColumns = `
	e.id, e.employee_code, e.full_name, e.email, e.phone, e.photo_url,
	e.joining_date::text, e.department_id, d.name, e.designation_id, des.title,
	e.position_id, pos.title,
	e.shift_id, s.name, e.reporting_manager_id, m.full_name, e.status,
	e.last_working_day::text, e.created_at, e.updated_at,
	e.user_id, u.username,
	EXISTS(SELECT 1 FROM face_registrations fr WHERE fr.employee_id = e.id AND fr.is_active = true),
	e.remote_self_checkin,
	-- 4 must stay in sync with RequiredDocumentCount (len(ValidDocumentTypes)
	-- in documents.go) — Offer Letter, Aadhar Card, PAN Card, Marksheet.
	(SELECT COUNT(DISTINCT document_type) FROM employee_documents ed WHERE ed.employee_id = e.id) >= 4,
	e.date_of_birth::text, e.address
`

const fromJoins = `
	FROM employees e
	LEFT JOIN departments d ON d.id = e.department_id
	LEFT JOIN designations des ON des.id = e.designation_id
	LEFT JOIN positions pos ON pos.id = e.position_id
	LEFT JOIN shifts s ON s.id = e.shift_id
	LEFT JOIN employees m ON m.id = e.reporting_manager_id
	LEFT JOIN users u ON u.id = e.user_id
`

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// nullableString turns a blank form value into a real SQL NULL. Needed for
// email specifically because the column is UNIQUE — inserting "" for every
// employee with no email on file would collide on the second one, where NULL
// never does.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func scanEmployee(row pgx.Row) (*Employee, error) {
	var e Employee
	err := row.Scan(
		&e.ID, &e.EmployeeCode, &e.FullName, &e.Email, &e.Phone, &e.PhotoURL,
		&e.JoiningDate, &e.DepartmentID, &e.DepartmentName, &e.DesignationID, &e.DesignationTitle,
		&e.PositionID, &e.PositionTitle,
		&e.ShiftID, &e.ShiftName, &e.ReportingManagerID, &e.ReportingManagerName, &e.Status,
		&e.LastWorkingDay, &e.CreatedAt, &e.UpdatedAt,
		&e.UserID, &e.LoginUsername,
		&e.HasFaceRegistered,
		&e.RemoteSelfCheckin,
		&e.DocumentsComplete,
		&e.DateOfBirth, &e.Address,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// resignExpiredNotices flips anyone whose notice period has elapsed from
// ON_NOTICE to RESIGNED. There is no background job runner yet, so this is
// swept lazily on every read instead — cheap, and always consistent with
// "today" by construction.
func (r *Repository) resignExpiredNotices(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		UPDATE employees SET status = 'RESIGNED', updated_at = now()
		WHERE status = 'ON_NOTICE' AND last_working_day <= CURRENT_DATE
	`)
	return err
}

func (r *Repository) List(ctx context.Context, f ListFilters) ([]Employee, error) {
	if err := r.resignExpiredNotices(ctx); err != nil {
		return nil, err
	}

	query := `SELECT ` + selectColumns + fromJoins + ` WHERE 1=1`
	args := []any{}
	argN := 1

	if f.EmployeeID != nil {
		query += fmt.Sprintf(" AND e.id = $%d", argN)
		args = append(args, *f.EmployeeID)
		argN++
	}
	if f.DepartmentID != nil {
		query += fmt.Sprintf(" AND e.department_id = $%d", argN)
		args = append(args, *f.DepartmentID)
		argN++
	}
	if f.DesignationID != nil {
		query += fmt.Sprintf(" AND e.designation_id = $%d", argN)
		args = append(args, *f.DesignationID)
		argN++
	}
	if f.PositionID != nil {
		query += fmt.Sprintf(" AND e.position_id = $%d", argN)
		args = append(args, *f.PositionID)
		argN++
	}
	if f.Status != "" {
		query += fmt.Sprintf(" AND e.status = $%d", argN)
		args = append(args, f.Status)
		argN++
	} else {
		query += " AND e.status NOT IN ('RESIGNED', 'TERMINATED')"
	}
	query += " ORDER BY e.full_name"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Employee
	for rows.Next() {
		e, err := scanEmployee(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id int64) (*Employee, error) {
	if err := r.resignExpiredNotices(ctx); err != nil {
		return nil, err
	}
	row := r.db.QueryRow(ctx, `SELECT `+selectColumns+fromJoins+` WHERE e.id = $1`, id)
	e, err := scanEmployee(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

// designationMatchesDepartment confirms a designation actually belongs to the
// department it's being assigned alongside — the linked dropdown in the UI
// already enforces this, but the API itself must too, since nothing stops a
// direct request from pairing them incorrectly.
func designationMatchesDepartment(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, departmentID, designationID *int64) error {
	if designationID == nil {
		return nil
	}
	if departmentID == nil {
		return ErrDesignationDepartmentMismatch
	}
	var actual int64
	err := q.QueryRow(ctx, `SELECT department_id FROM designations WHERE id = $1`, *designationID).Scan(&actual)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDesignationNotFound
	}
	if err != nil {
		return err
	}
	if actual != *departmentID {
		return ErrDesignationDepartmentMismatch
	}
	return nil
}

// shiftExists confirms a given shift id is real before it's assigned to an
// employee — nothing else validates this at the SQL level since shift_id has
// no NOT NULL/CHECK tying it to a live row beyond the FK, which would only
// fail with an opaque constraint error.
func shiftExists(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, shiftID *int64) error {
	if shiftID == nil {
		return nil
	}
	var exists bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shifts WHERE id = $1)`, *shiftID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrShiftNotFound
	}
	return nil
}

func (r *Repository) Create(ctx context.Context, in EmployeeInput) (*Employee, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := designationMatchesDepartment(ctx, tx, in.DepartmentID, in.DesignationID); err != nil {
		return nil, err
	}
	if err := shiftExists(ctx, tx, in.ShiftID); err != nil {
		return nil, err
	}

	// A shift is required for the calculation engine to have anything to
	// measure lateness against; falling back to the oldest active shift keeps
	// callers that don't pass one (e.g. bulk-import scripts) working.
	var id int64
	err = tx.QueryRow(ctx,
		`INSERT INTO employees
			(employee_code, full_name, email, phone, photo_url, joining_date,
			 department_id, designation_id, shift_id, date_of_birth, address, position_id)
		 VALUES ('PENDING', $1, $2, $3, $4, $5, $6, $7,
			COALESCE($8, (SELECT id FROM shifts WHERE is_active = true ORDER BY id LIMIT 1)),
			$9, $10, $11)
		 RETURNING id`,
		in.FullName, nullableString(in.Email), in.Phone, in.PhotoURL, in.JoiningDate,
		in.DepartmentID, in.DesignationID, in.ShiftID,
		nullableString(in.DateOfBirth), in.Address, in.PositionID,
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	// The employee code doubles as the login username, so it has to be derived
	// from the row's id and written back before any account is created.
	code := fmt.Sprintf("EMP%04d", id)
	if _, err := tx.Exec(ctx, `UPDATE employees SET employee_code = $2 WHERE id = $1`, id, code); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.Get(ctx, id)
}

// createLoginTx gives an employee a login account inside an existing
// transaction: username is the employee code, the role is always EMPLOYEE, and
// employees.user_id is set so the two records stay linked. Kept in one
// transaction with the employee row so a failure can't leave an orphan account.
func createLoginTx(ctx context.Context, tx pgx.Tx, employeeID int64, code, fullName, passwordHash string) error {
	var roleID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM roles WHERE name = 'EMPLOYEE'`).Scan(&roleID); err != nil {
		return err
	}

	var userID int64
	err := tx.QueryRow(ctx,
		`INSERT INTO users (username, password_hash, full_name) VALUES ($1, $2, $3) RETURNING id`,
		code, passwordHash, fullName,
	).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return ErrLoginExists
		}
		return err
	}

	if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE employees SET user_id = $2, updated_at = now() WHERE id = $1`, employeeID, userID); err != nil {
		return err
	}
	return nil
}

// CreateLogin gives an already-existing employee a login account. Separate from
// Create because employees added before logins existed still need one, and
// because an admin may deliberately create an employee record first and issue
// credentials later.
func (r *Repository) CreateLogin(ctx context.Context, employeeID int64, passwordHash string) (*Employee, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var code, fullName string
	var existingUserID *int64
	err = tx.QueryRow(ctx,
		`SELECT employee_code, full_name, user_id FROM employees WHERE id = $1`,
		employeeID,
	).Scan(&code, &fullName, &existingUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if existingUserID != nil {
		return nil, ErrLoginExists
	}

	if err := createLoginTx(ctx, tx, employeeID, code, fullName, passwordHash); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.Get(ctx, employeeID)
}

func (r *Repository) Update(ctx context.Context, id int64, in EmployeeInput) (*Employee, error) {
	if err := designationMatchesDepartment(ctx, r.db, in.DepartmentID, in.DesignationID); err != nil {
		return nil, err
	}
	if err := shiftExists(ctx, r.db, in.ShiftID); err != nil {
		return nil, err
	}

	// reporting_manager_id is deliberately left out of this SET list: there is
	// no manager-assignment UI yet, so touching it here would silently null
	// out whatever was set some other way. shift_id uses COALESCE so a caller
	// that omits it (nil) leaves the employee's current shift untouched.
	tag, err := r.db.Exec(ctx,
		`UPDATE employees SET
			full_name = $2, email = $3, phone = $4, photo_url = $5, joining_date = $6,
			department_id = $7, designation_id = $8, shift_id = COALESCE($9, shift_id),
			date_of_birth = $10, address = $11, position_id = $12,
			updated_at = now()
		 WHERE id = $1`,
		id, in.FullName, nullableString(in.Email), in.Phone, in.PhotoURL, in.JoiningDate,
		in.DepartmentID, in.DesignationID, in.ShiftID,
		nullableString(in.DateOfBirth), in.Address, in.PositionID,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, id)
}

// SetStatus updates an employee's status. When moving to ON_NOTICE,
// lastWorkingDay is required and starts the resignation countdown; for any
// other status it is cleared, since it's only meaningful during notice.
func (r *Repository) SetStatus(ctx context.Context, id int64, status string, lastWorkingDay *string) error {
	if !ValidStatuses[status] {
		return ErrInvalidInput
	}
	if status == "ON_NOTICE" && (lastWorkingDay == nil || *lastWorkingDay == "") {
		return ErrInvalidInput
	}
	if status != "ON_NOTICE" {
		lastWorkingDay = nil
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE employees SET status = $2, last_working_day = $3, updated_at = now() WHERE id = $1`,
		id, status, lastWorkingDay,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetRemoteSelfCheckin toggles whether this employee may self-report their
// own attendance from their dashboard, with no face verification — for
// someone with no camera to scan against.
func (r *Repository) SetRemoteSelfCheckin(ctx context.Context, id int64, enabled bool) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE employees SET remote_self_checkin = $2, updated_at = now() WHERE id = $1`,
		id, enabled,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
