package storage

import (
	"context"

	"therapeutic-engine/internal/models"
)

// GetChildDomainIndex calculates per-domain indices and an overall index
// for a given child over the last windowDays (e.g. 28) days.
// It uses wellbeing_score and progress_score from session_target_scores
// joined to child_targets (for domain).
func (r *Repo) GetChildDomainIndex(
	ctx context.Context,
	childID int64,
	windowDays int,
) (*models.ChildDomainIndex, error) {

	if windowDays <= 0 {
		windowDays = 28
	}

	// --- 1) Current window per-domain averages ---
	const currentSQL = `
        SELECT
            t.domain,
            AVG(sc.wellbeing_score)::float8 AS avg_wb,
            AVG(sc.progress_score)::float8 AS avg_prog
        FROM keyworking_sessions s
        JOIN session_target_scores sc ON sc.session_id = s.id
        JOIN child_targets t          ON t.id = sc.target_id
        WHERE s.child_id = $1
          AND s.session_date >= (CURRENT_DATE - $2::int + 1)
          AND s.session_date < (CURRENT_DATE + INTERVAL '1 day')
          AND sc.wellbeing_score IS NOT NULL
          AND sc.progress_score IS NOT NULL
        GROUP BY t.domain
    `

	rows, err := r.db.QueryContext(ctx, currentSQL, childID, windowDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type tmpDomain struct {
		domain string
		wb     float64
		prog   float64
	}

	currentMap := make(map[string]tmpDomain)

	for rows.Next() {
		var d tmpDomain
		if err := rows.Scan(&d.domain, &d.wb, &d.prog); err != nil {
			return nil, err
		}
		currentMap[d.domain] = d
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// If no data at all, return an empty structure (no error).
	if len(currentMap) == 0 {
		return &models.ChildDomainIndex{
			ChildID:    childID,
			WindowDays: windowDays,
			Overall: models.OverallIndex{
				Index: 0,
			},
			Domains: []models.DomainIndex{},
		}, nil
	}

	// --- 2) Previous window per-domain averages (same length of time before) ---
	const previousSQL = `
        SELECT
            t.domain,
            AVG(sc.wellbeing_score)::float8 AS avg_wb,
            AVG(sc.progress_score)::float8 AS avg_prog
        FROM keyworking_sessions s
        JOIN session_target_scores sc ON sc.session_id = s.id
        JOIN child_targets t          ON t.id = sc.target_id
        WHERE s.child_id = $1
          AND s.session_date >= (CURRENT_DATE - ($2::int * 2) + 1)
          AND s.session_date < (CURRENT_DATE - $2::int + 1)
          AND sc.wellbeing_score IS NOT NULL
          AND sc.progress_score IS NOT NULL
        GROUP BY t.domain
    `

	prevRows, err := r.db.QueryContext(ctx, previousSQL, childID, windowDays)
	if err != nil {
		return nil, err
	}
	defer prevRows.Close()

	prevMap := make(map[string]tmpDomain)
	for prevRows.Next() {
		var d tmpDomain
		if err := prevRows.Scan(&d.domain, &d.wb, &d.prog); err != nil {
			return nil, err
		}
		prevMap[d.domain] = d
	}
	if err := prevRows.Err(); err != nil {
		return nil, err
	}

	// --- 3) Build DomainIndex slice + overall index ---
	var (
		domainIndices []models.DomainIndex
		sumIndex      float64
		sumPrevIndex  float64
		count         float64
		countPrev     float64
	)

	for domain, cur := range currentMap {
		// Simple 0–10 domain index as average of wellbeing + progress
		idx := (cur.wb + cur.prog) / 2.0

		var prevIdxPtr *float64
		var changePtr *float64

		if prev, ok := prevMap[domain]; ok {
			prevIdx := (prev.wb + prev.prog) / 2.0
			prevIdxPtr = &prevIdx
			diff := idx - prevIdx
			changePtr = &diff

			sumPrevIndex += prevIdx
			countPrev++
		}

		domainIndices = append(domainIndices, models.DomainIndex{
			Domain:           domain,
			AverageWellbeing: cur.wb,
			AverageProgress:  cur.prog,
			Index:            idx,
			PreviousIndex:    prevIdxPtr,
			Change:           changePtr,
		})

		sumIndex += idx
		count++
	}

	overallIdx := 0.0
	if count > 0 {
		overallIdx = sumIndex / count
	}

	var overallPrevPtr *float64
	var overallChangePtr *float64
	if countPrev > 0 {
		overallPrev := sumPrevIndex / countPrev
		overallPrevPtr = &overallPrev
		diff := overallIdx - overallPrev
		overallChangePtr = &diff
	}

	result := &models.ChildDomainIndex{
		ChildID:    childID,
		WindowDays: windowDays,
		Overall: models.OverallIndex{
			Index:         overallIdx,
			PreviousIndex: overallPrevPtr,
			Change:        overallChangePtr,
		},
		Domains: domainIndices,
	}

	return result, nil
}