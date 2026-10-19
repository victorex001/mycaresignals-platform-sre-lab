package storage

import (
	"context"
	"fmt"
	"strings"

	"therapeutic-engine/internal/models"
)

func (r *Repo) CreateStakeholderProductFeedback(ctx context.Context, input models.StakeholderProductFeedbackInput) (*models.StakeholderProductFeedback, error) {
	if input.ReviewerStaffUserID <= 0 || input.OverallRating < 1 || input.OverallRating > 5 || strings.TrimSpace(input.Environment) != "demo" {
		return nil, fmt.Errorf("invalid stakeholder product feedback")
	}
	pagePath := safeFeedbackPagePath(input.PagePath)
	const query = `
INSERT INTO stakeholder_product_feedback (
  staff_user_id, reviewer_name, reviewer_email, organisation, environment, page_path,
  overall_rating, most_useful, unclear_or_difficult, missing, anything_else
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING id, reviewer_name, reviewer_email, COALESCE(organisation, ''),
          environment, page_path, overall_rating, COALESCE(most_useful, ''),
          COALESCE(unclear_or_difficult, ''), COALESCE(missing, ''),
          COALESCE(anything_else, ''), submitted_at`
	var item models.StakeholderProductFeedback
	if err := r.db.QueryRowContext(ctx, query,
		input.ReviewerStaffUserID,
		strings.TrimSpace(input.ReviewerName),
		strings.ToLower(strings.TrimSpace(input.ReviewerEmail)),
		nullableText(input.Organisation),
		"demo",
		pagePath,
		input.OverallRating,
		nullableText(input.MostUseful),
		nullableText(input.UnclearOrDifficult),
		nullableText(input.Missing),
		nullableText(input.AnythingElse),
	).Scan(
		&item.ID, &item.ReviewerName, &item.ReviewerEmail, &item.Organisation,
		&item.Environment, &item.PagePath, &item.OverallRating, &item.MostUseful,
		&item.UnclearOrDifficult, &item.Missing, &item.AnythingElse, &item.SubmittedAt,
	); err != nil {
		return nil, fmt.Errorf("create stakeholder product feedback: %w", err)
	}
	return &item, nil
}

func (r *Repo) ListStakeholderProductFeedback(ctx context.Context, limit int) ([]models.StakeholderProductFeedback, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	const query = `
SELECT id, reviewer_name, reviewer_email, COALESCE(organisation, ''),
       environment, page_path, overall_rating, COALESCE(most_useful, ''),
       COALESCE(unclear_or_difficult, ''), COALESCE(missing, ''),
       COALESCE(anything_else, ''), submitted_at
FROM stakeholder_product_feedback
ORDER BY submitted_at DESC, id DESC
LIMIT $1`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list stakeholder product feedback: %w", err)
	}
	defer rows.Close()
	items := make([]models.StakeholderProductFeedback, 0)
	for rows.Next() {
		var item models.StakeholderProductFeedback
		if err := rows.Scan(
			&item.ID, &item.ReviewerName, &item.ReviewerEmail, &item.Organisation,
			&item.Environment, &item.PagePath, &item.OverallRating, &item.MostUseful,
			&item.UnclearOrDifficult, &item.Missing, &item.AnythingElse, &item.SubmittedAt,
		); err != nil {
			return nil, fmt.Errorf("scan stakeholder product feedback: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stakeholder product feedback: %w", err)
	}
	return items, nil
}

func nullableText(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func safeFeedbackPagePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	if len(value) > 240 {
		value = value[:240]
	}
	return value
}
