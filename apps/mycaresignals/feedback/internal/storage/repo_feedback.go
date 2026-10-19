// internal/storage/repo_feedback.go
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"qr-feedback-api/internal/models"
)

// -----------------------------------------------------------------------------
// InsertFeedback – structured / safeguarding-aware insert
// -----------------------------------------------------------------------------

// InsertFeedback is used by the structured / safeguarding-aware handler.
func (r *Repo) InsertFeedback(
	ctx context.Context,
	in models.FeedbackIn,
	intakeContext *models.IntakeContext,
	metadata models.ReviewMetadata,
	ipHash string,
	ua string,
	answersJSON []byte,
	rawSubmission []byte,
) (string, time.Time, error) {
	if intakeContext == nil {
		return "", time.Time{}, ErrInvalidIntakeContext
	}
	matchedRules, err := json.Marshal(metadata.MatchedRuleIDs)
	if err != nil {
		return "", time.Time{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	defer func() { _ = tx.Rollback() }()

	const q = `
INSERT INTO feedback (
	source,
	who,
	reason,
	rating,
	what_went_well,
	what_to_improve,
	recommend,
	contact_name,
	contact_email,
	ip_hash,
	ua,
	safeguarding_flag,
	triage_status,
	feedback_type,
	answers,
	risk_level,
	intake_context_id,
	home_id,
	home_name,
	child_id,
	raw_submission,
	requires_human_review,
	review_priority,
	matched_rule_ids,
	rule_version
) VALUES (
	$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,FALSE,'New',$12,$13::jsonb,NULL,
	$14::uuid,$15,$16,$17,$18::jsonb,$19,$20,$21::jsonb,$22
)
RETURNING id::text, submitted_at;
`
	var id string
	var receivedAt time.Time

	err = tx.QueryRowContext(
		ctx,
		q,
		"qr",
		in.Who,
		nullIfEmpty(in.Reason),
		in.Rating,
		in.WhatWentWell,
		nullIfEmpty(in.WhatToImprove),
		in.Recommend,
		nullIfEmpty(in.ContactName),
		nullIfEmpty(in.ContactEmail),
		ipHash,
		ua,
		models.IntakeTypeVisitorFeedback,
		string(answersJSON), // JSON as text -> jsonb
		intakeContext.ID,
		intakeContext.HomeExternalID,
		intakeContext.HomeLabel,
		intakeContext.ChildExternalID,
		string(rawSubmission),
		metadata.RequiresHumanReview,
		metadata.ReviewPriority,
		string(matchedRules),
		metadata.RuleVersion,
	).Scan(&id, &receivedAt)
	if err != nil {
		return "", time.Time{}, err
	}
	if err := tx.Commit(); err != nil {
		return "", time.Time{}, err
	}

	return id, receivedAt, nil
}

// -----------------------------------------------------------------------------
// Admin list – richer list with feedback_type, answers, risk_level
// -----------------------------------------------------------------------------

func (r *Repo) ListFeedbackAdmin(
	ctx context.Context,
	filters models.FeedbackFilters,
) ([]models.FeedbackRow, error) {
	query := `
SELECT
	f.id::text, f.submitted_at::timestamptz, f.source, f.who, f.reason, f.rating,
	f.what_went_well, f.what_to_improve, f.recommend, f.safeguarding_flag,
	f.triage_status, f.feedback_type, f.answers, f.risk_level,
	f.requires_human_review, f.review_priority, f.matched_rule_ids, f.rule_version,
	COALESCE(latest.outcome, 'new'), COALESCE(latest.follow_up_status, 'none'), latest.reviewed_at,
	EXISTS (SELECT 1 FROM feedback_management_reviews history WHERE history.feedback_id = f.id AND history.outcome = 'follow_up_required')
FROM feedback f
LEFT JOIN LATERAL (
	SELECT outcome, follow_up_status, reviewed_at
	FROM feedback_management_reviews
	WHERE feedback_id = f.id
	ORDER BY reviewed_at DESC, id DESC
	LIMIT 1
) latest ON TRUE
WHERE f.feedback_type IN ('visitor_feedback', 'visitor')
`
	var args []any
	addFilter := func(clause string, value any) {
		args = append(args, value)
		query += fmt.Sprintf(" AND "+clause, len(args))
	}
	if filters.FeedbackID != "" {
		addFilter("f.id::text = $%d", filters.FeedbackID)
	}
	if filters.From != nil {
		addFilter("f.submitted_at >= $%d", *filters.From)
	}
	if filters.To != nil {
		addFilter("f.submitted_at < $%d", filters.To.Add(24*time.Hour))
	}
	if filters.Source != "" {
		addFilter("f.who = $%d", filters.Source)
	}
	if filters.Status != "" {
		addFilter("COALESCE(latest.outcome, 'new') = $%d", filters.Status)
	}
	if filters.RequiresReview != nil {
		addFilter("f.requires_human_review = $%d", *filters.RequiresReview)
	}

	query += " ORDER BY f.submitted_at DESC LIMIT 1000"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.FeedbackRow

	for rows.Next() {
		var row models.FeedbackRow
		var (
			reason       sql.NullString
			improve      sql.NullString
			rating       sql.NullInt32
			answers      json.RawMessage
			legacyRisk   sql.NullString
			matchedRules []byte
			reviewedAt   sql.NullTime
		)

		if err := rows.Scan(
			&row.ID,
			&row.SubmittedAt,
			&row.Source,
			&row.Who,
			&reason,
			&rating,
			&row.WhatWentWell,
			&improve,
			&row.Recommend,
			&row.Safeguarding,
			&row.TriageStatus,
			&row.FeedbackType,
			&answers,
			&legacyRisk,
			&row.RequiresHumanReview,
			&row.ReviewPriority,
			&matchedRules,
			&row.RuleVersion,
			&row.CurrentStatus,
			&row.FollowUpStatus,
			&reviewedAt,
			&row.HadFollowUp,
		); err != nil {
			return nil, err
		}

		if reason.Valid {
			s := reason.String
			row.Reason = &s
		}
		if improve.Valid {
			s := improve.String
			row.WhatToImprove = &s
		}
		if rating.Valid {
			v := int(rating.Int32)
			row.Rating = &v
		}

		row.Answers = answers
		if legacyRisk.Valid {
			value := legacyRisk.String
			row.RiskLevel = &value
		}
		if err := json.Unmarshal(matchedRules, &row.MatchedRuleIDs); err != nil {
			return nil, err
		}
		if reviewedAt.Valid {
			value := reviewedAt.Time
			row.ReviewedAt = &value
		}
		out = append(out, row)
	}

	return out, rows.Err()
}

func (r *Repo) GetFeedbackAdmin(ctx context.Context, feedbackID string) (*models.FeedbackRow, error) {
	items, err := r.ListFeedbackAdmin(ctx, models.FeedbackFilters{FeedbackID: feedbackID})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, sql.ErrNoRows
	}
	items[0].Reviews, err = r.ListFeedbackReviews(ctx, feedbackID)
	return &items[0], err
}

func (r *Repo) ListFeedbackReviews(ctx context.Context, feedbackID string) ([]models.FeedbackReview, error) {
	const query = `
SELECT id, feedback_id::text, reviewer_identity, reviewed_at, outcome,
       manager_note, follow_up_required, follow_up_status
FROM feedback_management_reviews
WHERE feedback_id = $1::uuid
ORDER BY reviewed_at, id`
	rows, err := r.db.QueryContext(ctx, query, feedbackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reviews := make([]models.FeedbackReview, 0)
	for rows.Next() {
		var review models.FeedbackReview
		if err := rows.Scan(&review.ID, &review.FeedbackID, &review.ReviewerIdentity,
			&review.ReviewedAt, &review.Outcome, &review.ManagerNote,
			&review.FollowUpRequired, &review.FollowUpStatus); err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (r *Repo) AddFeedbackReview(ctx context.Context, feedbackID, reviewer string, input models.FeedbackReviewInput) (*models.FeedbackReview, error) {
	followUpRequired, followUpStatus := feedbackReviewState(input.Outcome)
	const query = `
INSERT INTO feedback_management_reviews (
  feedback_id, reviewer_identity, outcome, manager_note, follow_up_required, follow_up_status
) VALUES ($1::uuid, $2, $3, $4, $5, $6)
RETURNING id, feedback_id::text, reviewer_identity, reviewed_at, outcome,
          manager_note, follow_up_required, follow_up_status`
	review := &models.FeedbackReview{}
	err := r.db.QueryRowContext(ctx, query, feedbackID, reviewer, input.Outcome,
		strings.TrimSpace(input.ManagerNote), followUpRequired, followUpStatus).Scan(
		&review.ID, &review.FeedbackID, &review.ReviewerIdentity, &review.ReviewedAt,
		&review.Outcome, &review.ManagerNote, &review.FollowUpRequired, &review.FollowUpStatus,
	)
	return review, err
}

func feedbackReviewState(outcome string) (bool, string) {
	if outcome == models.FeedbackOutcomeFollowUp {
		return true, "open"
	}
	if outcome == models.FeedbackOutcomeClosed {
		return false, "closed"
	}
	return false, "none"
}

// -----------------------------------------------------------------------------
// FeedbackSummary – lightweight stats for Reg 44 / Ofsted
// -----------------------------------------------------------------------------

func (r *Repo) FeedbackSummary(
	ctx context.Context,
	homeName string,
	from, to *time.Time,
) (models.FeedbackSummary, error) {
	var summary models.FeedbackSummary
	summary.ByWho = make(map[string]int)

	query := `
SELECT
	who,
	rating,
	requires_human_review,
	review_priority
FROM feedback
WHERE 1=1
`
	args := []any{}
	argIdx := 1

	if from != nil {
		query += fmt.Sprintf(" AND submitted_at >= $%d", argIdx)
		args = append(args, *from)
		argIdx++
	}
	if to != nil {
		query += fmt.Sprintf(" AND submitted_at < $%d", argIdx)
		args = append(args, to.Add(24*time.Hour))
		argIdx++
	}
	if homeName != "" {
		query += fmt.Sprintf(" AND home_name = $%d", argIdx)
		args = append(args, homeName)
		argIdx++
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return summary, err
	}
	defer rows.Close()

	var (
		total       int
		sumRating   float64
		ratingCount int
		prompts     int
		urgent      int
		priority    int
		routine     int
	)

	for rows.Next() {
		var (
			who            string
			rating         sql.NullInt32
			requiresReview bool
			reviewPriority string
		)
		if err := rows.Scan(&who, &rating, &requiresReview, &reviewPriority); err != nil {
			return summary, err
		}

		total++
		summary.ByWho[who]++

		if rating.Valid {
			sumRating += float64(rating.Int32)
			ratingCount++
		}

		if requiresReview {
			prompts++
		}

		switch reviewPriority {
		case "urgent":
			urgent++
		case "priority":
			priority++
		default:
			routine++
		}
	}

	summary.Total = total
	if ratingCount > 0 {
		summary.AvgRating = sumRating / float64(ratingCount)
	}
	summary.ReviewPrompts = models.ReviewPromptStats{
		Total: prompts, Urgent: urgent, Priority: priority, Routine: routine,
	}

	return summary, rows.Err()
}
