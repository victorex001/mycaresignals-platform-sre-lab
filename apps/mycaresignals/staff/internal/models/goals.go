// internal/models/goals.go
package models

import "time"

type GoalType string

const (
	GoalTypeCarePlan    GoalType = "care_plan"
	GoalTypePlacement   GoalType = "placement"
	GoalTypeTherapeutic GoalType = "therapeutic"
)

type GoalStatus string

const (
	GoalStatusActive   GoalStatus = "active"
	GoalStatusPaused   GoalStatus = "paused"
	GoalStatusAchieved GoalStatus = "achieved"
	GoalStatusClosed   GoalStatus = "closed"
)

// One medium-term goal for a child
type ChildGoal struct {
	ID          int64      `json:"id"`
	ChildID     int64      `json:"child_id"`
	GoalType    GoalType   `json:"goal_type"`
	Title       string     `json:"title"`
	Domain      *string    `json:"domain,omitempty"`
	Description string     `json:"description,omitempty"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	TargetDate  *time.Time `json:"target_date,omitempty"`
	Status      GoalStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// NEW – psychologically-informed fields
	ChildVoice         *string `json:"child_voice,omitempty"`
	FormulationSummary *string `json:"formulation_summary,omitempty"`
	ProtectiveFactors  *string `json:"protective_factors,omitempty"`
}

// Domains: emotional, behaviour, risk, education, relationships, self_care, independence, etc.
type ChildTarget struct {
	ID          int64      `json:"id"`
	ChildID     int64      `json:"child_id"`
	GoalID      *int64     `json:"goal_id,omitempty"`
	Domain      string     `json:"domain"`
	ShortLabel  string     `json:"short_label"`
	Description string     `json:"description,omitempty"`
	Status      GoalStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// NEW – barriers & supports for this target
	Barriers               *string    `json:"barriers,omitempty"`
	SupportsNeeded         *string    `json:"supports_needed,omitempty"`
	InterventionStrategy   *string    `json:"intervention_strategy,omitempty"`
	ReviewDate             *time.Time `json:"review_date,omitempty"`
	DailyObservationCount  int        `json:"daily_observation_count"`
	KeyworkingSessionCount int        `json:"keyworking_session_count"`
}
