package faceregistration

// CaptureResult is what the Python face service returns for a submitted
// attendance photo: confirmation that one real, live face is present, not an
// identity (the employee is already known — picked before the camera opens).
type CaptureResult struct {
	QualityScore  float64 `json:"quality_score"`
	LivenessScore float64 `json:"liveness_score"`
	MinifasReal   float64 `json:"minifas_real_prob"`
	ModelName     string  `json:"model_name"`
	ModelVersion  string  `json:"model_version"`
}
