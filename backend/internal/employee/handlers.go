package employee

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hr-management/backend/internal/auth"
)

// emailPattern is a standard "name@domain.tld" shape — the same format a
// gmail.com address follows, just not restricted to that one domain, since a
// company's staff may reasonably use any provider or their own company domain.
var emailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// phonePattern requires exactly 10 digits — no spaces, dashes, or country code.
var phonePattern = regexp.MustCompile(`^[0-9]{10}$`)

type Handlers struct {
	repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

func idParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}

func parseOptionalID(v string) *int64 {
	if v == "" {
		return nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

func (h *Handlers) List(c *gin.Context) {
	filters := ListFilters{
		DepartmentID:  parseOptionalID(c.Query("department_id")),
		DesignationID: parseOptionalID(c.Query("designation_id")),
		PositionID:    parseOptionalID(c.Query("position_id")),
		Status:        c.Query("status"),
	}
	items, err := h.repo.List(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load employees"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"employees": items})
}

func (h *Handlers) Get(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	e, err := h.repo.Get(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "employee not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load employee"})
		return
	}
	c.JSON(http.StatusOK, e)
}

func validateInput(in EmployeeInput) string {
	if in.FullName == "" {
		return "full name is required"
	}
	if in.Email != "" && !emailPattern.MatchString(in.Email) {
		return "email must be a properly formatted address, e.g. name@gmail.com"
	}
	if in.Phone == "" {
		return "phone number is required"
	}
	if !phonePattern.MatchString(in.Phone) {
		return "phone number must be exactly 10 digits"
	}
	if in.JoiningDate == "" {
		return "joining date is required"
	}
	// A designation is only meaningful alongside the department it belongs to
	// — the form always sends both together, so seeing one without the other
	// means something bypassed it.
	if in.DesignationID != nil && in.DepartmentID == nil {
		return "select a department before choosing a designation"
	}
	if in.DateOfBirth != "" {
		dob, err := time.Parse("2006-01-02", in.DateOfBirth)
		if err != nil {
			return "date of birth must be a valid date"
		}
		if dob.After(time.Now()) {
			return "date of birth cannot be in the future"
		}
	}
	return ""
}

func (h *Handlers) Create(c *gin.Context) {
	var in EmployeeInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if msg := validateInput(in); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	e, err := h.repo.Create(c.Request.Context(), in)
	if err != nil {
		writeCreateOrUpdateError(c, err)
		return
	}
	c.JSON(http.StatusCreated, e)
}

// writeCreateOrUpdateError distinguishes the two ways Create/Update can fail
// beyond a not-found: a bad designation/department pairing is the caller's
// mistake (400), anything else is presumed to be the email-uniqueness
// constraint, the only other thing the underlying SQL can reject on (409).
func writeCreateOrUpdateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrDesignationNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": "that designation does not exist"})
	case errors.Is(err, ErrDesignationDepartmentMismatch):
		c.JSON(http.StatusBadRequest, gin.H{"error": "that designation does not belong to the selected department"})
	case errors.Is(err, ErrShiftNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": "that shift does not exist"})
	default:
		c.JSON(http.StatusConflict, gin.H{"error": "an employee with that email already exists"})
	}
}

type createLoginRequest struct {
	Password string `json:"password" binding:"required"`
}

// CreateLogin issues a login account for an existing employee: username is the
// employee code, role is EMPLOYEE, password is set by the admin. Used for
// employees who were added before they needed to sign in.
func (h *Handlers) CreateLogin(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}

	var req createLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password is required"})
		return
	}
	if len(req.Password) < auth.MinPasswordLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	e, err := h.repo.CreateLogin(c.Request.Context(), id, hash)
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "employee not found"})
	case errors.Is(err, ErrLoginExists):
		c.JSON(http.StatusConflict, gin.H{"error": "this employee already has a login account"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create login account"})
	default:
		c.JSON(http.StatusCreated, e)
	}
}

func (h *Handlers) Update(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in EmployeeInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if msg := validateInput(in); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	e, err := h.repo.Update(c.Request.Context(), id, in)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "employee not found"})
		return
	}
	if err != nil {
		writeCreateOrUpdateError(c, err)
		return
	}
	c.JSON(http.StatusOK, e)
}

type statusRequest struct {
	Status         string  `json:"status" binding:"required"`
	LastWorkingDay *string `json:"last_working_day"`
}

func (h *Handlers) SetStatus(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req statusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
		return
	}
	if req.Status == "ON_NOTICE" && (req.LastWorkingDay == nil || *req.LastWorkingDay == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "last working day is required when marking an employee on notice"})
		return
	}
	err := h.repo.SetStatus(c.Request.Context(), id, req.Status, req.LastWorkingDay)
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "employee not found"})
	case errors.Is(err, ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status value"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update status"})
	default:
		c.JSON(http.StatusOK, gin.H{"status": "updated"})
	}
}

type remoteSelfCheckinRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *Handlers) SetRemoteSelfCheckin(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req remoteSelfCheckinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enabled is required"})
		return
	}
	if err := h.repo.SetRemoteSelfCheckin(c.Request.Context(), id, req.Enabled); errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "employee not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
