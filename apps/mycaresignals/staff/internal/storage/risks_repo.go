package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
)

func (r *TxRepo) CreateChildRisk(ctx context.Context, risk *models.ChildRisk) error {
	const q = `
INSERT INTO child_risks (
  child_id, title, category, domain, status, nature_of_risk, who_is_at_risk,
  information_status, source_note, initial_likelihood, initial_severity,
  residual_likelihood, residual_severity, management_approach,
  proactive_strategies, reactive_strategies, effective_from, last_reviewed_at,
  next_review_date
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
  $17, $18, $19
)
RETURNING id, created_at, updated_at
`
	return r.q.QueryRowContext(ctx, q, riskWriteArgs(risk)...).Scan(
		&risk.ID, &risk.CreatedAt, &risk.UpdatedAt,
	)
}

func (r *TxRepo) UpdateChildRisk(ctx context.Context, risk *models.ChildRisk) error {
	const q = `
UPDATE child_risks
SET title = $3,
    category = $4,
    domain = $5,
    status = $6,
    nature_of_risk = $7,
    who_is_at_risk = $8,
    information_status = $9,
    source_note = $10,
    initial_likelihood = $11,
    initial_severity = $12,
    residual_likelihood = $13,
    residual_severity = $14,
    management_approach = $15,
    proactive_strategies = $16,
    reactive_strategies = $17,
    effective_from = $18,
    next_review_date = $19,
    updated_at = now()
WHERE id = $1 AND child_id = $2
RETURNING created_at, updated_at
`
	args := []any{
		risk.ID, risk.ChildID, risk.Title, risk.Category, nullableStringPtr(risk.Domain),
		risk.Status, risk.NatureOfRisk, nullableStringPtr(risk.WhoIsAtRisk),
		risk.InformationStatus, nullableStringPtr(risk.SourceNote),
		nullableIntPtr(risk.InitialLikelihood), nullableIntPtr(risk.InitialSeverity),
		nullableIntPtr(risk.ResidualLikelihood), nullableIntPtr(risk.ResidualSeverity),
		risk.ManagementApproach, nullableStringPtr(risk.ProactiveStrategies),
		nullableStringPtr(risk.ReactiveStrategies), nullableTime(risk.EffectiveFrom),
		nullableTime(risk.NextReviewDate),
	}
	return r.q.QueryRowContext(ctx, q, args...).Scan(&risk.CreatedAt, &risk.UpdatedAt)
}

func riskWriteArgs(risk *models.ChildRisk) []any {
	return []any{
		risk.ChildID,
		risk.Title,
		risk.Category,
		nullableStringPtr(risk.Domain),
		risk.Status,
		risk.NatureOfRisk,
		nullableStringPtr(risk.WhoIsAtRisk),
		risk.InformationStatus,
		nullableStringPtr(risk.SourceNote),
		nullableIntPtr(risk.InitialLikelihood),
		nullableIntPtr(risk.InitialSeverity),
		nullableIntPtr(risk.ResidualLikelihood),
		nullableIntPtr(risk.ResidualSeverity),
		risk.ManagementApproach,
		nullableStringPtr(risk.ProactiveStrategies),
		nullableStringPtr(risk.ReactiveStrategies),
		nullableTime(risk.EffectiveFrom),
		nullableTime(risk.LastReviewedAt),
		nullableTime(risk.NextReviewDate),
	}
}

func (r *Repo) ListChildRisks(ctx context.Context, childID int64) ([]models.ChildRisk, error) {
	const q = `
SELECT id, child_id, title, category, domain, status, nature_of_risk,
       who_is_at_risk, information_status, source_note,
       initial_likelihood, initial_severity, residual_likelihood,
       residual_severity, management_approach, proactive_strategies,
       reactive_strategies, effective_from, last_reviewed_at, next_review_date,
       created_at, updated_at
FROM child_risks
WHERE child_id = $1
ORDER BY CASE status WHEN 'active' THEN 0 WHEN 'paused' THEN 1 ELSE 2 END,
         updated_at DESC
`
	rows, err := r.db.QueryContext(ctx, q, childID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	risks := make([]models.ChildRisk, 0)
	for rows.Next() {
		var risk models.ChildRisk
		var domain, whoIsAtRisk, sourceNote sql.NullString
		var proactive, reactive sql.NullString
		var initialLikelihood, initialSeverity, residualLikelihood, residualSeverity sql.NullInt64
		var effectiveFrom, lastReviewedAt, nextReviewDate sql.NullTime
		if err := rows.Scan(
			&risk.ID, &risk.ChildID, &risk.Title, &risk.Category, &domain,
			&risk.Status, &risk.NatureOfRisk, &whoIsAtRisk, &risk.InformationStatus,
			&sourceNote, &initialLikelihood, &initialSeverity, &residualLikelihood,
			&residualSeverity, &risk.ManagementApproach, &proactive, &reactive,
			&effectiveFrom, &lastReviewedAt, &nextReviewDate,
			&risk.CreatedAt, &risk.UpdatedAt,
		); err != nil {
			return nil, err
		}
		risk.Domain = stringFromNull(domain)
		risk.WhoIsAtRisk = stringFromNull(whoIsAtRisk)
		risk.SourceNote = stringFromNull(sourceNote)
		risk.ProactiveStrategies = stringFromNull(proactive)
		risk.ReactiveStrategies = stringFromNull(reactive)
		risk.InitialLikelihood = intFromNull(initialLikelihood)
		risk.InitialSeverity = intFromNull(initialSeverity)
		risk.ResidualLikelihood = intFromNull(residualLikelihood)
		risk.ResidualSeverity = intFromNull(residualSeverity)
		risk.EffectiveFrom = timeFromNull(effectiveFrom)
		risk.LastReviewedAt = timeFromNull(lastReviewedAt)
		risk.NextReviewDate = timeFromNull(nextReviewDate)
		risk.GoalIDs = []int64{}
		risk.TargetIDs = []int64{}
		risks = append(risks, risk)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	goalLinks, err := r.listRiskLinkMap(ctx, "child_risk_goals", "goal_id", childID)
	if err != nil {
		return nil, err
	}
	targetLinks, err := r.listRiskLinkMap(ctx, "child_risk_targets", "target_id", childID)
	if err != nil {
		return nil, err
	}
	for i := range risks {
		if ids := goalLinks[risks[i].ID]; ids != nil {
			risks[i].GoalIDs = ids
		}
		if ids := targetLinks[risks[i].ID]; ids != nil {
			risks[i].TargetIDs = ids
		}
	}
	return risks, nil
}

func (r *Repo) listRiskLinkMap(ctx context.Context, table, linkColumn string, childID int64) (map[int64][]int64, error) {
	if (table != "child_risk_goals" || linkColumn != "goal_id") && (table != "child_risk_targets" || linkColumn != "target_id") {
		return nil, errorsUnsupportedRiskLink(table)
	}
	q := fmt.Sprintf(`SELECT risk_id, %s FROM %s WHERE child_id = $1 ORDER BY risk_id, %s`, linkColumn, table, linkColumn)
	rows, err := r.db.QueryContext(ctx, q, childID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := make(map[int64][]int64)
	for rows.Next() {
		var riskID, linkedID int64
		if err := rows.Scan(&riskID, &linkedID); err != nil {
			return nil, err
		}
		links[riskID] = append(links[riskID], linkedID)
	}
	return links, rows.Err()
}

func errorsUnsupportedRiskLink(table string) error {
	return fmt.Errorf("unsupported risk link table %q", table)
}

func (r *TxRepo) LockRiskBelongsToChild(ctx context.Context, riskID, childID int64) (bool, error) {
	return lockResourceBelongsToChild(ctx, r.q, "child_risks", riskID, childID)
}

func (r *TxRepo) LockChildRiskForReview(ctx context.Context, riskID, childID int64) (*models.ChildRisk, error) {
	const q = `
SELECT id, child_id, status, residual_likelihood, residual_severity, next_review_date
FROM child_risks
WHERE id = $1 AND child_id = $2
FOR UPDATE`
	var risk models.ChildRisk
	var likelihood, severity sql.NullInt64
	var nextReviewDate sql.NullTime
	if err := r.q.QueryRowContext(ctx, q, riskID, childID).Scan(
		&risk.ID, &risk.ChildID, &risk.Status, &likelihood, &severity, &nextReviewDate,
	); err != nil {
		return nil, err
	}
	risk.ResidualLikelihood = intFromNull(likelihood)
	risk.ResidualSeverity = intFromNull(severity)
	risk.NextReviewDate = timeFromNull(nextReviewDate)
	return &risk, nil
}

func (r *TxRepo) CreateChildRiskReview(ctx context.Context, review *models.ChildRiskReview) error {
	const q = `
INSERT INTO child_risk_reviews (
  risk_id, child_id, reviewer_staff_id,
  previous_residual_likelihood, previous_residual_severity,
  reviewed_residual_likelihood, reviewed_residual_severity,
  review_note, outcome, next_review_date
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id, reviewed_at, created_at`
	return r.q.QueryRowContext(ctx, q,
		review.RiskID, review.ChildID, review.ReviewerStaffID,
		nullableIntPtr(review.PreviousResidualLikelihood), nullableIntPtr(review.PreviousResidualSeverity),
		review.ReviewedResidualLikelihood, review.ReviewedResidualSeverity,
		review.ReviewNote, review.Outcome, nullableTime(review.NextReviewDate),
	).Scan(&review.ID, &review.ReviewedAt, &review.CreatedAt)
}

func (r *TxRepo) UpdateRiskFromReview(ctx context.Context, review *models.ChildRiskReview) error {
	const q = `
UPDATE child_risks
SET residual_likelihood = CASE WHEN $3 IN ('modify', 'close') THEN $4 ELSE residual_likelihood END,
    residual_severity = CASE WHEN $3 IN ('modify', 'close') THEN $5 ELSE residual_severity END,
    status = CASE WHEN $3 = 'close' THEN 'closed' ELSE status END,
    last_reviewed_at = $6,
    next_review_date = CASE
      WHEN $3 = 'close' THEN NULL
      WHEN $7::date IS NOT NULL THEN $7::date
      ELSE next_review_date
    END,
    updated_at = now()
WHERE id = $1 AND child_id = $2`
	result, err := r.q.ExecContext(ctx, q,
		review.RiskID, review.ChildID, review.Outcome,
		review.ReviewedResidualLikelihood, review.ReviewedResidualSeverity,
		review.ReviewedAt, nullableTime(review.NextReviewDate),
	)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated != 1 {
		return security.ErrInvalidRelationship
	}
	return nil
}

func (r *Repo) ListChildRiskReviews(ctx context.Context, childID, riskID int64) ([]models.ChildRiskReview, error) {
	const q = `
SELECT rr.id, rr.risk_id, rr.child_id, rr.reviewed_at,
       rr.reviewer_staff_id, su.display_name,
       rr.previous_residual_likelihood, rr.previous_residual_severity,
       rr.reviewed_residual_likelihood, rr.reviewed_residual_severity,
       rr.review_note, rr.outcome, rr.next_review_date, rr.created_at
FROM child_risk_reviews AS rr
JOIN staff_users AS su ON su.id = rr.reviewer_staff_id
WHERE rr.child_id = $1 AND rr.risk_id = $2
ORDER BY rr.reviewed_at DESC, rr.id DESC`
	rows, err := r.db.QueryContext(ctx, q, childID, riskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reviews := make([]models.ChildRiskReview, 0)
	for rows.Next() {
		var review models.ChildRiskReview
		var previousLikelihood, previousSeverity sql.NullInt64
		var nextReviewDate sql.NullTime
		if err := rows.Scan(
			&review.ID, &review.RiskID, &review.ChildID, &review.ReviewedAt,
			&review.ReviewerStaffID, &review.ReviewerDisplayName,
			&previousLikelihood, &previousSeverity,
			&review.ReviewedResidualLikelihood, &review.ReviewedResidualSeverity,
			&review.ReviewNote, &review.Outcome, &nextReviewDate, &review.CreatedAt,
		); err != nil {
			return nil, err
		}
		review.PreviousResidualLikelihood = intFromNull(previousLikelihood)
		review.PreviousResidualSeverity = intFromNull(previousSeverity)
		review.NextReviewDate = timeFromNull(nextReviewDate)
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (r *TxRepo) LockRiskPlanningLinksBelongToChild(ctx context.Context, childID int64, goalIDs, targetIDs []int64) (bool, error) {
	for _, goalID := range uniqueSortedIDs(goalIDs) {
		belongs, err := r.LockGoalBelongsToChild(ctx, goalID, childID)
		if err != nil || !belongs {
			return false, err
		}
	}
	for _, targetID := range uniqueSortedIDs(targetIDs) {
		belongs, err := r.LockTargetBelongsToChild(ctx, targetID, childID)
		if err != nil || !belongs {
			return false, err
		}
	}
	return true, nil
}

func (r *TxRepo) ListRiskPlanningLinkIDs(ctx context.Context, riskID int64) ([]int64, []int64, error) {
	goals, err := queryRiskLinkIDs(ctx, r.q, "child_risk_goals", "goal_id", riskID)
	if err != nil {
		return nil, nil, err
	}
	targets, err := queryRiskLinkIDs(ctx, r.q, "child_risk_targets", "target_id", riskID)
	return goals, targets, err
}

func queryRiskLinkIDs(ctx context.Context, qtx DBTX, table, column string, riskID int64) ([]int64, error) {
	q := fmt.Sprintf(`SELECT %s FROM %s WHERE risk_id = $1 ORDER BY %s FOR UPDATE`, column, table, column)
	rows, err := qtx.QueryContext(ctx, q, riskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *TxRepo) AddRiskGoalLink(ctx context.Context, riskID, childID, goalID int64) error {
	_, err := r.q.ExecContext(ctx, `INSERT INTO child_risk_goals (risk_id, child_id, goal_id) VALUES ($1, $2, $3)`, riskID, childID, goalID)
	return err
}

func (r *TxRepo) DeleteRiskGoalLink(ctx context.Context, riskID, goalID int64) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM child_risk_goals WHERE risk_id = $1 AND goal_id = $2`, riskID, goalID)
	return err
}

func (r *TxRepo) AddRiskTargetLink(ctx context.Context, riskID, childID, targetID int64) error {
	_, err := r.q.ExecContext(ctx, `INSERT INTO child_risk_targets (risk_id, child_id, target_id) VALUES ($1, $2, $3)`, riskID, childID, targetID)
	return err
}

func (r *TxRepo) DeleteRiskTargetLink(ctx context.Context, riskID, targetID int64) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM child_risk_targets WHERE risk_id = $1 AND target_id = $2`, riskID, targetID)
	return err
}

func uniqueSortedIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			seen[id] = struct{}{}
		}
	}
	out := make([]int64, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func nullableIntPtr(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func stringFromNull(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	out := value.String
	return &out
}

func intFromNull(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	out := int(value.Int64)
	return &out
}

func timeFromNull(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	out := value.Time
	return &out
}
