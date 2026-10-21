package organization

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"hr-management/backend/internal/auth"
	"hr-management/backend/internal/calculation"
)

type Handlers struct {
	repo *Repository
	calc *calculation.Repository
}

func NewHandlers(repo *Repository, calc *calculation.Repository) *Handlers {
	return &Handlers{repo: repo, calc: calc}
}

func idParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}

// ---- Departments ----

type departmentRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

func (h *Handlers) ListDepartments(c *gin.Context) {
	items, err := h.repo.ListDepartments(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load departments"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"departments": items})
}

func (h *Handlers) CreateDepartment(c *gin.Context) {
	var req departmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	d, err := h.repo.CreateDepartment(c.Request.Context(), req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a department with that name already exists"})
		return
	}
	c.JSON(http.StatusCreated, d)
}

func (h *Handlers) UpdateDepartment(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req departmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	d, err := h.repo.UpdateDepartment(c.Request.Context(), id, req.Name, req.Description)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "department not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a department with that name already exists"})
		return
	}
	c.JSON(http.StatusOK, d)
}

type activeRequest struct {
	IsActive bool `json:"is_active"`
}

func (h *Handlers) SetDepartmentActive(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req activeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_active is required"})
		return
	}
	if err := h.repo.SetDepartmentActive(c.Request.Context(), id, req.IsActive); errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "department not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update department"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// ---- Positions ----

type positionRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

func (h *Handlers) ListPositions(c *gin.Context) {
	items, err := h.repo.ListPositions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load positions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"positions": items})
}

func (h *Handlers) CreatePosition(c *gin.Context) {
	var req positionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	p, err := h.repo.CreatePosition(c.Request.Context(), req.Title, req.Description)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a position with that title already exists"})
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *Handlers) UpdatePosition(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req positionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	p, err := h.repo.UpdatePosition(c.Request.Context(), id, req.Title, req.Description)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "position not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a position with that title already exists"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handlers) SetPositionActive(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req activeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_active is required"})
		return
	}
	if err := h.repo.SetPositionActive(c.Request.Context(), id, req.IsActive); errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "position not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update position"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// ---- Designations ----

type designationRequest struct {
	Title        string `json:"title" binding:"required"`
	Description  string `json:"description"`
	DepartmentID int64  `json:"department_id" binding:"required"`
}

func (h *Handlers) ListDesignations(c *gin.Context) {
	items, err := h.repo.ListDesignations(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load designations"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"designations": items})
}

func (h *Handlers) CreateDesignation(c *gin.Context) {
	var req designationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and department_id are required"})
		return
	}
	d, err := h.repo.CreateDesignation(c.Request.Context(), req.Title, req.Description, req.DepartmentID)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "that department already has a designation — each department has exactly one"})
		return
	}
	c.JSON(http.StatusCreated, d)
}

func (h *Handlers) UpdateDesignation(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req designationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and department_id are required"})
		return
	}
	d, err := h.repo.UpdateDesignation(c.Request.Context(), id, req.Title, req.Description, req.DepartmentID)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "designation not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "that department already has a designation — each department has exactly one"})
		return
	}
	c.JSON(http.StatusOK, d)
}

func (h *Handlers) SetDesignationActive(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req activeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_active is required"})
		return
	}
	if err := h.repo.SetDesignationActive(c.Request.Context(), id, req.IsActive); errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "designation not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update designation"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// ---- Shifts ----

type shiftRequest struct {
	Name               string `json:"name" binding:"required"`
	StartTime          string `json:"start_time" binding:"required"`
	EndTime            string `json:"end_time" binding:"required"`
	GracePeriodMinutes int    `json:"grace_period_minutes"`
	// Saturday fields are all optional; leaving one unset falls back to that
	// shift's regular weekday value for a Saturday (see ShiftInfo.ResolvedFor).
	SaturdayStartTime          *string `json:"saturday_start_time"`
	SaturdayEndTime            *string `json:"saturday_end_time"`
	SaturdayGracePeriodMinutes *int    `json:"saturday_grace_period_minutes"`
}

func (h *Handlers) ListShifts(c *gin.Context) {
	items, err := h.repo.ListShifts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load shifts"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"shifts": items})
}

func (h *Handlers) CreateShift(c *gin.Context) {
	var req shiftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, start_time, and end_time are required"})
		return
	}
	s, err := h.repo.CreateShift(c.Request.Context(), req.Name, req.StartTime, req.EndTime, req.GracePeriodMinutes,
		req.SaturdayStartTime, req.SaturdayEndTime, req.SaturdayGracePeriodMinutes)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a shift with that name already exists"})
		return
	}
	c.JSON(http.StatusCreated, s)
}

func (h *Handlers) UpdateShift(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req shiftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, start_time, and end_time are required"})
		return
	}
	s, err := h.repo.UpdateShift(c.Request.Context(), id, req.Name, req.StartTime, req.EndTime, req.GracePeriodMinutes,
		req.SaturdayStartTime, req.SaturdayEndTime, req.SaturdayGracePeriodMinutes)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "shift not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a shift with that name already exists"})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *Handlers) SetShiftActive(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req activeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_active is required"})
		return
	}
	if err := h.repo.SetShiftActive(c.Request.Context(), id, req.IsActive); errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "shift not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update shift"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// GetPolicySettings returns the calculation engine's admin-configurable
// thresholds — read straight from the live in-memory cache
// (calculation.CurrentPolicySettings), so this always matches exactly what
// the engine is actually using right now, not a possibly-stale database read.
func (h *Handlers) GetPolicySettings(c *gin.Context) {
	c.JSON(http.StatusOK, calculation.CurrentPolicySettings())
}

// policySettingsRequest mirrors calculation.PolicySettings field-for-field —
// a separate type only so the numeric bounds can be validated by name below
// before anything is persisted.
type policySettingsRequest struct {
	LateStreakLimit              int     `json:"late_streak_limit"`
	HalfDayCutoffHour            int     `json:"half_day_cutoff_hour"`
	HalfDayCutoffMinute          int     `json:"half_day_cutoff_minute"`
	AbsentCutoffHour             int     `json:"absent_cutoff_hour"`
	AbsentCutoffMinute           int     `json:"absent_cutoff_minute"`
	StayLatePassThresholdMinutes int     `json:"stay_late_pass_threshold_minutes"`
	UninformedLeavePenaltyDays   int     `json:"uninformed_leave_penalty_days"`
	EndOfShiftGraceMinutes       int     `json:"end_of_shift_grace_minutes"`
	LunchTimeLimitMinutes        int     `json:"lunch_time_limit_minutes"`
	LateBand1EndMinutes          int     `json:"late_band1_end_minutes"`
	LateBand1StreakLimit         int     `json:"late_band1_streak_limit"`
	LateBand2EndMinutes          int     `json:"late_band2_end_minutes"`
	LateBand2StreakLimit         int     `json:"late_band2_streak_limit"`
	LunchOTStreakLimit           int     `json:"lunch_ot_streak_limit"`
	LunchOTPenaltyDays           float64 `json:"lunch_ot_penalty_days"`
}

// UpdatePolicySettings validates and persists an edit to the calculation
// engine's thresholds, and — via calculation.Repository.SavePolicySettings —
// swaps the live in-memory cache to match immediately, so the new rule
// applies starting with the very next calculation.
func (h *Handlers) UpdatePolicySettings(c *gin.Context) {
	var req policySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.LateStreakLimit < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "late streak limit must be at least 1"})
		return
	}
	if req.HalfDayCutoffHour < 0 || req.HalfDayCutoffHour > 23 || req.HalfDayCutoffMinute < 0 || req.HalfDayCutoffMinute > 59 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "half day cutoff must be a valid time of day"})
		return
	}
	if req.AbsentCutoffHour < 0 || req.AbsentCutoffHour > 23 || req.AbsentCutoffMinute < 0 || req.AbsentCutoffMinute > 59 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "absent cutoff must be a valid time of day"})
		return
	}
	halfDay := req.HalfDayCutoffHour*60 + req.HalfDayCutoffMinute
	absent := req.AbsentCutoffHour*60 + req.AbsentCutoffMinute
	if absent < halfDay {
		c.JSON(http.StatusBadRequest, gin.H{"error": "absent cutoff must not be earlier than the half day cutoff"})
		return
	}
	if req.StayLatePassThresholdMinutes < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stay-late pass threshold cannot be negative"})
		return
	}
	if req.UninformedLeavePenaltyDays < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uninformed leave penalty cannot be negative"})
		return
	}
	if req.EndOfShiftGraceMinutes < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end of shift grace period cannot be negative"})
		return
	}
	if req.LunchTimeLimitMinutes < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lunch time limit must be at least 1 minute"})
		return
	}
	if req.LateBand1EndMinutes < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "late band 1 end must be at least 1 minute"})
		return
	}
	if req.LateBand1StreakLimit < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "late band 1 streak limit must be at least 1"})
		return
	}
	if req.LateBand2EndMinutes <= req.LateBand1EndMinutes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "late band 2 end must be later than late band 1 end"})
		return
	}
	if req.LateBand2StreakLimit < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "late band 2 streak limit must be at least 1"})
		return
	}
	if req.LunchOTStreakLimit < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lunch overtime streak limit must be at least 1"})
		return
	}
	if req.LunchOTPenaltyDays < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lunch overtime penalty cannot be negative"})
		return
	}

	settings := calculation.PolicySettings{
		LateStreakLimit:              req.LateStreakLimit,
		HalfDayCutoffHour:            req.HalfDayCutoffHour,
		HalfDayCutoffMinute:          req.HalfDayCutoffMinute,
		AbsentCutoffHour:             req.AbsentCutoffHour,
		AbsentCutoffMinute:           req.AbsentCutoffMinute,
		StayLatePassThresholdMinutes: req.StayLatePassThresholdMinutes,
		UninformedLeavePenaltyDays:   req.UninformedLeavePenaltyDays,
		EndOfShiftGraceMinutes:       req.EndOfShiftGraceMinutes,
		LunchTimeLimitMinutes:        req.LunchTimeLimitMinutes,
		LateBand1EndMinutes:          req.LateBand1EndMinutes,
		LateBand1StreakLimit:         req.LateBand1StreakLimit,
		LateBand2EndMinutes:          req.LateBand2EndMinutes,
		LateBand2StreakLimit:         req.LateBand2StreakLimit,
		LunchOTStreakLimit:           req.LunchOTStreakLimit,
		LunchOTPenaltyDays:           req.LunchOTPenaltyDays,
	}

	uc := auth.CurrentUser(c)
	if err := h.calc.SavePolicySettings(c.Request.Context(), settings, uc.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save calculation settings"})
		return
	}
	c.JSON(http.StatusOK, settings)
}
