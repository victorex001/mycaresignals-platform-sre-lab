package models

import "time"

type MissingFromHomePlanStatus string

const (
	MFHPlanDraft      MissingFromHomePlanStatus = "draft"
	MFHPlanSubmitted  MissingFromHomePlanStatus = "submitted"
	MFHPlanApproved   MissingFromHomePlanStatus = "approved"
	MFHPlanSuperseded MissingFromHomePlanStatus = "superseded"
	MFHPlanArchived   MissingFromHomePlanStatus = "archived"
)

type MFHInformationStatus string

const (
	MFHInformationKnownConfirmed       MFHInformationStatus = "known_confirmed"
	MFHInformationPartlyKnown          MFHInformationStatus = "partly_known"
	MFHInformationAwaitingConfirmation MFHInformationStatus = "awaiting_confirmation"
	MFHInformationNotKnownYet          MFHInformationStatus = "not_known_yet"
)

type MFHCurrentSituation struct {
	SupervisionArrangement string `json:"supervision_arrangement"`
	FreeTimeArrangement    string `json:"free_time_arrangement"`
	ConfirmedLegalContext  string `json:"confirmed_legal_context"`
	CurrentRiskContext     string `json:"current_risk_context"`
}

type MFHMissingHistory struct {
	HistoryType       string               `json:"history_type"`
	Patterns          string               `json:"patterns"`
	Circumstances     string               `json:"circumstances"`
	WhatHelpedReturn  string               `json:"what_helped_return"`
	Source            string               `json:"source"`
	InformationStatus MFHInformationStatus `json:"information_status"`
}

type MFHPersonPlace struct {
	Kind              string               `json:"kind"`
	NameOrLabel       string               `json:"name_or_label"`
	Details           string               `json:"details"`
	ContactOrRoute    string               `json:"contact_or_route"`
	Source            string               `json:"source"`
	InformationStatus MFHInformationStatus `json:"information_status"`
}

type MFHTrigger struct {
	Description       string               `json:"description"`
	Source            string               `json:"source"`
	InformationStatus MFHInformationStatus `json:"information_status"`
}

type MFHOperationalResponse struct {
	ImmediateChecks            string `json:"immediate_checks"`
	ContactAttempts            string `json:"contact_attempts"`
	LocalSearchExpectations    string `json:"local_search_expectations"`
	PolicePolicyWording        string `json:"police_policy_wording"`
	InformationForPolice       string `json:"information_for_police"`
	SafeguardingConsiderations string `json:"safeguarding_considerations"`
}

type MFHNotification struct {
	Role           string `json:"role"`
	ContactName    string `json:"contact_name"`
	ContactDetails string `json:"contact_details"`
	WhenToNotify   string `json:"when_to_notify"`
	Source         string `json:"source"`
}

type MFHReturnFollowUp struct {
	ImmediateWelfareResponse    string `json:"immediate_welfare_response"`
	MedicalSafeguardingCheck    string `json:"medical_safeguarding_check"`
	CalmWelcomeRecoveryApproach string `json:"calm_welcome_recovery_approach"`
	ReturnConversation          string `json:"return_conversation"`
	IndependentReturnInterview  string `json:"independent_return_interview"`
	ProfessionalNotifications   string `json:"professional_notifications"`
	KeyWorkingFollowUp          string `json:"key_working_follow_up"`
	ConsiderRiskReview          bool   `json:"consider_risk_review"`
	ConsiderCarePlanReview      bool   `json:"consider_care_plan_review"`
}

type MissingFromHomePlanContent struct {
	CurrentSituation MFHCurrentSituation    `json:"current_situation"`
	MissingHistory   MFHMissingHistory      `json:"missing_history"`
	PeoplePlaces     []MFHPersonPlace       `json:"people_places"`
	Triggers         []MFHTrigger           `json:"triggers"`
	Operational      MFHOperationalResponse `json:"operational_response"`
	Notifications    []MFHNotification      `json:"notifications"`
	ReturnFollowUp   MFHReturnFollowUp      `json:"return_follow_up"`
}

type MFHReuseCandidate struct {
	CandidateKey      string               `json:"candidate_key"`
	Category          string               `json:"category"`
	SourceType        string               `json:"source_type"`
	SourceID          *int64               `json:"source_id,omitempty"`
	Label             string               `json:"label"`
	Value             string               `json:"value"`
	SourceLabel       string               `json:"source_label"`
	InformationStatus MFHInformationStatus `json:"information_status"`
	SourceUpdatedAt   *time.Time           `json:"source_updated_at,omitempty"`
	SnapshotSHA256    string               `json:"-"`
}

type MFHSourceSnapshot struct {
	CandidateKey      string               `json:"candidate_key"`
	Category          string               `json:"category"`
	SourceType        string               `json:"source_type"`
	SourceID          *int64               `json:"source_id,omitempty"`
	Label             string               `json:"label"`
	Value             string               `json:"value"`
	SourceLabel       string               `json:"source_label"`
	InformationStatus MFHInformationStatus `json:"information_status"`
	SourceUpdatedAt   *time.Time           `json:"source_updated_at,omitempty"`
	SnapshotSHA256    string               `json:"snapshot_sha256"`
	Included          bool                 `json:"included"`
	Changed           bool                 `json:"changed"`
}

type MissingFromHomePlan struct {
	ID                 int64                      `json:"id"`
	ChildID            int64                      `json:"child_id"`
	Version            int                        `json:"version"`
	Status             MissingFromHomePlanStatus  `json:"status"`
	PredecessorPlanID  *int64                     `json:"predecessor_plan_id,omitempty"`
	Content            MissingFromHomePlanContent `json:"content"`
	SourceSnapshots    []MFHSourceSnapshot        `json:"source_snapshots"`
	SourcesChanged     bool                       `json:"sources_changed"`
	ReviewDate         *time.Time                 `json:"review_date,omitempty"`
	CreatedByStaffID   int64                      `json:"created_by_staff_id"`
	CreatedByName      string                     `json:"created_by_name"`
	SubmittedByStaffID *int64                     `json:"submitted_by_staff_id,omitempty"`
	SubmittedByName    *string                    `json:"submitted_by_name,omitempty"`
	ApprovedByStaffID  *int64                     `json:"approved_by_staff_id,omitempty"`
	ApprovedByName     *string                    `json:"approved_by_name,omitempty"`
	ArchivedByStaffID  *int64                     `json:"archived_by_staff_id,omitempty"`
	ArchivedByName     *string                    `json:"archived_by_name,omitempty"`
	SubmittedAt        *time.Time                 `json:"submitted_at,omitempty"`
	ApprovedAt         *time.Time                 `json:"approved_at,omitempty"`
	ArchivedAt         *time.Time                 `json:"archived_at,omitempty"`
	CreatedAt          time.Time                  `json:"created_at"`
	UpdatedAt          time.Time                  `json:"updated_at"`
}

type MFHSourceDecision struct {
	CandidateKey string `json:"candidate_key"`
	Included     bool   `json:"included"`
}
