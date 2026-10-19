// internal/models/signals_response.go
package models

import "time"

type ChildSignalResponse struct {
	ID         int64   `json:"id"`
	ChildID    ChildID `json:"child_id"`
	SignalKind string  `json:"signal_kind"`

	DidCheckIn      bool `json:"did_checkin"`
	AdjustedSupport bool `json:"adjusted_support"`
	InformedManager bool `json:"informed_manager"`
	InformedNetwork bool `json:"informed_network"`
	MonitorOnly     bool `json:"monitor_only"`

	DidReferral          bool   `json:"did_referral"`
	ReferralDestinations string `json:"referral_destinations"`

	ActionsSummary string `json:"actions_summary"`
	FutureNotes    string `json:"future_notes"`

	ExternalLogRef string    `json:"external_log_ref"`
	CreatedAt      time.Time `json:"created_at"`
}