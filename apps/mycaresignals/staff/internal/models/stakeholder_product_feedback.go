package models

import "time"

type StakeholderProductFeedbackInput struct {
	OverallRating       int
	MostUseful          string
	UnclearOrDifficult  string
	Missing             string
	AnythingElse        string
	PagePath            string
	Environment         string
	ReviewerName        string
	ReviewerEmail       string
	Organisation        string
	ReviewerStaffUserID int64
}

type StakeholderProductFeedback struct {
	ID                 int64     `json:"id"`
	ReviewerName       string    `json:"reviewer_name"`
	ReviewerEmail      string    `json:"reviewer_email"`
	Organisation       string    `json:"organisation"`
	Environment        string    `json:"environment"`
	PagePath           string    `json:"page_path"`
	OverallRating      int       `json:"overall_rating"`
	MostUseful         string    `json:"most_useful"`
	UnclearOrDifficult string    `json:"unclear_or_difficult"`
	Missing            string    `json:"missing"`
	AnythingElse       string    `json:"anything_else"`
	SubmittedAt        time.Time `json:"submitted_at"`
}
