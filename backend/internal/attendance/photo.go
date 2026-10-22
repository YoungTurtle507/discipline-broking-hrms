package attendance

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png" // registers the PNG decoder with image.Decode
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// stampPhoto burns a readable "<date> · <IN/OUT> · <time>" bar onto the
// bottom of the captured photo, server-side — not left to the client, so the
// stamp reflects the same clock (and can't be edited) regardless of what the
// browser's local time says. Always re-encodes as JPEG, same as
// faceregistration's UploadPhoto, so the stored file format doesn't depend on
// what the camera happened to capture.
func stampPhoto(imageBytes []byte, direction string, eventTime time.Time) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf("decode captured photo: %w", err)
	}

	bounds := src.Bounds()
	stamped := image.NewRGBA(bounds)
	draw.Draw(stamped, bounds, src, bounds.Min, draw.Src)

	label := fmt.Sprintf("%s  %s  %s",
		eventTime.In(time.Local).Format("Jan 2, 2006"),
		direction,
		eventTime.In(time.Local).Format("3:04:05 PM"))

	const barHeight = 28
	barTop := bounds.Max.Y - barHeight
	bar := image.Rect(bounds.Min.X, barTop, bounds.Max.X, bounds.Max.Y)
	// 70% opaque black bar so the white text stays legible over any photo.
	draw.Draw(stamped, bar, image.NewUniform(color.RGBA{R: 0, G: 0, B: 0, A: 178}), image.Point{}, draw.Over)

	face := basicfont.Face7x13
	drawer := &font.Drawer{
		Dst:  stamped,
		Src:  image.NewUniform(color.White),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(bounds.Min.X + 8),
			Y: fixed.I(barTop + barHeight - 9),
		},
	}
	drawer.DrawString(label)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, stamped, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("encode stamped photo: %w", err)
	}
	return buf.Bytes(), nil
}

func attendancePhotoDir(storagePath string) string {
	return filepath.Join(storagePath, "attendance-photos")
}

// attendancePhotoPath builds a deterministic path before the event row
// exists (so it can be written in the same INSERT as everything else,
// rather than a save-then-update) — unique per employee+moment since
// clock.Now() always advances between scans and MinOutGapMinutes already
// prevents two events from landing in the same second for one employee.
func attendancePhotoPath(employeeID int64, eventTime time.Time) string {
	return filepath.Join(fmt.Sprintf("%d", employeeID), fmt.Sprintf("%d.jpg", eventTime.UnixNano()))
}

func saveAttendancePhoto(storagePath, relativePath string, stampedBytes []byte) error {
	fullPath := filepath.Join(attendancePhotoDir(storagePath), relativePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, stampedBytes, 0o644)
}

// GetPhoto serves one attendance event's stamped capture photo. Any
// signed-in user who can already see attendance data at all (same
// attendance.manage gate as the rest of this group) may fetch it — it's no
// more sensitive than the kiosk screen everyone already sees it captured on.
func (h *Handlers) GetPhoto(c *gin.Context) {
	eventID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event id"})
		return
	}
	relativePath, err := h.repo.PhotoPath(c.Request.Context(), eventID)
	if err != nil || relativePath == nil {
		c.Status(http.StatusNotFound)
		return
	}
	fullPath := filepath.Join(attendancePhotoDir(h.storagePath), *relativePath)
	if _, err := os.Stat(fullPath); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.File(fullPath)
}
