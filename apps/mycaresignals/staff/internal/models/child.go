package models

import "time"

type HomeID int64
type ChildID int64

type Child struct {
    ID            ChildID   `db:"id" json:"id"`
    HomeID        HomeID    `db:"home_id" json:"home_id"`
    ExternalRef   string    `db:"external_ref" json:"external_ref,omitempty"`
    Initials      string    `db:"initials" json:"initials"`
    FirstName     string    `db:"first_name" json:"first_name,omitempty"`
    LastName      string    `db:"last_name" json:"last_name,omitempty"`
    DateOfBirth   time.Time `db:"date_of_birth" json:"date_of_birth"`
    LegalStatus   string    `db:"legal_status" json:"legal_status,omitempty"`
    AdmissionDate *time.Time `db:"admission_date" json:"admission_date,omitempty"`
    Gender        string    `db:"gender" json:"gender,omitempty"`
    Active        bool      `db:"active" json:"active"`
    CreatedAt     time.Time `db:"created_at" json:"created_at"`
    UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`

    BaselineStatus  string  `db:"-" json:"baseline_status,omitempty"`
}
