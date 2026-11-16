package faceregistration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Client talks to the Python face service for the live-capture check
// (detection + liveness + anti-spoofing). It no longer performs face
// matching — see CaptureResult.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	// Generous on purpose: a hosted face service on a free tier sleeps when
	// idle, and waking it plus loading its models takes well over the 15s
	// this used to allow — which surfaced as "face service unavailable" on
	// the first scan after a quiet spell.
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: 90 * time.Second}}
}

// Warm blocks until the face service answers /health or ctx expires, polling
// so a service that's still waking from sleep (where the host's proxy answers
// 502/503 or hangs) is retried rather than reported as down. Callers use it
// to wake the service before a scan is actually submitted.
func (c *Client) Warm(ctx context.Context) error {
	var lastErr error
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
		if err != nil {
			return err
		}
		resp, err := c.http.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("face service health returned %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("face service did not wake up: %w", lastErr)
		case <-time.After(3 * time.Second):
		}
	}
}

// ErrFaceService wraps a rejection from the face service (bad image, no
// face, liveness/anti-spoof failure, etc.) so handlers can surface the real
// reason.
type ErrFaceService struct {
	StatusCode int
	Message    string
}

func (e *ErrFaceService) Error() string { return e.Message }

// VerifyCapture submits a photo for the live-capture check — confirms
// exactly one real, live face is present. It does not identify who that is;
// the caller already knows (the employee is picked before the camera opens).
func (c *Client) VerifyCapture(imageBytes []byte, filename string) (*CaptureResult, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(imageBytes); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	payload := body.Bytes()
	contentType := writer.FormDataContentType()

	// One retry for transient failures only (connection error, or the host's
	// proxy answering 502/503/504 while the service is mid-wake). A real
	// rejection from the service itself (422 etc.) is final and never retried.
	var respBody []byte
	var status int
	for attempt := 0; attempt < 2; attempt++ {
		respBody, status, err = c.post(payload, contentType)
		transient := err != nil || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
		if !transient {
			break
		}
		if attempt == 0 {
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("face service unreachable: %w", err)
	}

	if status != http.StatusOK {
		var errResp struct {
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(respBody, &errResp)
		msg := errResp.Detail
		if msg == "" {
			msg = "face service request failed"
		}
		return nil, &ErrFaceService{StatusCode: status, Message: msg}
	}

	var result CaptureResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) post(payload []byte, contentType string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/verify-capture", bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return respBody, resp.StatusCode, nil
}
