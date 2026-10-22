package leave

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"hr-management/backend/internal/auth"
	"hr-management/backend/internal/calculation"
)

type Handlers struct {
	repo *Repository
	calc *calculation.Repository
	// storagePath is where uploaded leave documents are written, under a
	// "leave-documents" subdirectory of it.
	storagePath string
}

func NewHandlers(repo *Repository, calc *calculation.Repository, storagePath string) *Handlers {
	return &Handlers{repo: repo, calc: calc, storagePath: storagePath}
}

// maxDocumentSize/allowedDocumentExt mirror employee.saveDocumentFile — the
// same "what a supporting document realistically is" allowlist.
const maxDocumentSize = 10 << 20

var allowedDocumentExt = map[string]bool{
	".pdf": true, ".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
}

func documentDir(storagePath string) string {
	return filepath.Join(storagePath, "leave-documents")
}

func saveDocumentFile(storagePath string, requestID int64, header *multipart.FileHeader, file multipart.File) (string, error) {
	if header.Size > maxDocumentSize {
		return "", errors.New("file must be 10MB or smaller")
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedDocumentExt[ext] {
		return "", errors.New("file must be a PDF, JPG, PNG, or WEBP")
	}

	dir := documentDir(storagePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Server-generated filename from the request id and a fixed extension
	// allowlist — nothing from the client's own filename reaches the path.
	filename := fmt.Sprintf("%d%s", requestID, ext)
	dst, err := os.Create(filepath.Join(dir, filename))
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}
	return filename, nil
}

// Apply submits a leave request on behalf of whichever employee was picked —
// a multipart form (not JSON) since a supporting document can be attached in
// the same step, the same one-shot pattern the attendance module's own
// out-reason + document upload already uses.
func (h *Handlers) Apply(c *gin.Context) {
	employeeID, err := strconv.ParseInt(c.PostForm("employee_id"), 10, 64)
	if err != nil || employeeID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "employee_id is required"})
		return
	}
	leaveType := c.PostForm("leave_type")
	if !ValidLeaveTypes[leaveType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "leave_type must be PAID, COMPOFF, or UNPAID"})
		return
	}
	startDate := c.PostForm("start_date")
	endDate := c.PostForm("end_date")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date are required"})
		return
	}
	if endDate < startDate {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_date cannot be before start_date"})
		return
	}
	reason := c.PostForm("reason")

	created, err := h.repo.Create(c.Request.Context(), employeeID, leaveType, startDate, endDate, reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create leave request"})
		return
	}

	if file, header, ferr := c.Request.FormFile("document"); ferr == nil {
		defer file.Close()
		filename, err := saveDocumentFile(h.storagePath, created.ID, header, file)
		if err != nil {
			// The request itself is already saved — a bad attachment doesn't
			// undo that, it's reported so the applicant can retry the upload.
			c.JSON(http.StatusCreated, gin.H{"request": created, "document_error": err.Error()})
			return
		}
		if err := h.repo.SetDocumentPath(c.Request.Context(), created.ID, filename); err != nil {
			c.JSON(http.StatusCreated, gin.H{"request": created, "document_error": "failed to record the upload"})
			return
		}
		created.HasDocument = true
	}

	c.JSON(http.StatusCreated, created)
}

func (h *Handlers) List(c *gin.Context) {
	var employeeID *int64
	if raw := c.Query("employee_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid employee_id"})
			return
		}
		employeeID = &id
	}
	status := c.Query("status")

	requests, err := h.repo.List(c.Request.Context(), employeeID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list leave requests"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"requests": requests})
}

func requestIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid leave request id"})
		return 0, false
	}
	return id, true
}

// decide is shared by Approve/Reject — the only difference between the two
// routes is which decision they pass in, so a mistaken request body can
// never flip which one actually runs.
func (h *Handlers) decide(c *gin.Context, decision string) {
	id, ok := requestIDParam(c)
	if !ok {
		return
	}
	uc := auth.CurrentUser(c)
	if uc == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	req, err := h.repo.GetByID(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "leave request not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load leave request"})
		return
	}

	err = h.repo.Decide(c.Request.Context(), id, decision, uc.UserID)
	switch {
	case errors.Is(err, ErrAlreadyDecided):
		c.JSON(http.StatusConflict, gin.H{"error": "leave request has already been decided"})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decide leave request"})
		return
	}

	// Approving changes what the calculation engine sees for every day in the
	// range (hasApprovedLeave now reports true), so each of those days is
	// recalculated right away rather than waiting for something else to touch
	// them — best-effort: the decision itself already succeeded and is the
	// primary fact, a recalculation failure here is logged, not returned.
	if decision == "APPROVED" {
		h.recalculateRange(c.Request.Context(), req.EmployeeID, req.StartDate, req.EndDate)
	}

	updated, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decision recorded, but failed to reload it"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handlers) recalculateRange(ctx context.Context, employeeID int64, startDate, endDate string) {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		slog.Error("leave approval: invalid start_date for recalculation", "employee_id", employeeID, "start_date", startDate)
		return
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		slog.Error("leave approval: invalid end_date for recalculation", "employee_id", employeeID, "end_date", endDate)
		return
	}
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		if _, err := h.calc.CalculateAndStore(ctx, employeeID, date); err != nil {
			slog.Error("failed to recalculate attendance day after leave approval", "error", err, "employee_id", employeeID, "date", date)
		}
	}
}

func (h *Handlers) Approve(c *gin.Context) { h.decide(c, "APPROVED") }
func (h *Handlers) Reject(c *gin.Context)  { h.decide(c, "REJECTED") }

// DownloadDocument serves a leave request's supporting document.
func (h *Handlers) DownloadDocument(c *gin.Context) {
	id, ok := requestIDParam(c)
	if !ok {
		return
	}
	req, err := h.repo.GetByID(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "leave request not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load leave request"})
		return
	}
	if req.DocumentPath == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no document has been uploaded for this leave request"})
		return
	}
	c.File(filepath.Join(documentDir(h.storagePath), *req.DocumentPath))
}
