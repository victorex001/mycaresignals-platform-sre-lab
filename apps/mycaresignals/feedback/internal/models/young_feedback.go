// internal/models/young_feedback.go
package models

import "time"

type YoungVoice struct {
	ID               string         `json:"id"`
	SubmittedAt      time.Time      `json:"submitted_at"`
	Home             string         `json:"home"`
	YoungPersonAlias string         `json:"young_person_alias"`
	PeriodMonth      time.Time      `json:"period_month"`
	DomainRatings    map[string]int `json:"domain_ratings"`

	Q1Overall string `json:"q1_overall"`
	Q1Best    string `json:"q1_best"`
	Q1Hardest string `json:"q1_hardest"`

	Q2Safe       string `json:"q2_safe"`
	Q2Unsafe     string `json:"q2_unsafe"`
	Q2SupportWho string `json:"q2_support_who"`

	Q3Listened      string `json:"q3_listened"`
	Q3TrustedAdult  string `json:"q3_trusted_adult"`
	Q3ChangeSupport string `json:"q3_change_support"`

	Q4Peers         string `json:"q4_peers"`
	Q4Unkind        string `json:"q4_unkind"`
	Q4HelpWithPeers string `json:"q4_help_with_peers"`

	Q5Enjoy       string `json:"q5_enjoy"`
	Q5WishMore    string `json:"q5_wish_more"`
	Q5HouseChange string `json:"q5_house_change"`

	Q6Education     string `json:"q6_education"`
	Q6EducationHelp string `json:"q6_education_help"`

	Q7ContactFeelings string `json:"q7_contact_feelings"`
	Q7ContactChange   string `json:"q7_contact_change"`

	Q8SayInDecisions string `json:"q8_say_in_decisions"`
	Q8ChangeExample  string `json:"q8_change_example"`

	Q9Worries          string `json:"q9_worries"`
	Q9OneChange        string `json:"q9_one_change"`
	Q9WantTalk         bool   `json:"q9_want_talk"`
	Q9FollowUpPref     string `json:"q9_follow_up_preference"`
	FollowUpRequested  bool   `json:"follow_up_requested"`
	FollowUpPreference string `json:"follow_up_preference"`

	Q10WhatTellOfsted string `json:"q10_what_tell_ofsted"`
	Q10NeverForget    string `json:"q10_never_forget"`

	SafeguardingFlag    bool    `json:"safeguarding_flag"`
	RiskLevel           *string `json:"legacy_risk_level,omitempty"`
	SafeguardingSummary string  `json:"safeguarding_summary"`
	TriageStatus        string  `json:"triage_status"`
	RawSubmission       []byte  `json:"-"`
	ReviewMetadata
	IntakeContextID string `json:"-"`
	HomeExternalID  int64  `json:"-"`
	ChildExternalID *int64 `json:"-"`
}

var CanonicalMonthlyVoiceDomains = []string{
	"Emotional & Psychological Wellbeing",
	"Behaviour & Self-Regulation",
	"Relationships & Connection",
	"Daily Living, Learning & Participation",
	"Safety, Risk & Coping with Stress",
	"Skills, Identity & Future",
}

func ValidMonthlyVoiceRatings(ratings map[string]int) bool {
	if len(ratings) != len(CanonicalMonthlyVoiceDomains) {
		return false
	}
	for _, domain := range CanonicalMonthlyVoiceDomains {
		rating, ok := ratings[domain]
		if !ok || rating < 0 || rating > 4 {
			return false
		}
	}
	return true
}

type YoungVoiceAdminRow struct {
	ID                   string    `db:"id" json:"id"`
	Home                 string    `db:"home" json:"home"`
	YoungPersonAlias     string    `db:"young_person_alias" json:"young_person_alias"`
	PeriodMonth          string    `db:"period_month" json:"period_month"`
	SubmittedAt          time.Time `db:"submitted_at" json:"submitted_at"`
	Q1Overall            string    `db:"q1_overall" json:"q1_overall"`
	Q2Safe               string    `db:"q2_safe" json:"q2_safe"`
	Q2Unsafe             string    `db:"q2_unsafe" json:"q2_unsafe"`
	Q4Peers              string    `db:"q4_peers" json:"q4_peers"`
	Q4Unkind             string    `db:"q4_unkind" json:"q4_unkind"`
	Q9Worries            string    `db:"q9_worries" json:"q9_worries"`
	Q9WantTalk           bool      `db:"q9_want_talk" json:"q9_want_talk"`
	Q9FollowUpPreference string    `db:"q9_follow_up_preference" json:"q9_follow_up_preference"`
	RequiresHumanReview  bool      `db:"requires_human_review" json:"requires_human_review"`
	ReviewPriority       string    `db:"review_priority" json:"review_priority"`
	MatchedRuleIDs       []string  `db:"matched_rule_ids" json:"matched_rule_ids"`
	RuleVersion          string    `db:"rule_version" json:"rule_version"`
}
