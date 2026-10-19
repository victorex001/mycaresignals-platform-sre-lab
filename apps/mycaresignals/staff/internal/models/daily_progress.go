package models

import "time"

const YoungPersonMonthlyVoiceSource = "young_person_monthly_voice"

var CanonicalProgressDomains = []string{
	"Emotional & Psychological Wellbeing",
	"Behaviour & Self-Regulation",
	"Relationships & Connection",
	"Daily Living, Learning & Participation",
	"Safety, Risk & Coping with Stress",
	"Skills, Identity & Future",
}

func ValidProgressDomain(value string) bool {
	for _, domain := range CanonicalProgressDomains {
		if value == domain {
			return true
		}
	}
	return false
}

func ValidMonthlyVoiceRatings(ratings map[string]int) bool {
	if len(ratings) != len(CanonicalProgressDomains) {
		return false
	}
	for _, domain := range CanonicalProgressDomains {
		rating, ok := ratings[domain]
		if !ok || rating < 0 || rating > 4 {
			return false
		}
	}
	return true
}

type DailyProgressInput struct {
	ObservedOn         time.Time
	Domain             string
	TargetID           *int64
	WellbeingScore     *int
	ProgressScore      *int
	ObservationNote    *string
	YoungPersonComment *string
}

type ChildDailyProgress struct {
	ID                  int64     `json:"id"`
	ChildID             int64     `json:"child_id"`
	RecordedByStaffID   int64     `json:"recorded_by_staff_id"`
	RecordedByStaffName string    `json:"recorded_by_staff_name"`
	ObservedOn          time.Time `json:"observed_on"`
	Domain              string    `json:"domain"`
	TargetID            *int64    `json:"target_id,omitempty"`
	TargetLabel         *string   `json:"target_label,omitempty"`
	WellbeingScore      *int      `json:"wellbeing_score"`
	ProgressScore       *int      `json:"progress_score"`
	ObservationNote     *string   `json:"observation_note,omitempty"`
	YoungPersonComment  *string   `json:"young_person_comment,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

type ChildMonthlyVoiceRating struct {
	ID                int64     `json:"id"`
	ChildID           int64     `json:"child_id"`
	PeriodMonth       time.Time `json:"period_month"`
	Domain            string    `json:"domain"`
	YoungPersonRating int       `json:"young_person_rating"`
	SourceReceiptID   int64     `json:"source_receipt_id"`
	SourceReviewID    *int64    `json:"source_review_id,omitempty"`
	SourceContextID   *string   `json:"source_context_id,omitempty"`
	Source            string    `json:"source"`
	SubmittedAt       time.Time `json:"submitted_at"`
	CreatedAt         time.Time `json:"created_at"`
}

type MonthlyKeyworkingCount struct {
	PeriodMonth time.Time
	Domain      string
	Count       int
}

type MonthlyScoreStatistics struct {
	Count        int            `json:"count"`
	Distribution map[string]int `json:"distribution"`
	Mean         *float64       `json:"mean,omitempty"`
	Median       *float64       `json:"median,omitempty"`
	Min          *int           `json:"min,omitempty"`
	Max          *int           `json:"max,omitempty"`
	Latest       *int           `json:"latest,omitempty"`
}

type MonthlyPerspectiveComparison struct {
	PeriodMonth               string                 `json:"period_month"`
	Domain                    string                 `json:"domain"`
	ObservationCount          int                    `json:"observation_count"`
	Wellbeing                 MonthlyScoreStatistics `json:"wellbeing"`
	Progress                  MonthlyScoreStatistics `json:"progress"`
	StaffDescriptiveValue     *float64               `json:"staff_descriptive_value,omitempty"`
	StaffValueBasis           string                 `json:"staff_value_basis"`
	YoungPersonRating         *int                   `json:"young_person_rating,omitempty"`
	YoungPersonSource         *string                `json:"young_person_source,omitempty"`
	YoungPersonSourceReceipt  *int64                 `json:"young_person_source_receipt_id,omitempty"`
	YoungPersonSubmittedAt    *time.Time             `json:"young_person_submitted_at,omitempty"`
	PerspectiveDifference     *float64               `json:"perspective_difference,omitempty"`
	KeyworkingSessionCount    int                    `json:"keyworking_session_count"`
	ComparisonStatus          string                 `json:"comparison_status"`
	PersistentReviewSuggested bool                   `json:"persistent_review_suggested"`
	ReviewSuggestion          *string                `json:"review_suggestion,omitempty"`
	RuleVersion               string                 `json:"rule_version"`
}
