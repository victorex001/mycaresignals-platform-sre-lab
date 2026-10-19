package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"qr-feedback-api/internal/models"
)

// InsertYoungVoice stores a monthly young person check-in.
func (r *Repo) InsertYoungVoice(ctx context.Context, y *models.YoungVoice) error {
	if y == nil || y.IntakeContextID == "" || y.HomeExternalID <= 0 {
		return ErrInvalidIntakeContext
	}
	matchedRules, err := json.Marshal(y.MatchedRuleIDs)
	if err != nil {
		return err
	}
	ratings, err := json.Marshal(y.DomainRatings)
	if err != nil || !models.ValidMonthlyVoiceRatings(y.DomainRatings) {
		return ErrInvalidIntakeContext
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	const lockContext = `
SELECT home_external_id, child_external_id, purpose, period_month, expires_at, revoked_at, used_at
FROM intake_contexts
WHERE id = $1::uuid
FOR UPDATE`
	var homeID int64
	var childID sql.NullInt64
	var purpose string
	var periodMonth time.Time
	var expiresAt time.Time
	var revokedAt, usedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, lockContext, y.IntakeContextID).Scan(
		&homeID, &childID, &purpose, &periodMonth, &expiresAt, &revokedAt, &usedAt,
	); err != nil {
		return ErrInvalidIntakeContext
	}
	if homeID != y.HomeExternalID || !childID.Valid || y.ChildExternalID == nil || childID.Int64 != *y.ChildExternalID ||
		purpose != models.IntakePurposeMonthlyVoice || !sameMonth(periodMonth, y.PeriodMonth) || revokedAt.Valid || !expiresAt.After(time.Now().UTC()) {
		return ErrInvalidIntakeContext
	}
	if usedAt.Valid {
		const existing = `
SELECT id::text, submitted_at, raw_submission = $2::jsonb
FROM young_voice_feedback
WHERE intake_context_id = $1::uuid`
		var sameSubmission bool
		if err := tx.QueryRowContext(ctx, existing, y.IntakeContextID, string(y.RawSubmission)).Scan(&y.ID, &y.SubmittedAt, &sameSubmission); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrInvalidIntakeContext
			}
			return err
		}
		if !sameSubmission {
			return ErrMonthlyVoiceConflict
		}
		y.PeriodMonth = periodMonth
		return tx.Commit()
	}
	const q = `
INSERT INTO young_voice_feedback (
    home, home_id, child_id, intake_context_id,
    young_person_alias,
    period_month,
    q1_overall, q1_best, q1_hardest,
    q2_safe, q2_unsafe, q2_support_who,
    q3_listened, q3_trusted_adult, q3_change_support,
    q4_peers, q4_unkind, q4_help_with_peers,
    q5_enjoy, q5_wish_more, q5_house_change,
    q6_education, q6_education_help,
    q7_contact_feelings, q7_contact_change,
    q8_say_in_decisions, q8_change_example,
    q9_worries, q9_one_change, q9_want_talk, q9_follow_up_preference,
    q10_what_tell_ofsted, q10_never_forget,
    safeguarding_flag, risk_level, safeguarding_summary, triage_status,
    raw_submission, requires_human_review, review_priority, matched_rule_ids, rule_version,
    domain_ratings, follow_up_requested, follow_up_preference
)
VALUES (
    $1, $2, $3, $4::uuid, $5, $6,
    $7, $8, $9, $10, $11, $12, $13, $14, $15,
    $16, $17, $18, $19, $20, $21, $22, $23,
    $24, $25, $26, $27, $28, $29, $30, $31,
    $32, $33,
    FALSE, NULL, NULL, 'New',
    $34::jsonb, $35, $36, $37::jsonb, $38,
    $39::jsonb, $40, $41
)
RETURNING id, submitted_at;
`

	row := tx.QueryRowContext(
		ctx,
		q,
		y.Home,
		y.HomeExternalID,
		y.ChildExternalID,
		y.IntakeContextID,
		y.YoungPersonAlias,
		periodMonth,
		y.Q1Overall, y.Q1Best, y.Q1Hardest,
		y.Q2Safe, y.Q2Unsafe, y.Q2SupportWho,
		y.Q3Listened, y.Q3TrustedAdult, y.Q3ChangeSupport,
		y.Q4Peers, y.Q4Unkind, y.Q4HelpWithPeers,
		y.Q5Enjoy, y.Q5WishMore, y.Q5HouseChange,
		y.Q6Education, y.Q6EducationHelp,
		y.Q7ContactFeelings, y.Q7ContactChange,
		y.Q8SayInDecisions, y.Q8ChangeExample,
		y.Q9Worries, y.Q9OneChange, y.Q9WantTalk, y.Q9FollowUpPref,
		y.Q10WhatTellOfsted, y.Q10NeverForget,
		string(y.RawSubmission), y.RequiresHumanReview, y.ReviewPriority,
		string(matchedRules), y.RuleVersion,
		string(ratings), y.FollowUpRequested, nullStr(y.FollowUpPreference),
	)

	if err := row.Scan(&y.ID, &y.SubmittedAt); err != nil {
		return err
	}

	// Ensure the struct has the final period_month we stored.
	y.PeriodMonth = periodMonth
	delivery := models.IntakeDelivery{
		ReceiptID: y.ID, IntakeType: models.IntakeTypeYoungPersonVoice,
		SourceType: "YoungPersonVoice", HomeID: y.HomeExternalID,
		ChildID: y.ChildExternalID, ReceivedAt: y.SubmittedAt,
		RequiresHumanReview: y.RequiresHumanReview, ReviewPriority: y.ReviewPriority,
		MatchedRuleIDs: y.MatchedRuleIDs, RuleVersion: y.RuleVersion,
		RawPayload:      append(json.RawMessage(nil), y.RawSubmission...),
		IntakeContextID: y.IntakeContextID, IntakePurpose: models.IntakePurposeMonthlyVoice,
		PeriodMonth: periodMonth.Format("2006-01"), DomainRatings: copyDomainRatings(y.DomainRatings),
		FollowUpRequested: y.FollowUpRequested, FollowUpPreference: y.FollowUpPreference,
	}
	if err := insertOutbox(ctx, tx, delivery); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE intake_contexts SET used_at = $2 WHERE id = $1::uuid AND used_at IS NULL`, y.IntakeContextID, y.SubmittedAt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func sameMonth(left, right time.Time) bool {
	return left.UTC().Year() == right.UTC().Year() && left.UTC().Month() == right.UTC().Month()
}

func copyDomainRatings(input map[string]int) map[string]int {
	result := make(map[string]int, len(input))
	for domain, rating := range input {
		result[domain] = rating
	}
	return result
}

// ListYoungVoiceAdmin returns monthly check-ins for admin view/export.
func (r *Repo) ListYoungVoiceAdmin(
	ctx context.Context,
	home string,
	periodMonth string, // e.g. "2025-11"
) ([]models.YoungVoiceAdminRow, error) {
	const q = `
SELECT
    id,
    home,
    young_person_alias,
    period_month,
    submitted_at,
    q1_overall,
    q2_safe,
    q2_unsafe,
    q4_peers,
    q4_unkind,
    q9_worries,
    q9_want_talk,
    q9_follow_up_preference,
    requires_human_review,
    review_priority,
    matched_rule_ids,
    rule_version
FROM young_voice_feedback
WHERE ($1 = '' OR home = $1)
  AND ($2 = '' OR to_char(period_month, 'YYYY-MM') = $2)
ORDER BY submitted_at DESC;
`

	rows, err := r.db.QueryContext(ctx, q, home, periodMonth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.YoungVoiceAdminRow

	for rows.Next() {
		var row models.YoungVoiceAdminRow
		var matchedRules []byte
		if err := rows.Scan(
			&row.ID,
			&row.Home,
			&row.YoungPersonAlias,
			&row.PeriodMonth,
			&row.SubmittedAt,
			&row.Q1Overall,
			&row.Q2Safe,
			&row.Q2Unsafe,
			&row.Q4Peers,
			&row.Q4Unkind,
			&row.Q9Worries,
			&row.Q9WantTalk,
			&row.Q9FollowUpPreference,
			&row.RequiresHumanReview,
			&row.ReviewPriority,
			&matchedRules,
			&row.RuleVersion,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(matchedRules, &row.MatchedRuleIDs); err != nil {
			return nil, err
		}
		out = append(out, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}
