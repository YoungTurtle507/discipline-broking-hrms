package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// allowedOrigins are the Next.js dev server origins allowed to call this API
// with credentials. Cookie-based sessions require an exact origin match, not
// a wildcard: browsers refuse credentialed requests against "*".
var allowedOrigins = map[string]bool{
	"http://localhost:3000":     true,
	"http://127.0.0.1:3000":     true,
	"http://192.168.4.36:3000":  true,
	"https://localhost:3000":    true,
	"https://127.0.0.1:3000":    true,
	"https://192.168.4.36:3000": true,
}

// CORS allows the Next.js frontend (a different origin in dev) to call this API
// directly from the browser, with cookies included.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if allowedOrigins[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
