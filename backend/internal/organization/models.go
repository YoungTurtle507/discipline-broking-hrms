package organization

import "time"

type Department struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Designation struct {
	ID             int64     `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	DepartmentID   int64     `json:"department_id"`
	DepartmentName string    `json:"department_name"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Position is a seniority level (Senior, Junior, ...) — flat, not scoped to
// a department or designation the way Designation is; the same title can
// apply to an employee in any department.
type Position struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Shift struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	StartTime          string `json:"start_time"`
	EndTime            string `json:"end_time"`
	GracePeriodMinutes int    `json:"grace_period_minutes"`
	// Saturday* is an optional override of this shift's regular weekday
	// timing — nil means "same as weekday" for that field. Used when the
	// office runs shorter Saturday hours than Monday-Friday.
	SaturdayStartTime          *string   `json:"saturday_start_time"`
	SaturdayEndTime            *string   `json:"saturday_end_time"`
	SaturdayGracePeriodMinutes *int      `json:"saturday_grace_period_minutes"`
	IsActive                   bool      `json:"is_active"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}
