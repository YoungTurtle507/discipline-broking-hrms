package auth

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
)

// MinPasswordLength applies to both creating an account and resetting one.
const MinPasswordLength = 8

// usernamePattern requires a login username to be a proper email address
// (e.g. "hradmin@gmail.com") — a standard "name@domain.tld" shape.
var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// userIDParam parses the :id route param, answering the request itself on
// failure so callers can just return.
func userIDParam(c *gin.Context) (int64, bool) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return 0, false
	}
	return userID, true
}

// isSuperAdmin is a small helper so the privilege-escalation guards below
// read clearly at the call site.
func isSuperAdmin(uc *UserContext) bool {
	return uc.IsSuperAdmin()
}

func (h *Handlers) ListUsers(c *gin.Context) {
	users, err := h.repo.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

type createUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	FullName string `json:"full_name" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

func (h *Handlers) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username, password, full_name, and role are required"})
		return
	}
	if !AssignableRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}
	if !usernamePattern.MatchString(req.Username) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username must be a valid email address, e.g. name@gmail.com"})
		return
	}
	if len(req.Password) < MinPasswordLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	// Only a Super Admin can create another Super Admin — an ADMIN with
	// users.manage shouldn't be able to hand themselves (or anyone else) the
	// one role that bypasses every permission check.
	actor := CurrentUser(c)
	if req.Role == "SUPER_ADMIN" && (actor == nil || !isSuperAdmin(actor)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only a Super Admin can create another Super Admin"})
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user, err := h.repo.CreateUser(c.Request.Context(), req.Username, hash, req.FullName, req.Role)
	switch {
	case errors.Is(err, ErrUsernameTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
	case errors.Is(err, ErrRoleNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
	default:
		c.JSON(http.StatusCreated, user)
	}
}

type setActiveRequest struct {
	Active bool `json:"active"`
}

func (h *Handlers) SetUserActive(c *gin.Context) {
	userID, ok := userIDParam(c)
	if !ok {
		return
	}

	actor := CurrentUser(c)
	if actor != nil && actor.UserID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "you cannot deactivate your own account"})
		return
	}

	var req setActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "active is required"})
		return
	}

	if err := h.repo.SetUserActive(c.Request.Context(), userID, req.Active); err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		case errors.Is(err, ErrLastSuperAdmin):
			c.JSON(http.StatusConflict, gin.H{"error": "cannot deactivate the last active Super Admin"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type setRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

func (h *Handlers) SetUserRole(c *gin.Context) {
	userID, ok := userIDParam(c)
	if !ok {
		return
	}

	var req setRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role is required"})
		return
	}
	if !AssignableRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}

	actor := CurrentUser(c)
	if req.Role == "SUPER_ADMIN" && (actor == nil || !isSuperAdmin(actor)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only a Super Admin can grant the Super Admin role"})
		return
	}
	if actor != nil && actor.UserID == userID && req.Role != "SUPER_ADMIN" && isSuperAdmin(actor) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "you cannot change your own role away from Super Admin"})
		return
	}

	if err := h.repo.SetUserRole(c.Request.Context(), userID, req.Role); err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		case errors.Is(err, ErrRoleNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		case errors.Is(err, ErrLastSuperAdmin):
			c.JSON(http.StatusConflict, gin.H{"error": "cannot remove the last Super Admin"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update role"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type setPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// SetUserPassword lets an admin/super-admin give a user a new password. This is
// the whole password-recovery story: there is no email-based self-service
// reset, so a user who forgets their password asks an admin, who sets a new one
// here and tells them what it is. Nobody — including a super-admin — can read
// an existing password, because only its bcrypt hash is stored.
func (h *Handlers) SetUserPassword(c *gin.Context) {
	userID, ok := userIDParam(c)
	if !ok {
		return
	}

	var req setPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password is required"})
		return
	}
	if len(req.Password) < MinPasswordLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	// An ADMIN must not be able to reset a Super Admin's password — that would
	// let them take over the one account that bypasses every permission check.
	// Mirrors the CreateUser/SetUserRole escalation guards.
	actor := CurrentUser(c)
	targetIsSuper, err := h.repo.IsSuperAdmin(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}
	if targetIsSuper && (actor == nil || !isSuperAdmin(actor)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only a Super Admin can reset a Super Admin's password"})
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	if err := h.repo.SetUserPassword(c.Request.Context(), userID, hash); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}

	// Resetting your own password would invalidate the session you're using, so
	// say so rather than silently logging the admin out mid-task.
	selfReset := actor != nil && actor.UserID == userID
	c.JSON(http.StatusOK, gin.H{"status": "ok", "signed_out_everywhere": true, "self_reset": selfReset})
}
