package models

import "time"

type CarePlanStatus string

const (
	CarePlanStatusDraft    CarePlanStatus = "draft"
	CarePlanStatusActive   CarePlanStatus = "active"
	CarePlanStatusArchived CarePlanStatus = "archived"
)

type CarePlanInformationStatus string

const (
	CarePlanInformationKnown                CarePlanInformationStatus = "known"
	CarePlanInformationPartlyKnown          CarePlanInformationStatus = "partly_known"
	CarePlanInformationNotKnownYet          CarePlanInformationStatus = "not_known_yet"
	CarePlanInformationAwaitingConfirmation CarePlanInformationStatus = "awaiting_confirmation"
	CarePlanInformationNotApplicable        CarePlanInformationStatus = "not_applicable"
)

type CarePlanSectionKey string

const (
	CarePlanSectionAboutMe                 CarePlanSectionKey = "about_me"
	CarePlanSectionSafetyStability         CarePlanSectionKey = "safety_stability"
	CarePlanSectionEmotionalWellbeing      CarePlanSectionKey = "emotional_wellbeing"
	CarePlanSectionHealth                  CarePlanSectionKey = "health"
	CarePlanSectionEducationFuture         CarePlanSectionKey = "education_future"
	CarePlanSectionRelationshipsBelonging  CarePlanSectionKey = "relationships_belonging"
	CarePlanSectionIndependenceDailyLiving CarePlanSectionKey = "independence_daily_living"
	CarePlanSectionIdentityCulture         CarePlanSectionKey = "identity_culture"
)

var CanonicalCarePlanSectionKeys = []CarePlanSectionKey{
	CarePlanSectionAboutMe,
	CarePlanSectionSafetyStability,
	CarePlanSectionEmotionalWellbeing,
	CarePlanSectionHealth,
	CarePlanSectionEducationFuture,
	CarePlanSectionRelationshipsBelonging,
	CarePlanSectionIndependenceDailyLiving,
	CarePlanSectionIdentityCulture,
}

type ChildCarePlan struct {
	ID                    int64                  `json:"id"`
	ChildID               int64                  `json:"child_id"`
	Status                CarePlanStatus         `json:"status"`
	EffectiveFrom         time.Time              `json:"effective_from"`
	ReviewDate            *time.Time             `json:"review_date,omitempty"`
	PredecessorCarePlanID *int64                 `json:"predecessor_care_plan_id,omitempty"`
	CreatedByStaffID      int64                  `json:"created_by_staff_id"`
	ActivatedByStaffID    *int64                 `json:"activated_by_staff_id,omitempty"`
	ActivatedAt           *time.Time             `json:"activated_at,omitempty"`
	ArchivedByStaffID     *int64                 `json:"archived_by_staff_id,omitempty"`
	ArchivedAt            *time.Time             `json:"archived_at,omitempty"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
	Sections              []ChildCarePlanSection `json:"sections"`
}

type CarePlanReviewOutcome string

const (
	CarePlanReviewContinue CarePlanReviewOutcome = "continue"
	CarePlanReviewRevise   CarePlanReviewOutcome = "revise"
	CarePlanReviewClose    CarePlanReviewOutcome = "close"
)

type CarePlanReviewEvidence struct {
	EvidenceSince                     time.Time `json:"evidence_since"`
	ActiveGoalCount                   int       `json:"active_goal_count"`
	ActiveTargetCount                 int       `json:"active_target_count"`
	AchievedTargetCount               int       `json:"achieved_target_count"`
	PausedTargetCount                 int       `json:"paused_target_count"`
	TargetsChangedCount               int       `json:"targets_changed_count"`
	TargetsDueCount                   int       `json:"targets_due_count"`
	ActiveRiskCount                   int       `json:"active_risk_count"`
	RiskReviewsCompletedCount         int       `json:"risk_reviews_completed_count"`
	KeyworkingSessionCount            int       `json:"keyworking_session_count"`
	SectionsNotKnownCount             int       `json:"sections_not_known_count"`
	SectionsAwaitingConfirmationCount int       `json:"sections_awaiting_confirmation_count"`
}

type ChildCarePlanReview struct {
	ID                         int64                        `json:"id"`
	CarePlanID                 int64                        `json:"care_plan_id"`
	ChildID                    int64                        `json:"child_id"`
	ReviewedAt                 time.Time                    `json:"reviewed_at"`
	ReviewerStaffID            int64                        `json:"reviewer_staff_id"`
	ReviewerDisplayName        string                       `json:"reviewer_display_name"`
	ReviewNote                 string                       `json:"review_note"`
	YoungPersonInvolvementNote *string                      `json:"young_person_involvement_note,omitempty"`
	Outcome                    CarePlanReviewOutcome        `json:"outcome"`
	NextReviewDate             *time.Time                   `json:"next_review_date,omitempty"`
	SuccessorCarePlanID        *int64                       `json:"successor_care_plan_id,omitempty"`
	SnapshotStatus             CarePlanStatus               `json:"snapshot_status"`
	SnapshotEffectiveFrom      time.Time                    `json:"snapshot_effective_from"`
	SnapshotReviewDate         *time.Time                   `json:"snapshot_review_date,omitempty"`
	Evidence                   CarePlanReviewEvidence       `json:"evidence"`
	CreatedAt                  time.Time                    `json:"created_at"`
	Sections                   []ChildCarePlanReviewSection `json:"sections"`
}

type ChildCarePlanReviewSection struct {
	ID                         int64                     `json:"id"`
	ReviewID                   int64                     `json:"review_id"`
	SectionKey                 CarePlanSectionKey        `json:"section_key"`
	InformationStatus          CarePlanInformationStatus `json:"information_status"`
	CurrentUnderstanding       *string                   `json:"current_understanding,omitempty"`
	Strengths                  *string                   `json:"strengths,omitempty"`
	StaffSupport               *string                   `json:"staff_support,omitempty"`
	YoungPersonInvolvementNote *string                   `json:"young_person_involvement_note,omitempty"`
	CreatedAt                  time.Time                 `json:"created_at"`
}

type ChildCarePlanSection struct {
	ID                         int64                     `json:"id"`
	CarePlanID                 int64                     `json:"care_plan_id"`
	SectionKey                 CarePlanSectionKey        `json:"section_key"`
	InformationStatus          CarePlanInformationStatus `json:"information_status"`
	CurrentUnderstanding       *string                   `json:"current_understanding,omitempty"`
	Strengths                  *string                   `json:"strengths,omitempty"`
	StaffSupport               *string                   `json:"staff_support,omitempty"`
	YoungPersonInvolvementNote *string                   `json:"young_person_involvement_note,omitempty"`
	CreatedAt                  time.Time                 `json:"created_at"`
	UpdatedAt                  time.Time                 `json:"updated_at"`
}
