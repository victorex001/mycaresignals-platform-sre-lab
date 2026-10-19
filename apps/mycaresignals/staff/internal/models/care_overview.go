package models

import "time"

type ChildCareOverview struct {
	Child                        CareOverviewChild            `json:"child"`
	Baseline                     CareOverviewBaseline         `json:"baseline"`
	CurrentPriorities            []CareOverviewDomainPriority `json:"current_priorities"`
	GoalsTargets                 CareOverviewGoalsTargets     `json:"goals_targets"`
	RiskSafety                   CareOverviewRiskSafety       `json:"risk_safety"`
	CarePlan                     CareOverviewCarePlan         `json:"care_plan"`
	RecentKeyworking             []CareOverviewKeyworking     `json:"recent_keyworking"`
	RecentDailyProgress          []CareOverviewDailyProgress  `json:"recent_daily_progress"`
	DailyProgressReviewSuggested bool                         `json:"daily_progress_review_suggested"`
	ChildVoice                   CareOverviewChildVoice       `json:"child_voice"`
	NeedsAttention               []CareOverviewAttentionItem  `json:"needs_attention"`
	Timeline                     []CareOverviewTimelineEvent  `json:"timeline"`
}

type CareOverviewChild struct {
	DisplayName   string     `json:"display_name"`
	Initials      string     `json:"initials"`
	HomeName      string     `json:"home_name"`
	AdmissionDate *time.Time `json:"admission_date,omitempty"`
}

type CareOverviewBaseline struct {
	Status     string     `json:"status"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
	ApprovedAt *time.Time `json:"approved_at,omitempty"`
}

type CareOverviewDomainPriority struct {
	DomainKey        string `json:"domain_key"`
	Label            string `json:"label"`
	FocusSummary     string `json:"focus_summary,omitempty"`
	HopedForChange   string `json:"hoped_for_change,omitempty"`
	ActiveGoals      int    `json:"active_goals"`
	ActiveTargets    int    `json:"active_targets"`
	ActiveRisks      int    `json:"active_risks"`
	RecentKeyworking int    `json:"recent_keyworking"`
	TargetReviewsDue int    `json:"target_reviews_due"`
}

type CareOverviewGoalsTargets struct {
	ActiveGoals      int                      `json:"active_goals"`
	ActiveTargets    int                      `json:"active_targets"`
	TargetsDue       int                      `json:"targets_due"`
	TargetsOverdue   int                      `json:"targets_overdue"`
	AchievedRecently int                      `json:"achieved_recently"`
	Goals            []CareOverviewGoalItem   `json:"goals"`
	Targets          []CareOverviewTargetItem `json:"targets"`
}

type CareOverviewGoalItem struct {
	Title  string `json:"title"`
	Domain string `json:"domain,omitempty"`
	Status string `json:"status"`
}

type CareOverviewTargetItem struct {
	Label      string     `json:"label"`
	GoalTitle  string     `json:"goal_title,omitempty"`
	Domain     string     `json:"domain,omitempty"`
	ReviewDate *time.Time `json:"review_date,omitempty"`
	Status     string     `json:"status"`
}

type CareOverviewRiskSafety struct {
	ActiveRisks                   int                    `json:"active_risks"`
	RecordedLow                   int                    `json:"recorded_low"`
	RecordedMedium                int                    `json:"recorded_medium"`
	RecordedHigh                  int                    `json:"recorded_high"`
	RecordedNotRated              int                    `json:"recorded_not_rated"`
	ReviewsDue                    int                    `json:"reviews_due"`
	RecentCompletedReviews        int                    `json:"recent_completed_reviews"`
	DevelopmentalWorkMissingLinks int                    `json:"developmental_work_missing_links"`
	Items                         []CareOverviewRiskItem `json:"items"`
}

type CareOverviewRiskItem struct {
	Title          string     `json:"title"`
	Domain         string     `json:"domain,omitempty"`
	RecordedRating *int       `json:"recorded_rating,omitempty"`
	RecordedBand   string     `json:"recorded_band,omitempty"`
	NextReviewDate *time.Time `json:"next_review_date,omitempty"`
}

type CareOverviewCarePlan struct {
	Exists                       bool       `json:"exists"`
	Status                       string     `json:"status,omitempty"`
	Version                      int        `json:"version,omitempty"`
	EffectiveFrom                *time.Time `json:"effective_from,omitempty"`
	ReviewDate                   *time.Time `json:"review_date,omitempty"`
	SectionsNotKnown             int        `json:"sections_not_known"`
	SectionsAwaitingConfirmation int        `json:"sections_awaiting_confirmation"`
	LatestReviewOutcome          string     `json:"latest_review_outcome,omitempty"`
	LatestReviewedAt             *time.Time `json:"latest_reviewed_at,omitempty"`
	SuccessorDraftExists         bool       `json:"successor_draft_exists"`
	SuccessorDraftVersion        int        `json:"successor_draft_version,omitempty"`
}

type CareOverviewKeyworking struct {
	SessionAt         time.Time `json:"session_at"`
	TargetLabel       string    `json:"target_label,omitempty"`
	GoalTitle         string    `json:"goal_title,omitempty"`
	Domain            string    `json:"domain,omitempty"`
	SignedByKeyWorker bool      `json:"signed_by_key_worker"`
}

type CareOverviewDailyProgress struct {
	ObservedOn          time.Time `json:"observed_on"`
	Domain              string    `json:"domain"`
	TargetLabel         string    `json:"target_label,omitempty"`
	WellbeingScore      *int      `json:"wellbeing_score"`
	ProgressScore       *int      `json:"progress_score"`
	ObservationNote     string    `json:"observation_note,omitempty"`
	RecordedByStaffName string    `json:"recorded_by_staff_name"`
}

type CareOverviewChildVoice struct {
	RecentEntryCount int        `json:"recent_entry_count"`
	LatestDate       *time.Time `json:"latest_date,omitempty"`
}

type CareOverviewAttentionItem struct {
	Kind     string     `json:"kind"`
	Label    string     `json:"label"`
	DueDate  *time.Time `json:"due_date,omitempty"`
	Priority int        `json:"priority"`
	Source   string     `json:"source"`
}

type CareOverviewTimelineEvent struct {
	OccurredAt time.Time `json:"occurred_at"`
	Type       string    `json:"type"`
	Title      string    `json:"title"`
	Detail     string    `json:"detail,omitempty"`
	Source     string    `json:"source"`
}
