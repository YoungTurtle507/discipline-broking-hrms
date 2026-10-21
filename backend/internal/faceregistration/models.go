package faceregistration

import "time"

var ValidSampleTypes = map[string]bool{
	"FRONT":      true,
	"LEFT":       true,
	"RIGHT":      true,
	"LIGHTING":   true,
	"ADDITIONAL": true,
}

type FaceSample struct {
	ID           int64     `json:"id"`
	EmployeeID   int64     `json:"employee_id"`
	SampleType   string    `json:"sample_type"`
	ModelName    string    `json:"model_name"`
	ModelVersion string    `json:"model_version"`
	QualityScore float64   `json:"quality_score"`
	IsActive     bool      `json:"is_active"`
	RegisteredBy *int64    `json:"registered_by"`
	CreatedAt    time.Time `json:"created_at"`
}

// EmbeddingResult is what the Python face service returns for a submitted image.
type EmbeddingResult struct {
	Embedding    []float64 `json:"embedding"`
	QualityScore float64   `json:"quality_score"`
	ModelName    string    `json:"model_name"`
	ModelVersion string    `json:"model_version"`
}

// MatchThreshold is a starting-point cosine-similarity cutoff for "same
// person" on L2-normalized ArcFace (buffalo_l) embeddings. This has not been
// tuned against real false-accept/false-reject data yet — that calibration
// is Phase 9's job — so treat this as provisional.
const MatchThreshold = 0.36

// CosineSimilarity assumes both vectors are already L2-normalized (as the
// face service returns them), so this is just their dot product. Exported so
// other packages (e.g. attendance) can compare against the same embeddings
// without duplicating the math.
func CosineSimilarity(a, b []float64) float64 {
	var dot float64
	for i := range a {
		dot += a[i] * b[i]
	}
	return dot
}
