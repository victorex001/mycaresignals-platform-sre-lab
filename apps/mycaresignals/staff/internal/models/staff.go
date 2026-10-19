package models

import (
	"encoding/json"
	"time"
)

type StaffUser struct {
	ID                  int64     `db:"id" json:"id"`
	Email               string    `db:"email" json:"email"`
	DisplayName         string    `db:"display_name" json:"display_name"`
	ExternalAuthSubject *string   `db:"external_auth_subject" json:"-"`
	Status              string    `db:"status" json:"status"`
	CreatedAt           time.Time `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time `db:"updated_at" json:"updated_at"`
}

type StaffHomeRole struct {
	ID          int64     `db:"id" json:"id"`
	StaffUserID int64     `db:"staff_user_id" json:"staff_user_id"`
	HomeID      int64     `db:"home_id" json:"home_id"`
	Role        string    `db:"role" json:"role"`
	Active      bool      `db:"active" json:"active"`
	AssignedAt  time.Time `db:"assigned_at" json:"assigned_at"`
}

type AuditEvent struct {
	ID          int64  `db:"id" json:"id"`
	HomeID      *int64 `db:"home_id" json:"home_id,omitempty"`
	ChildID     *int64 `db:"child_id" json:"child_id,omitempty"`
	StaffUserID *int64 `db:"staff_user_id" json:"staff_user_id,omitempty"`

	ActorEmail *string `db:"actor_email" json:"actor_email,omitempty"`
	ActorName  *string `db:"actor_name" json:"actor_name,omitempty"`

	EventType  string `db:"event_type" json:"event_type"`
	EntityType string `db:"entity_type" json:"entity_type"`
	EntityID   *int64 `db:"entity_id" json:"entity_id,omitempty"`

	RequestMethod *string `db:"request_method" json:"request_method,omitempty"`
	RequestPath   *string `db:"request_path" json:"request_path,omitempty"`

	Metadata  json.RawMessage `db:"metadata" json:"metadata"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
}
