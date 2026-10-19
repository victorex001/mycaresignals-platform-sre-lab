package storage

import (
	"context"
	"fmt"
	"strings"

	"therapeutic-engine/internal/security"
)

type DemoStakeholderMapping struct {
	StaffUserID   int64
	AlreadyMapped bool
}

type DemoStakeholderInvite struct {
	Name         string `json:"name"`
	Organisation string `json:"organisation"`
	Email        string `json:"email"`
	Status       string `json:"status"`
	InvitedDate  string `json:"invited_date"`
}

const demoStakeholderHomeRole = security.RoleResponsibleIndividual

func (r *Repo) UpsertDemoStakeholder(ctx context.Context, email, displayName, externalSubject string, homeID int64) (DemoStakeholderMapping, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	displayName = strings.TrimSpace(displayName)
	externalSubject = strings.TrimSpace(externalSubject)
	if email == "" || externalSubject == "" || homeID <= 0 {
		return DemoStakeholderMapping{}, fmt.Errorf("invalid demo stakeholder mapping")
	}
	if displayName == "" {
		displayName = email
	}

	var result DemoStakeholderMapping
	err := r.WithTx(ctx, nil, func(tx *TxRepo) error {
		const staffQuery = `
INSERT INTO staff_users (email, display_name, external_auth_subject, status)
VALUES ($1, $2, $3, 'active')
ON CONFLICT (email) DO UPDATE
SET display_name = EXCLUDED.display_name,
    external_auth_subject = EXCLUDED.external_auth_subject,
    status = 'active',
    updated_at = now()
RETURNING id`
		if err := tx.q.QueryRowContext(ctx, staffQuery, email, displayName, externalSubject).Scan(&result.StaffUserID); err != nil {
			return fmt.Errorf("upsert demo stakeholder staff user: %w", err)
		}

		const existingRoleQuery = `
SELECT EXISTS (
  SELECT 1
  FROM staff_home_roles
  WHERE staff_user_id = $1
    AND home_id = $2
    AND role = $3
    AND active = TRUE
)`
		if err := tx.q.QueryRowContext(ctx, existingRoleQuery, result.StaffUserID, homeID, string(demoStakeholderHomeRole)).Scan(&result.AlreadyMapped); err != nil {
			return fmt.Errorf("check demo stakeholder role: %w", err)
		}

		const roleQuery = `
INSERT INTO staff_home_roles (staff_user_id, home_id, role, active)
VALUES ($1, $2, $3, TRUE)
ON CONFLICT (staff_user_id, home_id, role) DO UPDATE
SET active = TRUE`
		if _, err := tx.q.ExecContext(ctx, roleQuery, result.StaffUserID, homeID, string(demoStakeholderHomeRole)); err != nil {
			return fmt.Errorf("upsert demo stakeholder home role: %w", err)
		}
		return nil
	})
	if err != nil {
		return DemoStakeholderMapping{}, err
	}
	return result, nil
}

func (r *Repo) ListDemoStakeholders(ctx context.Context, homeID int64) ([]DemoStakeholderInvite, error) {
	if homeID <= 0 {
		return nil, fmt.Errorf("invalid demo home id")
	}
	const query = `
SELECT su.display_name, su.email, shr.assigned_at::date::text
FROM staff_users AS su
JOIN staff_home_roles AS shr ON shr.staff_user_id = su.id
WHERE shr.home_id = $1
  AND shr.role = $2
  AND shr.active = TRUE
  AND su.status = 'active'
ORDER BY shr.assigned_at DESC, su.email`
	rows, err := r.db.QueryContext(ctx, query, homeID, string(demoStakeholderHomeRole))
	if err != nil {
		return nil, fmt.Errorf("list demo stakeholders: %w", err)
	}
	defer rows.Close()

	invites := make([]DemoStakeholderInvite, 0)
	for rows.Next() {
		var invite DemoStakeholderInvite
		if err := rows.Scan(&invite.Name, &invite.Email, &invite.InvitedDate); err != nil {
			return nil, fmt.Errorf("scan demo stakeholder: %w", err)
		}
		invite.Status = "Invited"
		invites = append(invites, invite)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate demo stakeholders: %w", err)
	}
	return invites, nil
}
