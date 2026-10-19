package storage

import (
	"context"

	"therapeutic-engine/internal/models"
)

func (r *Repo) CreatePsychologicalReview(
	ctx context.Context,
	rev *models.PsychologicalReview,
) error {
	return createPsychologicalReview(ctx, r.db, rev)
}

// CreatePsychologicalReview performs the insert through the caller's
// transaction so authorization, mutation, and the audit event commit together.
func (r *TxRepo) CreatePsychologicalReview(
	ctx context.Context,
	rev *models.PsychologicalReview,
) error {
	return createPsychologicalReview(ctx, r.q, rev)
}

func createPsychologicalReview(
	ctx context.Context,
	qtx DBTX,
	rev *models.PsychologicalReview,
) error {
	const q = `
INSERT INTO psychological_reviews (
	child_id, window_days, review_from, review_to,
	formulation, protective_factors, recommendations,
	created_at, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), $8)
RETURNING id, created_at;
`
	return qtx.QueryRowContext(
		ctx,
		q,
		rev.ChildID,
		rev.WindowDays,
		rev.ReviewFrom,
		rev.ReviewTo,
		rev.Formulation,
		rev.ProtectiveFactors,
		rev.Recommendations,
		rev.CreatedBy,
	).Scan(&rev.ID, &rev.CreatedAt)
}

func (r *Repo) ListPsychologicalReviewsByChild(
	ctx context.Context,
	childID int64,
) ([]*models.PsychologicalReview, error) {
	const q = `
SELECT
	id, child_id, window_days, review_from, review_to,
	formulation,
	COALESCE(protective_factors, '') AS protective_factors,
	COALESCE(recommendations, '') AS recommendations,
	created_at, created_by
FROM psychological_reviews
WHERE child_id = $1
ORDER BY created_at DESC;
`
	rows, err := r.db.QueryContext(ctx, q, childID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.PsychologicalReview
	for rows.Next() {
		var rev models.PsychologicalReview
		if err := rows.Scan(
			&rev.ID,
			&rev.ChildID,
			&rev.WindowDays,
			&rev.ReviewFrom,
			&rev.ReviewTo,
			&rev.Formulation,
			&rev.ProtectiveFactors,
			&rev.Recommendations,
			&rev.CreatedAt,
			&rev.CreatedBy,
		); err != nil {
			return nil, err
		}
		out = append(out, &rev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
