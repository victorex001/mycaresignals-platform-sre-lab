// internal/storage/children_storage.go
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"therapeutic-engine/internal/models"
)

type ChildrenStore struct {
	db *sql.DB
}

var ErrInvalidBaselineTransition = errors.New("invalid onboarding baseline status transition")

func NewChildrenStore(db *sql.DB) *ChildrenStore {
	return &ChildrenStore{db: db}
}

func (s *ChildrenStore) GetByID(ctx context.Context, id int64) (*models.Child, error) {
	const q = `
SELECT
  c.id, c.home_id, c.external_ref, c.initials, c.first_name, c.last_name,
  c.date_of_birth, c.legal_status, c.admission_date, c.gender,
  c.active, c.created_at, c.updated_at,
  COALESCE((
    SELECT b.status
    FROM child_onboarding_baselines b
    WHERE b.child_id = c.id
    ORDER BY b.created_at DESC, b.id DESC
    LIMIT 1
  ), '') AS baseline_status
FROM children c
WHERE c.id = $1;
`
	var c models.Child
	var baselineStatus sql.NullString

	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&c.ID,
		&c.HomeID,
		&c.ExternalRef,
		&c.Initials,
		&c.FirstName,
		&c.LastName,
		&c.DateOfBirth,
		&c.LegalStatus,
		&c.AdmissionDate,
		&c.Gender,
		&c.Active,
		&c.CreatedAt,
		&c.UpdatedAt,
		&baselineStatus,
	)
	if err != nil {
		return nil, err
	}
	if baselineStatus.Valid {
		c.BaselineStatus = baselineStatus.String
	}
	return &c, nil
}

func (s *ChildrenStore) CreateChild(ctx context.Context, c *models.Child) error {
	return NewRepo(s.db).WithTx(ctx, nil, func(tx *TxRepo) error {
		return tx.CreateChild(ctx, c)
	})
}

func (r *TxRepo) CreateChild(ctx context.Context, c *models.Child) error {
	if c == nil {
		return errors.New("child is required")
	}
	const q = `
INSERT INTO children (
    home_id, external_ref, initials, first_name, last_name,
    date_of_birth, legal_status, admission_date, gender, active
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id, created_at, updated_at`
	return r.q.QueryRowContext(ctx, q,
		c.HomeID, c.ExternalRef, c.Initials, c.FirstName, c.LastName,
		c.DateOfBirth, c.LegalStatus, c.AdmissionDate, c.Gender, c.Active,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func (s *ChildrenStore) ListChildrenByHome(
	ctx context.Context,
	homeID models.HomeID,
) ([]*models.Child, error) {
	const q = `
SELECT
  c.id, c.home_id, c.external_ref, c.initials, c.first_name, c.last_name,
  c.date_of_birth, c.legal_status, c.admission_date, c.gender, c.active,
  c.created_at, c.updated_at,
  COALESCE((
    SELECT b.status
    FROM child_onboarding_baselines b
    WHERE b.child_id = c.id
    ORDER BY b.created_at DESC, b.id DESC
    LIMIT 1
  ), '') AS baseline_status
FROM children c
WHERE c.home_id = $1 AND c.active = true
ORDER BY c.initials`
	rows, err := s.db.QueryContext(ctx, q, homeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]*models.Child, 0)
	for rows.Next() {
		var c models.Child
		var baselineStatus sql.NullString
		if err := rows.Scan(
			&c.ID, &c.HomeID, &c.ExternalRef, &c.Initials, &c.FirstName, &c.LastName,
			&c.DateOfBirth, &c.LegalStatus, &c.AdmissionDate, &c.Gender, &c.Active,
			&c.CreatedAt, &c.UpdatedAt,
			&baselineStatus,
		); err != nil {
			return nil, err
		}
		if baselineStatus.Valid {
			c.BaselineStatus = baselineStatus.String
		}
		res = append(res, &c)
	}
	return res, rows.Err()
}

// SaveOnboardingBaseline stores a full baseline + domains + priorities with a status.
func (s *ChildrenStore) SaveOnboardingBaseline(
	ctx context.Context,
	baseline *models.ChildOnboardingBaseline,
	domains []models.ChildOnboardingDomainState,
	priorities []models.ChildPriorityDomain,
	status string,
) error {
	return NewRepo(s.db).WithTx(ctx, nil, func(tx *TxRepo) error {
		return tx.SaveOnboardingBaseline(ctx, baseline, domains, priorities, status)
	})
}

func (r *TxRepo) SaveOnboardingBaseline(
	ctx context.Context,
	baseline *models.ChildOnboardingBaseline,
	domains []models.ChildOnboardingDomainState,
	priorities []models.ChildPriorityDomain,
	status string,
) error {
	if baseline == nil {
		return errors.New("onboarding baseline is required")
	}
	if status == "" {
		status = "submitted"
	}

	err := r.q.QueryRowContext(
		ctx,
		`INSERT INTO child_onboarding_baselines
         (child_id, summary_of_history, recent_placement_pattern, current_concerns,
          strengths, existing_plans, childs_voice,
          relapse_risks, relapse_early_signals, relapse_support_plan,
          review_frequency, next_review_date, lead_reviewer,
          other_professionals, early_warning_signals, notes_for_team,
          status)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
         RETURNING id, status, created_at, updated_at`,
		baseline.ChildID,
		baseline.SummaryOfHistory,
		baseline.RecentPlacement,
		baseline.CurrentConcerns,
		baseline.Strengths,
		baseline.ExistingPlans,
		baseline.ChildsVoice,
		baseline.RelapseRisks,
		baseline.RelapseEarlySignals,
		baseline.RelapseSupportPlan,
		baseline.ReviewFrequency,
		baseline.NextReviewDate,
		baseline.LeadReviewer,
		baseline.OtherProfessionals,
		baseline.EarlyWarningSignals,
		baseline.NotesForTeam,
		status,
	).Scan(&baseline.ID, &baseline.Status, &baseline.CreatedAt, &baseline.UpdatedAt)
	if err != nil {
		return err
	}

	for _, d := range domains {
		_, err = r.q.ExecContext(
			ctx,
			`INSERT INTO child_onboarding_domain_states
             (baseline_id, domain_key, active, reason)
             VALUES ($1,$2,$3,$4)`,
			baseline.ID,
			d.DomainKey,
			d.Active,
			d.Reason,
		)
		if err != nil {
			return err
		}
	}

	for _, p := range priorities {
		_, err = r.q.ExecContext(
			ctx,
			`INSERT INTO child_priority_domains
             (baseline_id, domain_key, focus_summary, hoped_for_change, sort_order)
             VALUES ($1,$2,$3,$4,$5)`,
			baseline.ID,
			p.DomainKey,
			p.FocusSummary,
			p.HopedForChange,
			p.SortOrder,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetLatestOnboardingBaseline returns the latest baseline + domains + priorities.
func (s *ChildrenStore) GetLatestOnboardingBaseline(
	ctx context.Context,
	childID models.ChildID,
) (*models.ChildOnboardingBaseline, []models.ChildOnboardingDomainState, []models.ChildPriorityDomain, error) {
	const qb = `
SELECT id, child_id,
       summary_of_history, recent_placement_pattern, current_concerns,
       strengths, existing_plans, childs_voice,
       relapse_risks, relapse_early_signals, relapse_support_plan,
       review_frequency, next_review_date, lead_reviewer,
       other_professionals, early_warning_signals, notes_for_team,
       status, approved_by, approved_at,
       created_at, updated_at
FROM child_onboarding_baselines
WHERE child_id = $1
  AND framework_model = 'legacy_eight_domain'
ORDER BY created_at DESC, id DESC
LIMIT 1`
	var b models.ChildOnboardingBaseline
	var summaryOfHistory, recentPlacement, currentConcerns sql.NullString
	var strengths, existingPlans, childsVoice sql.NullString
	var relapseRisks, relapseEarlySignals, relapseSupportPlan sql.NullString
	var reviewFrequency, leadReviewer, otherProfessionals sql.NullString
	var earlyWarningSignals, notesForTeam, approvedBy sql.NullString
	var nextReviewDate, approvedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, qb, childID).Scan(
		&b.ID,
		&b.ChildID,
		&summaryOfHistory,
		&recentPlacement,
		&currentConcerns,
		&strengths,
		&existingPlans,
		&childsVoice,
		&relapseRisks,
		&relapseEarlySignals,
		&relapseSupportPlan,
		&reviewFrequency,
		&nextReviewDate,
		&leadReviewer,
		&otherProfessionals,
		&earlyWarningSignals,
		&notesForTeam,
		&b.Status,
		&approvedBy,
		&approvedAt,
		&b.CreatedAt,
		&b.UpdatedAt,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	b.SummaryOfHistory = summaryOfHistory.String
	b.RecentPlacement = recentPlacement.String
	b.CurrentConcerns = currentConcerns.String
	b.Strengths = strengths.String
	b.ExistingPlans = existingPlans.String
	b.ChildsVoice = childsVoice.String
	b.RelapseRisks = relapseRisks.String
	b.RelapseEarlySignals = relapseEarlySignals.String
	b.RelapseSupportPlan = relapseSupportPlan.String
	b.ReviewFrequency = reviewFrequency.String
	b.LeadReviewer = leadReviewer.String
	b.OtherProfessionals = otherProfessionals.String
	b.EarlyWarningSignals = earlyWarningSignals.String
	b.NotesForTeam = notesForTeam.String
	if nextReviewDate.Valid {
		value := nextReviewDate.Time
		b.NextReviewDate = &value
	}
	if approvedBy.Valid {
		value := approvedBy.String
		b.ApprovedBy = &value
	}
	if approvedAt.Valid {
		value := approvedAt.Time
		b.ApprovedAt = &value
	}

	// domains
	const qd = `
SELECT id, baseline_id, domain_key, active, reason
FROM child_onboarding_domain_states
WHERE baseline_id = $1
ORDER BY id`
	rowsD, err := s.db.QueryContext(ctx, qd, b.ID)
	if err != nil {
		return nil, nil, nil, err
	}

	var domains []models.ChildOnboardingDomainState
	for rowsD.Next() {
		var d models.ChildOnboardingDomainState
		var reason sql.NullString
		if err := rowsD.Scan(
			&d.ID,
			&d.BaselineID,
			&d.DomainKey,
			&d.Active,
			&reason,
		); err != nil {
			_ = rowsD.Close()
			return nil, nil, nil, err
		}
		d.Reason = reason.String
		domains = append(domains, d)
	}
	if err := rowsD.Err(); err != nil {
		_ = rowsD.Close()
		return nil, nil, nil, err
	}
	if err := rowsD.Close(); err != nil {
		return nil, nil, nil, err
	}

	// priorities
	const qp = `
SELECT id, baseline_id, domain_key, focus_summary, hoped_for_change, sort_order
FROM child_priority_domains
WHERE baseline_id = $1
ORDER BY sort_order`
	rowsP, err := s.db.QueryContext(ctx, qp, b.ID)
	if err != nil {
		return nil, nil, nil, err
	}

	var priorities []models.ChildPriorityDomain
	for rowsP.Next() {
		var p models.ChildPriorityDomain
		var focusSummary, hopedForChange sql.NullString
		if err := rowsP.Scan(
			&p.ID,
			&p.BaselineID,
			&p.DomainKey,
			&focusSummary,
			&hopedForChange,
			&p.SortOrder,
		); err != nil {
			_ = rowsP.Close()
			return nil, nil, nil, err
		}
		p.FocusSummary = focusSummary.String
		p.HopedForChange = hopedForChange.String
		priorities = append(priorities, p)
	}
	if err := rowsP.Err(); err != nil {
		_ = rowsP.Close()
		return nil, nil, nil, err
	}
	if err := rowsP.Close(); err != nil {
		return nil, nil, nil, err
	}

	return &b, domains, priorities, nil
}

// ApproveLatestOnboardingBaseline marks the latest baseline as approved.
func (s *ChildrenStore) ApproveLatestOnboardingBaseline(
	ctx context.Context,
	childID models.ChildID,
	approvedBy string,
) error {
	return NewRepo(s.db).WithTx(ctx, nil, func(tx *TxRepo) error {
		_, err := tx.ApproveLatestOnboardingBaseline(ctx, childID, approvedBy)
		return err
	})
}

// ApproveLatestOnboardingBaseline deterministically locks the latest baseline
// and permits only the submitted -> approved transition.
func (r *TxRepo) ApproveLatestOnboardingBaseline(
	ctx context.Context,
	childID models.ChildID,
	approvedBy string,
) (*models.ChildOnboardingBaseline, error) {
	const lockQuery = `
SELECT id, status
FROM child_onboarding_baselines
WHERE child_id = $1
  AND framework_model = 'legacy_eight_domain'
ORDER BY created_at DESC, id DESC
LIMIT 1
FOR UPDATE`
	var baseline models.ChildOnboardingBaseline
	baseline.ChildID = childID
	if err := r.q.QueryRowContext(ctx, lockQuery, childID).Scan(&baseline.ID, &baseline.Status); err != nil {
		return nil, fmt.Errorf("lock latest onboarding baseline: %w", err)
	}
	if baseline.Status != "submitted" {
		return nil, ErrInvalidBaselineTransition
	}

	const updateQuery = `
UPDATE child_onboarding_baselines
SET status = 'approved',
    approved_by = $2,
    approved_at = now(),
    updated_at = now()
WHERE id = $1
  AND status = 'submitted'
RETURNING status, approved_by, approved_at, updated_at`
	var approvedByValue sql.NullString
	var approvedAt sql.NullTime
	if err := r.q.QueryRowContext(ctx, updateQuery, baseline.ID, approvedBy).Scan(
		&baseline.Status,
		&approvedByValue,
		&approvedAt,
		&baseline.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("approve onboarding baseline: %w", err)
	}
	if approvedByValue.Valid {
		value := approvedByValue.String
		baseline.ApprovedBy = &value
	}
	if approvedAt.Valid {
		value := approvedAt.Time
		baseline.ApprovedAt = &value
	}
	return &baseline, nil
}

// ---------- Signal responses ----------

func (s *ChildrenStore) CreateSignalResponse(
	ctx context.Context,
	resp *models.ChildSignalResponse,
) error {
	return NewRepo(s.db).WithTx(ctx, nil, func(tx *TxRepo) error {
		return tx.CreateSignalResponse(ctx, resp)
	})
}

func (r *TxRepo) CreateSignalResponse(
	ctx context.Context,
	resp *models.ChildSignalResponse,
) error {
	if resp == nil {
		return errors.New("signal response is required")
	}
	const q = `
INSERT INTO child_signal_responses (
    child_id, signal_kind,
    did_checkin, adjusted_support,
    informed_manager, informed_network,
    monitor_only,
    did_referral, referral_destinations,
    actions_summary, future_notes,
    external_log_ref
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
RETURNING id, created_at`
	return r.q.QueryRowContext(ctx, q,
		resp.ChildID,
		resp.SignalKind,
		resp.DidCheckIn,
		resp.AdjustedSupport,
		resp.InformedManager,
		resp.InformedNetwork,
		resp.MonitorOnly,
		resp.DidReferral,
		resp.ReferralDestinations,
		resp.ActionsSummary,
		resp.FutureNotes,
		resp.ExternalLogRef,
	).Scan(&resp.ID, &resp.CreatedAt)
}

func (s *ChildrenStore) ListSignalResponses(
	ctx context.Context,
	childID models.ChildID,
	limit int,
) ([]*models.ChildSignalResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	const q = `
SELECT id, child_id, signal_kind,
       did_checkin, adjusted_support,
       informed_manager, informed_network,
       monitor_only,
       did_referral, referral_destinations,
       COALESCE(actions_summary, '') AS actions_summary,
       COALESCE(future_notes, '') AS future_notes,
       external_log_ref,
       created_at
FROM child_signal_responses
WHERE child_id = $1
ORDER BY created_at DESC
LIMIT $2`
	rows, err := s.db.QueryContext(ctx, q, childID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*models.ChildSignalResponse
	for rows.Next() {
		var r models.ChildSignalResponse
		if err := rows.Scan(
			&r.ID,
			&r.ChildID,
			&r.SignalKind,
			&r.DidCheckIn,
			&r.AdjustedSupport,
			&r.InformedManager,
			&r.InformedNetwork,
			&r.MonitorOnly,
			&r.DidReferral,
			&r.ReferralDestinations,
			&r.ActionsSummary,
			&r.FutureNotes,
			&r.ExternalLogRef,
			&r.CreatedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, &r)
	}
	return res, rows.Err()
}
