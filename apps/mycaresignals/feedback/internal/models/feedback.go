package models

import (
	"encoding/json"
	"time"
)

type FeedbackIn struct {
	Source        string `json:"source"          binding:"omitempty,max=50"`
	Who           string `json:"who"             binding:"required,oneof=SocialWorker Ofsted Parent Professional Other"`
	Reason        string `json:"reason"          binding:"omitempty,max=200"`
	Rating        *int   `json:"rating"          binding:"omitempty,min=1,max=5"`
	WhatWentWell  string `json:"what_went_well"  binding:"required,min=3,max=2000"`
	WhatToImprove string `json:"what_to_improve" binding:"omitempty,max=2000"`
	Recommend     bool   `json:"recommend"`
	ContactName   string `json:"contact_name"    binding:"omitempty,max=100"`
	ContactEmail  string `json:"contact_email"   binding:"omitempty,email,max=200"`

	// structured visitor questions
	HouseRating   *int   `json:"house_rating"   binding:"omitempty,min=1,max=5"`
	StaffRating   *int   `json:"staff_rating"   binding:"omitempty,min=1,max=5"`
	FeltSafe      string `json:"felt_safe"     binding:"omitempty,oneof=yes no unsure"`
	OpenComments  string `json:"open_comments" binding:"omitempty,max=2000"`
	ContactWanted bool   `json:"contact_wanted"`
}

type FeedbackRow struct {
	ID                  string           `json:"id"`
	SubmittedAt         time.Time        `json:"submitted_at"`
	Source              string           `json:"source"`
	Who                 string           `json:"who"`
	Reason              *string          `json:"reason"`
	Rating              *int             `json:"rating"`
	WhatWentWell        string           `json:"went_well"`
	WhatToImprove       *string          `json:"improve"`
	Recommend           bool             `json:"recommend"`
	Safeguarding        bool             `json:"legacy_safeguarding_flag"`
	TriageStatus        string           `json:"legacy_triage_status"`
	FeedbackType        string           `json:"feedback_type"`
	RiskLevel           *string          `json:"legacy_risk_level,omitempty"`
	Answers             json.RawMessage  `json:"answers"`
	RequiresHumanReview bool             `json:"requires_human_review"`
	ReviewPriority      string           `json:"review_priority"`
	MatchedRuleIDs      []string         `json:"matched_rule_ids"`
	RuleVersion         string           `json:"rule_version"`
	CurrentStatus       string           `json:"current_status"`
	FollowUpStatus      string           `json:"follow_up_status"`
	HadFollowUp         bool             `json:"had_follow_up"`
	ReviewedAt          *time.Time       `json:"reviewed_at,omitempty"`
	Reviews             []FeedbackReview `json:"reviews,omitempty"`
}

const (
	FeedbackStatusNew           = "new"
	FeedbackOutcomeAcknowledged = "acknowledged"
	FeedbackOutcomeNoAction     = "no_action_required"
	FeedbackOutcomeFollowUp     = "follow_up_required"
	FeedbackOutcomeClosed       = "closed"
)

type FeedbackFilters struct {
	FeedbackID     string
	From           *time.Time
	To             *time.Time
	Source         string
	Status         string
	RequiresReview *bool
}

type FeedbackReview struct {
	ID               int64     `json:"id"`
	FeedbackID       string    `json:"feedback_id"`
	ReviewerIdentity string    `json:"reviewer"`
	ReviewedAt       time.Time `json:"reviewed_at"`
	Outcome          string    `json:"outcome"`
	ManagerNote      string    `json:"manager_note"`
	FollowUpRequired bool      `json:"follow_up_required"`
	FollowUpStatus   string    `json:"follow_up_status"`
}

type FeedbackReviewInput struct {
	Outcome     string `json:"outcome"`
	ManagerNote string `json:"manager_note"`
}

func ValidFeedbackOutcome(value string) bool {
	return value == FeedbackOutcomeAcknowledged || value == FeedbackOutcomeNoAction ||
		value == FeedbackOutcomeFollowUp || value == FeedbackOutcomeClosed
}
