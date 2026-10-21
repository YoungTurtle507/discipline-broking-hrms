package faceregistration

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // registers the PNG decoder with image.Decode
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func photoDir(storagePath string) string {
	return filepath.Join(storagePath, "face-photos")
}

func photoPath(storagePath string, employeeID int64) string {
	return filepath.Join(photoDir(storagePath), fmt.Sprintf("%d.jpg", employeeID))
}

// savePhoto writes the FRONT sample's capture to disk as the employee's
// avatar photo — always a JPEG (see captureFrameBlob on the frontend, the
// only source of these captures) and always at the same path, so a
// re-registration simply replaces it. Deliberately independent of the
// embedding/matching data (see Register): a deactivated FRONT sample doesn't
// remove this file — it's cosmetic only, never read for face matching, and
// only ever replaced by the next FRONT registration.
func savePhoto(storagePath string, employeeID int64, imageBytes []byte) error {
	dir := photoDir(storagePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(photoPath(storagePath, employeeID), imageBytes, 0o644)
}

// GetPhoto serves an employee's saved front-facing photo as their avatar —
// 404 if they've never registered a FRONT sample, so the frontend's <img>
// onError handler can fall back to the initials avatar. Any signed-in user
// may fetch any employee's photo by id: avatars appear across shared views
// (attendance feeds, approval lists, the topbar's own-user avatar) that
// aren't all gated behind employees.manage, and a face photo is no more
// sensitive than what's already visible in person or on the kiosk.
func (h *Handlers) GetPhoto(c *gin.Context) {
	employeeID, ok := employeeIDParam(c)
	if !ok {
		return
	}
	path := photoPath(h.storagePath, employeeID)
	if _, err := os.Stat(path); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.File(path)
}

// UploadPhoto lets an admin manually set (or replace) an employee's avatar
// photo directly — the exact same storage slot savePhoto/GetPhoto already
// use for a FRONT face-registration capture, so there's no separate "manual
// photo" concept layered on top: whichever was set most recently, a manual
// upload or a fresh FRONT registration, is simply what shows as the avatar.
// Accepts JPEG or PNG and always re-encodes to JPEG, so the file on disk (and
// its content type when served) is consistent regardless of what was
// uploaded.
func (h *Handlers) UploadPhoto(c *gin.Context) {
	employeeID, ok := employeeIDParam(c)
	if !ok {
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is not a supported image (JPEG or PNG)"})
		return
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process image"})
		return
	}

	if err := savePhoto(h.storagePath, employeeID, buf.Bytes()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save photo"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "saved"})
}
