package storage

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"therapeutic-engine/internal/models"
)

// ───── Filters ─────────────────────────────────────

type GoalFilters struct {
	ChildID int64
	Status  *models.GoalStatus
}

type TargetFilters struct {
	ChildID int64
	GoalID  *int64
	Status  *models.GoalStatus
}

// ───── Child goals ─────────────────────────────────

func (r *Repo) CreateChildGoal(ctx context.Context, g *models.ChildGoal) error {
	return createChildGoal(ctx, r.db, g)
}

func (r *TxRepo) CreateChildGoal(ctx context.Context, g *models.ChildGoal) error {
	return createChildGoal(ctx, r.q, g)
}

func createChildGoal(ctx context.Context, qtx DBTX, g *models.ChildGoal) error {
	const q = `
INSERT INTO child_goals (
  child_id, goal_type, title, domain, description, start_date, target_date, status,
  child_voice, formulation_summary, protective_factors
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING id, created_at, updated_at
`
	return qtx.QueryRowContext(
		ctx,
		q,
		g.ChildID,
		g.GoalType,
		g.Title,
		nullableStringPtr(g.Domain),
		nullableString(g.Description),
		nullableTime(g.StartDate),
		nullableTime(g.TargetDate),
		g.Status,
		nullableStringPtr(g.ChildVoice),
		nullableStringPtr(g.FormulationSummary),
		nullableStringPtr(g.ProtectiveFactors),
	).Scan(&g.ID, &g.CreatedAt, &g.UpdatedAt)
}

func (r *TxRepo) UpdateChildGoal(ctx context.Context, g *models.ChildGoal) error {
	const q = `
UPDATE child_goals
SET goal_type = $3,
    title = $4,
    domain = $5,
    description = $6,
    start_date = $7,
    target_date = $8,
    status = $9,
    child_voice = $10,
    formulation_summary = $11,
    protective_factors = $12,
    updated_at = NOW()
WHERE id = $1 AND child_id = $2
RETURNING created_at, updated_at
`
	return r.q.QueryRowContext(
		ctx,
		q,
		g.ID,
		g.ChildID,
		g.GoalType,
		g.Title,
		nullableStringPtr(g.Domain),
		nullableString(g.Description),
		nullableTime(g.StartDate),
		nullableTime(g.TargetDate),
		g.Status,
		nullableStringPtr(g.ChildVoice),
		nullableStringPtr(g.FormulationSummary),
		nullableStringPtr(g.ProtectiveFactors),
	).Scan(&g.CreatedAt, &g.UpdatedAt)
}

func (r *Repo) ListChildGoals(ctx context.Context, f GoalFilters) ([]models.ChildGoal, error) {
	q := `
SELECT id, child_id, goal_type, title, domain, COALESCE(description, ''),
       start_date, target_date, status,
       child_voice, formulation_summary, protective_factors,
       created_at, updated_at
FROM child_goals
WHERE child_id = $1
`
	args := []any{f.ChildID}

	if f.Status != nil {
		q += " AND status = $2"
		args = append(args, *f.Status)
	}
	q += " ORDER BY created_at ASC"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ChildGoal
	for rows.Next() {
		var g models.ChildGoal
		var startDate, targetDate sql.NullTime
		var domain, childVoice, formulationSummary, protectiveFactors sql.NullString
		if err := rows.Scan(
			&g.ID,
			&g.ChildID,
			&g.GoalType,
			&g.Title,
			&domain,
			&g.Description,
			&startDate,
			&targetDate,
			&g.Status,
			&childVoice,
			&formulationSummary,
			&protectiveFactors,
			&g.CreatedAt,
			&g.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if startDate.Valid {
			t := startDate.Time
			g.StartDate = &t
		}
		if domain.Valid {
			v := domain.String
			g.Domain = &v
		}
		if targetDate.Valid {
			t := targetDate.Time
			g.TargetDate = &t
		}
		if childVoice.Valid {
			v := childVoice.String
			g.ChildVoice = &v
		}
		if formulationSummary.Valid {
			v := formulationSummary.String
			g.FormulationSummary = &v
		}
		if protectiveFactors.Valid {
			v := protectiveFactors.String
			g.ProtectiveFactors = &v
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ───── Child targets ───────────────────────────────

func (r *Repo) CreateChildTarget(ctx context.Context, t *models.ChildTarget) error {
	return createChildTarget(ctx, r.db, t)
}

func (r *TxRepo) CreateChildTarget(ctx context.Context, t *models.ChildTarget) error {
	return createChildTarget(ctx, r.q, t)
}

func createChildTarget(ctx context.Context, qtx DBTX, t *models.ChildTarget) error {
	const q = `
INSERT INTO child_targets (
  child_id, goal_id, domain, short_label, description, status,
  barriers, supports_needed, intervention_strategy, review_date
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING id, created_at, updated_at
`
	return qtx.QueryRowContext(
		ctx,
		q,
		t.ChildID,
		nullableInt64Ptr(t.GoalID),
		t.Domain,
		t.ShortLabel,
		nullableString(t.Description),
		t.Status,
		nullableStringPtr(t.Barriers),
		nullableStringPtr(t.SupportsNeeded),
		nullableStringPtr(t.InterventionStrategy),
		nullableTime(t.ReviewDate),
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *TxRepo) UpdateChildTarget(ctx context.Context, t *models.ChildTarget) error {
	const q = `
UPDATE child_targets
SET goal_id = $3,
    domain = $4,
    short_label = $5,
    description = $6,
    status = $7,
    barriers = $8,
    supports_needed = $9,
    intervention_strategy = $10,
    review_date = $11,
    updated_at = NOW()
WHERE id = $1 AND child_id = $2
RETURNING created_at, updated_at
`
	return r.q.QueryRowContext(
		ctx,
		q,
		t.ID,
		t.ChildID,
		nullableInt64Ptr(t.GoalID),
		t.Domain,
		t.ShortLabel,
		nullableString(t.Description),
		t.Status,
		nullableStringPtr(t.Barriers),
		nullableStringPtr(t.SupportsNeeded),
		nullableStringPtr(t.InterventionStrategy),
		nullableTime(t.ReviewDate),
	).Scan(&t.CreatedAt, &t.UpdatedAt)
}

func (r *Repo) ListChildTargets(ctx context.Context, f TargetFilters) ([]models.ChildTarget, error) {
	q := `
SELECT target.id, target.child_id, target.goal_id, target.domain, target.short_label,
       COALESCE(description, ''),
       status, barriers, supports_needed, intervention_strategy, review_date,
	       created_at, updated_at,
	       (SELECT count(*) FROM child_daily_progress progress WHERE progress.target_id = target.id),
	       (SELECT count(*) FROM keyworking_sessions session
	        WHERE session.target_id = target.id AND session.child_id = target.child_id)
FROM child_targets target
WHERE target.child_id = $1
`
	args := []any{f.ChildID}

	argPos := 2
	if f.GoalID != nil {
		q += " AND goal_id = $" + itoa(argPos)
		args = append(args, *f.GoalID)
		argPos++
	}
	if f.Status != nil {
		q += " AND status = $" + itoa(argPos)
		args = append(args, *f.Status)
		argPos++
	}
	q += " ORDER BY created_at ASC"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ChildTarget
	for rows.Next() {
		var t models.ChildTarget
		var goalID sql.NullInt64
		var barriers, supportsNeeded, interventionStrategy sql.NullString
		var reviewDate sql.NullTime
		if err := rows.Scan(
			&t.ID,
			&t.ChildID,
			&goalID,
			&t.Domain,
			&t.ShortLabel,
			&t.Description,
			&t.Status,
			&barriers,
			&supportsNeeded,
			&interventionStrategy,
			&reviewDate,
			&t.CreatedAt,
			&t.UpdatedAt,
			&t.DailyObservationCount,
			&t.KeyworkingSessionCount,
		); err != nil {
			return nil, err
		}
		if goalID.Valid {
			id := goalID.Int64
			t.GoalID = &id
		}
		if barriers.Valid {
			v := barriers.String
			t.Barriers = &v
		}
		if supportsNeeded.Valid {
			v := supportsNeeded.String
			t.SupportsNeeded = &v
		}
		if interventionStrategy.Valid {
			v := interventionStrategy.String
			t.InterventionStrategy = &v
		}
		if reviewDate.Valid {
			v := reviewDate.Time
			t.ReviewDate = &v
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ───── small helpers ──────────────────────────────

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableStringPtr(s *string) any {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func nullableInt64Ptr(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
