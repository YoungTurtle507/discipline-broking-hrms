package main

import (
	"context"
	"log/slog"
	"os"

	"hr-management/backend/internal/alert"
	"hr-management/backend/internal/attendance"
	"hr-management/backend/internal/auth"
	"hr-management/backend/internal/calculation"
	"hr-management/backend/internal/clock"
	"hr-management/backend/internal/config"
	"hr-management/backend/internal/dashboard"
	"hr-management/backend/internal/employee"
	"hr-management/backend/internal/faceregistration"
	"hr-management/backend/internal/health"
	"hr-management/backend/internal/leave"
	"hr-management/backend/internal/organization"
	"hr-management/backend/pkg/database"
	"hr-management/backend/pkg/middleware"
	"hr-management/backend/pkg/redisclient"

	"github.com/gin-gonic/gin"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()
	alert.Start(cfg.NtfyTopic)

	ctx := context.Background()

	db, err := database.Connect(ctx, cfg)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	rdb, err := redisclient.Connect(cfg)
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	authRepo := auth.NewRepository(db)
	sessions := auth.NewSessionStore(rdb, cfg.SessionTTLHours)
	limiter := auth.NewLoginLimiter(rdb)
	authHandlers := auth.NewHandlers(authRepo, sessions, limiter, cfg.CookieSecure)

	if err := auth.EnsureSuperAdmin(ctx, authRepo, cfg.BootstrapAdminUser, cfg.BootstrapAdminPass); err != nil {
		slog.Error("failed to bootstrap super admin", "error", err)
		os.Exit(1)
	}

	orgRepo := organization.NewRepository(db)

	empRepo := employee.NewRepository(db)
	empHandlers := employee.NewHandlers(empRepo)
	empDocHandlers := employee.NewDocumentHandlers(empRepo, cfg.StoragePath)

	faceClient := faceregistration.NewClient(cfg.FaceServiceURL)
	faceHandlers := faceregistration.NewHandlers(cfg.StoragePath)

	calcRepo := calculation.NewRepository(db)
	calcHandlers := calculation.NewHandlers(calcRepo)

	// Calculation policy (late-streak limit, half-day/absent cutoffs, the
	// stay-late pass, lunch OT streak) is admin-configurable — loaded from
	// the database once here so it takes effect from the very first request,
	// not just after the first save from Organization.
	if policy, err := calcRepo.LoadPolicySettings(ctx); err != nil {
		slog.Error("failed to load calculation policy settings, using defaults", "error", err)
	} else {
		calculation.SetCurrentPolicySettings(policy)
	}

	orgHandlers := organization.NewHandlers(orgRepo, calcRepo)

	attendanceRepo := attendance.NewRepository(db)
	attendanceHandlers := attendance.NewHandlers(attendanceRepo, faceClient, authRepo, calcRepo, cfg.StoragePath)

	// The dashboard reports through the engine rather than querying raw events,
	// so its numbers can't drift from what the shared attendance screen shows.
	dashboardRepo := dashboard.NewRepository(db, calcRepo, empRepo)
	dashboardHandlers := dashboard.NewHandlers(dashboardRepo)

	leaveRepo := leave.NewRepository(db)
	leaveHandlers := leave.NewHandlers(leaveRepo, calcRepo, cfg.StoragePath)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), middleware.CORS())

	router.GET("/health", health.Handler(db, rdb))

	api := router.Group("/api/v1")
	{
		authGroup := api.Group("/auth")
		authGroup.POST("/login", authHandlers.Login)
		authGroup.POST("/logout", authHandlers.Logout)
		authGroup.POST("/refresh", authHandlers.Refresh)
		authGroup.GET("/me", auth.RequireAuth(authRepo, sessions), authHandlers.Me)

		requireAuth := auth.RequireAuth(authRepo, sessions)
		// Super Admin only, same as the original — the literal role, not a
		// delegable permission. Departments/Positions/Designations CRUD is
		// deliberately not mounted at all in this duplicate (those tables
		// stay, pre-seeded with one sensible default each, just with no
		// management UI) — only Shifts + Calculation Settings are exposed.
		requireSettings := auth.RequireRole("SUPER_ADMIN")

		// Any signed-in user can see the clock — the same time every
		// attendance calculation already uses, nothing sensitive about it.
		api.GET("/clock", requireAuth, clock.Handler)

		shifts := api.Group("/shifts", requireAuth)
		shifts.GET("", orgHandlers.ListShifts)
		shifts.POST("", requireSettings, orgHandlers.CreateShift)
		shifts.PUT("/:id", requireSettings, orgHandlers.UpdateShift)
		shifts.PATCH("/:id/active", requireSettings, orgHandlers.SetShiftActive)

		// Departments/Positions/Designations: any signed-in user can read
		// them (the Employee form's own dropdowns need them to populate),
		// only a Super Admin can manage them — same split as Shifts.
		departments := api.Group("/departments", requireAuth)
		departments.GET("", orgHandlers.ListDepartments)
		departments.POST("", requireSettings, orgHandlers.CreateDepartment)
		departments.PUT("/:id", requireSettings, orgHandlers.UpdateDepartment)
		departments.PATCH("/:id/active", requireSettings, orgHandlers.SetDepartmentActive)

		positions := api.Group("/positions", requireAuth)
		positions.GET("", orgHandlers.ListPositions)
		positions.POST("", requireSettings, orgHandlers.CreatePosition)
		positions.PUT("/:id", requireSettings, orgHandlers.UpdatePosition)
		positions.PATCH("/:id/active", requireSettings, orgHandlers.SetPositionActive)

		designations := api.Group("/designations", requireAuth)
		designations.GET("", orgHandlers.ListDesignations)
		designations.POST("", requireSettings, orgHandlers.CreateDesignation)
		designations.PUT("/:id", requireSettings, orgHandlers.UpdateDesignation)
		designations.PATCH("/:id/active", requireSettings, orgHandlers.SetDesignationActive)

		policySettings := api.Group("/organization/policy-settings", requireAuth, requireSettings)
		policySettings.GET("", orgHandlers.GetPolicySettings)
		policySettings.PUT("", orgHandlers.UpdatePolicySettings)

		// employees.view (read-only) vs employees.manage (add/edit) — a split
		// specific to this duplicate's two-tier model: Super Admin can add/
		// edit, the shared kiosk login can only view + register faces.
		requireEmployeesView := auth.RequirePermission("employees.view")
		requireEmployeesManage := auth.RequirePermission("employees.manage")
		employees := api.Group("/employees", requireAuth, requireEmployeesView)
		employees.GET("", empHandlers.List)
		employees.GET("/:id", empHandlers.Get)
		employees.POST("", requireEmployeesManage, empHandlers.Create)
		employees.PUT("/:id", requireEmployeesManage, empHandlers.Update)
		employees.PATCH("/:id/status", requireEmployeesManage, empHandlers.SetStatus)
		employees.PATCH("/:id/self-checkin", requireEmployeesManage, empHandlers.SetRemoteSelfCheckin)
		employees.POST("/:id/login", requireEmployeesManage, empHandlers.CreateLogin)

		employees.GET("/:id/documents", empDocHandlers.ListDocuments)
		employees.POST("/:id/documents/:type", requireEmployeesManage, empDocHandlers.UploadDocument)
		employees.GET("/:id/documents/:type", empDocHandlers.DownloadDocument)

		employees.POST("/:id/photo", requireEmployeesManage, faceHandlers.UploadPhoto)

		// Deliberately outside the employees-gated group above: an avatar
		// photo shows up in views that aren't all restricted to that
		// permission, so any signed-in user can fetch any employee's photo.
		api.GET("/employees/:id/face-photo", requireAuth, faceHandlers.GetPhoto)

		requireAttendance := auth.RequirePermission("attendance.manage")
		attendanceGroup := api.Group("/attendance", requireAuth, requireAttendance)
		// The kiosk's "Mark Attendance" action: employee picked first, then
		// a live-capture photo confirms a real person, not an auto-match.
		attendanceGroup.POST("", attendanceHandlers.Mark)
		attendanceGroup.GET("/face-warmup", attendanceHandlers.FaceWarmup)
		attendanceGroup.GET("/today", attendanceHandlers.ListToday)
		attendanceGroup.GET("/events/:id/photo", attendanceHandlers.GetPhoto)
		attendanceGroup.POST("/manual", attendanceHandlers.RecordManual)
		attendanceGroup.GET("/events", attendanceHandlers.ListForEmployeeOn)

		// The kiosk login's manual corrections land here pending a Super
		// Admin's approval (see RecordManual) — Super Admin only, same as
		// the Organization pages, no permission grant can substitute.
		requireSuperAdmin := auth.RequireRole("SUPER_ADMIN")
		attendanceGroup.GET("/manual-requests", requireSuperAdmin, attendanceHandlers.ListManualRequests)
		attendanceGroup.POST("/manual-requests/:id/approve", requireSuperAdmin, attendanceHandlers.ApproveManualRequest)
		attendanceGroup.POST("/manual-requests/:id/reject", requireSuperAdmin, attendanceHandlers.RejectManualRequest)

		// Reports: attendance IN/OUT history and the captured-photos viewer,
		// Super Admin only — the same event data Today's Attendance already
		// shows everyone, just over an arbitrary range instead of just today.
		attendanceGroup.GET("/report", requireSuperAdmin, attendanceHandlers.GetReport)
		attendanceGroup.GET("/history", calcHandlers.GetHistory)
		attendanceGroup.GET("/calculated", calcHandlers.GetCalculated)
		attendanceGroup.POST("/recalculate", calcHandlers.Recalculate)
		attendanceGroup.POST("/absence-override", auth.RequireRole("SUPER_ADMIN"), calcHandlers.SetAbsenceOverride)

		// Leave: apply uses the same shared-login + pick-your-name pattern as
		// Mark Attendance above (leave.request_own, held by SUPER_ADMIN and
		// the KIOSK/"Employee Panel" login — see migration 000053), approve/
		// reject is leave.approve (SUPER_ADMIN only today). Listing is open
		// to either so both sides can see status, not just whoever can act.
		requireLeaveRequest := auth.RequirePermission("leave.request_own")
		requireLeaveApprove := auth.RequirePermission("leave.approve")
		leaveGroup := api.Group("/leave", requireAuth)
		leaveGroup.GET("", auth.RequireAnyPermission("leave.request_own", "leave.approve", "leave.manage"), leaveHandlers.List)
		leaveGroup.POST("", requireLeaveRequest, leaveHandlers.Apply)
		leaveGroup.POST("/:id/approve", requireLeaveApprove, leaveHandlers.Approve)
		leaveGroup.POST("/:id/reject", requireLeaveApprove, leaveHandlers.Reject)
		leaveGroup.GET("/:id/document", auth.RequireAnyPermission("leave.request_own", "leave.approve", "leave.manage"), leaveHandlers.DownloadDocument)

		dashboardGroup := api.Group("/dashboard", requireAuth, requireAttendance)
		dashboardGroup.GET("/summary", dashboardHandlers.GetSummary)
		dashboardGroup.GET("/trend", dashboardHandlers.GetTrend)
		dashboardGroup.GET("/holidays", dashboardHandlers.GetUpcomingHolidays)
		dashboardGroup.GET("/birthdays", dashboardHandlers.GetUpcomingBirthdays)
		dashboardGroup.GET("/day-detail", dashboardHandlers.GetDayDetail)
	}

	addr := ":" + cfg.BackendPort
	slog.Info("starting server", "addr", addr)
	if err := router.Run(addr); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
