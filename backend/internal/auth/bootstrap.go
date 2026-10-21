package auth

import (
	"context"
	"log/slog"
)

// EnsureSuperAdmin creates the first SUPER_ADMIN account from config on a
// fresh database. Idempotent: no-ops once any SUPER_ADMIN exists, since there
// is no self-signup flow for anyone to create accounts otherwise.
func EnsureSuperAdmin(ctx context.Context, repo *Repository, username, password string) error {
	count, err := repo.CountUsersWithRole(ctx, "SUPER_ADMIN")
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}

	if err := repo.CreateUserWithRole(ctx, username, hash, "Super Admin", "SUPER_ADMIN"); err != nil {
		return err
	}

	slog.Info("bootstrapped initial super admin account", "username", username)
	return nil
}
