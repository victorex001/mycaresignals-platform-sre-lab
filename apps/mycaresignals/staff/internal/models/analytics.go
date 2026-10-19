// internal/models/analytics.go
package models

type DomainIndex struct {
	Domain           string   `json:"domain"`
	AverageWellbeing float64  `json:"average_wellbeing"`
	AverageProgress  float64  `json:"average_progress"`
	Index            float64  `json:"index"` // 0–10 average of wellbeing+progress
	PreviousIndex    *float64 `json:"previous_index,omitempty"`
	Change           *float64 `json:"change,omitempty"`
}

type OverallIndex struct {
	Index         float64  `json:"index"`
	PreviousIndex *float64 `json:"previous_index,omitempty"`
	Change        *float64 `json:"change,omitempty"`
}

type ChildDomainIndex struct {
	ChildID    int64         `json:"child_id"`
	WindowDays int           `json:"window_days"`
	Overall    OverallIndex  `json:"overall"`
	Domains    []DomainIndex `json:"domains"`
}