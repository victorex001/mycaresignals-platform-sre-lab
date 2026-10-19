package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"therapeutic-engine/internal/models"
)

var (
	ErrMFHOpenPlanExists   = errors.New("an open Missing From Home plan already exists")
	ErrMFHPlanNotDraft     = errors.New("Missing From Home plan is not a draft")
	ErrMFHPlanNotSubmitted = errors.New("Missing From Home plan is not submitted")
	ErrMFHPlanNotApproved  = errors.New("Missing From Home plan is not approved")
	ErrMFHUnknownSource    = errors.New("Missing From Home source candidate is no longer available")
)

func emptyMFHContent() models.MissingFromHomePlanContent {
	return models.MissingFromHomePlanContent{
		PeoplePlaces:  []models.MFHPersonPlace{},
		Triggers:      []models.MFHTrigger{},
		Notifications: []models.MFHNotification{},
	}
}

func candidateHash(candidate models.MFHReuseCandidate) string {
	updated := ""
	if candidate.SourceUpdatedAt != nil {
		updated = candidate.SourceUpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{
		candidate.CandidateKey, candidate.SourceType, candidate.Label, candidate.Value,
		candidate.SourceLabel, string(candidate.InformationStatus), updated,
	}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func appendMFHCandidate(items []models.MFHReuseCandidate, candidate models.MFHReuseCandidate) []models.MFHReuseCandidate {
	candidate.Value = strings.TrimSpace(candidate.Value)
	if candidate.Value == "" {
		return items
	}
	candidate.SnapshotSHA256 = candidateHash(candidate)
	return append(items, candidate)
}

func mfhInformationStatus(source string) models.MFHInformationStatus {
	switch strings.TrimSpace(source) {
	case string(models.MFHInformationKnownConfirmed), "current_confirmed":
		return models.MFHInformationKnownConfirmed
	case string(models.MFHInformationAwaitingConfirmation):
		return models.MFHInformationAwaitingConfirmation
	case string(models.MFHInformationNotKnownYet):
		return models.MFHInformationNotKnownYet
	default:
		return models.MFHInformationPartlyKnown
	}
}

func (r *Repo) ListMFHReuseCandidates(ctx context.Context, childID int64) ([]models.MFHReuseCandidate, error) {
	return listMFHReuseCandidates(ctx, r.db, childID)
}

func (r *TxRepo) ListMFHReuseCandidates(ctx context.Context, childID int64) ([]models.MFHReuseCandidate, error) {
	return listMFHReuseCandidates(ctx, r.q, childID)
}

func listMFHReuseCandidates(ctx context.Context, q DBTX, childID int64) ([]models.MFHReuseCandidate, error) {
	items := make([]models.MFHReuseCandidate, 0)
	var homeName, legalStatus sql.NullString
	var admissionDate, childUpdated sql.NullTime
	if err := q.QueryRowContext(ctx, `
SELECT h.name, NULLIF(c.legal_status, ''), c.admission_date, c.updated_at
FROM children c JOIN homes h ON h.id = c.home_id
WHERE c.id = $1`, childID).Scan(&homeName, &legalStatus, &admissionDate, &childUpdated); err != nil {
		return nil, err
	}
	updated := timeFromNull(childUpdated)
	items = appendMFHCandidate(items, models.MFHReuseCandidate{
		CandidateKey: "child:current_placement", Category: "current_situation", SourceType: "child_record",
		Label: "Current placement", Value: homeName.String, SourceLabel: "Child record",
		InformationStatus: models.MFHInformationKnownConfirmed, SourceUpdatedAt: updated,
	})
	if admissionDate.Valid {
		items = appendMFHCandidate(items, models.MFHReuseCandidate{
			CandidateKey: "child:admission_date", Category: "current_situation", SourceType: "child_record",
			Label: "Placement start date", Value: admissionDate.Time.Format("2006-01-02"), SourceLabel: "Child record",
			InformationStatus: models.MFHInformationKnownConfirmed, SourceUpdatedAt: updated,
		})
	}
	if legalStatus.Valid {
		items = appendMFHCandidate(items, models.MFHReuseCandidate{
			CandidateKey: "child:legal_status", Category: "current_situation", SourceType: "child_record",
			Label: "Current legal information", Value: legalStatus.String, SourceLabel: "Child record",
			InformationStatus: models.MFHInformationKnownConfirmed, SourceUpdatedAt: updated,
		})
	}

	var baselineID int64
	var immediate, safety, earlyWarning, otherProfessionals, frameworkStatus string
	var frameworkUpdated time.Time
	err := q.QueryRowContext(ctx, `
SELECT id, COALESCE(immediate_information, ''), COALESCE(known_safety_information, ''),
       COALESCE(early_warning_signals, ''), COALESCE(other_professionals, ''), status, updated_at
FROM child_onboarding_baselines
WHERE child_id = $1 AND framework_model = 'six_domain_v1'
ORDER BY framework_version DESC, id DESC LIMIT 1`, childID).Scan(
		&baselineID, &immediate, &safety, &earlyWarning, &otherProfessionals, &frameworkStatus, &frameworkUpdated,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		baselineSource := fmt.Sprintf("Initial Framework · %s", frameworkStatus)
		baselineSourceID := baselineID
		items = appendMFHCandidate(items, models.MFHReuseCandidate{
			CandidateKey: "framework:immediate_information", Category: "current_situation", SourceType: "initial_framework",
			SourceID: &baselineSourceID, Label: "What staff need to know now", Value: immediate, SourceLabel: baselineSource,
			InformationStatus: models.MFHInformationPartlyKnown, SourceUpdatedAt: &frameworkUpdated,
		})
		items = appendMFHCandidate(items, models.MFHReuseCandidate{
			CandidateKey: "framework:known_safety_information", Category: "current_situation", SourceType: "initial_framework",
			SourceID: &baselineSourceID, Label: "Known safety information", Value: safety, SourceLabel: baselineSource,
			InformationStatus: models.MFHInformationPartlyKnown, SourceUpdatedAt: &frameworkUpdated,
		})
		items = appendMFHCandidate(items, models.MFHReuseCandidate{
			CandidateKey: "framework:early_warning_signals", Category: "triggers", SourceType: "initial_framework",
			SourceID: &baselineSourceID, Label: "Existing warning signs", Value: earlyWarning, SourceLabel: baselineSource,
			InformationStatus: models.MFHInformationPartlyKnown, SourceUpdatedAt: &frameworkUpdated,
		})
		items = appendMFHCandidate(items, models.MFHReuseCandidate{
			CandidateKey: "framework:professionals", Category: "notifications", SourceType: "initial_framework",
			SourceID: &baselineSourceID, Label: "Professionals already recorded", Value: otherProfessionals, SourceLabel: baselineSource,
			InformationStatus: models.MFHInformationPartlyKnown, SourceUpdatedAt: &frameworkUpdated,
		})
		var understanding, whatHelps, infoStatus string
		err = q.QueryRowContext(ctx, `
SELECT COALESCE(current_understanding, ''), COALESCE(what_helps, ''), COALESCE(information_status, 'partly_known')
FROM child_onboarding_domain_states
WHERE baseline_id = $1 AND domain_key = 'Safety, Risk & Coping with Stress'`, baselineID).Scan(&understanding, &whatHelps, &infoStatus)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			status := mfhInformationStatus(infoStatus)
			items = appendMFHCandidate(items, models.MFHReuseCandidate{
				CandidateKey: "framework:safety_understanding", Category: "triggers", SourceType: "initial_framework",
				SourceID: &baselineSourceID, Label: "Existing safety understanding", Value: understanding, SourceLabel: baselineSource,
				InformationStatus: status, SourceUpdatedAt: &frameworkUpdated,
			})
			items = appendMFHCandidate(items, models.MFHReuseCandidate{
				CandidateKey: "framework:what_helps", Category: "triggers", SourceType: "initial_framework",
				SourceID: &baselineSourceID, Label: "What is recorded as helpful", Value: whatHelps, SourceLabel: baselineSource,
				InformationStatus: status, SourceUpdatedAt: &frameworkUpdated,
			})
		}
	}

	rows, err := q.QueryContext(ctx, `
SELECT id, title, category, nature_of_risk, information_status, updated_at
FROM child_risks
WHERE child_id = $1 AND status = 'active'
  AND (lower(title) LIKE '%missing%' OR lower(category) LIKE '%missing%' OR lower(title) LIKE '%abscond%')
ORDER BY updated_at DESC, id DESC`, childID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var title, category, nature, informationStatus string
		var riskUpdated time.Time
		if err := rows.Scan(&id, &title, &category, &nature, &informationStatus, &riskUpdated); err != nil {
			return nil, err
		}
		value := title
		if strings.TrimSpace(nature) != "" {
			value += " — " + nature
		}
		items = appendMFHCandidate(items, models.MFHReuseCandidate{
			CandidateKey: fmt.Sprintf("risk:%d", id), Category: "current_situation", SourceType: "risk_safety",
			SourceID: &id, Label: "Active Missing From Home risk", Value: value,
			SourceLabel: "Risk & Safety · " + category, InformationStatus: mfhInformationStatus(informationStatus),
			SourceUpdatedAt: &riskUpdated,
		})
	}
	return items, rows.Err()
}

func (r *TxRepo) MaterializeMFHSourceDecisions(ctx context.Context, childID int64, decisions []models.MFHSourceDecision) ([]models.MFHSourceSnapshot, error) {
	if len(decisions) > 50 {
		return nil, ErrMFHUnknownSource
	}
	candidates, err := r.ListMFHReuseCandidates(ctx, childID)
	if err != nil {
		return nil, err
	}
	available := make(map[string]models.MFHReuseCandidate, len(candidates))
	for _, candidate := range candidates {
		available[candidate.CandidateKey] = candidate
	}
	seen := make(map[string]struct{}, len(decisions))
	snapshots := make([]models.MFHSourceSnapshot, 0, len(decisions))
	for _, decision := range decisions {
		key := strings.TrimSpace(decision.CandidateKey)
		candidate, ok := available[key]
		if !ok {
			return nil, ErrMFHUnknownSource
		}
		if _, duplicate := seen[key]; duplicate {
			return nil, ErrMFHUnknownSource
		}
		seen[key] = struct{}{}
		snapshots = append(snapshots, models.MFHSourceSnapshot{
			CandidateKey: candidate.CandidateKey, Category: candidate.Category, SourceType: candidate.SourceType,
			SourceID: candidate.SourceID, Label: candidate.Label, Value: candidate.Value,
			SourceLabel: candidate.SourceLabel, InformationStatus: candidate.InformationStatus,
			SourceUpdatedAt: candidate.SourceUpdatedAt, SnapshotSHA256: candidate.SnapshotSHA256,
			Included: decision.Included,
		})
	}
	return snapshots, nil
}

func (r *TxRepo) CreateMFHDraft(ctx context.Context, childID, staffID int64) (*models.MissingFromHomePlan, error) {
	var openCount int
	if err := r.q.QueryRowContext(ctx, `SELECT count(*) FROM child_missing_from_home_plans WHERE child_id = $1 AND status IN ('draft','submitted')`, childID).Scan(&openCount); err != nil {
		return nil, err
	}
	if openCount > 0 {
		return nil, ErrMFHOpenPlanExists
	}
	var version int
	if err := r.q.QueryRowContext(ctx, `SELECT COALESCE(max(version), 0) + 1 FROM child_missing_from_home_plans WHERE child_id = $1`, childID).Scan(&version); err != nil {
		return nil, err
	}
	contentJSON, _ := json.Marshal(emptyMFHContent())
	plan := &models.MissingFromHomePlan{ChildID: childID, Version: version, Status: models.MFHPlanDraft, CreatedByStaffID: staffID}
	if err := r.q.QueryRowContext(ctx, `
INSERT INTO child_missing_from_home_plans (child_id, version, status, content, source_snapshots, created_by_staff_id)
VALUES ($1,$2,'draft',$3,'[]'::jsonb,$4)
RETURNING id, created_at, updated_at`, childID, version, contentJSON, staffID).Scan(&plan.ID, &plan.CreatedAt, &plan.UpdatedAt); err != nil {
		return nil, err
	}
	plan.Content = emptyMFHContent()
	plan.SourceSnapshots = []models.MFHSourceSnapshot{}
	return plan, nil
}

func (r *Repo) GetCurrentMFHPlan(ctx context.Context, childID int64) (*models.MissingFromHomePlan, error) {
	plan, err := scanMFHPlan(r.db.QueryRowContext(ctx, mfhPlanSelect+`
WHERE plan.child_id = $1
ORDER BY CASE plan.status WHEN 'draft' THEN 0 WHEN 'submitted' THEN 1 WHEN 'approved' THEN 2 ELSE 3 END,
         plan.version DESC LIMIT 1`, childID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := r.markMFHSourceChanges(ctx, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func (r *Repo) GetMFHPlan(ctx context.Context, childID, planID int64) (*models.MissingFromHomePlan, error) {
	plan, err := scanMFHPlan(r.db.QueryRowContext(ctx, mfhPlanSelect+` WHERE plan.child_id = $1 AND plan.id = $2`, childID, planID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := r.markMFHSourceChanges(ctx, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func (r *Repo) ListMFHPlans(ctx context.Context, childID int64) ([]models.MissingFromHomePlan, error) {
	rows, err := r.db.QueryContext(ctx, mfhPlanSelect+` WHERE plan.child_id = $1 ORDER BY plan.version DESC, plan.id DESC`, childID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.MissingFromHomePlan, 0)
	for rows.Next() {
		plan, err := scanMFHPlan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *plan)
	}
	return items, rows.Err()
}

const mfhPlanSelect = `
SELECT plan.id, plan.child_id, plan.version, plan.status, plan.predecessor_plan_id,
       plan.content, plan.source_snapshots, plan.review_date,
       plan.created_by_staff_id, creator.display_name,
       plan.submitted_by_staff_id, submitter.display_name,
       plan.approved_by_staff_id, approver.display_name,
       plan.archived_by_staff_id, archiver.display_name,
       plan.submitted_at, plan.approved_at, plan.archived_at, plan.created_at, plan.updated_at
FROM child_missing_from_home_plans plan
JOIN staff_users creator ON creator.id = plan.created_by_staff_id
LEFT JOIN staff_users submitter ON submitter.id = plan.submitted_by_staff_id
LEFT JOIN staff_users approver ON approver.id = plan.approved_by_staff_id
LEFT JOIN staff_users archiver ON archiver.id = plan.archived_by_staff_id`

func scanMFHPlan(scanner rowScanner) (*models.MissingFromHomePlan, error) {
	var plan models.MissingFromHomePlan
	var predecessor, submittedBy, approvedBy, archivedBy sql.NullInt64
	var submittedName, approvedName, archivedName sql.NullString
	var reviewDate, submittedAt, approvedAt, archivedAt sql.NullTime
	var contentJSON, snapshotsJSON []byte
	if err := scanner.Scan(
		&plan.ID, &plan.ChildID, &plan.Version, &plan.Status, &predecessor,
		&contentJSON, &snapshotsJSON, &reviewDate,
		&plan.CreatedByStaffID, &plan.CreatedByName, &submittedBy, &submittedName,
		&approvedBy, &approvedName, &archivedBy, &archivedName,
		&submittedAt, &approvedAt, &archivedAt, &plan.CreatedAt, &plan.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(contentJSON, &plan.Content); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(snapshotsJSON, &plan.SourceSnapshots); err != nil {
		return nil, err
	}
	plan.PredecessorPlanID = int64FromNull(predecessor)
	plan.SubmittedByStaffID, plan.ApprovedByStaffID, plan.ArchivedByStaffID = int64FromNull(submittedBy), int64FromNull(approvedBy), int64FromNull(archivedBy)
	plan.SubmittedByName, plan.ApprovedByName, plan.ArchivedByName = stringFromNull(submittedName), stringFromNull(approvedName), stringFromNull(archivedName)
	plan.ReviewDate, plan.SubmittedAt, plan.ApprovedAt, plan.ArchivedAt = timeFromNull(reviewDate), timeFromNull(submittedAt), timeFromNull(approvedAt), timeFromNull(archivedAt)
	if plan.SourceSnapshots == nil {
		plan.SourceSnapshots = []models.MFHSourceSnapshot{}
	}
	return &plan, nil
}

func (r *Repo) markMFHSourceChanges(ctx context.Context, plan *models.MissingFromHomePlan) error {
	if plan == nil || len(plan.SourceSnapshots) == 0 {
		return nil
	}
	candidates, err := r.ListMFHReuseCandidates(ctx, plan.ChildID)
	if err != nil {
		return err
	}
	current := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		current[candidate.CandidateKey] = candidate.SnapshotSHA256
	}
	for index := range plan.SourceSnapshots {
		if current[plan.SourceSnapshots[index].CandidateKey] != plan.SourceSnapshots[index].SnapshotSHA256 {
			plan.SourceSnapshots[index].Changed = true
			if plan.SourceSnapshots[index].Included {
				plan.SourcesChanged = true
			}
		}
	}
	return nil
}

func (r *TxRepo) LockMFHPlan(ctx context.Context, childID, planID int64) (*models.MissingFromHomePlan, error) {
	return scanMFHPlan(r.q.QueryRowContext(ctx, mfhPlanSelect+` WHERE plan.child_id = $1 AND plan.id = $2 FOR UPDATE OF plan`, childID, planID))
}

func (r *TxRepo) UpdateMFHDraft(ctx context.Context, plan *models.MissingFromHomePlan) error {
	if plan.Status != models.MFHPlanDraft {
		return ErrMFHPlanNotDraft
	}
	contentJSON, err := json.Marshal(plan.Content)
	if err != nil {
		return err
	}
	snapshotsJSON, err := json.Marshal(plan.SourceSnapshots)
	if err != nil {
		return err
	}
	result, err := r.q.ExecContext(ctx, `
UPDATE child_missing_from_home_plans
SET content = $3, source_snapshots = $4, review_date = $5, updated_at = now()
WHERE id = $1 AND child_id = $2 AND status = 'draft'`, plan.ID, plan.ChildID, contentJSON, snapshotsJSON, nullableTime(plan.ReviewDate))
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return ErrMFHPlanNotDraft
	}
	return nil
}

func (r *TxRepo) SubmitMFHPlan(ctx context.Context, plan *models.MissingFromHomePlan, staffID int64) error {
	if plan.Status != models.MFHPlanDraft {
		return ErrMFHPlanNotDraft
	}
	if _, err := r.q.ExecContext(ctx, `
UPDATE child_missing_from_home_plans
SET status='submitted', submitted_by_staff_id=$3, submitted_at=now(), updated_at=now()
WHERE id=$1 AND child_id=$2 AND status='draft'`, plan.ID, plan.ChildID, staffID); err != nil {
		return err
	}
	plan.Status = models.MFHPlanSubmitted
	return nil
}

func (r *TxRepo) ApproveMFHPlan(ctx context.Context, plan *models.MissingFromHomePlan, staffID int64) error {
	if plan.Status != models.MFHPlanSubmitted {
		return ErrMFHPlanNotSubmitted
	}
	if _, err := r.q.ExecContext(ctx, `
UPDATE child_missing_from_home_plans
SET status='superseded', updated_at=now()
WHERE child_id=$1 AND status='approved'`, plan.ChildID); err != nil {
		return err
	}
	if _, err := r.q.ExecContext(ctx, `
UPDATE child_missing_from_home_plans
SET status='approved', approved_by_staff_id=$3, approved_at=now(), updated_at=now()
WHERE id=$1 AND child_id=$2 AND status='submitted'`, plan.ID, plan.ChildID, staffID); err != nil {
		return err
	}
	plan.Status = models.MFHPlanApproved
	return nil
}

func (r *TxRepo) CreateMFHRevision(ctx context.Context, predecessor *models.MissingFromHomePlan, staffID int64) (*models.MissingFromHomePlan, error) {
	if predecessor.Status != models.MFHPlanApproved {
		return nil, ErrMFHPlanNotApproved
	}
	var openCount int
	if err := r.q.QueryRowContext(ctx, `SELECT count(*) FROM child_missing_from_home_plans WHERE child_id=$1 AND status IN ('draft','submitted')`, predecessor.ChildID).Scan(&openCount); err != nil {
		return nil, err
	}
	if openCount > 0 {
		return nil, ErrMFHOpenPlanExists
	}
	var version int
	if err := r.q.QueryRowContext(ctx, `SELECT COALESCE(max(version),0)+1 FROM child_missing_from_home_plans WHERE child_id=$1`, predecessor.ChildID).Scan(&version); err != nil {
		return nil, err
	}
	contentJSON, _ := json.Marshal(predecessor.Content)
	snapshotsJSON, _ := json.Marshal(predecessor.SourceSnapshots)
	plan := &models.MissingFromHomePlan{
		ChildID: predecessor.ChildID, Version: version, Status: models.MFHPlanDraft,
		PredecessorPlanID: &predecessor.ID, Content: predecessor.Content,
		SourceSnapshots: predecessor.SourceSnapshots, ReviewDate: predecessor.ReviewDate,
		CreatedByStaffID: staffID,
	}
	if err := r.q.QueryRowContext(ctx, `
INSERT INTO child_missing_from_home_plans
  (child_id, version, status, predecessor_plan_id, content, source_snapshots, review_date, created_by_staff_id)
VALUES ($1,$2,'draft',$3,$4,$5,$6,$7)
RETURNING id, created_at, updated_at`, plan.ChildID, plan.Version, predecessor.ID,
		contentJSON, snapshotsJSON, nullableTime(plan.ReviewDate), staffID,
	).Scan(&plan.ID, &plan.CreatedAt, &plan.UpdatedAt); err != nil {
		return nil, err
	}
	return plan, nil
}

func (r *TxRepo) ArchiveMFHPlan(ctx context.Context, plan *models.MissingFromHomePlan, staffID int64) error {
	if plan.Status != models.MFHPlanApproved {
		return ErrMFHPlanNotApproved
	}
	if _, err := r.q.ExecContext(ctx, `
UPDATE child_missing_from_home_plans
SET status='archived', archived_by_staff_id=$3, archived_at=now(), updated_at=now()
WHERE id=$1 AND child_id=$2 AND status='approved'`, plan.ID, plan.ChildID, staffID); err != nil {
		return err
	}
	plan.Status = models.MFHPlanArchived
	return nil
}
