package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const contextUserKey = "auth_user"

// RequireAuth resolves the session cookie to a live user and attaches it to
// the request context. It also slides the session's expiry forward, so an
// active user never gets logged out mid-use.
func RequireAuth(repo *Repository, sessions *SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie(sessionCookieName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		userID, issuedAt, ok := sessions.Lookup(c.Request.Context(), sessionID)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
			return
		}

		uc, err := repo.LoadUserContext(c.Request.Context(), userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user no longer active"})
			return
		}

		// A password reset ends every session issued before it, so an admin
		// resetting someone's password locks out whoever was using the old one.
		if issuedAt.Before(uc.PasswordChangedAt) {
			sessions.Delete(c.Request.Context(), sessionID)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "password was changed; please sign in again"})
			return
		}

		sessions.Touch(c.Request.Context(), sessionID)
		c.Set(contextUserKey, uc)
		c.Next()
	}
}

// RequirePermission gates a route on a specific permission code. Must run
// after RequireAuth.
func RequirePermission(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		uc := CurrentUser(c)
		if uc == nil || !uc.HasPermission(code) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}

// RequireAnyPermission gates a route on holding at least one of several
// permission codes — e.g. face-sample registration, which both the
// employees.manage (edit) and employees.view (read-only) roles should be
// able to reach even though only one of them can also edit the employee
// record itself.
func RequireAnyPermission(codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		uc := CurrentUser(c)
		if uc == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}
		for _, code := range codes {
			if uc.HasPermission(code) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
	}
}

// RequireRole gates a route on holding a specific role by name, literally —
// unlike RequirePermission, this does NOT let Super Admin's usual "bypasses
// every permission check" rule through. Use only where a module is
// deliberately scoped to one exact role and even Super Admin shouldn't reach
// it (e.g. the candidate/interview pipeline, which is Admin-only by design).
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		uc := CurrentUser(c)
		if uc == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}
		for _, r := range uc.Roles {
			if r == role {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
	}
}

func CurrentUser(c *gin.Context) *UserContext {
	val, exists := c.Get(contextUserKey)
	if !exists {
		return nil
	}
	uc, _ := val.(*UserContext)
	return uc
}
