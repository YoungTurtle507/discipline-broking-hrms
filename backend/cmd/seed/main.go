// Command seed creates local test accounts (a SUPER_ADMIN and an ADMIN) for
// development and QA. Not part of the production bootstrap flow — run by
// hand when you need known-password test accounts in a fresh database.
package main

import (
	"context"
	"log/slog"
	"os"

	"hr-management/backend/internal/auth"
	"hr-management/backend/internal/config"
	"hr-management/backend/pkg/database"
)

type seedAccount struct {
	username string
	password string
	fullName string
	role     string
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx := context.Background()
	cfg := config.Load()

	db, err := database.Connect(ctx, cfg)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	repo := auth.NewRepository(db)

	accounts := []seedAccount{
		{username: "Vrajesh@123", password: "Vrajesh123", fullName: "Vrajesh", role: "SUPER_ADMIN"},
		{username: "Admin@123", password: "Admin123", fullName: "Admin", role: "ADMIN"},
	}

	for _, acc := range accounts {
		if _, err := repo.GetUserByUsername(ctx, acc.username); err == nil {
			slog.Info("account already exists, skipping", "username", acc.username)
			continue
		}

		hash, err := auth.HashPassword(acc.password)
		if err != nil {
			slog.Error("failed to hash password", "username", acc.username, "error", err)
			os.Exit(1)
		}

		if err := repo.CreateUserWithRole(ctx, acc.username, hash, acc.fullName, acc.role); err != nil {
			slog.Error("failed to create account", "username", acc.username, "error", err)
			os.Exit(1)
		}

		slog.Info("created account", "username", acc.username, "role", acc.role)
	}
}
