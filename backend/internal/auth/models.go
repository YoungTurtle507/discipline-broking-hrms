package auth

import "time"

// AssignableRoles is every role a Super Admin/Admin can hand out through the
// user-management UI. SUPER_ADMIN is deliberately excluded from what ADMIN
// can grant — see CreateUser/SetUserRole in repository.go for the enforcement.
//
// The company runs on three roles: one Super Admin, one Admin (the HR person),
// and Employees. HR and MANAGER still exist in the roles table with their
// permission grants intact — they are simply not offered, so re-enabling either
// later means adding it back here and to ALL_ROLES in the users panel. ADMIN is
// a superset of HR, so nothing is lost by putting HR staff on it.
var AssignableRoles = map[string]bool{
	"SUPER_ADMIN": true,
	"ADMIN":       true,
	"EMPLOYEE":    true,
}

// UserSummary is the list/detail shape returned by the user-management API.
type UserSummary struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	IsActive  bool      `json:"is_active"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
}

// User is the raw login-account row.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	FullName     string
	IsActive     bool
}

// UserContext is what the rest of the app sees once a session resolves to a user:
// identity plus everything needed for permission checks.
type UserContext struct {
	UserID      int64    `json:"user_id"`
	Username    string   `json:"username"`
	FullName    string   `json:"full_name"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`

	// PasswordChangedAt is compared against the session's issue time by
	// RequireAuth, so resetting a password ends that user's other sessions.
	// Not serialized: it's internal bookkeeping, not identity the client needs.
	PasswordChangedAt time.Time `json:"-"`
}

// HasPermission reports whether this user is allowed to do something requiring
// the given permission code. SUPER_ADMIN always passes, regardless of what's
// been explicitly granted, so newly added permissions never need a migration
// to also grant them to SUPER_ADMIN.
func (u *UserContext) HasPermission(code string) bool {
	if u.IsSuperAdmin() {
		return true
	}
	for _, p := range u.Permissions {
		if p == code {
			return true
		}
	}
	return false
}

// IsSuperAdmin reports whether this user holds the one role that bypasses
// every permission check — used directly where a rule is "Super Admin only,
// no permission grant can substitute" (e.g. skipping the password confirmation
// on a manual attendance entry).
func (u *UserContext) IsSuperAdmin() bool {
	for _, r := range u.Roles {
		if r == "SUPER_ADMIN" {
			return true
		}
	}
	return false
}
