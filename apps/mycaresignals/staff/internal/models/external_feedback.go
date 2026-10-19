package models

import (
	"encoding/json"
	"time"
)

type FeedbackTriageStatus string

const (
	FeedbackStatusNew          FeedbackTriageStatus = "new"
	FeedbackStatusAcknowledged FeedbackTriageStatus = "acknowledged"
	FeedbackStatusFollowUp     FeedbackTriageStatus = "follow_up"
	FeedbackStatusReviewed     FeedbackTriageStatus = "reviewed"
)

type FeedbackReviewOutcome string

const (
	FeedbackOutcomeAcknowledge           FeedbackReviewOutcome = "acknowledge"
	FeedbackOutcomeNoFurtherAction       FeedbackReviewOutcome = "no_further_action"
	FeedbackOutcomeFollowUpRequired      FeedbackReviewOutcome = "follow_up_required"
	FeedbackOutcomeLinkAsEvidence        FeedbackReviewOutcome = "link_as_evidence"
	FeedbackOutcomeEscalateForRiskReview FeedbackReviewOutcome = "escalate_for_risk_review"
)

func (o FeedbackReviewOutcome) Valid() bool {
	switch o {
	case FeedbackOutcomeAcknowledge, FeedbackOutcomeNoFurtherAction,
		FeedbackOutcomeFollowUpRequired, FeedbackOutcomeLinkAsEvidence,
		FeedbackOutcomeEscalateForRiskReview:
		return true
	default:
		return false
	}
}

type ExternalFeedbackReceipt struct {
	ID                  int64                    `json:"id"`
	IntakeReceiptID     string                   `json:"intake_receipt_id"`
	IntakeType          string                   `json:"intake_type"`
	SourceType          string                   `json:"source_type"`
	HomeID              int64                    `json:"home_id"`
	ChildID             *int64                   `json:"child_id,omitempty"`
	ReceivedAt          time.Time                `json:"received_at"`
	RequiresHumanReview bool                     `json:"requires_human_review"`
	ReviewPriority      string                   `json:"review_priority"`
	MatchedRuleIDs      []string                 `json:"matched_rule_ids"`
	RuleVersion         string                   `json:"rule_version"`
	RawPayload          json.RawMessage          `json:"raw_payload"`
	IntakeContextID     *string                  `json:"intake_context_id,omitempty"`
	IntakePurpose       *string                  `json:"intake_purpose,omitempty"`
	PeriodMonth         *string                  `json:"period_month,omitempty"`
	DomainRatings       map[string]int           `json:"domain_ratings,omitempty"`
	FollowUpRequested   bool                     `json:"follow_up_requested"`
	FollowUpPreference  *string                  `json:"follow_up_preference,omitempty"`
	TriageStatus        FeedbackTriageStatus     `json:"triage_status"`
	CreatedAt           time.Time                `json:"created_at"`
	UpdatedAt           time.Time                `json:"updated_at"`
	Reviews             []ExternalFeedbackReview `json:"reviews,omitempty"`
}

type ExternalFeedbackReview struct {
	ID                  int64                 `json:"id"`
	ReceiptID           int64                 `json:"receipt_id"`
	HomeID              int64                 `json:"home_id"`
	ReviewerStaffID     int64                 `json:"reviewer_staff_id"`
	ReviewerDisplayName string                `json:"reviewer_display_name"`
	ReviewedAt          time.Time             `json:"reviewed_at"`
	Outcome             FeedbackReviewOutcome `json:"outcome"`
	Rationale           string                `json:"rationale"`
	FollowUpRequired    bool                  `json:"follow_up_required"`
	ChildID             *int64                `json:"child_id,omitempty"`
	Domain              *string               `json:"domain,omitempty"`
	GoalID              *int64                `json:"goal_id,omitempty"`
	TargetID            *int64                `json:"target_id,omitempty"`
	RiskID              *int64                `json:"risk_id,omitempty"`
	CreatedAt           time.Time             `json:"created_at"`
}

type ExternalFeedbackIngest struct {
	ReceiptID           string          `json:"receipt_id"`
	IntakeType          string          `json:"intake_type"`
	SourceType          string          `json:"source_type"`
	HomeID              int64           `json:"home_id"`
	ChildID             *int64          `json:"child_id,omitempty"`
	ReceivedAt          time.Time       `json:"received_at"`
	RequiresHumanReview bool            `json:"requires_human_review"`
	ReviewPriority      string          `json:"review_priority"`
	MatchedRuleIDs      []string        `json:"matched_rule_ids"`
	RuleVersion         string          `json:"rule_version"`
	RawPayload          json.RawMessage `json:"raw_payload"`
	IntakeContextID     string          `json:"intake_context_id,omitempty"`
	IntakePurpose       string          `json:"intake_purpose,omitempty"`
	PeriodMonth         string          `json:"period_month,omitempty"`
	DomainRatings       map[string]int  `json:"domain_ratings,omitempty"`
	FollowUpRequested   bool            `json:"follow_up_requested"`
	FollowUpPreference  string          `json:"follow_up_preference,omitempty"`
}

type ExternalFeedbackReviewInput struct {
	Outcome   FeedbackReviewOutcome `json:"outcome"`
	Rationale string                `json:"rationale"`
	ChildID   *int64                `json:"child_id,omitempty"`
	Domain    *string               `json:"domain,omitempty"`
	GoalID    *int64                `json:"goal_id,omitempty"`
	TargetID  *int64                `json:"target_id,omitempty"`
	RiskID    *int64                `json:"risk_id,omitempty"`
}
