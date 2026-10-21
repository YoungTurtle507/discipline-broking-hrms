package clock

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler reports this software's current time — what every attendance
// calculation treats as "now" (see Now) — so a dashboard clock can display
// the same time the backend actually uses rather than the viewer's own
// unadjusted system clock.
func Handler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"time": Now()})
}
