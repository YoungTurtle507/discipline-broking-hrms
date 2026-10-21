package calculation

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hr-management/backend/internal/auth"
)

type Handlers struct {
	repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

// GetHistory returns the stored calculation records for a range. Read-only.
//
// It used to back-fill any missing past day with CalculateAndStore, which syncs
// the leave ledger — so merely opening an employee's profile wrote
// unapproved-absence penalties for every untracked day before the system went
// live, silently destroying leave balances. Persisting now happens only where
// someone asks for it: POST /attendance/recalculate. For a live view of a range
// without side effects, use GetCalculated.
func (h *Handlers) GetHistory(c *gin.Context) {
	employeeID, err := strconv.ParseInt(c.Query("employee_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid employee_id is required"})
		return
	}
	from := c.Query("from")
	to := c.Query("to")
	if _, err := time.Parse("2006-01-02", from); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid from date is required (YYYY-MM-DD)"})
		return
	}
	if _, err := time.Parse("2006-01-02", to); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid to date is required (YYYY-MM-DD)"})
		return
	}

	stored, err := h.repo.GetStored(c.Request.Context(), employeeID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load attendance history"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": stored})
}

// Recalculate runs the engine for one employee-day and persists the result,
// including the leave-ledger entries that follow from it (comp-off earned, an
// unapproved-absence penalty). This is the only path that writes, and it is
// deliberately an explicit action rather than something a page view triggers.
func (h *Handlers) Recalculate(c *gin.Context) {
	employeeID, err := strconv.ParseInt(c.PostForm("employee_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid employee_id is required"})
		return
	}
	date := c.PostForm("date")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid date is required (YYYY-MM-DD)"})
		return
	}

	result, err := h.repo.CalculateAndStore(c.Request.Context(), employeeID, date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to calculate attendance"})
		return
	}
	c.JSON(http.StatusOK, result)
}

// SetAbsenceOverride excuses or un-excuses one employee-day's absence — e.g.
// the employee actually called in sick but never filed a leave request.
// Super Admin only, enforced by the route (auth.RequireRole("SUPER_ADMIN")),
// deliberately not delegable via the usual permission-code bypass. Treated
// identically to approved leave everywhere the engine's output is consumed —
// the leave-ledger penalty, Reports, Employee Performance — not just a flag
// on the Attendance History screen.
func (h *Handlers) SetAbsenceOverride(c *gin.Context) {
	employeeID, err := strconv.ParseInt(c.PostForm("employee_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid employee_id is required"})
		return
	}
	date := c.PostForm("date")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid date is required (YYYY-MM-DD)"})
		return
	}
	excuse := c.PostForm("excuse") == "true"
	note := c.PostForm("note")

	uc := auth.CurrentUser(c)
	result, err := h.repo.SetAbsenceOverride(c.Request.Context(), employeeID, date, note, uc.UserID, excuse)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update the absence override"})
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetCalculated returns an employee's day-by-day attendance for a range,
// computed on the fly and not persisted — the admin-side counterpart to an
// employee's own /me/attendance.
//
// GetHistory reads attendance_calculations, which only holds days somebody has
// explicitly recalculated, so it shows nothing on a system that has never run
// one. This computes from the raw events every time, which is the architecture's
// own rule: the daily result is a view over the events, not a separate record
// that has to be kept in step.
func (h *Handlers) GetCalculated(c *gin.Context) {
	employeeID, err := strconv.ParseInt(c.Query("employee_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid employee_id is required"})
		return
	}
	from := c.Query("from")
	to := c.Query("to")
	if _, err := time.Parse("2006-01-02", from); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid from date is required (YYYY-MM-DD)"})
		return
	}
	parsedTo, err := time.Parse("2006-01-02", to)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid to date is required (YYYY-MM-DD)"})
		return
	}
	parsedFrom, _ := time.Parse("2006-01-02", from)
	if parsedTo.Before(parsedFrom) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "to date cannot be before from date"})
		return
	}
	// Each day is a separate engine run, so cap the span rather than let an
	// unbounded range hammer the database.
	if parsedTo.Sub(parsedFrom) > 366*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date range cannot exceed one year"})
		return
	}

	results, err := h.repo.CalculateOnly(c.Request.Context(), employeeID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to calculate attendance"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}
