package models

import (
	"encoding/json"
	"time"
)

const (
	IntakeTypeVisitorFeedback  = "visitor_feedback"
	IntakeTypeYoungPersonVoice = "young_person_voice"
	IntakePurposeMonthlyVoice  = "young_person_monthly_voice"
)

type IntakeContext struct {
	ID              string
	HomeExternalID  int64
	ChildExternalID *int64
	HomeLabel       string
	Purpose         string
	PeriodMonth     *time.Time
	ExpiresAt       *time.Time
	UsedAt          *time.Time
}

type MonthlyVoiceContextInput struct {
	HomeExternalID  int64
	ChildExternalID int64
	PeriodMonth     time.Time
	ExpiresAt       time.Time
}

type MonthlyVoiceContext struct {
	ContextID   string    `json:"context_id"`
	Token       string    `json:"context_token"`
	PeriodMonth string    `json:"period_month"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type ReviewMetadata struct {
	RequiresHumanReview bool     `json:"requires_human_review"`
	ReviewPriority      string   `json:"review_priority"`
	MatchedRuleIDs      []string `json:"matched_rule_ids"`
	RuleVersion         string   `json:"rule_version"`
}

type IntakeDelivery struct {
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

type PendingDelivery struct {
	ID      int64
	Payload json.RawMessage
}
