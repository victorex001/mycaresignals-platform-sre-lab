package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"therapeutic-engine/internal/models"
	progressview "therapeutic-engine/internal/progress"
	"therapeutic-engine/internal/security"
)

var (
	ErrDailyProgressTargetNotActive = errors.New("daily progress target is not active")
	ErrDailyProgressDomainMismatch  = errors.New("daily progress domain does not match target")
	ErrDailyProgressInvalidDomain   = errors.New("daily progress domain is invalid")
	DailyProgressWriteRoles         = []security.Role{security.RoleKeyWorker, security.RoleSenior, security.RoleManager}
)

func (r *Repo) CreateDailyProgress(
	ctx context.Context,
	childID int64,
	input models.DailyProgressInput,
	requestID string,
) (*models.ChildDailyProgress, error) {
	entry := &models.ChildDailyProgress{
		ChildID: childID, ObservedOn: dayOnly(input.ObservedOn), Domain: strings.TrimSpace(input.Domain),
		TargetID: input.TargetID, WellbeingScore: input.WellbeingScore, ProgressScore: input.ProgressScore,
		ObservationNote: trimStringPointer(input.ObservationNote), YoungPersonComment: trimStringPointer(input.YoungPersonComment),
	}
	err := r.WithTx(ctx, nil, func(tx *TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(ctx, tx, childID, DailyProgressWriteRoles...)
		if err != nil {
			return err
		}
		entry.RecordedByStaffID = principal.StaffUserID()
		entry.RecordedByStaffName = principal.DisplayName()

		if entry.TargetID != nil {
			domain, status, err := tx.lockDailyProgressTarget(ctx, childID, *entry.TargetID)
			if err != nil {
				return err
			}
			if status != models.GoalStatusActive {
				return ErrDailyProgressTargetNotActive
			}
			if entry.Domain != "" && entry.Domain != domain {
				return ErrDailyProgressDomainMismatch
			}
			entry.Domain = domain
		}
		if !models.ValidProgressDomain(entry.Domain) {
			return ErrDailyProgressInvalidDomain
		}
		if err := tx.insertDailyProgress(ctx, entry); err != nil {
			return err
		}
		bound, err := tx.NewDailyProgressAuditEvent(
			ctx, principal, entry.ID, AuditEventDailyProgressCreate,
			security.AuditMetadataFields{TargetID: entry.TargetID, RequestID: requestID},
			AuditRouteDailyProgress,
		)
		if err != nil {
			return err
		}
		_, err = tx.InsertAuditEvent(ctx, bound)
		return err
	})
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func (r *TxRepo) lockDailyProgressTarget(ctx context.Context, childID, targetID int64) (string, models.GoalStatus, error) {
	const query = `
SELECT domain, status
FROM child_targets
WHERE id = $1 AND child_id = $2
FOR UPDATE`
	var domain string
	var status models.GoalStatus
	if err := r.q.QueryRowContext(ctx, query, targetID, childID).Scan(&domain, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", security.ErrInvalidRelationship
		}
		return "", "", err
	}
	return domain, status, nil
}

func (r *TxRepo) insertDailyProgress(ctx context.Context, entry *models.ChildDailyProgress) error {
	const query = `
INSERT INTO child_daily_progress (
  child_id, recorded_by_staff_id, observed_on, domain, target_id,
  wellbeing_score, progress_score, observation_note, young_person_comment
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
RETURNING id, created_at`
	return r.q.QueryRowContext(ctx, query,
		entry.ChildID, entry.RecordedByStaffID, entry.ObservedOn, entry.Domain, entry.TargetID,
		entry.WellbeingScore, entry.ProgressScore, entry.ObservationNote, entry.YoungPersonComment,
	).Scan(&entry.ID, &entry.CreatedAt)
}

func (r *Repo) ListDailyProgress(ctx context.Context, childID int64, limit int) ([]models.ChildDailyProgress, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const query = `
SELECT progress.id, progress.child_id, progress.recorded_by_staff_id,
       staff.display_name, progress.observed_on, progress.domain,
       progress.target_id, target.short_label, progress.wellbeing_score,
       progress.progress_score, progress.observation_note,
       progress.young_person_comment, progress.created_at
FROM child_daily_progress progress
JOIN staff_users staff ON staff.id = progress.recorded_by_staff_id
LEFT JOIN child_targets target ON target.id = progress.target_id AND target.child_id = progress.child_id
WHERE progress.child_id = $1
ORDER BY progress.observed_on DESC, progress.id DESC
LIMIT $2`
	rows, err := r.db.QueryContext(ctx, query, childID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.ChildDailyProgress, 0)
	for rows.Next() {
		entry, err := scanDailyProgress(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *entry)
	}
	return items, rows.Err()
}

func scanDailyProgress(scanner interface{ Scan(...any) error }) (*models.ChildDailyProgress, error) {
	var entry models.ChildDailyProgress
	var targetID sql.NullInt64
	var targetLabel, note, comment sql.NullString
	var wellbeing, progress sql.NullInt64
	if err := scanner.Scan(
		&entry.ID, &entry.ChildID, &entry.RecordedByStaffID, &entry.RecordedByStaffName,
		&entry.ObservedOn, &entry.Domain, &targetID, &targetLabel, &wellbeing,
		&progress, &note, &comment, &entry.CreatedAt,
	); err != nil {
		return nil, err
	}
	entry.TargetID = int64FromNull(targetID)
	entry.TargetLabel = stringFromNull(targetLabel)
	entry.WellbeingScore = intFromNull(wellbeing)
	entry.ProgressScore = intFromNull(progress)
	entry.ObservationNote = stringFromNull(note)
	entry.YoungPersonComment = stringFromNull(comment)
	return &entry, nil
}

func (r *Repo) ListDailyProgressMonthly(
	ctx context.Context,
	childID int64,
	fromMonth, toMonthExclusive time.Time,
) ([]models.MonthlyPerspectiveComparison, error) {
	observations, err := r.listDailyProgressForMonths(ctx, childID, fromMonth, toMonthExclusive)
	if err != nil {
		return nil, err
	}
	voices, err := r.listMonthlyVoiceRatings(ctx, childID, fromMonth, toMonthExclusive)
	if err != nil {
		return nil, err
	}
	keyworking, err := r.listMonthlyKeyworkingCounts(ctx, childID, fromMonth, toMonthExclusive)
	if err != nil {
		return nil, err
	}
	return progressview.BuildMonthlyComparisons(observations, voices, keyworking), nil
}

func (r *Repo) listDailyProgressForMonths(ctx context.Context, childID int64, fromMonth, toMonthExclusive time.Time) ([]models.ChildDailyProgress, error) {
	const query = `
SELECT id, child_id, recorded_by_staff_id, '' AS staff_name, observed_on, domain,
       target_id, NULL::text AS target_label, wellbeing_score, progress_score,
       observation_note, young_person_comment, created_at
FROM child_daily_progress
WHERE child_id = $1 AND observed_on >= $2::date AND observed_on < $3::date
ORDER BY observed_on, id`
	rows, err := r.db.QueryContext(ctx, query, childID, fromMonth, toMonthExclusive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.ChildDailyProgress, 0)
	for rows.Next() {
		entry, err := scanDailyProgress(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *entry)
	}
	return items, rows.Err()
}

func (r *Repo) listMonthlyVoiceRatings(ctx context.Context, childID int64, fromMonth, toMonthExclusive time.Time) ([]models.ChildMonthlyVoiceRating, error) {
	const query = `
SELECT id, child_id, period_month, domain, young_person_rating,
       source_receipt_id, source_review_id, source_context_id::text,
       source, submitted_at, created_at
FROM child_monthly_voice_ratings
WHERE child_id = $1 AND period_month >= $2::date AND period_month < $3::date
ORDER BY period_month, domain, id`
	rows, err := r.db.QueryContext(ctx, query, childID, fromMonth, toMonthExclusive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.ChildMonthlyVoiceRating, 0)
	for rows.Next() {
		var item models.ChildMonthlyVoiceRating
		if err := rows.Scan(
			&item.ID, &item.ChildID, &item.PeriodMonth, &item.Domain, &item.YoungPersonRating,
			&item.SourceReceiptID, &item.SourceReviewID, &item.SourceContextID,
			&item.Source, &item.SubmittedAt, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repo) listMonthlyKeyworkingCounts(ctx context.Context, childID int64, fromMonth, toMonthExclusive time.Time) ([]models.MonthlyKeyworkingCount, error) {
	const query = `
SELECT date_trunc('month', session.session_date)::date,
       COALESCE(NULLIF(btrim(session.primary_domain), ''), target.domain),
       count(DISTINCT session.id)
FROM keyworking_sessions session
LEFT JOIN child_targets target ON target.id = session.target_id AND target.child_id = session.child_id
WHERE session.child_id = $1
  AND session.session_date >= $2
  AND session.session_date < $3
  AND COALESCE(NULLIF(btrim(session.primary_domain), ''), target.domain) IS NOT NULL
GROUP BY 1, 2
ORDER BY 1, 2`
	rows, err := r.db.QueryContext(ctx, query, childID, fromMonth, toMonthExclusive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.MonthlyKeyworkingCount, 0)
	for rows.Next() {
		var item models.MonthlyKeyworkingCount
		if err := rows.Scan(&item.PeriodMonth, &item.Domain, &item.Count); err != nil {
			return nil, err
		}
		if !models.ValidProgressDomain(item.Domain) {
			continue
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func trimStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func dayOnly(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
