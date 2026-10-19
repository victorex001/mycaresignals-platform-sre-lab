package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"therapeutic-engine/internal/models"
)

func (r *TxRepo) CreateChildCarePlan(ctx context.Context, plan *models.ChildCarePlan) error {
	const insertPlan = `
INSERT INTO child_care_plans (
  child_id, status, effective_from, review_date, created_by_staff_id
) VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at, updated_at`
	if err := r.q.QueryRowContext(
		ctx, insertPlan, plan.ChildID, plan.Status, plan.EffectiveFrom,
		nullableTime(plan.ReviewDate), plan.CreatedByStaffID,
	).Scan(&plan.ID, &plan.CreatedAt, &plan.UpdatedAt); err != nil {
		return fmt.Errorf("create child care plan: %w", err)
	}

	plan.Sections = make([]models.ChildCarePlanSection, 0, len(models.CanonicalCarePlanSectionKeys))
	for _, key := range models.CanonicalCarePlanSectionKeys {
		section := models.ChildCarePlanSection{
			CarePlanID: plan.ID, SectionKey: key,
			InformationStatus: models.CarePlanInformationNotKnownYet,
		}
		if err := r.UpsertChildCarePlanSection(ctx, &section); err != nil {
			return err
		}
		plan.Sections = append(plan.Sections, section)
	}
	return nil
}

func (r *Repo) GetCurrentChildCarePlan(ctx context.Context, childID int64) (*models.ChildCarePlan, error) {
	const query = `
SELECT id, child_id, status, effective_from, review_date, predecessor_care_plan_id, created_by_staff_id,
       activated_by_staff_id, activated_at, archived_by_staff_id, archived_at,
       created_at, updated_at
FROM child_care_plans
WHERE child_id = $1
ORDER BY CASE status WHEN 'active' THEN 0 WHEN 'draft' THEN 1 ELSE 2 END,
         updated_at DESC, id DESC
LIMIT 1`
	plan, err := scanCarePlan(r.db.QueryRowContext(ctx, query, childID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get current child care plan: %w", err)
	}
	sections, err := r.ListChildCarePlanSections(ctx, plan.ID)
	if err != nil {
		return nil, err
	}
	plan.Sections = sections
	return plan, nil
}

func (r *Repo) GetChildCarePlan(ctx context.Context, childID, planID int64) (*models.ChildCarePlan, error) {
	const query = `
SELECT id, child_id, status, effective_from, review_date, predecessor_care_plan_id, created_by_staff_id,
       activated_by_staff_id, activated_at, archived_by_staff_id, archived_at,
       created_at, updated_at
FROM child_care_plans
WHERE id = $1 AND child_id = $2`
	plan, err := scanCarePlan(r.db.QueryRowContext(ctx, query, planID, childID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get child care plan: %w", err)
	}
	plan.Sections, err = r.ListChildCarePlanSections(ctx, plan.ID)
	return plan, err
}

func (r *Repo) ListChildCarePlans(ctx context.Context, childID int64) ([]models.ChildCarePlan, error) {
	const query = `
SELECT id, child_id, status, effective_from, review_date, predecessor_care_plan_id, created_by_staff_id,
       activated_by_staff_id, activated_at, archived_by_staff_id, archived_at,
       created_at, updated_at
FROM child_care_plans
WHERE child_id = $1
ORDER BY created_at ASC, id ASC`
	rows, err := r.db.QueryContext(ctx, query, childID)
	if err != nil {
		return nil, fmt.Errorf("list child care plans: %w", err)
	}
	defer rows.Close()
	plans := make([]models.ChildCarePlan, 0)
	for rows.Next() {
		plan, err := scanCarePlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, *plan)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range plans {
		plans[i].Sections, err = r.ListChildCarePlanSections(ctx, plans[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return plans, nil
}

func (r *Repo) ListChildCarePlanSections(ctx context.Context, carePlanID int64) ([]models.ChildCarePlanSection, error) {
	const query = `
SELECT id, care_plan_id, section_key, information_status,
       current_understanding, strengths, staff_support,
       young_person_involvement_note, created_at, updated_at
FROM child_care_plan_sections
WHERE care_plan_id = $1
ORDER BY CASE section_key
  WHEN 'about_me' THEN 1 WHEN 'safety_stability' THEN 2
  WHEN 'emotional_wellbeing' THEN 3 WHEN 'health' THEN 4
  WHEN 'education_future' THEN 5 WHEN 'relationships_belonging' THEN 6
  WHEN 'independence_daily_living' THEN 7 WHEN 'identity_culture' THEN 8
  ELSE 9 END`
	rows, err := r.db.QueryContext(ctx, query, carePlanID)
	if err != nil {
		return nil, fmt.Errorf("list child care plan sections: %w", err)
	}
	defer rows.Close()
	sections := make([]models.ChildCarePlanSection, 0, len(models.CanonicalCarePlanSectionKeys))
	for rows.Next() {
		section, err := scanCarePlanSection(rows)
		if err != nil {
			return nil, err
		}
		sections = append(sections, *section)
	}
	return sections, rows.Err()
}

func (r *TxRepo) UpsertChildCarePlanSection(ctx context.Context, section *models.ChildCarePlanSection) error {
	const query = `
INSERT INTO child_care_plan_sections (
  care_plan_id, section_key, information_status, current_understanding,
  strengths, staff_support, young_person_involvement_note
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (care_plan_id, section_key) DO UPDATE SET
  information_status = EXCLUDED.information_status,
  current_understanding = EXCLUDED.current_understanding,
  strengths = EXCLUDED.strengths,
  staff_support = EXCLUDED.staff_support,
  young_person_involvement_note = EXCLUDED.young_person_involvement_note,
  updated_at = now()
RETURNING id, created_at, updated_at`
	return r.q.QueryRowContext(
		ctx, query, section.CarePlanID, section.SectionKey, section.InformationStatus,
		nullableStringPtr(section.CurrentUnderstanding), nullableStringPtr(section.Strengths),
		nullableStringPtr(section.StaffSupport), nullableStringPtr(section.YoungPersonInvolvementNote),
	).Scan(&section.ID, &section.CreatedAt, &section.UpdatedAt)
}

func (r *TxRepo) LockChildCarePlan(ctx context.Context, planID, childID int64) (*models.ChildCarePlan, error) {
	const query = `
SELECT id, child_id, status, effective_from, review_date, predecessor_care_plan_id, created_by_staff_id,
       activated_by_staff_id, activated_at, archived_by_staff_id, archived_at,
       created_at, updated_at
FROM child_care_plans
WHERE id = $1 AND child_id = $2
FOR UPDATE`
	plan, err := scanCarePlan(r.q.QueryRowContext(ctx, query, planID, childID))
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func (r *TxRepo) ListChildCarePlanSectionsForUpdate(ctx context.Context, carePlanID int64) ([]models.ChildCarePlanSection, error) {
	const query = `
SELECT id, care_plan_id, section_key, information_status,
       current_understanding, strengths, staff_support,
       young_person_involvement_note, created_at, updated_at
FROM child_care_plan_sections
WHERE care_plan_id = $1
ORDER BY section_key
FOR UPDATE`
	rows, err := r.q.QueryContext(ctx, query, carePlanID)
	if err != nil {
		return nil, fmt.Errorf("lock child care plan sections: %w", err)
	}
	defer rows.Close()
	sections := make([]models.ChildCarePlanSection, 0, len(models.CanonicalCarePlanSectionKeys))
	for rows.Next() {
		section, err := scanCarePlanSection(rows)
		if err != nil {
			return nil, err
		}
		sections = append(sections, *section)
	}
	return sections, rows.Err()
}

func (r *TxRepo) CreateSuccessorChildCarePlan(ctx context.Context, predecessor *models.ChildCarePlan, sections []models.ChildCarePlanSection, staffID int64, reviewDate *time.Time) (*models.ChildCarePlan, error) {
	const query = `
INSERT INTO child_care_plans (
  child_id, status, effective_from, review_date, predecessor_care_plan_id, created_by_staff_id
) VALUES ($1, 'draft', CURRENT_DATE, $2, $3, $4)
RETURNING id, effective_from, created_at, updated_at`
	successor := &models.ChildCarePlan{
		ChildID: predecessor.ChildID, Status: models.CarePlanStatusDraft,
		ReviewDate: reviewDate, PredecessorCarePlanID: &predecessor.ID, CreatedByStaffID: staffID,
	}
	if err := r.q.QueryRowContext(
		ctx, query, successor.ChildID, nullableTime(reviewDate), predecessor.ID, staffID,
	).Scan(&successor.ID, &successor.EffectiveFrom, &successor.CreatedAt, &successor.UpdatedAt); err != nil {
		return nil, fmt.Errorf("create successor care plan: %w", err)
	}
	successor.Sections = make([]models.ChildCarePlanSection, 0, len(sections))
	for _, source := range sections {
		copy := models.ChildCarePlanSection{
			CarePlanID: successor.ID, SectionKey: source.SectionKey,
			InformationStatus:    source.InformationStatus,
			CurrentUnderstanding: source.CurrentUnderstanding, Strengths: source.Strengths,
			StaffSupport: source.StaffSupport, YoungPersonInvolvementNote: source.YoungPersonInvolvementNote,
		}
		if err := r.UpsertChildCarePlanSection(ctx, &copy); err != nil {
			return nil, err
		}
		successor.Sections = append(successor.Sections, copy)
	}
	return successor, nil
}

func (r *TxRepo) GetCarePlanReviewEvidence(ctx context.Context, plan *models.ChildCarePlan) (models.CarePlanReviewEvidence, error) {
	return getCarePlanReviewEvidence(ctx, r.q, plan)
}

func (r *Repo) GetCarePlanReviewEvidence(ctx context.Context, plan *models.ChildCarePlan) (models.CarePlanReviewEvidence, error) {
	return getCarePlanReviewEvidence(ctx, r.db, plan)
}

func getCarePlanReviewEvidence(ctx context.Context, q DBTX, plan *models.ChildCarePlan) (models.CarePlanReviewEvidence, error) {
	var evidence models.CarePlanReviewEvidence
	const sinceQuery = `
SELECT COALESCE(
  (SELECT max(reviewed_at) FROM child_care_plan_reviews WHERE care_plan_id = $1 AND child_id = $2),
  activated_at,
  effective_from::timestamptz
)
FROM child_care_plans
WHERE id = $1 AND child_id = $2`
	if err := q.QueryRowContext(ctx, sinceQuery, plan.ID, plan.ChildID).Scan(&evidence.EvidenceSince); err != nil {
		return evidence, err
	}
	const countsQuery = `
SELECT
  (SELECT count(*) FROM child_goals WHERE child_id = $1 AND status = 'active'),
  (SELECT count(*) FROM child_targets WHERE child_id = $1 AND status = 'active'),
  (SELECT count(*) FROM child_targets WHERE child_id = $1 AND status = 'achieved'),
  (SELECT count(*) FROM child_targets WHERE child_id = $1 AND status = 'paused'),
  (SELECT count(*) FROM child_targets WHERE child_id = $1 AND updated_at >= $2),
  (SELECT count(*) FROM child_targets WHERE child_id = $1 AND status = 'active' AND review_date <= CURRENT_DATE),
  (SELECT count(*) FROM child_risks WHERE child_id = $1 AND status = 'active'),
  (SELECT count(*) FROM child_risk_reviews WHERE child_id = $1 AND reviewed_at >= $2),
  (SELECT count(*) FROM keyworking_sessions WHERE child_id = $1 AND created_at >= $2),
  (SELECT count(*) FROM child_care_plan_sections WHERE care_plan_id = $3 AND information_status = 'not_known_yet'),
  (SELECT count(*) FROM child_care_plan_sections WHERE care_plan_id = $3 AND information_status = 'awaiting_confirmation')`
	if err := q.QueryRowContext(ctx, countsQuery, plan.ChildID, evidence.EvidenceSince, plan.ID).Scan(
		&evidence.ActiveGoalCount, &evidence.ActiveTargetCount, &evidence.AchievedTargetCount,
		&evidence.PausedTargetCount, &evidence.TargetsChangedCount, &evidence.TargetsDueCount,
		&evidence.ActiveRiskCount, &evidence.RiskReviewsCompletedCount,
		&evidence.KeyworkingSessionCount, &evidence.SectionsNotKnownCount,
		&evidence.SectionsAwaitingConfirmationCount,
	); err != nil {
		return evidence, err
	}
	return evidence, nil
}

func (r *TxRepo) CreateChildCarePlanReview(ctx context.Context, review *models.ChildCarePlanReview, sections []models.ChildCarePlanSection) error {
	const query = `
INSERT INTO child_care_plan_reviews (
  care_plan_id, child_id, reviewer_staff_id, review_note,
  young_person_involvement_note, outcome, next_review_date, successor_care_plan_id,
  snapshot_status, snapshot_effective_from, snapshot_review_date, evidence_since,
  active_goal_count, active_target_count, achieved_target_count, paused_target_count,
  targets_changed_count, targets_due_count, active_risk_count,
  risk_reviews_completed_count, keyworking_session_count,
  sections_not_known_count, sections_awaiting_confirmation_count
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23
)
RETURNING id, reviewed_at, created_at`
	if err := r.q.QueryRowContext(ctx, query,
		review.CarePlanID, review.ChildID, review.ReviewerStaffID, review.ReviewNote,
		nullableStringPtr(review.YoungPersonInvolvementNote), review.Outcome,
		nullableTime(review.NextReviewDate), nullableInt64Ptr(review.SuccessorCarePlanID),
		review.SnapshotStatus, review.SnapshotEffectiveFrom, nullableTime(review.SnapshotReviewDate),
		review.Evidence.EvidenceSince, review.Evidence.ActiveGoalCount,
		review.Evidence.ActiveTargetCount, review.Evidence.AchievedTargetCount,
		review.Evidence.PausedTargetCount, review.Evidence.TargetsChangedCount,
		review.Evidence.TargetsDueCount, review.Evidence.ActiveRiskCount,
		review.Evidence.RiskReviewsCompletedCount, review.Evidence.KeyworkingSessionCount,
		review.Evidence.SectionsNotKnownCount, review.Evidence.SectionsAwaitingConfirmationCount,
	).Scan(&review.ID, &review.ReviewedAt, &review.CreatedAt); err != nil {
		return fmt.Errorf("create child care plan review: %w", err)
	}
	review.Sections = make([]models.ChildCarePlanReviewSection, 0, len(sections))
	const snapshotQuery = `
INSERT INTO child_care_plan_review_sections (
  review_id, section_key, information_status, current_understanding,
  strengths, staff_support, young_person_involvement_note
) VALUES ($1,$2,$3,$4,$5,$6,$7)
RETURNING id, created_at`
	for _, source := range sections {
		snapshot := models.ChildCarePlanReviewSection{
			ReviewID: review.ID, SectionKey: source.SectionKey,
			InformationStatus:    source.InformationStatus,
			CurrentUnderstanding: source.CurrentUnderstanding, Strengths: source.Strengths,
			StaffSupport: source.StaffSupport, YoungPersonInvolvementNote: source.YoungPersonInvolvementNote,
		}
		if err := r.q.QueryRowContext(ctx, snapshotQuery,
			snapshot.ReviewID, snapshot.SectionKey, snapshot.InformationStatus,
			nullableStringPtr(snapshot.CurrentUnderstanding), nullableStringPtr(snapshot.Strengths),
			nullableStringPtr(snapshot.StaffSupport), nullableStringPtr(snapshot.YoungPersonInvolvementNote),
		).Scan(&snapshot.ID, &snapshot.CreatedAt); err != nil {
			return fmt.Errorf("create child care plan review snapshot: %w", err)
		}
		review.Sections = append(review.Sections, snapshot)
	}
	return nil
}

func (r *TxRepo) UpdateChildCarePlanReviewDate(ctx context.Context, plan *models.ChildCarePlan, reviewDate *time.Time) error {
	const query = `
UPDATE child_care_plans
SET review_date = $3, updated_at = now()
WHERE id = $1 AND child_id = $2 AND status = 'active'
RETURNING updated_at`
	if err := r.q.QueryRowContext(ctx, query, plan.ID, plan.ChildID, nullableTime(reviewDate)).Scan(&plan.UpdatedAt); err != nil {
		return err
	}
	plan.ReviewDate = reviewDate
	return nil
}

func (r *Repo) ListChildCarePlanReviews(ctx context.Context, childID, planID int64) ([]models.ChildCarePlanReview, error) {
	const query = `
SELECT r.id, r.care_plan_id, r.child_id, r.reviewed_at,
       r.reviewer_staff_id, s.display_name, r.review_note,
       r.young_person_involvement_note, r.outcome, r.next_review_date,
       r.successor_care_plan_id, r.snapshot_status, r.snapshot_effective_from,
       r.snapshot_review_date, r.evidence_since,
       r.active_goal_count, r.active_target_count, r.achieved_target_count,
       r.paused_target_count, r.targets_changed_count, r.targets_due_count,
       r.active_risk_count, r.risk_reviews_completed_count,
       r.keyworking_session_count, r.sections_not_known_count,
       r.sections_awaiting_confirmation_count, r.created_at
FROM child_care_plan_reviews r
JOIN staff_users s ON s.id = r.reviewer_staff_id
WHERE r.child_id = $1 AND r.care_plan_id = $2
ORDER BY r.reviewed_at DESC, r.id DESC`
	rows, err := r.db.QueryContext(ctx, query, childID, planID)
	if err != nil {
		return nil, fmt.Errorf("list child care plan reviews: %w", err)
	}
	defer rows.Close()
	reviews := make([]models.ChildCarePlanReview, 0)
	for rows.Next() {
		review := models.ChildCarePlanReview{}
		var involvement sql.NullString
		var nextReview, snapshotReview sql.NullTime
		var successor sql.NullInt64
		if err := rows.Scan(
			&review.ID, &review.CarePlanID, &review.ChildID, &review.ReviewedAt,
			&review.ReviewerStaffID, &review.ReviewerDisplayName, &review.ReviewNote,
			&involvement, &review.Outcome, &nextReview, &successor,
			&review.SnapshotStatus, &review.SnapshotEffectiveFrom, &snapshotReview,
			&review.Evidence.EvidenceSince, &review.Evidence.ActiveGoalCount,
			&review.Evidence.ActiveTargetCount, &review.Evidence.AchievedTargetCount,
			&review.Evidence.PausedTargetCount, &review.Evidence.TargetsChangedCount,
			&review.Evidence.TargetsDueCount, &review.Evidence.ActiveRiskCount,
			&review.Evidence.RiskReviewsCompletedCount, &review.Evidence.KeyworkingSessionCount,
			&review.Evidence.SectionsNotKnownCount,
			&review.Evidence.SectionsAwaitingConfirmationCount, &review.CreatedAt,
		); err != nil {
			return nil, err
		}
		review.YoungPersonInvolvementNote = stringFromNull(involvement)
		review.NextReviewDate = timeFromNull(nextReview)
		review.SuccessorCarePlanID = int64FromNull(successor)
		review.SnapshotReviewDate = timeFromNull(snapshotReview)
		review.Sections, err = r.listChildCarePlanReviewSections(ctx, review.ID)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (r *Repo) listChildCarePlanReviewSections(ctx context.Context, reviewID int64) ([]models.ChildCarePlanReviewSection, error) {
	const query = `
SELECT id, review_id, section_key, information_status, current_understanding,
       strengths, staff_support, young_person_involvement_note, created_at
FROM child_care_plan_review_sections
WHERE review_id = $1
ORDER BY section_key`
	rows, err := r.db.QueryContext(ctx, query, reviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sections := make([]models.ChildCarePlanReviewSection, 0)
	for rows.Next() {
		section := models.ChildCarePlanReviewSection{}
		var understanding, strengths, support, involvement sql.NullString
		if err := rows.Scan(
			&section.ID, &section.ReviewID, &section.SectionKey, &section.InformationStatus,
			&understanding, &strengths, &support, &involvement, &section.CreatedAt,
		); err != nil {
			return nil, err
		}
		section.CurrentUnderstanding = stringFromNull(understanding)
		section.Strengths = stringFromNull(strengths)
		section.StaffSupport = stringFromNull(support)
		section.YoungPersonInvolvementNote = stringFromNull(involvement)
		sections = append(sections, section)
	}
	return sections, rows.Err()
}

func (r *TxRepo) HasOtherActiveCarePlan(ctx context.Context, childID, planID int64) (bool, error) {
	const query = `
SELECT id
FROM child_care_plans
WHERE child_id = $1 AND status = 'active' AND id <> $2
FOR UPDATE`
	var id int64
	err := r.q.QueryRowContext(ctx, query, childID, planID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *TxRepo) ActivateChildCarePlan(ctx context.Context, plan *models.ChildCarePlan, staffID int64) error {
	const query = `
UPDATE child_care_plans
SET status = 'active', activated_by_staff_id = $3, activated_at = now(),
    updated_at = now()
WHERE id = $1 AND child_id = $2 AND status = 'draft'
RETURNING activated_at, updated_at`
	if err := r.q.QueryRowContext(ctx, query, plan.ID, plan.ChildID, staffID).Scan(&plan.ActivatedAt, &plan.UpdatedAt); err != nil {
		return err
	}
	plan.Status = models.CarePlanStatusActive
	plan.ActivatedByStaffID = &staffID
	return nil
}

func (r *TxRepo) ArchiveChildCarePlan(ctx context.Context, plan *models.ChildCarePlan, staffID int64) error {
	const query = `
UPDATE child_care_plans
SET status = 'archived', archived_by_staff_id = $3, archived_at = now(),
    updated_at = now()
WHERE id = $1 AND child_id = $2 AND status IN ('draft', 'active')
RETURNING archived_at, updated_at`
	if err := r.q.QueryRowContext(ctx, query, plan.ID, plan.ChildID, staffID).Scan(&plan.ArchivedAt, &plan.UpdatedAt); err != nil {
		return err
	}
	plan.Status = models.CarePlanStatusArchived
	plan.ArchivedByStaffID = &staffID
	return nil
}

type carePlanSectionScanner interface {
	Scan(...any) error
}

func scanCarePlanSection(scanner carePlanSectionScanner) (*models.ChildCarePlanSection, error) {
	section := &models.ChildCarePlanSection{}
	var understanding, strengths, support, involvement sql.NullString
	if err := scanner.Scan(
		&section.ID, &section.CarePlanID, &section.SectionKey, &section.InformationStatus,
		&understanding, &strengths, &support, &involvement,
		&section.CreatedAt, &section.UpdatedAt,
	); err != nil {
		return nil, err
	}
	section.CurrentUnderstanding = stringFromNull(understanding)
	section.Strengths = stringFromNull(strengths)
	section.StaffSupport = stringFromNull(support)
	section.YoungPersonInvolvementNote = stringFromNull(involvement)
	return section, nil
}

func scanCarePlan(scanner carePlanSectionScanner) (*models.ChildCarePlan, error) {
	plan := &models.ChildCarePlan{}
	var reviewDate, activatedAt, archivedAt sql.NullTime
	var predecessor, activatedBy, archivedBy sql.NullInt64
	if err := scanner.Scan(
		&plan.ID, &plan.ChildID, &plan.Status, &plan.EffectiveFrom, &reviewDate,
		&predecessor, &plan.CreatedByStaffID, &activatedBy, &activatedAt,
		&archivedBy, &archivedAt, &plan.CreatedAt, &plan.UpdatedAt,
	); err != nil {
		return nil, err
	}
	plan.ReviewDate = timeFromNull(reviewDate)
	plan.PredecessorCarePlanID = int64FromNull(predecessor)
	plan.ActivatedByStaffID = int64FromNull(activatedBy)
	plan.ActivatedAt = timeFromNull(activatedAt)
	plan.ArchivedByStaffID = int64FromNull(archivedBy)
	plan.ArchivedAt = timeFromNull(archivedAt)
	return plan, nil
}

func int64FromNull(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}
