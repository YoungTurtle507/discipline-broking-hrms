package dashboard

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"hr-management/backend/internal/calculation"
	"hr-management/backend/internal/clock"
)

type Handlers struct {
	repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

func (h *Handlers) GetSummary(c *gin.Context) {
	summary, err := h.repo.Summary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load dashboard summary"})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *Handlers) GetTrend(c *gin.Context) {
	trend, err := h.repo.Trend(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load attendance trend"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"trend": trend})
}

func (h *Handlers) GetUpcomingHolidays(c *gin.Context) {
	limit := 5
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 20 {
			limit = n
		}
	}
	holidays, err := h.repo.UpcomingHolidays(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load holidays"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"holidays": holidays})
}

func (h *Handlers) GetUpcomingBirthdays(c *gin.Context) {
	birthdays, err := h.repo.UpcomingBirthdays(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load birthdays"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"birthdays": birthdays})
}

// GetDayDetail is the drill-down behind a dashboard stat card: which
// employees actually make up that count on that date. status is a
// comma-separated list of engine DayStatus values (e.g.
// "PRESENT,HALF_DAY" for the Present card, which reports both) — omitted
// entirely, every employee's result for the date comes back.
func (h *Handlers) GetDayDetail(c *gin.Context) {
	date := c.Query("date")
	if date == "" {
		date = clock.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date must look like 2026-09-17"})
		return
	}

	var wanted map[calculation.DayStatus]bool
	if raw := c.Query("status"); raw != "" {
		wanted = map[calculation.DayStatus]bool{}
		for _, s := range strings.Split(raw, ",") {
			if s = strings.TrimSpace(s); s != "" {
				wanted[calculation.DayStatus(s)] = true
			}
		}
	}

	entries, err := h.repo.DayDetail(c.Request.Context(), date, wanted)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load day detail"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"date": date, "employees": entries})
}
