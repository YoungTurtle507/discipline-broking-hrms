package attendance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"hr-management/backend/internal/auth"
	"hr-management/backend/internal/calculation"
	"hr-management/backend/internal/clock"
	"hr-management/backend/internal/faceregistration"
)

type Handlers struct {
	repo   *Repository
	client *faceregistration.Client
	// authRepo backs the password confirmation on a manual entry (see
	// RecordManual) — a Super Admin skips it, anyone else must re-enter their
	// own login password.
	authRepo *auth.Repository
	// calc answers "is this before the employee's own shift end" (see
	// needsOutReason) — the same shift data (Saturday override included) the
	// calculation engine itself uses, so the kiosk's reason prompt and the
	// engine's own rules can never disagree about when a day is "over".
	calc *calculation.Repository
	// storagePath is where a capture's stamped photo is written — see photo.go.
	storagePath string
}

func NewHandlers(repo *Repository, client *faceregistration.Client, authRepo *auth.Repository, calc *calculation.Repository, storagePath string) *Handlers {
	return &Handlers{repo: repo, client: client, authRepo: authRepo, calc: calc, storagePath: storagePath}
}

// recalculateDay persists the engine's result for employeeID's date right
// after a new event changes it — Comp Off, the Half Day Comp Off halving,
// the Uninformed Leave penalty, and every other ledger-affecting consequence
// only take effect once a day is calculated-and-stored, so this keeps that
// step automatic instead of needing an admin to run Recalculate by hand
// every time. Narrower than the old lazy-sweep-on-view pattern this
// replaces the need for: it only ever recalculates the exact day a real
// event just landed on (today for a live scan, the backdated date for a
// manual correction) — never a whole range, never triggered by merely
// viewing a page, so it can't silently backfill penalties on untracked past
// days the way GetHistory's old behavior did. Best-effort: the attendance
// event itself is already recorded and is the primary fact, so a failure
// here is logged, never returned to the caller.
func (h *Handlers) recalculateDay(ctx context.Context, employeeID int64, date string) {
	if _, err := h.calc.CalculateAndStore(ctx, employeeID, date); err != nil {
		slog.Error("failed to auto-recalculate attendance day", "error", err, "employee_id", employeeID, "date", date)
	}
}

func attemptedByFrom(c *gin.Context) *int64 {
	if uc := auth.CurrentUser(c); uc != nil {
		return &uc.UserID
	}
	return nil
}

func readUploadedImage(c *gin.Context) ([]byte, string, error) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	imageBytes, err := io.ReadAll(file)
	return imageBytes, header.Filename, err
}

// parseOutReason reads the optional reason/note fields used when marking OUT
// before the employee's shift ends (see needsOutReason) — "HALF_DAY" (leaving
// for the rest of the day), "LUNCH" or "PERSONAL" (expected back today), or
// "EARLY_LEAVE" (final checkout for the day), plus a free-text note. Once
// their shift is over these are simply ignored if absent; nothing here
// enforces the requirement, that's needsOutReason's job.
func parseOutReason(c *gin.Context) (*string, string, error) {
	reason := strings.TrimSpace(c.PostForm("reason"))
	note := strings.TrimSpace(c.PostForm("note"))
	if reason == "" {
		return nil, note, nil
	}
	if !ValidOutReasons[reason] {
		return nil, "", fmt.Errorf("reason must be HALF_DAY, LUNCH, PERSONAL or EARLY_LEAVE")
	}
	if NoteRequiredReasons[reason] && note == "" {
		return nil, "", fmt.Errorf("a note is required for this reason")
	}
	return &reason, note, nil
}

// needsOutReason reports whether direction is OUT and this employee's shift
// hasn't ended yet — the kiosk asks why in that case, same rule regardless of
// clock time, weekday vs. Saturday hours, or which shift they're on.
func (h *Handlers) needsOutReason(ctx context.Context, direction string, employeeID int64) (bool, error) {
	if direction != "OUT" {
		return false, nil
	}
	return h.calc.IsBeforeShiftEnd(ctx, employeeID, clock.Now())
}

// reasonRequiredResponse is the body the frontend recognizes as "ask the
// person why they're going out, then resubmit the same photo with a reason" —
// returned without creating any event or logging an attempt, so nothing needs
// undoing once the reason comes back.
func reasonRequiredResponse(employeeID int64) gin.H {
	return gin.H{
		"error":           "a reason is required when marking out before your shift ends",
		"reason_required": true,
		"employee_id":     employeeID,
	}
}

// MinOutGapMinutes guards against the same visit being read as two events: a
// face scanned twice in quick succession (a camera glitch, someone lingering
// in frame, walking past again) would otherwise flip straight from IN to OUT
// seconds later. Requiring this much of a gap makes that far more likely to
// be caught than a genuine sub-10-minute day ever would be.
const MinOutGapMinutes = 10

// tooSoonToMarkOut reports whether direction is OUT within MinOutGapMinutes
// of the same employee's last event (their IN) today.
func tooSoonToMarkOut(direction string, lastEventTime *time.Time) bool {
	return direction == "OUT" && lastEventTime != nil && clock.Now().Sub(*lastEventTime) < MinOutGapMinutes*time.Minute
}

func tooSoonResponse(employeeID int64) gin.H {
	return gin.H{
		"error":       fmt.Sprintf("wait at least %d minutes after marking IN before marking OUT", MinOutGapMinutes),
		"employee_id": employeeID,
	}
}

// checkLiveCapture submits the photo to the face service for the
// live-capture check (one real, live face present — see
// faceregistration.Client.VerifyCapture) and writes the right error response
// itself on rejection, same shape as the old embed() helper.
func (h *Handlers) checkLiveCapture(c *gin.Context, imageBytes []byte, filename string) (*faceregistration.CaptureResult, bool) {
	result, err := h.client.VerifyCapture(imageBytes, filename)
	if err != nil {
		var faceErr *faceregistration.ErrFaceService
		if errors.As(err, &faceErr) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": faceErr.Message})
			return nil, false
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "face service unavailable"})
		return nil, false
	}
	return result, true
}

// Mark is attendance marking's one entry point: the employee is picked
// explicitly (by whoever's operating the kiosk) before the camera opens, so
// this never has to figure out who's in frame — only whether the photo shows
// one real, live person. Direction is always computed server-side from
// today's events — never taken from the request — so it's impossible to
// record two INs in a row no matter what the client sends. The accepted
// photo is stamped with date/direction/time and stored against the event it
// creates, so there's a visual record of exactly when and for whom it was
// taken.
func (h *Handlers) Mark(c *gin.Context) {
	employeeID, err := strconv.ParseInt(c.PostForm("employee_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid employee_id is required"})
		return
	}

	imageBytes, filename, err := readUploadedImage(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}

	attemptedBy := attemptedByFrom(c)
	ctx := c.Request.Context()

	direction, lastEventTime, err := h.repo.NextDirection(ctx, employeeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to determine next attendance direction"})
		return
	}
	if tooSoonToMarkOut(direction, lastEventTime) {
		c.JSON(http.StatusUnprocessableEntity, tooSoonResponse(employeeID))
		return
	}

	outReason, note, err := parseOutReason(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if outReason == nil {
		needsReason, err := h.needsOutReason(ctx, direction, employeeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check shift timing"})
			return
		}
		if needsReason {
			c.JSON(http.StatusUnprocessableEntity, reasonRequiredResponse(employeeID))
			return
		}
	}

	result, ok := h.checkLiveCapture(c, imageBytes, filename)
	if !ok {
		_ = h.repo.LogAttempt(ctx, employeeID, direction, false, 0, "face service rejected image", attemptedBy)
		return
	}

	eventTime := clock.Now()
	stamped, err := stampPhoto(imageBytes, direction, eventTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process captured photo"})
		return
	}
	photoRelPath := attendancePhotoPath(employeeID, eventTime)
	if err := saveAttendancePhoto(h.storagePath, photoRelPath, stamped); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save captured photo"})
		return
	}

	_ = h.repo.LogAttempt(ctx, employeeID, direction, true, result.LivenessScore, "", attemptedBy)

	event, err := h.repo.Create(ctx, employeeID, direction, eventTime, result.LivenessScore, result.ModelName, result.ModelVersion, attemptedBy, outReason, note, photoRelPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record attendance event"})
		return
	}
	today := eventTime.Format("2006-01-02")
	h.recalculateDay(ctx, employeeID, today)
	c.JSON(http.StatusCreated, event)
}

func (h *Handlers) ListToday(c *gin.Context) {
	events, err := h.repo.ListToday(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load today's attendance"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": events})
}

// reasonNote mirrors the frontend's OUT_REASON_LABEL — used only to make a
// duplicate-direction conflict message readable, e.g. "already has an OUT at
// 2:17 PM (Lunch)" instead of just a bare timestamp.
var reasonNote = map[string]string{
	"HALF_DAY":    "Half Day",
	"LUNCH":       "Lunch",
	"PERSONAL":    "Personal",
	"EARLY_LEAVE": "Early Leave",
	"LATE_COMING": "Late Coming",
}

// duplicateDirectionConflict reports why direction can't be recorded next to
// prev/next (either may be nil) — IN/OUT must strictly alternate, so two OUTs
// (or two INs) in a row back-to-back is always a mistake, whichever end of
// the pair was entered second. Returns "" when there's no conflict.
func duplicateDirectionConflict(direction string, prev, next *Event) string {
	describe := func(e *Event) string {
		when := e.EventTime.In(time.Local).Format("3:04 PM")
		if e.OutReason != nil {
			if label, ok := reasonNote[*e.OutReason]; ok {
				return fmt.Sprintf("%s at %s (%s)", e.Direction, when, label)
			}
		}
		return fmt.Sprintf("%s at %s", e.Direction, when)
	}
	if prev != nil && prev.Direction == direction {
		return fmt.Sprintf("This employee already has an %s — the next entry must be %s",
			describe(prev), oppositeDirection(direction))
	}
	if next != nil && next.Direction == direction {
		return fmt.Sprintf("This employee already has an %s right after this time — this entry must be %s",
			describe(next), oppositeDirection(direction))
	}
	return ""
}

func oppositeDirection(direction string) string {
	if direction == "IN" {
		return "OUT"
	}
	return "IN"
}

type manualEntryRequest struct {
	EmployeeID int64  `json:"employee_id"`
	Direction  string `json:"direction"`
	// Local wall-clock time of the scan the employee missed, "YYYY-MM-DDTHH:MM".
	EventTime string `json:"event_time"`
	Note      string `json:"note"`
	// Password re-confirms the acting admin's own identity before this
	// overrides what the cameras saw. Required for everyone except a Super
	// Admin, who already holds every permission unconditionally.
	Password string `json:"password"`
	// OutReason applies the same rules a live scan would have: on an IN,
	// "LATE_COMING" or "HALF_DAY"; on an OUT, anything in ValidOutReasons.
	// Optional — see ValidManualReason.
	OutReason string `json:"out_reason"`
}

// RecordManual lets an admin or HR enter a scan the employee forgot — almost
// always a check-out. It adds an event rather than editing one: the spec
// requires corrections to be additive so the original record survives, and the
// calculation engine then reads the completed day like any other.
func (h *Handlers) RecordManual(c *gin.Context) {
	uc := auth.CurrentUser(c)
	if uc == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	var req manualEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.EmployeeID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "employee_id is required"})
		return
	}
	if !ValidDirections[req.Direction] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "direction must be IN or OUT"})
		return
	}
	if !ValidManualReason(req.Direction, req.OutReason) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reason for this direction"})
		return
	}

	// Parsed in the server's local zone, which is the zone the shift times and
	// the rest of the attendance data are already interpreted in.
	eventTime, err := time.ParseInLocation("2006-01-02T15:04", req.EventTime, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event_time must look like 2026-09-15T18:30"})
		return
	}
	if eventTime.After(clock.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot record a scan in the future"})
		return
	}

	// A reason is required: a manual entry overrides what the cameras saw, so
	// the record has to say why it was added.
	if strings.TrimSpace(req.Note) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a reason is required for a manual entry"})
		return
	}

	// IN/OUT must keep strictly alternating — two OUTs (or two INs) in a row is
	// always a mistake (exactly what happened here: a "missed OUT" entered
	// after the employee already had one, from a lunch/personal break that
	// hadn't been closed with its IN yet). Checked against whichever event
	// falls immediately before and after this one chronologically, not just
	// "today", since a manual entry can backdate into any past day.
	prevEvent, nextEvent, err := h.repo.NeighboringEvents(c.Request.Context(), req.EmployeeID, eventTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check the existing entries"})
		return
	}
	if conflict := duplicateDirectionConflict(req.Direction, prevEvent, nextEvent); conflict != "" {
		c.JSON(http.StatusConflict, gin.H{"error": conflict})
		return
	}

	// A Super Admin already holds every permission unconditionally; anyone
	// else must re-confirm their own login password before this overrides
	// what the cameras saw.
	if !uc.IsSuperAdmin() {
		if req.Password == "" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "password is required", "password_required": true})
			return
		}
		ok, err := h.authRepo.VerifyPassword(c.Request.Context(), uc.UserID, req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify password"})
			return
		}
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "incorrect password"})
			return
		}
	}

	var reasonPtr *string
	if req.OutReason != "" {
		reasonPtr = &req.OutReason
	}
	event, err := h.repo.RecordManual(c.Request.Context(), req.EmployeeID, req.Direction, eventTime, strings.TrimSpace(req.Note), uc.UserID, reasonPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record the entry"})
		return
	}
	manualDate := eventTime.Format("2006-01-02")
	h.recalculateDay(c.Request.Context(), req.EmployeeID, manualDate)

	slog.Info("manual attendance entry recorded",
		"employee_id", req.EmployeeID,
		"direction", req.Direction,
		"event_time", eventTime,
		"entered_by", uc.UserID,
	)
	c.JSON(http.StatusCreated, event)
}

// ListForEmployeeOn shows what is already recorded for one employee on one
// date, so an admin can see the gap before filling it.
func (h *Handlers) ListForEmployeeOn(c *gin.Context) {
	employeeID, err := strconv.ParseInt(c.Query("employee_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid employee_id is required"})
		return
	}
	date := c.Query("date")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid date is required (YYYY-MM-DD)"})
		return
	}

	events, err := h.repo.EventsOn(c.Request.Context(), employeeID, date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load events"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": events})
}
