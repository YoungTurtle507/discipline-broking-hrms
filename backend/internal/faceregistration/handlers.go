package faceregistration

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers owns the employee avatar photo — the one piece of this package
// that's never been about face matching (see photo.go). Attendance capture
// verification is a separate concern, called directly via Client from
// attendance.Handlers.
type Handlers struct {
	storagePath string
}

func NewHandlers(storagePath string) *Handlers {
	return &Handlers{storagePath: storagePath}
}

func employeeIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid employee id"})
		return 0, false
	}
	return id, true
}
