package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type status struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	Cache    string `json:"cache"`
}

// Handler checks real connectivity to Postgres and Redis on every call,
// so a 200 response is proof the full stack is actually wired up correctly.
func Handler(db *pgxpool.Pool, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		dbStatus := "ok"
		if err := db.Ping(ctx); err != nil {
			dbStatus = "unreachable"
		}

		cacheStatus := "ok"
		if err := rdb.Ping(ctx).Err(); err != nil {
			cacheStatus = "unreachable"
		}

		overall := "ok"
		code := http.StatusOK
		if dbStatus != "ok" || cacheStatus != "ok" {
			overall = "degraded"
			code = http.StatusServiceUnavailable
		}

		c.JSON(code, status{Status: overall, Database: dbStatus, Cache: cacheStatus})
	}
}
