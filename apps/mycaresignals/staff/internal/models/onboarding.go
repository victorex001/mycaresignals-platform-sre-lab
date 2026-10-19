package models

import "time"

type ChildOnboardingBaseline struct {
	ID                          int64      `db:"id" json:"id"`
	ChildID                     ChildID    `db:"child_id" json:"child_id"`
	SummaryOfHistory            string     `db:"summary_of_history" json:"summary_of_history"`
	RecentPlacement             string     `db:"recent_placement_pattern" json:"recent_placement_pattern"`
	CurrentConcerns             string     `db:"current_concerns" json:"current_concerns"`
	Strengths                   string     `db:"strengths" json:"strengths"`
	ExistingPlans               string     `db:"existing_plans" json:"existing_plans"`
	ChildsVoice                 string     `db:"childs_voice" json:"childs_voice"`
	RelapseRisks                string     `db:"relapse_risks" json:"relapse_risks"`
	RelapseEarlySignals         string     `db:"relapse_early_signals" json:"relapse_early_signals"`
	RelapseSupportPlan          string     `db:"relapse_support_plan" json:"relapse_support_plan"`
	ReviewFrequency             string     `db:"review_frequency" json:"review_frequency"`
	NextReviewDate              *time.Time `db:"next_review_date" json:"next_review_date,omitempty"`
	LeadReviewer                string     `db:"lead_reviewer" json:"lead_reviewer"`
	OtherProfessionals          string     `db:"other_professionals" json:"other_professionals"`
	EarlyWarningSignals         string     `db:"early_warning_signals" json:"early_warning_signals"`
	NotesForTeam                string     `db:"notes_for_team" json:"notes_for_team"`
	FrameworkModel              string     `db:"framework_model" json:"framework_model"`
	FrameworkVersion            int        `db:"framework_version" json:"framework_version"`
	OnboardingPathway           string     `db:"onboarding_pathway" json:"onboarding_pathway,omitempty"`
	ImmediateInformation        string     `db:"immediate_information" json:"immediate_information"`
	ImmediateInformationStatus  string     `db:"immediate_information_status" json:"immediate_information_status"`
	BackgroundInformationStatus string     `db:"background_information_status" json:"background_information_status"`
	KnownSafetyInformation      string     `db:"known_safety_information" json:"known_safety_information"`
	SafetyInformationStatus     string     `db:"safety_information_status" json:"safety_information_status"`
	CreatedByStaffID            *int64     `db:"created_by_staff_id" json:"created_by_staff_id,omitempty"`

	Status     string     `db:"status" json:"status"` // draft | submitted | approved
	ApprovedBy *string    `db:"approved_by" json:"approved_by,omitempty"`
	ApprovedAt *time.Time `db:"approved_at" json:"approved_at,omitempty"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type ChildOnboardingDomainState struct {
	ID                         int64      `db:"id" json:"id"`
	BaselineID                 int64      `db:"baseline_id" json:"baseline_id"`
	DomainKey                  string     `db:"domain_key" json:"domain_key"`
	Active                     bool       `db:"active" json:"active"`
	Reason                     string     `db:"reason" json:"reason"`
	CurrentUnderstanding       string     `db:"current_understanding" json:"current_understanding"`
	StrengthsProtectiveFactors string     `db:"strengths_protective_factors" json:"strengths_protective_factors"`
	SupportNeeds               string     `db:"support_needs" json:"support_needs"`
	WhatHelps                  string     `db:"what_helps" json:"what_helps"`
	CurrentSupport             string     `db:"current_support" json:"current_support"`
	YoungPersonView            string     `db:"young_person_view" json:"young_person_view"`
	InformationStatus          string     `db:"information_status" json:"information_status"`
	InformationSource          string     `db:"information_source" json:"information_source"`
	SourceDetails              string     `db:"source_details" json:"source_details"`
	SourceDate                 *time.Time `db:"source_date" json:"source_date,omitempty"`
	PriorityReason             string     `db:"priority_reason" json:"priority_reason"`
}

type ChildOnboardingFrameworkSource struct {
	ID         int64      `db:"id" json:"id"`
	BaselineID int64      `db:"baseline_id" json:"baseline_id"`
	SourceType string     `db:"source_type" json:"source_type"`
	SourceName string     `db:"source_name" json:"source_name"`
	SourceDate *time.Time `db:"source_date" json:"source_date,omitempty"`
	Notes      string     `db:"notes" json:"notes"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
}

type ChildOnboardingFrameworkSummary struct {
	ID                int64      `json:"id"`
	Version           int        `json:"version"`
	Status            string     `json:"status"`
	OnboardingPathway string     `json:"onboarding_pathway"`
	CreatedAt         time.Time  `json:"created_at"`
	ApprovedBy        *string    `json:"approved_by,omitempty"`
	ApprovedAt        *time.Time `json:"approved_at,omitempty"`
}

type ChildPriorityDomain struct {
	ID             int64  `db:"id" json:"id"`
	BaselineID     int64  `db:"baseline_id" json:"baseline_id"`
	DomainKey      string `db:"domain_key" json:"domain_key"`
	FocusSummary   string `db:"focus_summary" json:"focus_summary"`
	HopedForChange string `db:"hoped_for_change" json:"hoped_for_change"`
	SortOrder      int    `db:"sort_order" json:"sort_order"`
}
