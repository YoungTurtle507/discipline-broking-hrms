package faceregistration

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"hr-management/backend/internal/auth"
)

type Handlers struct {
	repo   *Repository
	client *Client
	// storagePath is where a FRONT sample's raw capture is written as the
	// employee's avatar photo — see photo.go.
	storagePath string
}

func NewHandlers(repo *Repository, client *Client, storagePath string) *Handlers {
	return &Handlers{repo: repo, client: client, storagePath: storagePath}
}

func employeeIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid employee id"})
		return 0, false
	}
	return id, true
}

// Register accepts a multipart image upload for one sample type (front,
// left, right, lighting, additional), runs it through the face service to
// get an embedding, and stores it — replacing any existing active sample of
// the same type.
func (h *Handlers) Register(c *gin.Context) {
	employeeID, ok := employeeIDParam(c)
	if !ok {
		return
	}

	sampleType := c.PostForm("sample_type")
	if !ValidSampleTypes[sampleType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sample_type"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}
	defer file.Close()

	imageBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read uploaded image"})
		return
	}

	result, err := h.client.Embed(imageBytes, header.Filename)
	if err != nil {
		var faceErr *ErrFaceService
		if errors.As(err, &faceErr) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": faceErr.Message})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "face service unavailable"})
		return
	}

	var registeredBy *int64
	if uc := auth.CurrentUser(c); uc != nil {
		registeredBy = &uc.UserID
	}

	sample, err := h.repo.Register(c.Request.Context(), employeeID, sampleType, result.Embedding, *result, registeredBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save face registration"})
		return
	}

	// The FRONT sample doubles as the employee's avatar photo — best-effort:
	// the registration itself (the embedding, already saved above) is what
	// actually matters for attendance, so a failure here never fails the
	// request, just leaves the avatar on its initials fallback.
	if sampleType == "FRONT" {
		if err := savePhoto(h.storagePath, employeeID, imageBytes); err != nil {
			slog.Warn("failed to save avatar photo", "employee_id", employeeID, "error", err)
		}
	}

	c.JSON(http.StatusCreated, sample)
}

// Verify captures a fresh photo and compares it against the employee's
// registered face samples, returning the best similarity score found. This
// is a standalone check for confirming registration quality — it does not
// record an attendance event (that's Phase 4).
func (h *Handlers) Verify(c *gin.Context) {
	employeeID, ok := employeeIDParam(c)
	if !ok {
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}
	defer file.Close()

	imageBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read uploaded image"})
		return
	}

	result, err := h.client.Embed(imageBytes, header.Filename)
	if err != nil {
		var faceErr *ErrFaceService
		if errors.As(err, &faceErr) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": faceErr.Message})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "face service unavailable"})
		return
	}

	stored, err := h.repo.ActiveEmbeddings(c.Request.Context(), employeeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load face registrations"})
		return
	}
	if len(stored) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "no registered face samples for this employee yet"})
		return
	}

	var best float64
	for _, emb := range stored {
		if sim := CosineSimilarity(result.Embedding, emb); sim > best {
			best = sim
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"matched":    best >= MatchThreshold,
		"confidence": best,
		"threshold":  MatchThreshold,
	})
}

func (h *Handlers) List(c *gin.Context) {
	employeeID, ok := employeeIDParam(c)
	if !ok {
		return
	}
	samples, err := h.repo.ListActive(c.Request.Context(), employeeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load face registrations"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"samples": samples})
}

func (h *Handlers) Deactivate(c *gin.Context) {
	employeeID, ok := employeeIDParam(c)
	if !ok {
		return
	}
	sampleType := c.Param("sampleType")
	if !ValidSampleTypes[sampleType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sample_type"})
		return
	}
	err := h.repo.Deactivate(c.Request.Context(), employeeID, sampleType)
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no active sample of that type"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to deactivate sample"})
	default:
		c.JSON(http.StatusOK, gin.H{"status": "deactivated"})
	}
}
