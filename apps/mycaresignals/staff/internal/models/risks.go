package models

import "time"

type RiskStatus string

const (
	RiskStatusActive RiskStatus = "active"
	RiskStatusPaused RiskStatus = "paused"
	RiskStatusClosed RiskStatus = "closed"
)

type RiskInformationStatus string

const (
	RiskInformationCurrentConfirmed         RiskInformationStatus = "current_confirmed"
	RiskInformationReported                 RiskInformationStatus = "reported"
	RiskInformationHistorical               RiskInformationStatus = "historical"
	RiskInformationAllegationNotEstablished RiskInformationStatus = "allegation_not_established"
	RiskInformationAwaitingConfirmation     RiskInformationStatus = "awaiting_confirmation"
)

type RiskManagementApproach string

const (
	RiskControlsOnly                 RiskManagementApproach = "controls_only"
	RiskControlsAndDevelopmentalWork RiskManagementApproach = "controls_and_developmental_work"
)

type RiskReviewOutcome string

const (
	RiskReviewContinue RiskReviewOutcome = "continue"
	RiskReviewModify   RiskReviewOutcome = "modify"
	RiskReviewClose    RiskReviewOutcome = "close"
)

type ChildRiskReview struct {
	ID                         int64             `json:"id"`
	RiskID                     int64             `json:"risk_id"`
	ChildID                    int64             `json:"child_id"`
	ReviewedAt                 time.Time         `json:"reviewed_at"`
	ReviewerStaffID            int64             `json:"reviewer_staff_id"`
	ReviewerDisplayName        string            `json:"reviewer_display_name"`
	PreviousResidualLikelihood *int              `json:"previous_residual_likelihood,omitempty"`
	PreviousResidualSeverity   *int              `json:"previous_residual_severity,omitempty"`
	ReviewedResidualLikelihood int               `json:"reviewed_residual_likelihood"`
	ReviewedResidualSeverity   int               `json:"reviewed_residual_severity"`
	ReviewNote                 string            `json:"review_note"`
	Outcome                    RiskReviewOutcome `json:"outcome"`
	NextReviewDate             *time.Time        `json:"next_review_date,omitempty"`
	CreatedAt                  time.Time         `json:"created_at"`
}

type ChildRisk struct {
	ID                  int64                  `json:"id"`
	ChildID             int64                  `json:"child_id"`
	Title               string                 `json:"title"`
	Category            string                 `json:"category"`
	Domain              *string                `json:"domain,omitempty"`
	Status              RiskStatus             `json:"status"`
	NatureOfRisk        string                 `json:"nature_of_risk"`
	WhoIsAtRisk         *string                `json:"who_is_at_risk,omitempty"`
	InformationStatus   RiskInformationStatus  `json:"information_status"`
	SourceNote          *string                `json:"source_note,omitempty"`
	InitialLikelihood   *int                   `json:"initial_likelihood,omitempty"`
	InitialSeverity     *int                   `json:"initial_severity,omitempty"`
	ResidualLikelihood  *int                   `json:"residual_likelihood,omitempty"`
	ResidualSeverity    *int                   `json:"residual_severity,omitempty"`
	ManagementApproach  RiskManagementApproach `json:"management_approach"`
	ProactiveStrategies *string                `json:"proactive_strategies,omitempty"`
	ReactiveStrategies  *string                `json:"reactive_strategies,omitempty"`
	EffectiveFrom       *time.Time             `json:"effective_from,omitempty"`
	LastReviewedAt      *time.Time             `json:"last_reviewed_at,omitempty"`
	NextReviewDate      *time.Time             `json:"next_review_date,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
	GoalIDs             []int64                `json:"goal_ids"`
	TargetIDs           []int64                `json:"target_ids"`
}
