// internal/models/therapeutic.go
package models

import "time"

type ManagerReviewDecision string

const (
	ManagerReviewApproved          ManagerReviewDecision = "approved"
	ManagerReviewReturnedForReview ManagerReviewDecision = "returned_for_review"
)

// KeyworkSession represents one key-working session or period of work,
// usually matching something in ClearCare notes.
type KeyworkSession struct {
	ID        int64     `db:"id" json:"id"`
	ChildID   int64     `db:"child_id" json:"child_id"`
	SessionAt time.Time `db:"session_at" json:"session_at"` // when the work happened
	StaffName string    `db:"staff_name" json:"staff_name"`
	Notes     string    `db:"notes" json:"notes"` // optional textual summary / link ref

	// Structured key-working journey fields
	SessionType              string                 `db:"session_type" json:"session_type"`
	PrimaryDomain            *string                `db:"primary_domain" json:"primary_domain,omitempty"`
	TargetID                 *int64                 `db:"target_id" json:"target_id,omitempty"`
	YoungPersonPresentation  *string                `db:"young_person_presentation" json:"young_person_presentation,omitempty"`
	YoungPersonVoiceNote     *string                `db:"young_person_voice_note" json:"young_person_voice_note,omitempty"`
	KeyWorkerReflection      *string                `db:"key_worker_reflection" json:"key_worker_reflection,omitempty"`
	SignedByKeyWorker        bool                   `db:"signed_by_key_worker" json:"signed_by_key_worker"`
	KeyWorkerSignedAt        *time.Time             `db:"key_worker_signed_at" json:"key_worker_signed_at,omitempty"`
	SignedByManager          bool                   `db:"signed_by_manager" json:"signed_by_manager"`
	ManagerSignedAt          *time.Time             `db:"manager_signed_at" json:"manager_signed_at,omitempty"`
	ManagerName              *string                `db:"manager_name" json:"-"`
	ManagerReviewDecision    *ManagerReviewDecision `db:"manager_review_decision" json:"manager_review_decision,omitempty"`
	ManagerReviewComment     *string                `db:"manager_review_comment" json:"manager_review_comment,omitempty"`
	ManagerReviewedByStaffID *int64                 `db:"manager_reviewed_by_staff_id" json:"-"`
	ManagerReviewerName      *string                `db:"-" json:"manager_reviewer_name,omitempty"`
	CanManagerReview         bool                   `db:"-" json:"can_manager_review"`
	Scores                   []KeyworkSessionScore  `db:"-" json:"scores"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type KeyworkSessionScore struct {
	TargetID       int64  `json:"target_id"`
	WellbeingScore *int32 `json:"wellbeing_score,omitempty"`
	ProgressScore  *int32 `json:"progress_score,omitempty"`
	Comment        string `json:"comment,omitempty"`
}

type KeyworkManagerReview struct {
	SessionID    int64                 `json:"session_id"`
	Decision     ManagerReviewDecision `json:"decision"`
	Comment      *string               `json:"comment,omitempty"`
	ReviewerName string                `json:"reviewer_name"`
	ReviewedAt   time.Time             `json:"reviewed_at"`
}

// SessionScore holds the “double scoring” for one target in one session.
// Example: (Wellbeing=2, Progress=4).
type SessionScore struct {
	ID             int64     `db:"id" json:"id"`
	SessionID      int64     `db:"session_id" json:"session_id"`
	TargetID       int64     `db:"target_id" json:"target_id"`
	WellbeingScore int32     `db:"wellbeing_score" json:"wellbeing_score"`
	ProgressScore  int32     `db:"progress_score" json:"progress_score"`
	Comment        string    `db:"comment" json:"comment"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
}

// SessionScoreWithTarget is a helper view object for timelines / APIs:
// it bundles score + target label so UI can render a nice line.
type SessionScoreWithTarget struct {
	ScoreID        int64     `json:"score_id"`
	SessionID      int64     `json:"session_id"`
	TargetID       int64     `json:"target_id"`
	TargetName     string    `json:"target_name"`
	TargetDomain   string    `json:"target_domain"`
	WellbeingScore int32     `json:"wellbeing_score"`
	ProgressScore  int32     `json:"progress_score"`
	Comment        string    `json:"comment"`
	CreatedAt      time.Time `json:"created_at"`
}

// TimelineItem is a flattened view used for the "child timeline" API.
// Each item is one scored target in a session, with all the context you need
// for a chart or narrative timeline.
type TimelineItem struct {
	SessionID      int64     `json:"session_id"`
	SessionAt      time.Time `json:"session_at"`
	StaffName      string    `json:"staff_name"`
	TargetID       int64     `json:"target_id"`
	TargetName     string    `json:"target_name"`
	TargetDomain   string    `json:"target_domain"`
	WellbeingScore int32     `json:"wellbeing_score"`
	ProgressScore  int32     `json:"progress_score"`
	Comment        string    `json:"comment"`
}

type PsychologicalReview struct {
	ID                int64     `json:"id" db:"id"`
	ChildID           int64     `json:"child_id" db:"child_id"`
	WindowDays        int       `json:"window_days" db:"window_days"`
	ReviewFrom        time.Time `json:"review_from" db:"review_from"`
	ReviewTo          time.Time `json:"review_to" db:"review_to"`
	Formulation       string    `json:"formulation" db:"formulation"`
	ProtectiveFactors string    `json:"protective_factors" db:"protective_factors"`
	Recommendations   string    `json:"recommendations" db:"recommendations"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	CreatedBy         string    `json:"created_by" db:"created_by"`
}
