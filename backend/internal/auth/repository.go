package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")
var ErrUsernameTaken = errors.New("username already taken")
var ErrRoleNotFound = errors.New("role not found")
var ErrLastSuperAdmin = errors.New("cannot remove the last Super Admin")

const uniqueViolation = "23505"

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx,
		`SELECT id, username, password_hash, full_name, is_active FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.FullName, &u.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// VerifyPassword checks plain against the given user's stored hash — used to
// re-confirm identity for a sensitive action within an already-authenticated
// session, not to log in. false (no error) means the password was simply
// wrong; an error means the check itself couldn't be performed.
func (r *Repository) VerifyPassword(ctx context.Context, userID int64, plain string) (bool, error) {
	var hash string
	err := r.db.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrUserNotFound
	}
	if err != nil {
		return false, err
	}
	return CheckPassword(hash, plain), nil
}

// LoadUserContext loads identity + roles + permissions fresh from Postgres.
// Deliberately not cached in the session: a deactivated account or a role
// change takes effect on the user's very next request.
func (r *Repository) LoadUserContext(ctx context.Context, userID int64) (*UserContext, error) {
	uc := &UserContext{UserID: userID}

	err := r.db.QueryRow(ctx,
		`SELECT username, full_name, password_changed_at FROM users WHERE id = $1 AND is_active = true`,
		userID,
	).Scan(&uc.Username, &uc.FullName, &uc.PasswordChangedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	roleRows, err := r.db.Query(ctx,
		`SELECT r.name FROM roles r JOIN user_roles ur ON ur.role_id = r.id WHERE ur.user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	for roleRows.Next() {
		var name string
		if err := roleRows.Scan(&name); err != nil {
			roleRows.Close()
			return nil, err
		}
		uc.Roles = append(uc.Roles, name)
	}
	roleRows.Close()
	if err := roleRows.Err(); err != nil {
		return nil, err
	}

	permRows, err := r.db.Query(ctx,
		`SELECT DISTINCT p.code FROM permissions p
		 JOIN role_permissions rp ON rp.permission_id = p.id
		 JOIN user_roles ur ON ur.role_id = rp.role_id
		 WHERE ur.user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	for permRows.Next() {
		var code string
		if err := permRows.Scan(&code); err != nil {
			permRows.Close()
			return nil, err
		}
		uc.Permissions = append(uc.Permissions, code)
	}
	permRows.Close()
	if err := permRows.Err(); err != nil {
		return nil, err
	}

	return uc, nil
}

func (r *Repository) CountUsersWithRole(ctx context.Context, roleName string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE r.name = $1`,
		roleName,
	).Scan(&count)
	return count, err
}

func (r *Repository) CreateUserWithRole(ctx context.Context, username, passwordHash, fullName, roleName string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var userID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO users (username, password_hash, full_name) VALUES ($1, $2, $3) RETURNING id`,
		username, passwordHash, fullName,
	).Scan(&userID)
	if err != nil {
		return err
	}

	var roleID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM roles WHERE name = $1`, roleName).Scan(&roleID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ListUsers returns every login account with its assigned role names,
// newest first.
func (r *Repository) ListUsers(ctx context.Context) ([]UserSummary, error) {
	rows, err := r.db.Query(ctx,
		`SELECT u.id, u.username, u.full_name, u.is_active, u.created_at,
		        COALESCE(array_agg(ro.name) FILTER (WHERE ro.name IS NOT NULL), '{}')
		 FROM users u
		 LEFT JOIN user_roles ur ON ur.user_id = u.id
		 LEFT JOIN roles ro ON ro.id = ur.role_id
		 GROUP BY u.id
		 ORDER BY u.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserSummary
	for rows.Next() {
		var u UserSummary
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.IsActive, &u.CreatedAt, &u.Roles); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CreateUser creates a login account with exactly one role and returns its
// summary. Returns ErrUsernameTaken on a duplicate username and
// ErrRoleNotFound if roleName isn't a real role.
func (r *Repository) CreateUser(ctx context.Context, username, passwordHash, fullName, roleName string) (*UserSummary, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var roleID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM roles WHERE name = $1`, roleName).Scan(&roleID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}

	var u UserSummary
	err = tx.QueryRow(ctx,
		`INSERT INTO users (username, password_hash, full_name) VALUES ($1, $2, $3)
		 RETURNING id, username, full_name, is_active, created_at`,
		username, passwordHash, fullName,
	).Scan(&u.ID, &u.Username, &u.FullName, &u.IsActive, &u.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return nil, ErrUsernameTaken
		}
		return nil, err
	}

	if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, u.ID, roleID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	u.Roles = []string{roleName}
	return &u, nil
}

// SetUserActive activates or deactivates a login account. A deactivated
// account fails LoadUserContext on its very next request, so any existing
// session is effectively dead immediately.
func (r *Repository) SetUserActive(ctx context.Context, userID int64, active bool) error {
	if !active {
		var isTheLastActiveSuperAdmin bool
		err := r.db.QueryRow(ctx,
			`SELECT EXISTS(
			   SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
			   WHERE ur.user_id = $1 AND r.name = 'SUPER_ADMIN'
			 ) AND (
			   SELECT COUNT(*) FROM user_roles ur JOIN roles r ON r.id = ur.role_id
			   JOIN users u ON u.id = ur.user_id
			   WHERE r.name = 'SUPER_ADMIN' AND u.is_active = true
			 ) <= 1`,
			userID,
		).Scan(&isTheLastActiveSuperAdmin)
		if err != nil {
			return err
		}
		if isTheLastActiveSuperAdmin {
			return ErrLastSuperAdmin
		}
	}

	tag, err := r.db.Exec(ctx,
		`UPDATE users SET is_active = $2, updated_at = now() WHERE id = $1`,
		userID, active,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// SetUserRole replaces a user's role assignment with a single new role
// (the schema allows many-to-many, but this app only ever uses one role per
// user, so this keeps that invariant rather than accumulating roles).
func (r *Repository) SetUserRole(ctx context.Context, userID int64, roleName string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrUserNotFound
	}

	if roleName != "SUPER_ADMIN" {
		var isCurrentlySuperAdmin bool
		err := tx.QueryRow(ctx,
			`SELECT EXISTS(
			   SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
			   WHERE ur.user_id = $1 AND r.name = 'SUPER_ADMIN'
			 )`, userID,
		).Scan(&isCurrentlySuperAdmin)
		if err != nil {
			return err
		}
		if isCurrentlySuperAdmin {
			var superAdminCount int
			if err := tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE r.name = 'SUPER_ADMIN'`,
			).Scan(&superAdminCount); err != nil {
				return err
			}
			if superAdminCount <= 1 {
				return ErrLastSuperAdmin
			}
		}
	}

	var roleID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM roles WHERE name = $1`, roleName).Scan(&roleID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotFound
		}
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// SetUserPassword replaces a user's password hash and stamps
// password_changed_at, which invalidates every session issued before now (see
// RequireAuth). Admin-driven only: there is no self-service email reset, so an
// admin or super-admin sets a new password and tells the person what it is.
// The previous password is never recoverable — it's a bcrypt hash, so an admin
// can reset it but can never read it.
func (r *Repository) SetUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET password_hash = $2, password_changed_at = now(), updated_at = now()
		 WHERE id = $1`,
		userID, passwordHash,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// IsSuperAdmin reports whether a user holds the SUPER_ADMIN role. Used to stop
// an ADMIN from resetting a super-admin's password and taking over the account.
func (r *Repository) IsSuperAdmin(ctx context.Context, userID int64) (bool, error) {
	var yes bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
		   WHERE ur.user_id = $1 AND r.name = 'SUPER_ADMIN'
		 )`, userID,
	).Scan(&yes)
	return yes, err
}
