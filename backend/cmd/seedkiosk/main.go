// One-time seed for Discipline Broking's trimmed HRMS: the shared KIOSK
// login, plus one default department/designation/shift so Super Admin can
// add employees immediately without needing an Organization-management UI.
// Safe to re-run — every step checks for existing data first.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"hr-management/backend/internal/auth"
	"hr-management/backend/internal/config"
	"hr-management/backend/internal/organization"
	"hr-management/backend/pkg/database"
)

func main() {
	username := flag.String("username", "kiosk@disciplinebroking.com", "shared kiosk login username")
	password := flag.String("password", "", "shared kiosk login password (required)")
	fullName := flag.String("name", "Attendance Kiosk", "display name for the shared login")
	flag.Parse()

	if *password == "" {
		fmt.Fprintln(os.Stderr, "error: -password is required")
		os.Exit(1)
	}

	ctx := context.Background()
	cfg := config.Load()
	db, err := database.Connect(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "db connect failed:", err)
		os.Exit(1)
	}
	defer db.Close()

	authRepo := auth.NewRepository(db)
	orgRepo := organization.NewRepository(db)

	// --- KIOSK account ---
	hash, err := auth.HashPassword(*password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hash failed:", err)
		os.Exit(1)
	}
	if _, err := authRepo.CreateUser(ctx, *username, hash, *fullName, "KIOSK"); err != nil {
		if err == auth.ErrUsernameTaken {
			fmt.Println("kiosk account already exists — leaving it alone")
		} else {
			fmt.Fprintln(os.Stderr, "create kiosk user failed:", err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("created kiosk account: %s\n", *username)
	}

	// --- Default department/designation/shift ---
	departments, err := orgRepo.ListDepartments(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list departments failed:", err)
		os.Exit(1)
	}
	var deptID int64
	if len(departments) > 0 {
		deptID = departments[0].ID
		fmt.Printf("department already exists: %s\n", departments[0].Name)
	} else {
		d, err := orgRepo.CreateDepartment(ctx, "General", "Default department")
		if err != nil {
			fmt.Fprintln(os.Stderr, "create department failed:", err)
			os.Exit(1)
		}
		deptID = d.ID
		fmt.Println("created department: General")
	}

	designations, err := orgRepo.ListDesignations(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list designations failed:", err)
		os.Exit(1)
	}
	if len(designations) > 0 {
		fmt.Printf("designation already exists: %s\n", designations[0].Title)
	} else {
		if _, err := orgRepo.CreateDesignation(ctx, "Staff", "Default designation", deptID); err != nil {
			fmt.Fprintln(os.Stderr, "create designation failed:", err)
			os.Exit(1)
		}
		fmt.Println("created designation: Staff")
	}

	shifts, err := orgRepo.ListShifts(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list shifts failed:", err)
		os.Exit(1)
	}
	if len(shifts) > 0 {
		fmt.Printf("shift already exists: %s\n", shifts[0].Name)
	} else {
		if _, err := orgRepo.CreateShift(ctx, "General Shift", "09:00", "18:00", 10, nil, nil, nil); err != nil {
			fmt.Fprintln(os.Stderr, "create shift failed:", err)
			os.Exit(1)
		}
		fmt.Println("created shift: General Shift (09:00–18:00, 10 min grace)")
	}

	fmt.Println("\nseed complete.")
}
