package employee

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"hr-management/backend/internal/auth"
)

// ValidDocumentTypes are the four documents HR keeps on file for an
// employee — Offer Letter, Aadhar Card, PAN Card, and a single Marksheet slot
// covering either the 10th or 12th (whichever the employee has), matching the
// employee_document_type enum. RequiredDocumentCount is len(ValidDocumentTypes) —
// an employee counts as "documents complete" once they have one of each.
var ValidDocumentTypes = map[string]bool{
	"OFFER_LETTER": true,
	"AADHAR_CARD":  true,
	"PAN_CARD":     true,
	"MARKSHEET":    true,
}

var RequiredDocumentCount = len(ValidDocumentTypes)

// maxDocumentSize/allowedDocumentExt mirror leave.SaveDocumentFile — the same
// "what a supporting document realistically is" allowlist, not tailored
// per-document-type since all three are scans or photos of paperwork.
const maxDocumentSize = 10 << 20

var allowedDocumentExt = map[string]bool{
	".pdf": true, ".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
}

func documentDir(storagePath string) string {
	return filepath.Join(storagePath, "employee-documents")
}

// saveDocumentFile validates and writes an uploaded employee document to
// disk. The filename is entirely server-generated from the employee id and
// document type plus a fixed extension allowlist — nothing from the client's
// own filename reaches the filesystem path.
func saveDocumentFile(storagePath string, employeeID int64, docType string, header *multipart.FileHeader, file multipart.File) (string, error) {
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
	filename := fmt.Sprintf("%d_%s%s", employeeID, docType, ext)
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

// DocumentInfo is what's shown on an employee's profile for one document
// slot — just enough to say "this one's on file" and when it was last
// replaced, without exposing the server-side filename.
type DocumentInfo struct {
	DocumentType string    `json:"document_type"`
	UploadedAt   time.Time `json:"uploaded_at"`
}

// SetDocument records (or replaces) the file on record for one employee's
// document slot — re-uploading overwrites the previous file on disk and this
// row in place, the same "one current file" model leave documents use.
func (r *Repository) SetDocument(ctx context.Context, employeeID int64, docType, fileName string, uploadedBy int64) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO employee_documents (employee_id, document_type, file_name, uploaded_at, uploaded_by)
		 VALUES ($1, $2, $3, now(), $4)
		 ON CONFLICT (employee_id, document_type)
		 DO UPDATE SET file_name = $3, uploaded_at = now(), uploaded_by = $4`,
		employeeID, docType, fileName, uploadedBy,
	)
	return err
}

// ListDocuments returns whichever of the three document slots this employee
// has on file, keyed by document type.
func (r *Repository) ListDocuments(ctx context.Context, employeeID int64) (map[string]DocumentInfo, error) {
	rows, err := r.db.Query(ctx,
		`SELECT document_type, uploaded_at FROM employee_documents WHERE employee_id = $1`,
		employeeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]DocumentInfo{}
	for rows.Next() {
		var info DocumentInfo
		if err := rows.Scan(&info.DocumentType, &info.UploadedAt); err != nil {
			return nil, err
		}
		out[info.DocumentType] = info
	}
	return out, rows.Err()
}

// GetDocumentFileName looks up the stored filename for one employee's
// document slot, nil if nothing has been uploaded there yet.
func (r *Repository) GetDocumentFileName(ctx context.Context, employeeID int64, docType string) (*string, error) {
	var fileName string
	err := r.db.QueryRow(ctx,
		`SELECT file_name FROM employee_documents WHERE employee_id = $1 AND document_type = $2`,
		employeeID, docType,
	).Scan(&fileName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &fileName, nil
}

// DocumentHandlers holds the storage path uploaded employee documents are
// written under — separate from Handlers since it's the only part of this
// package that touches the filesystem rather than just the database.
type DocumentHandlers struct {
	repo        *Repository
	storagePath string
}

func NewDocumentHandlers(repo *Repository, storagePath string) *DocumentHandlers {
	return &DocumentHandlers{repo: repo, storagePath: storagePath}
}

func docTypeParam(c *gin.Context) (string, bool) {
	t := strings.ToUpper(c.Param("type"))
	if !ValidDocumentTypes[t] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be OFFER_LETTER, AADHAR_CARD, PAN_CARD, or MARKSHEET"})
		return "", false
	}
	return t, true
}

// ListDocuments reports which of the four document slots an employee has on
// file, for the profile page to show "on file" / "not uploaded" per slot.
func (h *DocumentHandlers) ListDocuments(c *gin.Context) {
	employeeID, ok := idParam(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid employee id"})
		return
	}
	docs, err := h.repo.ListDocuments(c.Request.Context(), employeeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load documents"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"documents": docs})
}

// UploadDocument attaches (or replaces) one document slot for an employee.
func (h *DocumentHandlers) UploadDocument(c *gin.Context) {
	employeeID, ok := idParam(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid employee id"})
		return
	}
	docType, ok := docTypeParam(c)
	if !ok {
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a file is required"})
		return
	}
	defer file.Close()

	filename, err := saveDocumentFile(h.storagePath, employeeID, docType, header, file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uc := auth.CurrentUser(c)
	if err := h.repo.SetDocument(c.Request.Context(), employeeID, docType, filename, uc.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record the upload"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "uploaded"})
}

// DownloadDocument serves one of an employee's uploaded documents.
func (h *DocumentHandlers) DownloadDocument(c *gin.Context) {
	employeeID, ok := idParam(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid employee id"})
		return
	}
	docType, ok := docTypeParam(c)
	if !ok {
		return
	}

	filename, err := h.repo.GetDocumentFileName(c.Request.Context(), employeeID, docType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load the document"})
		return
	}
	if filename == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no document has been uploaded for this slot"})
		return
	}
	c.File(filepath.Join(documentDir(h.storagePath), *filename))
}
