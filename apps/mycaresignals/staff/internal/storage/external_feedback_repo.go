package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
)

var (
	ErrFeedbackReceiptNotFound = errors.New("feedback receipt not found")
	ErrFeedbackReceiptConflict = errors.New("feedback receipt relationship conflict")
)

func (r *Repo) IngestExternalFeedback(ctx context.Context, input models.ExternalFeedbackIngest) (*models.ExternalFeedbackReceipt, error) {
	if input.IntakeType != "young_person_voice" {
		return nil, ErrFeedbackReceiptConflict
	}
	rules, err := json.Marshal(input.MatchedRuleIDs)
	if err != nil {
		return nil, err
	}
	ratings := ""
	if len(input.DomainRatings) > 0 {
		encoded, err := json.Marshal(input.DomainRatings)
		if err != nil {
			return nil, err
		}
		ratings = string(encoded)
	}
	followUpPreference := strings.TrimSpace(input.FollowUpPreference)
	var receiptID int64
	err = r.WithTx(ctx, nil, func(tx *TxRepo) error {
		homeExists, err := tx.LockHomeID(ctx, input.HomeID)
		if err != nil {
			return err
		}
		if !homeExists {
			return security.ErrInvalidRelationship
		}
		if input.ChildID != nil {
			homeID, err := tx.LockChildHomeID(ctx, *input.ChildID)
			if err != nil || homeID != input.HomeID {
				return security.ErrInvalidRelationship
			}
		}
		const q = `
INSERT INTO external_feedback_receipts (
  intake_receipt_id, intake_type, source_type, home_id, child_id, received_at,
  requires_human_review, review_priority, matched_rule_ids, rule_version, raw_payload,
  intake_context_id, intake_purpose, period_month, domain_ratings,
  follow_up_requested, follow_up_preference
) VALUES (
  $1::uuid,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11::jsonb,
  NULLIF($12, '')::uuid,NULLIF($13, ''),to_date(NULLIF($14, ''), 'YYYY-MM'),
  CASE WHEN $15 = '' THEN NULL ELSE $15::jsonb END,$16,NULLIF($17, '')
)
ON CONFLICT DO NOTHING
RETURNING id`
		err = tx.q.QueryRowContext(ctx, q,
			input.ReceiptID, input.IntakeType, input.SourceType, input.HomeID,
			input.ChildID, input.ReceivedAt, input.RequiresHumanReview,
			input.ReviewPriority, string(rules), input.RuleVersion, string(input.RawPayload),
			input.IntakeContextID, input.IntakePurpose, input.PeriodMonth, ratings,
			input.FollowUpRequested, followUpPreference,
		).Scan(&receiptID)
		if errors.Is(err, sql.ErrNoRows) {
			const existing = `
SELECT id
FROM external_feedback_receipts
WHERE intake_receipt_id = $1::uuid
  AND intake_type = $2
  AND source_type = $3
  AND home_id = $4
  AND child_id IS NOT DISTINCT FROM $5
  AND received_at = $6
  AND requires_human_review = $7
  AND review_priority = $8
  AND matched_rule_ids = $9::jsonb
  AND rule_version = $10
  AND raw_payload = $11::jsonb
  AND intake_context_id IS NOT DISTINCT FROM NULLIF($12, '')::uuid
  AND intake_purpose IS NOT DISTINCT FROM NULLIF($13, '')
  AND period_month IS NOT DISTINCT FROM to_date(NULLIF($14, ''), 'YYYY-MM')
  AND domain_ratings IS NOT DISTINCT FROM CASE WHEN $15 = '' THEN NULL ELSE $15::jsonb END
  AND follow_up_requested = $16
  AND follow_up_preference IS NOT DISTINCT FROM NULLIF($17, '')
FOR UPDATE`
			if err := tx.q.QueryRowContext(ctx, existing,
				input.ReceiptID, input.IntakeType, input.SourceType, input.HomeID,
				input.ChildID, input.ReceivedAt, input.RequiresHumanReview,
				input.ReviewPriority, string(rules), input.RuleVersion, string(input.RawPayload),
				input.IntakeContextID, input.IntakePurpose, input.PeriodMonth, ratings,
				input.FollowUpRequested, followUpPreference,
			).Scan(&receiptID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrFeedbackReceiptConflict
				}
				return err
			}
		} else if err != nil {
			return err
		}
		if input.IntakePurpose == models.YoungPersonMonthlyVoiceSource {
			if err := tx.insertVerifiedMonthlyVoiceRatings(ctx, receiptID, input); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetExternalFeedbackReceipt(ctx, input.HomeID, receiptID)
}

func (r *TxRepo) insertVerifiedMonthlyVoiceRatings(ctx context.Context, receiptID int64, input models.ExternalFeedbackIngest) error {
	if input.ChildID == nil || !models.ValidMonthlyVoiceRatings(input.DomainRatings) {
		return ErrFeedbackReceiptConflict
	}
	periodMonth, err := time.Parse("2006-01", input.PeriodMonth)
	if err != nil {
		return err
	}
	for _, domain := range models.CanonicalProgressDomains {
		rating := input.DomainRatings[domain]
		const insert = `
INSERT INTO child_monthly_voice_ratings (
  child_id, period_month, domain, young_person_rating,
  source_receipt_id, source_review_id, source_context_id,
  source, submitted_at
) VALUES ($1,$2,$3,$4,$5,NULL,$6::uuid,$7,$8)
ON CONFLICT DO NOTHING
RETURNING id`
		var ratingID int64
		err := r.q.QueryRowContext(ctx, insert,
			*input.ChildID, periodMonth, domain, rating, receiptID,
			input.IntakeContextID, models.YoungPersonMonthlyVoiceSource, input.ReceivedAt,
		).Scan(&ratingID)
		if errors.Is(err, sql.ErrNoRows) {
			const existing = `
SELECT id
FROM child_monthly_voice_ratings
WHERE source_receipt_id = $1
  AND source_context_id = $2::uuid
  AND child_id = $3
  AND period_month = $4
  AND domain = $5
  AND young_person_rating = $6
FOR UPDATE`
			if err := r.q.QueryRowContext(ctx, existing,
				receiptID, input.IntakeContextID, *input.ChildID, periodMonth, domain, rating,
			).Scan(&ratingID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrFeedbackReceiptConflict
				}
				return err
			}
		} else if err != nil {
			return fmt.Errorf("insert verified monthly voice rating: %w", err)
		}
	}
	return nil
}

func (r *Repo) ListChildMonthlyVoiceReceipts(ctx context.Context, childID int64, status string) ([]models.ExternalFeedbackReceipt, error) {
	const q = `
SELECT id, intake_receipt_id::text, intake_type, source_type, home_id, child_id,
       received_at, requires_human_review, review_priority, matched_rule_ids,
       rule_version, raw_payload, intake_context_id::text, intake_purpose,
       period_month, domain_ratings, follow_up_requested, follow_up_preference,
       triage_status, created_at, updated_at
FROM external_feedback_receipts
WHERE child_id = $1
  AND intake_type = 'young_person_voice'
  AND intake_purpose = 'young_person_monthly_voice'
  AND ($2 = '' OR triage_status = $2)
ORDER BY period_month DESC, received_at DESC, id DESC`
	return r.listExternalFeedbackReceipts(ctx, q, childID, status)
}

func (r *Repo) listExternalFeedbackReceipts(ctx context.Context, query string, args ...any) ([]models.ExternalFeedbackReceipt, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.ExternalFeedbackReceipt, 0)
	for rows.Next() {
		item, err := scanExternalFeedbackReceipt(rows)
		if err != nil {
			return nil, err
		}
		item.RawPayload = redactContactForQueue(item.RawPayload)
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *Repo) GetChildMonthlyVoiceReceipt(ctx context.Context, childID, receiptID int64) (*models.ExternalFeedbackReceipt, error) {
	return r.getExternalFeedbackReceipt(ctx, `child_id = $1 AND id = $2 AND intake_type = 'young_person_voice' AND intake_purpose = 'young_person_monthly_voice'`, childID, receiptID)
}

func (r *Repo) GetExternalFeedbackReceipt(ctx context.Context, homeID, receiptID int64) (*models.ExternalFeedbackReceipt, error) {
	return r.getExternalFeedbackReceipt(ctx, `home_id = $1 AND id = $2`, homeID, receiptID)
}

func (r *Repo) getExternalFeedbackReceipt(ctx context.Context, predicate string, firstID, receiptID int64) (*models.ExternalFeedbackReceipt, error) {
	q := `
SELECT id, intake_receipt_id::text, intake_type, source_type, home_id, child_id,
       received_at, requires_human_review, review_priority, matched_rule_ids,
       rule_version, raw_payload, intake_context_id::text, intake_purpose,
       period_month, domain_ratings, follow_up_requested, follow_up_preference,
       triage_status, created_at, updated_at
FROM external_feedback_receipts
WHERE ` + predicate
	item, err := scanExternalFeedbackReceipt(r.db.QueryRowContext(ctx, q, firstID, receiptID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrFeedbackReceiptNotFound
	}
	if err != nil {
		return nil, err
	}
	item.RawPayload = redactContactForQueue(item.RawPayload)
	item.Reviews, err = r.ListExternalFeedbackReviews(ctx, item.HomeID, receiptID)
	return item, err
}

type feedbackReceiptScanner interface{ Scan(...any) error }

func scanExternalFeedbackReceipt(scanner feedbackReceiptScanner) (*models.ExternalFeedbackReceipt, error) {
	var item models.ExternalFeedbackReceipt
	var childID sql.NullInt64
	var contextID, purpose, followUpPreference sql.NullString
	var periodMonth sql.NullTime
	var rules, ratings []byte
	if err := scanner.Scan(
		&item.ID, &item.IntakeReceiptID, &item.IntakeType, &item.SourceType,
		&item.HomeID, &childID, &item.ReceivedAt, &item.RequiresHumanReview,
		&item.ReviewPriority, &rules, &item.RuleVersion, &item.RawPayload,
		&contextID, &purpose, &periodMonth, &ratings, &item.FollowUpRequested, &followUpPreference,
		&item.TriageStatus, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if childID.Valid {
		value := childID.Int64
		item.ChildID = &value
	}
	if err := json.Unmarshal(rules, &item.MatchedRuleIDs); err != nil {
		return nil, err
	}
	item.IntakeContextID = stringFromNull(contextID)
	item.IntakePurpose = stringFromNull(purpose)
	item.FollowUpPreference = stringFromNull(followUpPreference)
	if periodMonth.Valid {
		value := periodMonth.Time.Format("2006-01")
		item.PeriodMonth = &value
	}
	if len(ratings) > 0 {
		if err := json.Unmarshal(ratings, &item.DomainRatings); err != nil {
			return nil, err
		}
	}
	return &item, nil
}

func redactContactForQueue(raw json.RawMessage) json.RawMessage {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return json.RawMessage(`{}`)
	}
	delete(payload, "contact_name")
	delete(payload, "contact_email")
	encoded, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

func (r *Repo) ListExternalFeedbackReviews(ctx context.Context, homeID, receiptID int64) ([]models.ExternalFeedbackReview, error) {
	const q = `
SELECT fr.id, fr.receipt_id, fr.home_id, fr.reviewer_staff_id, su.display_name,
       fr.reviewed_at, fr.outcome, fr.rationale, fr.follow_up_required,
       fr.child_id, fr.domain, fr.goal_id, fr.target_id, fr.risk_id, fr.created_at
FROM external_feedback_reviews fr
JOIN staff_users su ON su.id = fr.reviewer_staff_id
WHERE fr.home_id = $1 AND fr.receipt_id = $2
ORDER BY fr.reviewed_at DESC, fr.id DESC`
	rows, err := r.db.QueryContext(ctx, q, homeID, receiptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reviews := make([]models.ExternalFeedbackReview, 0)
	for rows.Next() {
		var review models.ExternalFeedbackReview
		var childID, goalID, targetID, riskID sql.NullInt64
		var domain sql.NullString
		if err := rows.Scan(
			&review.ID, &review.ReceiptID, &review.HomeID, &review.ReviewerStaffID,
			&review.ReviewerDisplayName, &review.ReviewedAt, &review.Outcome,
			&review.Rationale, &review.FollowUpRequired, &childID, &domain,
			&goalID, &targetID, &riskID, &review.CreatedAt,
		); err != nil {
			return nil, err
		}
		review.ChildID = int64FromNull(childID)
		review.Domain = stringFromNull(domain)
		review.GoalID = int64FromNull(goalID)
		review.TargetID = int64FromNull(targetID)
		review.RiskID = int64FromNull(riskID)
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (r *Repo) ReviewExternalFeedback(
	ctx context.Context,
	homeID, receiptID int64,
	principal security.Principal,
	input models.ExternalFeedbackReviewInput,
	requestID string,
) (*models.ExternalFeedbackReview, error) {
	review := &models.ExternalFeedbackReview{
		ReceiptID: receiptID, HomeID: homeID, ReviewerStaffID: principal.StaffUserID(),
		ReviewerDisplayName: principal.DisplayName(),
		Outcome:             input.Outcome, Rationale: strings.TrimSpace(input.Rationale),
		FollowUpRequired: input.Outcome == models.FeedbackOutcomeFollowUpRequired || input.Outcome == models.FeedbackOutcomeEscalateForRiskReview,
		ChildID:          input.ChildID, Domain: trimOptional(input.Domain), GoalID: input.GoalID,
		TargetID: input.TargetID, RiskID: input.RiskID,
	}
	err := r.WithTx(ctx, nil, func(tx *TxRepo) error {
		if ok, err := tx.LockHomeID(ctx, homeID); err != nil || !ok {
			return security.ErrInvalidRelationship
		}
		locked, err := tx.lockExternalFeedbackReceipt(ctx, homeID, receiptID)
		if err != nil {
			return err
		}
		switch locked.IntakeType {
		case "young_person_voice":
			if locked.IntakePurpose != models.YoungPersonMonthlyVoiceSource || locked.ChildID == nil {
				return ErrFeedbackReceiptConflict
			}
			if _, err := security.RequireHomeRoleForMutation(ctx, tx, homeID, security.RoleSenior, security.RoleManager); err != nil {
				return err
			}
			if review.Outcome == models.FeedbackOutcomeLinkAsEvidence || hasFeedbackLinks(review) {
				return ErrFeedbackReceiptConflict
			}
			review.ChildID = locked.ChildID
		default:
			return ErrFeedbackReceiptConflict
		}
		if err := tx.validateExternalFeedbackLinks(ctx, homeID, review); err != nil {
			return err
		}
		if err := tx.insertExternalFeedbackReview(ctx, review); err != nil {
			return err
		}
		status := feedbackStatusForOutcome(review.Outcome)
		if err := tx.updateExternalFeedbackStatus(ctx, homeID, receiptID, status); err != nil {
			return err
		}
		return tx.insertExternalFeedbackAudit(ctx, principal, review, requestID, locked.IntakeType)
	})
	if err != nil {
		return nil, err
	}
	return review, nil
}

type lockedFeedbackReceipt struct {
	ChildID       *int64
	IntakeType    string
	IntakePurpose string
}

func (r *TxRepo) lockExternalFeedbackReceipt(ctx context.Context, homeID, receiptID int64) (lockedFeedbackReceipt, error) {
	const q = `SELECT child_id, intake_type, COALESCE(intake_purpose, '') FROM external_feedback_receipts WHERE id = $1 AND home_id = $2 FOR UPDATE`
	var locked lockedFeedbackReceipt
	var childID sql.NullInt64
	if err := r.q.QueryRowContext(ctx, q, receiptID, homeID).Scan(&childID, &locked.IntakeType, &locked.IntakePurpose); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return locked, ErrFeedbackReceiptNotFound
		}
		return locked, err
	}
	locked.ChildID = int64FromNull(childID)
	return locked, nil
}

func hasFeedbackLinks(review *models.ExternalFeedbackReview) bool {
	return review.ChildID != nil || review.Domain != nil || review.GoalID != nil || review.TargetID != nil || review.RiskID != nil
}

func (r *TxRepo) validateExternalFeedbackLinks(ctx context.Context, homeID int64, review *models.ExternalFeedbackReview) error {
	if review.Outcome == models.FeedbackOutcomeLinkAsEvidence && review.ChildID == nil {
		return security.ErrInvalidRelationship
	}
	if review.ChildID == nil {
		if review.Domain != nil || review.GoalID != nil || review.TargetID != nil || review.RiskID != nil {
			return security.ErrInvalidRelationship
		}
		return nil
	}
	childHomeID, err := r.LockChildHomeID(ctx, *review.ChildID)
	if err != nil || childHomeID != homeID {
		return security.ErrInvalidRelationship
	}
	if review.GoalID != nil {
		ok, err := r.LockGoalBelongsToChild(ctx, *review.GoalID, *review.ChildID)
		if err != nil || !ok {
			return security.ErrInvalidRelationship
		}
	}
	if review.TargetID != nil {
		ok, err := r.LockTargetBelongsToChild(ctx, *review.TargetID, *review.ChildID)
		if err != nil || !ok {
			return security.ErrInvalidRelationship
		}
	}
	if review.RiskID != nil {
		ok, err := r.LockRiskBelongsToChild(ctx, *review.RiskID, *review.ChildID)
		if err != nil || !ok {
			return security.ErrInvalidRelationship
		}
	}
	return nil
}

func (r *TxRepo) insertExternalFeedbackReview(ctx context.Context, review *models.ExternalFeedbackReview) error {
	const q = `
INSERT INTO external_feedback_reviews (
  receipt_id, home_id, reviewer_staff_id, outcome, rationale,
  follow_up_required, child_id, domain, goal_id, target_id, risk_id
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING id, reviewed_at, created_at`
	return r.q.QueryRowContext(ctx, q,
		review.ReceiptID, review.HomeID, review.ReviewerStaffID, review.Outcome,
		review.Rationale, review.FollowUpRequired, review.ChildID, review.Domain,
		review.GoalID, review.TargetID, review.RiskID,
	).Scan(&review.ID, &review.ReviewedAt, &review.CreatedAt)
}

func (r *TxRepo) updateExternalFeedbackStatus(ctx context.Context, homeID, receiptID int64, status models.FeedbackTriageStatus) error {
	const q = `
UPDATE external_feedback_receipts
SET triage_status = $3, updated_at = now()
WHERE id = $1 AND home_id = $2`
	result, err := r.q.ExecContext(ctx, q, receiptID, homeID, status)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return security.ErrInvalidRelationship
	}
	return nil
}

func feedbackStatusForOutcome(outcome models.FeedbackReviewOutcome) models.FeedbackTriageStatus {
	switch outcome {
	case models.FeedbackOutcomeAcknowledge:
		return models.FeedbackStatusAcknowledged
	case models.FeedbackOutcomeFollowUpRequired, models.FeedbackOutcomeEscalateForRiskReview:
		return models.FeedbackStatusFollowUp
	default:
		return models.FeedbackStatusReviewed
	}
}

func (r *TxRepo) insertExternalFeedbackAudit(ctx context.Context, principal security.Principal, review *models.ExternalFeedbackReview, requestID, intakeType string) error {
	metadata, err := security.NewAuditMetadata(security.AuditMetadataFields{
		ResourceID: &review.ReceiptID, FeedbackReceiptID: &review.ReceiptID,
		FeedbackReviewID: &review.ID, FeedbackTriageOutcome: security.FeedbackTriageOutcome(review.Outcome),
		RequestID: requestID,
	})
	if err != nil {
		return err
	}
	method := "POST"
	if intakeType != "young_person_voice" {
		return ErrFeedbackReceiptConflict
	}
	path := string(AuditRouteMonthlyVoiceReviews)
	eventType := string(AuditEventFeedbackReviewCreate)
	entityType := "external_feedback_receipt"
	eventID := review.ReceiptID
	event := models.AuditEvent{
		HomeID: &review.HomeID, ChildID: review.ChildID, StaffUserID: &review.ReviewerStaffID,
		ActorEmail: stringPointer(principal.Email()), ActorName: stringPointer(principal.DisplayName()),
		EventType: eventType, EntityType: entityType, EntityID: &eventID,
		RequestMethod: &method, RequestPath: &path,
	}
	return createAuditEvent(ctx, r.q, &event, metadata)
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func stringPointer(value string) *string { return &value }
