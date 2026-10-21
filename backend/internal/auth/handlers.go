package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const sessionCookieName = "hr_session"

type Handlers struct {
	repo         *Repository
	sessions     *SessionStore
	limiter      *LoginLimiter
	cookieSecure bool
}

func NewHandlers(repo *Repository, sessions *SessionStore, limiter *LoginLimiter, cookieSecure bool) *Handlers {
	return &Handlers{repo: repo, sessions: sessions, limiter: limiter, cookieSecure: cookieSecure}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handlers) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}

	if h.limiter.TooManyAttempts(c.Request.Context(), req.Username) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failed attempts, try again later"})
		return
	}

	user, err := h.repo.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil || !user.IsActive || !CheckPassword(user.PasswordHash, req.Password) {
		h.limiter.RecordFailure(c.Request.Context(), req.Username)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	h.limiter.Reset(c.Request.Context(), req.Username)

	sessionID, err := h.sessions.Create(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	h.setSessionCookie(c, sessionID)

	uc, err := h.repo.LoadUserContext(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
		return
	}
	c.JSON(http.StatusOK, uc)
}

func (h *Handlers) Logout(c *gin.Context) {
	sessionID, err := c.Cookie(sessionCookieName)
	if err == nil {
		h.sessions.Delete(c.Request.Context(), sessionID)
	}
	c.SetCookie(sessionCookieName, "", -1, "/", "", h.cookieSecure, true)
	c.JSON(http.StatusOK, gin.H{"status": "logged out"})
}

// Refresh slides the session's expiry forward. With Redis-backed sessions the
// access cookie IS the session, so this is a lightweight keep-alive rather
// than a token exchange.
func (h *Handlers) Refresh(c *gin.Context) {
	sessionID, err := c.Cookie(sessionCookieName)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	if _, ok := h.sessions.UserID(c.Request.Context(), sessionID); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
		return
	}
	h.sessions.Touch(c.Request.Context(), sessionID)
	h.setSessionCookie(c, sessionID)
	c.JSON(http.StatusOK, gin.H{"status": "refreshed"})
}

func (h *Handlers) Me(c *gin.Context) {
	uc := CurrentUser(c)
	if uc == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	c.JSON(http.StatusOK, uc)
}

func (h *Handlers) setSessionCookie(c *gin.Context, sessionID string) {
	c.SetCookie(sessionCookieName, sessionID, int(h.sessions.TTL().Seconds()), "/", "", h.cookieSecure, true)
}
