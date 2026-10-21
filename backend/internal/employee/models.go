package employee

import "time"

var ValidStatuses = map[string]bool{
	"ACTIVE":     true,
	"INACTIVE":   true,
	"ON_NOTICE":  true,
	"RESIGNED":   true,
	"TERMINATED": true,
	"SUSPENDED":  true,
}

type Employee struct {
	ID                   int64   `json:"id"`
	EmployeeCode         string  `json:"employee_code"`
	FullName             string  `json:"full_name"`
	Email                *string `json:"email"`
	Phone                string  `json:"phone"`
	PhotoURL             string  `json:"photo_url"`
	JoiningDate          string  `json:"joining_date"`
	DepartmentID         *int64  `json:"department_id"`
	DepartmentName       *string `json:"department_name,omitempty"`
	DesignationID        *int64  `json:"designation_id"`
	DesignationTitle     *string `json:"designation_title,omitempty"`
	PositionID           *int64  `json:"position_id"`
	PositionTitle        *string `json:"position_title,omitempty"`
	ShiftID              *int64  `json:"shift_id"`
	ShiftName            *string `json:"shift_name,omitempty"`
	ReportingManagerID   *int64  `json:"reporting_manager_id"`
	ReportingManagerName *string `json:"reporting_manager_name,omitempty"`
	Status               string  `json:"status"`
	LastWorkingDay       *string `json:"last_working_day"`
	// UserID/LoginUsername describe the employee's own login account, if an
	// admin has issued one. Nil means they cannot sign in yet.
	UserID        *int64    `json:"user_id"`
	LoginUsername *string   `json:"login_username,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	// HasFaceRegistered is true once at least one active face sample exists —
	// the minimum needed for this employee to clock in via face scan at all.
	HasFaceRegistered bool `json:"has_face_registered"`
	// RemoteSelfCheckin lets this specific employee mark their own attendance
	// from their self-service dashboard, with no face verification at all —
	// for someone with no camera to scan against (e.g. working from home).
	// A per-employee flag, not a role-wide feature: everyone else still must
	// clock in by face scan.
	RemoteSelfCheckin bool `json:"remote_self_checkin"`
	// DocumentsComplete is true once every required document type (see
	// ValidDocumentTypes) has been uploaded — the Employees list page's "X
	// missing documents" banner is built from this, the same way
	// HasFaceRegistered drives the equivalent face-registration banner.
	DocumentsComplete bool `json:"documents_complete"`
	// DateOfBirth backs the dashboard's upcoming-birthday reminder — nil for
	// any employee added before this field existed.
	DateOfBirth *string `json:"date_of_birth"`
	Address     string  `json:"address"`
}

// DefaultNoticePeriodDays is the suggested notice period used to prefill the
// last working day when an employee is moved to ON_NOTICE; admins can pick
// any date, this is just a starting point.
const DefaultNoticePeriodDays = 60

type EmployeeInput struct {
	FullName string `json:"full_name"`
	// Email is optional — left blank, the column stores NULL rather than an
	// empty string (see nullableString in repository.go), since it's UNIQUE
	// and Postgres only treats NULLs as distinct from each other.
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	PhotoURL      string `json:"photo_url"`
	JoiningDate   string `json:"joining_date"`
	DepartmentID  *int64 `json:"department_id"`
	DesignationID *int64 `json:"designation_id"`
	// PositionID is a seniority level (Senior, Junior, ...) — independent of
	// department/designation, unlike DesignationID.
	PositionID *int64 `json:"position_id"`
	// ShiftID picks which shift's start/end/grace period the calculation
	// engine uses for this employee's late/half-day/overtime math. Nil on
	// Create falls back to the sole active shift for back-compat callers; nil
	// on Update leaves whatever shift is already assigned untouched.
	ShiftID *int64 `json:"shift_id"`
	// Reporting manager is not set through this form — there is currently no
	// manager-assignment UI. The column and the Employee struct's field below
	// still exist so a profile can display it; Update never touches it.

	// DateOfBirth is optional — blank stores NULL (see nullableString), the
	// same way Email does, since most existing employees have none on file yet.
	DateOfBirth string `json:"date_of_birth"`
	Address     string `json:"address"`
}

type ListFilters struct {
	DepartmentID  *int64
	DesignationID *int64
	PositionID    *int64
	Status        string
	EmployeeID    *int64
}
