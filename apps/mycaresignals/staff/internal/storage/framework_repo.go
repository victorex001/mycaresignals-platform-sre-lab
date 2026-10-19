package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"therapeutic-engine/internal/models"
)

const SixDomainFrameworkModel = "six_domain_v1"

// SaveSixDomainFramework resumes the current draft in place. Once submitted
// or approved, a framework is immutable; a later draft becomes a new version.
func (r *TxRepo) SaveSixDomainFramework(
	ctx context.Context,
	baseline *models.ChildOnboardingBaseline,
	domains []models.ChildOnboardingDomainState,
	priorities []models.ChildPriorityDomain,
	sources []models.ChildOnboardingFrameworkSource,
	status string,
	staffID int64,
) error {
	if baseline == nil {
		return errors.New("onboarding framework is required")
	}

	const latest = `
SELECT id, framework_version, status
FROM child_onboarding_baselines
WHERE child_id = $1 AND framework_model = 'six_domain_v1'
ORDER BY framework_version DESC, id DESC
LIMIT 1
FOR UPDATE`
	var latestID int64
	var latestVersion int
	var latestStatus string
	err := r.q.QueryRowContext(ctx, latest, baseline.ChildID).Scan(&latestID, &latestVersion, &latestStatus)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("lock latest onboarding framework: %w", err)
	}

	if err == nil && latestStatus == "submitted" {
		return ErrInvalidBaselineTransition
	}

	if err == nil && latestStatus == "draft" {
		baseline.ID = latestID
		baseline.FrameworkVersion = latestVersion
		const update = `
UPDATE child_onboarding_baselines SET
  summary_of_history = $2, recent_placement_pattern = $3,
  current_concerns = $4, strengths = $5, existing_plans = $6,
  childs_voice = $7, relapse_risks = $8, relapse_early_signals = $9,
  relapse_support_plan = $10, review_frequency = $11,
  next_review_date = $12, lead_reviewer = $13, other_professionals = $14,
  early_warning_signals = $15, notes_for_team = $16,
  onboarding_pathway = $17, immediate_information = $18,
  immediate_information_status = $19, background_information_status = $20,
  known_safety_information = $21, safety_information_status = $22,
  status = $23, updated_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING framework_model, framework_version, status, created_at, updated_at`
		if scanErr := r.q.QueryRowContext(ctx, update,
			baseline.ID, baseline.SummaryOfHistory, baseline.RecentPlacement,
			baseline.CurrentConcerns, baseline.Strengths, baseline.ExistingPlans,
			baseline.ChildsVoice, baseline.RelapseRisks, baseline.RelapseEarlySignals,
			baseline.RelapseSupportPlan, baseline.ReviewFrequency, baseline.NextReviewDate,
			baseline.LeadReviewer, baseline.OtherProfessionals, baseline.EarlyWarningSignals,
			baseline.NotesForTeam, baseline.OnboardingPathway, baseline.ImmediateInformation,
			baseline.ImmediateInformationStatus, baseline.BackgroundInformationStatus,
			baseline.KnownSafetyInformation, baseline.SafetyInformationStatus, status,
		).Scan(&baseline.FrameworkModel, &baseline.FrameworkVersion, &baseline.Status, &baseline.CreatedAt, &baseline.UpdatedAt); scanErr != nil {
			return fmt.Errorf("update onboarding framework draft: %w", scanErr)
		}
		for _, table := range []string{"child_onboarding_domain_states", "child_priority_domains", "child_onboarding_framework_sources"} {
			if _, execErr := r.q.ExecContext(ctx, "DELETE FROM "+table+" WHERE baseline_id = $1", baseline.ID); execErr != nil {
				return fmt.Errorf("replace onboarding framework children: %w", execErr)
			}
		}
	} else {
		baseline.FrameworkVersion = latestVersion + 1
		baseline.FrameworkModel = SixDomainFrameworkModel
		createdBy := staffID
		baseline.CreatedByStaffID = &createdBy
		const insert = `
INSERT INTO child_onboarding_baselines (
  child_id, summary_of_history, recent_placement_pattern, current_concerns,
  strengths, existing_plans, childs_voice, relapse_risks,
  relapse_early_signals, relapse_support_plan, review_frequency,
  next_review_date, lead_reviewer, other_professionals,
  early_warning_signals, notes_for_team, status, framework_model,
  framework_version, onboarding_pathway, immediate_information,
  immediate_information_status, background_information_status,
  known_safety_information, safety_information_status, created_by_staff_id
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,
  'six_domain_v1',$18,$19,$20,$21,$22,$23,$24,$25
)
RETURNING id, framework_model, framework_version, status, created_at, updated_at`
		if scanErr := r.q.QueryRowContext(ctx, insert,
			baseline.ChildID, baseline.SummaryOfHistory, baseline.RecentPlacement,
			baseline.CurrentConcerns, baseline.Strengths, baseline.ExistingPlans,
			baseline.ChildsVoice, baseline.RelapseRisks, baseline.RelapseEarlySignals,
			baseline.RelapseSupportPlan, baseline.ReviewFrequency, baseline.NextReviewDate,
			baseline.LeadReviewer, baseline.OtherProfessionals, baseline.EarlyWarningSignals,
			baseline.NotesForTeam, status, baseline.FrameworkVersion,
			baseline.OnboardingPathway, baseline.ImmediateInformation,
			baseline.ImmediateInformationStatus, baseline.BackgroundInformationStatus,
			baseline.KnownSafetyInformation, baseline.SafetyInformationStatus, staffID,
		).Scan(&baseline.ID, &baseline.FrameworkModel, &baseline.FrameworkVersion, &baseline.Status, &baseline.CreatedAt, &baseline.UpdatedAt); scanErr != nil {
			return fmt.Errorf("insert onboarding framework: %w", scanErr)
		}
	}

	for _, domain := range domains {
		_, err = r.q.ExecContext(ctx, `
INSERT INTO child_onboarding_domain_states (
  baseline_id, domain_key, active, reason, current_understanding,
  strengths_protective_factors, support_needs, what_helps, current_support,
  young_person_view, information_status, information_source, source_details,
  source_date, priority_reason
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			baseline.ID, domain.DomainKey, domain.Active, domain.Reason,
			domain.CurrentUnderstanding, domain.StrengthsProtectiveFactors,
			domain.SupportNeeds, domain.WhatHelps, domain.CurrentSupport,
			domain.YoungPersonView, domain.InformationStatus,
			domain.InformationSource, domain.SourceDetails, domain.SourceDate,
			domain.PriorityReason,
		)
		if err != nil {
			return fmt.Errorf("insert onboarding framework domain: %w", err)
		}
	}

	for _, priority := range priorities {
		_, err = r.q.ExecContext(ctx, `
INSERT INTO child_priority_domains
  (baseline_id, domain_key, focus_summary, hoped_for_change, sort_order)
VALUES ($1,$2,$3,$4,$5)`, baseline.ID, priority.DomainKey,
			priority.FocusSummary, priority.HopedForChange, priority.SortOrder)
		if err != nil {
			return fmt.Errorf("insert onboarding framework priority: %w", err)
		}
	}

	for _, source := range sources {
		_, err = r.q.ExecContext(ctx, `
INSERT INTO child_onboarding_framework_sources
  (baseline_id, source_type, source_name, source_date, notes)
VALUES ($1,$2,$3,$4,$5)`, baseline.ID, source.SourceType,
			source.SourceName, source.SourceDate, source.Notes)
		if err != nil {
			return fmt.Errorf("insert onboarding framework source: %w", err)
		}
	}
	return nil
}

func (s *ChildrenStore) GetLatestSixDomainFramework(
	ctx context.Context,
	childID models.ChildID,
) (*models.ChildOnboardingBaseline, []models.ChildOnboardingDomainState, []models.ChildPriorityDomain, []models.ChildOnboardingFrameworkSource, error) {
	const query = `
SELECT id, child_id, summary_of_history, recent_placement_pattern,
  current_concerns, strengths, existing_plans, childs_voice,
  relapse_risks, relapse_early_signals, relapse_support_plan,
  review_frequency, next_review_date, lead_reviewer, other_professionals,
  early_warning_signals, notes_for_team, status, approved_by, approved_at,
  framework_model, framework_version, onboarding_pathway,
  immediate_information, immediate_information_status,
  background_information_status, known_safety_information,
  safety_information_status, created_by_staff_id, created_at, updated_at
FROM child_onboarding_baselines
WHERE child_id = $1 AND framework_model = 'six_domain_v1'
ORDER BY framework_version DESC, id DESC
LIMIT 1`
	baseline, err := scanFrameworkBaseline(s.db.QueryRowContext(ctx, query, childID))
	if err != nil {
		return nil, nil, nil, nil, err
	}
	domains, err := s.getFrameworkDomains(ctx, baseline.ID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	priorities, err := s.getFrameworkPriorities(ctx, baseline.ID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	sources, err := s.getFrameworkSources(ctx, baseline.ID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return baseline, domains, priorities, sources, nil
}

type rowScanner interface{ Scan(...any) error }

func scanFrameworkBaseline(row rowScanner) (*models.ChildOnboardingBaseline, error) {
	var b models.ChildOnboardingBaseline
	var stringsNullable [17]sql.NullString
	var nextReview, approvedAt sql.NullTime
	var createdBy sql.NullInt64
	err := row.Scan(
		&b.ID, &b.ChildID,
		&stringsNullable[0], &stringsNullable[1], &stringsNullable[2],
		&stringsNullable[3], &stringsNullable[4], &stringsNullable[5],
		&stringsNullable[6], &stringsNullable[7], &stringsNullable[8],
		&stringsNullable[9], &nextReview, &stringsNullable[10],
		&stringsNullable[11], &stringsNullable[12], &stringsNullable[13],
		&b.Status, &stringsNullable[14], &approvedAt,
		&b.FrameworkModel, &b.FrameworkVersion, &b.OnboardingPathway,
		&b.ImmediateInformation, &b.ImmediateInformationStatus,
		&b.BackgroundInformationStatus, &b.KnownSafetyInformation,
		&b.SafetyInformationStatus, &createdBy, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	b.SummaryOfHistory = stringsNullable[0].String
	b.RecentPlacement = stringsNullable[1].String
	b.CurrentConcerns = stringsNullable[2].String
	b.Strengths = stringsNullable[3].String
	b.ExistingPlans = stringsNullable[4].String
	b.ChildsVoice = stringsNullable[5].String
	b.RelapseRisks = stringsNullable[6].String
	b.RelapseEarlySignals = stringsNullable[7].String
	b.RelapseSupportPlan = stringsNullable[8].String
	b.ReviewFrequency = stringsNullable[9].String
	b.LeadReviewer = stringsNullable[10].String
	b.OtherProfessionals = stringsNullable[11].String
	b.EarlyWarningSignals = stringsNullable[12].String
	b.NotesForTeam = stringsNullable[13].String
	if nextReview.Valid {
		value := nextReview.Time
		b.NextReviewDate = &value
	}
	if stringsNullable[14].Valid {
		value := stringsNullable[14].String
		b.ApprovedBy = &value
	}
	if approvedAt.Valid {
		value := approvedAt.Time
		b.ApprovedAt = &value
	}
	if createdBy.Valid {
		value := createdBy.Int64
		b.CreatedByStaffID = &value
	}
	return &b, nil
}

func (s *ChildrenStore) getFrameworkDomains(ctx context.Context, baselineID int64) ([]models.ChildOnboardingDomainState, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, baseline_id, domain_key, active, COALESCE(reason, ''),
  COALESCE(current_understanding, ''), COALESCE(strengths_protective_factors, ''),
  COALESCE(support_needs, ''), COALESCE(what_helps, ''), COALESCE(current_support, ''),
  COALESCE(young_person_view, ''), COALESCE(information_status, ''),
  COALESCE(information_source, ''), COALESCE(source_details, ''), source_date,
  COALESCE(priority_reason, '')
FROM child_onboarding_domain_states WHERE baseline_id = $1 ORDER BY id`, baselineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.ChildOnboardingDomainState
	for rows.Next() {
		var item models.ChildOnboardingDomainState
		var sourceDate sql.NullTime
		if err := rows.Scan(&item.ID, &item.BaselineID, &item.DomainKey, &item.Active,
			&item.Reason, &item.CurrentUnderstanding, &item.StrengthsProtectiveFactors,
			&item.SupportNeeds, &item.WhatHelps, &item.CurrentSupport,
			&item.YoungPersonView, &item.InformationStatus, &item.InformationSource,
			&item.SourceDetails, &sourceDate, &item.PriorityReason); err != nil {
			return nil, err
		}
		if sourceDate.Valid {
			value := sourceDate.Time
			item.SourceDate = &value
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *ChildrenStore) getFrameworkPriorities(ctx context.Context, baselineID int64) ([]models.ChildPriorityDomain, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, baseline_id, domain_key, COALESCE(focus_summary, ''),
  COALESCE(hoped_for_change, ''), sort_order
FROM child_priority_domains WHERE baseline_id = $1 ORDER BY sort_order`, baselineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.ChildPriorityDomain
	for rows.Next() {
		var item models.ChildPriorityDomain
		if err := rows.Scan(&item.ID, &item.BaselineID, &item.DomainKey,
			&item.FocusSummary, &item.HopedForChange, &item.SortOrder); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *ChildrenStore) getFrameworkSources(ctx context.Context, baselineID int64) ([]models.ChildOnboardingFrameworkSource, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, baseline_id, source_type, COALESCE(source_name, ''), source_date,
  COALESCE(notes, ''), created_at
FROM child_onboarding_framework_sources WHERE baseline_id = $1 ORDER BY id`, baselineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.ChildOnboardingFrameworkSource
	for rows.Next() {
		var item models.ChildOnboardingFrameworkSource
		var sourceDate sql.NullTime
		if err := rows.Scan(&item.ID, &item.BaselineID, &item.SourceType,
			&item.SourceName, &sourceDate, &item.Notes, &item.CreatedAt); err != nil {
			return nil, err
		}
		if sourceDate.Valid {
			value := sourceDate.Time
			item.SourceDate = &value
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *ChildrenStore) ListSixDomainFrameworkHistory(ctx context.Context, childID models.ChildID) ([]models.ChildOnboardingFrameworkSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, framework_version, status, COALESCE(onboarding_pathway, ''),
  created_at, approved_by, approved_at
FROM child_onboarding_baselines
WHERE child_id = $1 AND framework_model = 'six_domain_v1'
ORDER BY framework_version DESC, id DESC`, childID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.ChildOnboardingFrameworkSummary
	for rows.Next() {
		var item models.ChildOnboardingFrameworkSummary
		var approvedBy sql.NullString
		var approvedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Version, &item.Status, &item.OnboardingPathway,
			&item.CreatedAt, &approvedBy, &approvedAt); err != nil {
			return nil, err
		}
		if approvedBy.Valid {
			value := approvedBy.String
			item.ApprovedBy = &value
		}
		if approvedAt.Valid {
			value := approvedAt.Time
			item.ApprovedAt = &value
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *TxRepo) ApproveLatestSixDomainFramework(ctx context.Context, childID models.ChildID, approvedBy string) (*models.ChildOnboardingBaseline, error) {
	const lockQuery = `
SELECT id, framework_version, status
FROM child_onboarding_baselines
WHERE child_id = $1 AND framework_model = 'six_domain_v1'
ORDER BY framework_version DESC, id DESC
LIMIT 1 FOR UPDATE`
	var baseline models.ChildOnboardingBaseline
	baseline.ChildID = childID
	if err := r.q.QueryRowContext(ctx, lockQuery, childID).Scan(&baseline.ID, &baseline.FrameworkVersion, &baseline.Status); err != nil {
		return nil, fmt.Errorf("lock latest onboarding framework: %w", err)
	}
	if baseline.Status != "submitted" {
		return nil, ErrInvalidBaselineTransition
	}
	var approvedByValue sql.NullString
	var approvedAt sql.NullTime
	err := r.q.QueryRowContext(ctx, `
UPDATE child_onboarding_baselines SET status = 'approved', approved_by = $2,
  approved_at = now(), updated_at = now()
WHERE id = $1 AND status = 'submitted'
RETURNING status, approved_by, approved_at, updated_at`, baseline.ID, approvedBy).Scan(
		&baseline.Status, &approvedByValue, &approvedAt, &baseline.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("approve onboarding framework: %w", err)
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
