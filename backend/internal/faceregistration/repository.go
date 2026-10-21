package faceregistration

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("face sample not found")

type Repository struct {
	db *pgxpool.Pool

	// allActiveCache holds the last AllActiveEmbeddings result, refreshed
	// on write (Register/Deactivate) and on TTL expiry. MarkAuto's 1:N
	// kiosk identification was re-querying and re-scanning every active
	// embedding for every currently-employed person on every ~1.5s poll
	// tick — this is what removes that round trip from the hot path.
	// The TTL exists only as a safety net for changes this package didn't
	// cause (e.g. an employee resigning, which flips them out of
	// AllActiveEmbeddings' WHERE clause elsewhere) — it bounds how long a
	// person no longer eligible could still be matched to a short window
	// rather than leaving it unbounded.
	cacheMu  sync.RWMutex
	cache    map[int64][][]float64
	cachedAt time.Time
}

const allActiveCacheTTL = 20 * time.Second

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Register deactivates any existing active sample of the same type for this
// employee, then inserts the new one as active — re-registration replaces
// what's currently used for verification while keeping prior samples for
// history, never deleting them.
func (r *Repository) Register(ctx context.Context, employeeID int64, sampleType string, embedding []float64, result EmbeddingResult, registeredBy *int64) (*FaceSample, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE face_registrations SET is_active = false WHERE employee_id = $1 AND sample_type = $2 AND is_active = true`,
		employeeID, sampleType,
	); err != nil {
		return nil, err
	}

	var s FaceSample
	err = tx.QueryRow(ctx,
		`INSERT INTO face_registrations
			(employee_id, sample_type, embedding, model_name, model_version, quality_score, registered_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, employee_id, sample_type, model_name, model_version, quality_score, is_active, registered_by, created_at`,
		employeeID, sampleType, embedding, result.ModelName, result.ModelVersion, result.QualityScore, registeredBy,
	).Scan(&s.ID, &s.EmployeeID, &s.SampleType, &s.ModelName, &s.ModelVersion, &s.QualityScore, &s.IsActive, &s.RegisteredBy, &s.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	r.invalidateAllActiveCache()
	return &s, nil
}

func (r *Repository) ListActive(ctx context.Context, employeeID int64) ([]FaceSample, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, employee_id, sample_type, model_name, model_version, quality_score, is_active, registered_by, created_at
		 FROM face_registrations WHERE employee_id = $1 AND is_active = true ORDER BY sample_type`,
		employeeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FaceSample
	for rows.Next() {
		var s FaceSample
		if err := rows.Scan(&s.ID, &s.EmployeeID, &s.SampleType, &s.ModelName, &s.ModelVersion, &s.QualityScore, &s.IsActive, &s.RegisteredBy, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ActiveEmbeddings returns the embeddings of every active sample for an
// employee, used to compare against a freshly captured verification photo.
func (r *Repository) ActiveEmbeddings(ctx context.Context, employeeID int64) ([][]float64, error) {
	rows, err := r.db.Query(ctx,
		`SELECT embedding FROM face_registrations WHERE employee_id = $1 AND is_active = true`,
		employeeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out [][]float64
	for rows.Next() {
		var emb []float64
		if err := rows.Scan(&emb); err != nil {
			return nil, err
		}
		out = append(out, emb)
	}
	return out, rows.Err()
}

// AllActiveEmbeddings returns every active embedding for every
// currently-employed person (excluding resigned/terminated), grouped by
// employee ID — used for 1:N identification where the employee isn't known
// up front, unlike ActiveEmbeddings which checks one specific employee.
//
// Cached for allActiveCacheTTL (see Repository.cache) since the kiosk's
// auto-scan flow calls this on every poll tick regardless of whether a face
// is even in frame yet.
func (r *Repository) AllActiveEmbeddings(ctx context.Context) (map[int64][][]float64, error) {
	r.cacheMu.RLock()
	if r.cache != nil && time.Since(r.cachedAt) < allActiveCacheTTL {
		cached := r.cache
		r.cacheMu.RUnlock()
		return cached, nil
	}
	r.cacheMu.RUnlock()

	out, err := r.loadAllActiveEmbeddings(ctx)
	if err != nil {
		return nil, err
	}

	r.cacheMu.Lock()
	r.cache = out
	r.cachedAt = time.Now()
	r.cacheMu.Unlock()

	return out, nil
}

func (r *Repository) loadAllActiveEmbeddings(ctx context.Context) (map[int64][][]float64, error) {
	rows, err := r.db.Query(ctx,
		`SELECT fr.employee_id, fr.embedding
		 FROM face_registrations fr
		 JOIN employees e ON e.id = fr.employee_id
		 WHERE fr.is_active = true AND e.status NOT IN ('RESIGNED', 'TERMINATED')`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64][][]float64{}
	for rows.Next() {
		var employeeID int64
		var emb []float64
		if err := rows.Scan(&employeeID, &emb); err != nil {
			return nil, err
		}
		out[employeeID] = append(out[employeeID], emb)
	}
	return out, rows.Err()
}

// invalidateAllActiveCache drops the cached AllActiveEmbeddings result so
// the next call reflects a just-written change immediately rather than
// waiting up to allActiveCacheTTL.
func (r *Repository) invalidateAllActiveCache() {
	r.cacheMu.Lock()
	r.cache = nil
	r.cacheMu.Unlock()
}

// Deactivate turns off a single active sample (e.g. an admin removing one bad
// sample without a full re-registration). It never deletes the row.
func (r *Repository) Deactivate(ctx context.Context, employeeID int64, sampleType string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE face_registrations SET is_active = false WHERE employee_id = $1 AND sample_type = $2 AND is_active = true`,
		employeeID, sampleType,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	r.invalidateAllActiveCache()
	return nil
}
