package faceregistration

import (
	"bytes"
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
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: 15 * time.Second}}
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

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/verify-capture", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("face service unreachable: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(respBody, &errResp)
		msg := errResp.Detail
		if msg == "" {
			msg = "face service request failed"
		}
		return nil, &ErrFaceService{StatusCode: resp.StatusCode, Message: msg}
	}

	var result CaptureResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
