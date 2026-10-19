package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
)

var (
	ErrSessionNotCompleted           = errors.New("keyworking session is not completed")
	ErrSessionAlreadyManagerReviewed = errors.New("keyworking session already has a manager review")
)

// ---------------- Sessions & scores ----------------

// CreateKeyworkSessionWithScores inserts a session + its target scores in one tx.
func (r *Repo) CreateKeyworkSessionWithScores(
	ctx context.Context,
	s *models.KeyworkSession,
	scores []models.SessionScore,
) error {
	return r.WithTx(ctx, nil, func(tx *TxRepo) error {
		return tx.CreateKeyworkSessionWithScores(ctx, s, scores)
	})
}

// CreateKeyworkSessionWithScores persists a session and all scores through the
// caller's transaction. Protected handlers must use this method so locked
// authorization, relationships, writes, and audit share one transaction.
func (r *TxRepo) CreateKeyworkSessionWithScores(
	ctx context.Context,
	s *models.KeyworkSession,
	scores []models.SessionScore,
) error {

	// keyworking_sessions stores both the legacy notes_summary and
	// the structured journey fields used by the Key Working Journey.
	const qSession = `
INSERT INTO keyworking_sessions (
    child_id, session_date, staff_name, notes_summary,
    session_type, primary_domain, target_id,
    young_person_presentation, young_person_voice_note, key_worker_reflection,
    signed_by_key_worker, key_worker_signed_at,
    signed_by_manager, manager_signed_at, manager_name
)
VALUES ($1, ($2)::date, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING id, created_at`

	if err := r.q.QueryRowContext(
		ctx, qSession,
		s.ChildID,
		s.SessionAt, // time.Time -> cast to date in SQL
		s.StaffName,
		s.Notes, // stored as notes_summary
		s.SessionType,
		nullableStringPtr(s.PrimaryDomain),
		nullableInt64Ptr(s.TargetID),
		nullableStringPtr(s.YoungPersonPresentation),
		nullableStringPtr(s.YoungPersonVoiceNote),
		nullableStringPtr(s.KeyWorkerReflection),
		s.SignedByKeyWorker,
		nullableTime(s.KeyWorkerSignedAt),
		false,
		nil,
		nil,
	).Scan(&s.ID, &s.CreatedAt); err != nil {
		return fmt.Errorf("insert keyworking_session: %w", err)
	}

	// ✅ session_target_scores now has: wellbeing_score, progress_score, comment
	const qScore = `
INSERT INTO session_target_scores (
    session_id, target_id,
    wellbeing_score, progress_score,
    comment
)
VALUES ($1,$2,$3,$4,$5)
RETURNING id, created_at`

	for i := range scores {
		score := &scores[i]
		if err := r.q.QueryRowContext(
			ctx, qScore,
			s.ID,
			score.TargetID,
			score.WellbeingScore,
			score.ProgressScore,
			score.Comment,
		).Scan(&score.ID, &score.CreatedAt); err != nil {
			return fmt.Errorf("insert session_target_score: %w", err)
		}
		score.SessionID = s.ID
	}

	return nil
}

func (r *Repo) ListKeyworkSessionsByChild(ctx context.Context, childID int64) ([]*models.KeyworkSession, error) {
	// session_date is DATE; cast to timestamptz for your model SessionAt (time.Time)
	const q = `
SELECT
  ks.id,
  ks.child_id,
  ks.session_date::timestamptz AS session_at,
  COALESCE(ks.staff_name, '') AS staff_name,
  COALESCE(ks.notes_summary, '') AS notes,
  COALESCE(ks.session_type, 'keywork') AS session_type,
  ks.primary_domain,
  ks.target_id,
  ks.young_person_presentation,
  ks.young_person_voice_note,
  ks.key_worker_reflection,
  ks.signed_by_key_worker,
  ks.key_worker_signed_at,
  ks.signed_by_manager,
  ks.manager_signed_at,
  ks.manager_name,
  ks.manager_review_decision,
  ks.manager_review_comment,
  ks.manager_reviewed_by_staff_id,
  reviewer.display_name,
  ks.created_at
FROM keyworking_sessions ks
LEFT JOIN staff_users reviewer ON reviewer.id = ks.manager_reviewed_by_staff_id
WHERE ks.child_id = $1
ORDER BY ks.session_date DESC, ks.id DESC`

	rows, err := r.db.QueryContext(ctx, q, childID)
	if err != nil {
		return nil, fmt.Errorf("query keyworking_sessions: %w", err)
	}
	defer rows.Close()

	var out []*models.KeyworkSession
	for rows.Next() {
		var s models.KeyworkSession
		var primaryDomain, youngPersonPresentation, youngPersonVoiceNote sql.NullString
		var keyWorkerReflection, managerName, managerDecision, managerComment sql.NullString
		var managerReviewerName sql.NullString
		var targetID, managerReviewedByStaffID sql.NullInt64
		var keyWorkerSignedAt, managerSignedAt sql.NullTime

		if err := rows.Scan(
			&s.ID,
			&s.ChildID,
			&s.SessionAt,
			&s.StaffName,
			&s.Notes,
			&s.SessionType,
			&primaryDomain,
			&targetID,
			&youngPersonPresentation,
			&youngPersonVoiceNote,
			&keyWorkerReflection,
			&s.SignedByKeyWorker,
			&keyWorkerSignedAt,
			&s.SignedByManager,
			&managerSignedAt,
			&managerName,
			&managerDecision,
			&managerComment,
			&managerReviewedByStaffID,
			&managerReviewerName,
			&s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan keyworking_session: %w", err)
		}

		if primaryDomain.Valid {
			v := primaryDomain.String
			s.PrimaryDomain = &v
		}
		if targetID.Valid {
			v := targetID.Int64
			s.TargetID = &v
		}
		if youngPersonPresentation.Valid {
			v := youngPersonPresentation.String
			s.YoungPersonPresentation = &v
		}
		if youngPersonVoiceNote.Valid {
			v := youngPersonVoiceNote.String
			s.YoungPersonVoiceNote = &v
		}
		if keyWorkerReflection.Valid {
			v := keyWorkerReflection.String
			s.KeyWorkerReflection = &v
		}
		if keyWorkerSignedAt.Valid {
			v := keyWorkerSignedAt.Time
			s.KeyWorkerSignedAt = &v
		}
		if managerSignedAt.Valid {
			v := managerSignedAt.Time
			s.ManagerSignedAt = &v
		}
		if managerName.Valid {
			v := managerName.String
			s.ManagerName = &v
		}
		if managerDecision.Valid {
			v := models.ManagerReviewDecision(managerDecision.String)
			s.ManagerReviewDecision = &v
		}
		if managerComment.Valid {
			v := managerComment.String
			s.ManagerReviewComment = &v
		}
		if managerReviewedByStaffID.Valid {
			v := managerReviewedByStaffID.Int64
			s.ManagerReviewedByStaffID = &v
		}
		if managerReviewerName.Valid {
			v := managerReviewerName.String
			s.ManagerReviewerName = &v
		}
		s.Scores = []models.KeyworkSessionScore{}

		out = append(out, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate keyworking_sessions: %w", err)
	}
	scoresBySession, err := r.listKeyworkSessionScoresByChild(ctx, childID)
	if err != nil {
		return nil, err
	}
	for _, session := range out {
		if scores := scoresBySession[session.ID]; scores != nil {
			session.Scores = scores
		}
	}
	return out, nil
}

func (r *Repo) listKeyworkSessionScoresByChild(ctx context.Context, childID int64) (map[int64][]models.KeyworkSessionScore, error) {
	const query = `
SELECT scores.session_id, scores.target_id, scores.wellbeing_score,
       scores.progress_score, COALESCE(scores.comment, '')
FROM session_target_scores scores
JOIN keyworking_sessions sessions ON sessions.id = scores.session_id
WHERE sessions.child_id = $1
ORDER BY scores.session_id, scores.id`
	rows, err := r.db.QueryContext(ctx, query, childID)
	if err != nil {
		return nil, fmt.Errorf("query keyworking session scores: %w", err)
	}
	defer rows.Close()
	bySession := make(map[int64][]models.KeyworkSessionScore)
	for rows.Next() {
		var sessionID int64
		var score models.KeyworkSessionScore
		var wellbeing, progress sql.NullInt64
		if err := rows.Scan(&sessionID, &score.TargetID, &wellbeing, &progress, &score.Comment); err != nil {
			return nil, fmt.Errorf("scan keyworking session score: %w", err)
		}
		if wellbeing.Valid {
			value := int32(wellbeing.Int64)
			score.WellbeingScore = &value
		}
		if progress.Valid {
			value := int32(progress.Int64)
			score.ProgressScore = &value
		}
		bySession[sessionID] = append(bySession[sessionID], score)
	}
	return bySession, rows.Err()
}

func (r *TxRepo) LockKeyworkSessionForManagerReview(ctx context.Context, childID, sessionID int64) (*models.KeyworkSession, error) {
	const query = `
SELECT id, child_id, signed_by_key_worker, key_worker_signed_at,
       manager_review_decision
FROM keyworking_sessions
WHERE id = $1 AND child_id = $2
FOR UPDATE`
	var session models.KeyworkSession
	var keyWorkerSignedAt sql.NullTime
	var managerDecision sql.NullString
	if err := r.q.QueryRowContext(ctx, query, sessionID, childID).Scan(
		&session.ID, &session.ChildID, &session.SignedByKeyWorker,
		&keyWorkerSignedAt, &managerDecision,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, security.ErrInvalidRelationship
		}
		return nil, fmt.Errorf("lock keyworking session for manager review: %w", err)
	}
	if keyWorkerSignedAt.Valid {
		value := keyWorkerSignedAt.Time
		session.KeyWorkerSignedAt = &value
	}
	if managerDecision.Valid {
		value := models.ManagerReviewDecision(managerDecision.String)
		session.ManagerReviewDecision = &value
	}
	return &session, nil
}

func (r *TxRepo) CreateKeyworkManagerReview(
	ctx context.Context,
	session *models.KeyworkSession,
	review *models.KeyworkManagerReview,
	reviewerStaffID int64,
) error {
	if session == nil || review == nil || session.ID <= 0 || session.ChildID <= 0 || reviewerStaffID <= 0 {
		return security.ErrInvalidRelationship
	}
	if !session.SignedByKeyWorker || session.KeyWorkerSignedAt == nil {
		return ErrSessionNotCompleted
	}
	if session.ManagerReviewDecision != nil {
		return ErrSessionAlreadyManagerReviewed
	}
	const query = `
UPDATE keyworking_sessions
SET signed_by_manager = ($3 = 'approved'),
    manager_signed_at = now(),
    manager_review_decision = $3,
    manager_review_comment = $4,
    manager_reviewed_by_staff_id = $5,
    updated_at = now()
WHERE id = $1 AND child_id = $2
  AND signed_by_key_worker = TRUE
  AND key_worker_signed_at IS NOT NULL
  AND manager_review_decision IS NULL
RETURNING manager_signed_at`
	if err := r.q.QueryRowContext(
		ctx, query, session.ID, session.ChildID, review.Decision,
		nullableStringPtr(review.Comment), reviewerStaffID,
	).Scan(&review.ReviewedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSessionAlreadyManagerReviewed
		}
		return fmt.Errorf("create keyworking manager review: %w", err)
	}
	review.SessionID = session.ID
	return nil
}

// ListChildrenByHome returns all active children for a home.
func (r *Repo) ListChildrenByHome(ctx context.Context, homeID int64) ([]*models.Child, error) {
	const q = `
SELECT id, home_id, external_ref, initials,
       first_name, last_name, date_of_birth,
       legal_status, admission_date, gender,
       active, created_at, updated_at
FROM children
WHERE home_id = $1 AND active = true
ORDER BY initials, id`

	rows, err := r.db.QueryContext(ctx, q, homeID)
	if err != nil {
		return nil, fmt.Errorf("query children by home: %w", err)
	}
	defer rows.Close()

	var out []*models.Child
	for rows.Next() {
		var c models.Child
		if err := rows.Scan(
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
		); err != nil {
			return nil, fmt.Errorf("scan child: %w", err)
		}
		out = append(out, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate children: %w", err)
	}
	return out, nil
}

// ListTimelineItemsForChild returns a flattened timeline of scores + targets + sessions.
func (r *Repo) ListTimelineItemsForChild(ctx context.Context, childID int64) ([]models.TimelineItem, error) {
	// ✅ Match your live targets table: child_targets (short_label, domain)
	// ✅ Match sessions: keyworking_sessions(session_date)
	const q = `
SELECT
    s.id                          AS session_id,
    s.session_date::timestamptz   AS session_at,
    COALESCE(s.staff_name, '')    AS staff_name,
    t.id                          AS target_id,
    t.short_label                 AS target_name,
    t.domain                      AS target_domain,
    sc.wellbeing_score,
    sc.progress_score,
    COALESCE(sc.comment, '')      AS comment
FROM keyworking_sessions s
JOIN session_target_scores sc ON sc.session_id = s.id
JOIN child_targets t          ON t.id = sc.target_id
WHERE s.child_id = $1
ORDER BY s.session_date ASC, s.id ASC, t.domain, t.short_label`

	rows, err := r.db.QueryContext(ctx, q, childID)
	if err != nil {
		return nil, fmt.Errorf("query timeline items: %w", err)
	}
	defer rows.Close()

	var out []models.TimelineItem
	for rows.Next() {
		var item models.TimelineItem
		var wellbeingScore, progressScore sql.NullInt64
		if err := rows.Scan(
			&item.SessionID,
			&item.SessionAt,
			&item.StaffName,
			&item.TargetID,
			&item.TargetName,
			&item.TargetDomain,
			&wellbeingScore,
			&progressScore,
			&item.Comment,
		); err != nil {
			return nil, fmt.Errorf("scan timeline item: %w", err)
		}
		if !applyTimelineScores(&item, wellbeingScore, progressScore) {
			continue
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate timeline items: %w", err)
	}
	return out, nil
}

func applyTimelineScores(item *models.TimelineItem, wellbeingScore, progressScore sql.NullInt64) bool {
	if !wellbeingScore.Valid || !progressScore.Valid {
		return false
	}
	item.WellbeingScore = int32(wellbeingScore.Int64)
	item.ProgressScore = int32(progressScore.Int64)
	return true
}
