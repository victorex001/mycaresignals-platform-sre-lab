// internal/models/summary.go
package models

type FeedbackSummary struct {
	Total         int               `json:"total"`
	ByWho         map[string]int    `json:"by_who"`
	AvgRating     float64           `json:"avg_rating"`
	ReviewPrompts ReviewPromptStats `json:"review_prompts"`
}

type ReviewPromptStats struct {
	Total    int `json:"total"`
	Urgent   int `json:"urgent"`
	Priority int `json:"priority"`
	Routine  int `json:"routine"`
}
