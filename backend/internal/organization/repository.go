package organization

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("record not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ---- Departments ----

func (r *Repository) ListDepartments(ctx context.Context) ([]Department, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, description, is_active, created_at, updated_at
		 FROM departments ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Department
	for rows.Next() {
		var d Department
		if err := rows.Scan(&d.ID, &d.Name, &d.Description, &d.IsActive, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repository) CreateDepartment(ctx context.Context, name, description string) (*Department, error) {
	var d Department
	err := r.db.QueryRow(ctx,
		`INSERT INTO departments (name, description) VALUES ($1, $2)
		 RETURNING id, name, description, is_active, created_at, updated_at`,
		name, description,
	).Scan(&d.ID, &d.Name, &d.Description, &d.IsActive, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) UpdateDepartment(ctx context.Context, id int64, name, description string) (*Department, error) {
	var d Department
	err := r.db.QueryRow(ctx,
		`UPDATE departments SET name = $2, description = $3, updated_at = now()
		 WHERE id = $1
		 RETURNING id, name, description, is_active, created_at, updated_at`,
		id, name, description,
	).Scan(&d.ID, &d.Name, &d.Description, &d.IsActive, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) SetDepartmentActive(ctx context.Context, id int64, active bool) error {
	tag, err := r.db.Exec(ctx, `UPDATE departments SET is_active = $2, updated_at = now() WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Positions ----

func (r *Repository) ListPositions(ctx context.Context) ([]Position, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, title, description, is_active, created_at, updated_at
		 FROM positions ORDER BY title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Position
	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) CreatePosition(ctx context.Context, title, description string) (*Position, error) {
	var p Position
	err := r.db.QueryRow(ctx,
		`INSERT INTO positions (title, description) VALUES ($1, $2)
		 RETURNING id, title, description, is_active, created_at, updated_at`,
		title, description,
	).Scan(&p.ID, &p.Title, &p.Description, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) UpdatePosition(ctx context.Context, id int64, title, description string) (*Position, error) {
	var p Position
	err := r.db.QueryRow(ctx,
		`UPDATE positions SET title = $2, description = $3, updated_at = now()
		 WHERE id = $1
		 RETURNING id, title, description, is_active, created_at, updated_at`,
		id, title, description,
	).Scan(&p.ID, &p.Title, &p.Description, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) SetPositionActive(ctx context.Context, id int64, active bool) error {
	tag, err := r.db.Exec(ctx, `UPDATE positions SET is_active = $2, updated_at = now() WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Designations ----

const designationSelect = `
	SELECT d.id, d.title, d.description, d.department_id, dep.name,
	       d.is_active, d.created_at, d.updated_at
	FROM designations d
	JOIN departments dep ON dep.id = d.department_id
`

func scanDesignation(row pgx.Row) (*Designation, error) {
	var d Designation
	if err := row.Scan(&d.ID, &d.Title, &d.Description, &d.DepartmentID, &d.DepartmentName,
		&d.IsActive, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return nil, err
	}
	return &d, nil
}

// ListDesignations returns every designation across every department. The
// employee form groups these by department_id itself rather than the API
// filtering server-side, since the same list also drives the combined
// Departments & Designations table in Organization.
func (r *Repository) ListDesignations(ctx context.Context) ([]Designation, error) {
	rows, err := r.db.Query(ctx, designationSelect+` ORDER BY dep.name, d.title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Designation
	for rows.Next() {
		d, err := scanDesignation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

// CreateDesignation adds a designation under a specific department — every
// designation belongs to exactly one department, so a Coding designation is
// never offered when adding someone to Dealer.
func (r *Repository) CreateDesignation(ctx context.Context, title, description string, departmentID int64) (*Designation, error) {
	var id int64
	err := r.db.QueryRow(ctx,
		`INSERT INTO designations (title, description, department_id) VALUES ($1, $2, $3) RETURNING id`,
		title, description, departmentID,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return scanDesignation(r.db.QueryRow(ctx, designationSelect+` WHERE d.id = $1`, id))
}

func (r *Repository) UpdateDesignation(ctx context.Context, id int64, title, description string, departmentID int64) (*Designation, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE designations SET title = $2, description = $3, department_id = $4, updated_at = now() WHERE id = $1`,
		id, title, description, departmentID,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return scanDesignation(r.db.QueryRow(ctx, designationSelect+` WHERE d.id = $1`, id))
}

func (r *Repository) SetDesignationActive(ctx context.Context, id int64, active bool) error {
	tag, err := r.db.Exec(ctx, `UPDATE designations SET is_active = $2, updated_at = now() WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Shifts ----

const shiftColumns = `id, name, start_time::text, end_time::text, grace_period_minutes,
	saturday_start_time::text, saturday_end_time::text, saturday_grace_period_minutes,
	is_active, created_at, updated_at`

func scanShift(row pgx.Row) (*Shift, error) {
	var s Shift
	if err := row.Scan(&s.ID, &s.Name, &s.StartTime, &s.EndTime, &s.GracePeriodMinutes,
		&s.SaturdayStartTime, &s.SaturdayEndTime, &s.SaturdayGracePeriodMinutes,
		&s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) ListShifts(ctx context.Context) ([]Shift, error) {
	rows, err := r.db.Query(ctx, `SELECT `+shiftColumns+` FROM shifts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Shift
	for rows.Next() {
		s, err := scanShift(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (r *Repository) CreateShift(ctx context.Context, name, startTime, endTime string, gracePeriodMinutes int,
	saturdayStartTime, saturdayEndTime *string, saturdayGracePeriodMinutes *int) (*Shift, error) {
	row := r.db.QueryRow(ctx,
		`INSERT INTO shifts (name, start_time, end_time, grace_period_minutes,
		                     saturday_start_time, saturday_end_time, saturday_grace_period_minutes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING `+shiftColumns,
		name, startTime, endTime, gracePeriodMinutes,
		saturdayStartTime, saturdayEndTime, saturdayGracePeriodMinutes,
	)
	return scanShift(row)
}

func (r *Repository) UpdateShift(ctx context.Context, id int64, name, startTime, endTime string, gracePeriodMinutes int,
	saturdayStartTime, saturdayEndTime *string, saturdayGracePeriodMinutes *int) (*Shift, error) {
	row := r.db.QueryRow(ctx,
		`UPDATE shifts SET name = $2, start_time = $3, end_time = $4, grace_period_minutes = $5,
		                    saturday_start_time = $6, saturday_end_time = $7, saturday_grace_period_minutes = $8,
		                    updated_at = now()
		 WHERE id = $1
		 RETURNING `+shiftColumns,
		id, name, startTime, endTime, gracePeriodMinutes,
		saturdayStartTime, saturdayEndTime, saturdayGracePeriodMinutes,
	)
	s, err := scanShift(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) SetShiftActive(ctx context.Context, id int64, active bool) error {
	tag, err := r.db.Exec(ctx, `UPDATE shifts SET is_active = $2, updated_at = now() WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
