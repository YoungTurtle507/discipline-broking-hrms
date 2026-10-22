package attendance

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"hr-management/backend/internal/auth"
)

// ManualRequest is a kiosk-submitted correction waiting on a Super Admin's
// approval before it becomes a real attendance_events row — see
// RecordManual, which branches here only for a non-Super-Admin caller (a
// Super Admin's own correction still records immediately, nothing to
// approve when the approver is also the submitter).
type ManualRequest struct {
	ID              int64      `json:"id"`
	EmployeeID      int64      `json:"employee_id"`
	EmployeeName    string     `json:"employee_name"`
	Direction       string     `json:"direction"`
	EventTime       time.Time  `json:"event_time"`
	Note            string     `json:"note"`
	OutReason       *string    `json:"out_reason"`
	Status          string     `json:"status"`
	RequestedBy     int64      `json:"requested_by"`
	RequestedByName string     `json:"requested_by_name"`
	ReviewedBy      *int64     `json:"reviewed_by"`
	ReviewedAt      *time.Time `json:"reviewed_at"`
	CreatedEventID  *int64     `json:"created_event_id"`
	CreatedAt       time.Time  `json:"created_at"`
}

const (
	RequestStatusPending  = "PENDING"
	RequestStatusApproved = "APPROVED"
	RequestStatusRejected = "REJECTED"
)

func (r *Repository) CreateManualRequest(ctx context.Context, employeeID int64, direction string, eventTime time.Time, note string, outReason *string, requestedBy int64) (*ManualRequest, error) {
	var req ManualRequest
	err := r.db.QueryRow(ctx,
		`INSERT INTO attendance_manual_requests (employee_id, direction, event_time, note, out_reason, requested_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, employee_id, direction, event_time, note, out_reason, status, requested_by, reviewed_by, reviewed_at, created_event_id, created_at`,
		employeeID, direction, eventTime, note, outReason, requestedBy,
	).Scan(&req.ID, &req.EmployeeID, &req.Direction, &req.EventTime, &req.Note, &req.OutReason,
		&req.Status, &req.RequestedBy, &req.ReviewedBy, &req.ReviewedAt, &req.CreatedEventID, &req.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := r.fillRequestNames(ctx, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *Repository) fillRequestNames(ctx context.Context, req *ManualRequest) error {
	if err := r.db.QueryRow(ctx, `SELECT full_name FROM employees WHERE id = $1`, req.EmployeeID).Scan(&req.EmployeeName); err != nil {
		return err
	}
	return r.db.QueryRow(ctx, `SELECT full_name FROM users WHERE id = $1`, req.RequestedBy).Scan(&req.RequestedByName)
}

// ListManualRequests returns requests in the given status (typically
// PENDING for the Super Admin panel's main view), oldest first so the
// longest-waiting one surfaces at the top.
func (r *Repository) ListManualRequests(ctx context.Context, status string) ([]ManualRequest, error) {
	rows, err := r.db.Query(ctx,
		`SELECT mr.id, mr.employee_id, e.full_name, mr.direction, mr.event_time, mr.note, mr.out_reason,
		        mr.status, mr.requested_by, u.full_name, mr.reviewed_by, mr.reviewed_at, mr.created_event_id, mr.created_at
		 FROM attendance_manual_requests mr
		 JOIN employees e ON e.id = mr.employee_id
		 JOIN users u ON u.id = mr.requested_by
		 WHERE mr.status = $1
		 ORDER BY mr.created_at ASC`,
		status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ManualRequest{}
	for rows.Next() {
		var req ManualRequest
		if err := rows.Scan(&req.ID, &req.EmployeeID, &req.EmployeeName, &req.Direction, &req.EventTime, &req.Note, &req.OutReason,
			&req.Status, &req.RequestedBy, &req.RequestedByName, &req.ReviewedBy, &req.ReviewedAt, &req.CreatedEventID, &req.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

func (r *Repository) GetManualRequest(ctx context.Context, id int64) (*ManualRequest, error) {
	var req ManualRequest
	err := r.db.QueryRow(ctx,
		`SELECT mr.id, mr.employee_id, e.full_name, mr.direction, mr.event_time, mr.note, mr.out_reason,
		        mr.status, mr.requested_by, u.full_name, mr.reviewed_by, mr.reviewed_at, mr.created_event_id, mr.created_at
		 FROM attendance_manual_requests mr
		 JOIN employees e ON e.id = mr.employee_id
		 JOIN users u ON u.id = mr.requested_by
		 WHERE mr.id = $1`,
		id,
	).Scan(&req.ID, &req.EmployeeID, &req.EmployeeName, &req.Direction, &req.EventTime, &req.Note, &req.OutReason,
		&req.Status, &req.RequestedBy, &req.RequestedByName, &req.ReviewedBy, &req.ReviewedAt, &req.CreatedEventID, &req.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &req, nil
}

// ApproveManualRequest marks a request approved and links it to the event
// that was just created for it (see Handlers.ApproveManualRequest, which
// creates that event via the same RecordManual path a Super Admin's own
// direct correction uses, then calls this).
func (r *Repository) ApproveManualRequest(ctx context.Context, id, reviewedBy, createdEventID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE attendance_manual_requests SET status = $1, reviewed_by = $2, reviewed_at = now(), created_event_id = $3 WHERE id = $4`,
		RequestStatusApproved, reviewedBy, createdEventID, id,
	)
	return err
}

func (r *Repository) RejectManualRequest(ctx context.Context, id, reviewedBy int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE attendance_manual_requests SET status = $1, reviewed_by = $2, reviewed_at = now() WHERE id = $3`,
		RequestStatusRejected, reviewedBy, id,
	)
	return err
}

// ListManualRequests (handler) is the Super Admin panel's main feed —
// defaults to PENDING, the only status that needs action.
func (h *Handlers) ListManualRequests(c *gin.Context) {
	status := c.DefaultQuery("status", RequestStatusPending)
	requests, err := h.repo.ListManualRequests(c.Request.Context(), status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load requests"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"requests": requests})
}

func (h *Handlers) manualRequestIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return 0, false
	}
	return id, true
}

// ApproveManualRequest re-validates the correction against whatever's
// recorded *now* (not just at submission time — a conflicting event could
// have landed in between) before creating it, the same alternation check
// RecordManual itself applies to a direct entry.
func (h *Handlers) ApproveManualRequest(c *gin.Context) {
	uc := auth.CurrentUser(c)
	if uc == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	id, ok := h.manualRequestIDParam(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	req, err := h.repo.GetManualRequest(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load request"})
		return
	}
	if req.Status != RequestStatusPending {
		c.JSON(http.StatusConflict, gin.H{"error": "this request has already been reviewed"})
		return
	}

	prevEvent, nextEvent, err := h.repo.NeighboringEvents(ctx, req.EmployeeID, req.EventTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check the existing entries"})
		return
	}
	if conflict := duplicateDirectionConflict(req.Direction, prevEvent, nextEvent); conflict != "" {
		c.JSON(http.StatusConflict, gin.H{"error": conflict})
		return
	}

	event, err := h.repo.RecordManual(ctx, req.EmployeeID, req.Direction, req.EventTime, req.Note, uc.UserID, req.OutReason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record the entry"})
		return
	}
	if err := h.repo.ApproveManualRequest(ctx, id, uc.UserID, event.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "entry recorded, but failed to update the request"})
		return
	}

	manualDate := req.EventTime.Format("2006-01-02")
	h.recalculateDay(ctx, req.EmployeeID, manualDate)

	c.JSON(http.StatusOK, event)
}

func (h *Handlers) RejectManualRequest(c *gin.Context) {
	uc := auth.CurrentUser(c)
	if uc == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	id, ok := h.manualRequestIDParam(c)
	if !ok {
		return
	}
	req, err := h.repo.GetManualRequest(c.Request.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load request"})
		return
	}
	if req.Status != RequestStatusPending {
		c.JSON(http.StatusConflict, gin.H{"error": "this request has already been reviewed"})
		return
	}
	if err := h.repo.RejectManualRequest(c.Request.Context(), id, uc.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject the request"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "rejected"})
}
