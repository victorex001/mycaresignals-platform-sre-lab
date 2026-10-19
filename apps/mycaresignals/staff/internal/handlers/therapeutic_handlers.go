package handlers

import (
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
	"therapeutic-engine/internal/storage"
)

type TherapeuticHandler struct {
	Repo *storage.Repo
}

func NewTherapeuticHandler(repo *storage.Repo) *TherapeuticHandler {
	return &TherapeuticHandler{Repo: repo}
}

type scoreInput struct {
	TargetID       int64  `json:"target_id"`
	WellbeingScore int32  `json:"wellbeing_score"`
	ProgressScore  int32  `json:"progress_score"`
	Comment        string `json:"comment"`
}

type createSessionRequest struct {
	SessionAt string       `json:"session_at"`
	StaffName string       `json:"staff_name"`
	Notes     string       `json:"notes"`
	Scores    []scoreInput `json:"scores"`

	SessionType             string `json:"session_type"`
	PrimaryDomain           string `json:"primary_domain"`
	TargetID                *int64 `json:"target_id"`
	YoungPersonPresentation string `json:"young_person_presentation"`
	YoungPersonVoiceNote    string `json:"young_person_voice_note"`
	KeyWorkerReflection     string `json:"key_worker_reflection"`
	SignedByKeyWorker       bool   `json:"signed_by_key_worker"`
}

type managerReviewRequest struct {
	Decision string `json:"decision"`
	Comment  string `json:"comment"`
}

type psychReviewRequest struct {
	WindowDays        int    `json:"window_days"`
	ReviewFrom        string `json:"review_from"`
	ReviewTo          string `json:"review_to"`
	Formulation       string `json:"formulation"`
	ProtectiveFactors string `json:"protective_factors"`
	Recommendations   string `json:"recommendations"`
	CreatedBy         string `json:"created_by"` // accepted for compatibility; ignored
}

func (h *TherapeuticHandler) CreateSessionWithScores(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("therapeutic repo not initialised"))
		return
	}

	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}

	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, "invalid json body")
		return
	}
	if len(req.Scores) == 0 {
		writeBadRequest(w, "at least one score is required")
		return
	}

	sessionTime := time.Now().UTC()
	if req.SessionAt != "" {
		if t, err := time.Parse(time.RFC3339, req.SessionAt); err == nil {
			sessionTime = t
		} else if t2, err2 := time.Parse("2006-01-02", req.SessionAt); err2 == nil {
			sessionTime = t2
		} else {
			writeBadRequest(w, "invalid session_at format")
			return
		}
	}

	sessionType := strings.TrimSpace(req.SessionType)
	if sessionType == "" {
		sessionType = string(security.SessionTypeKeywork)
	}
	if sessionType != string(security.SessionTypeKeywork) {
		writeBadRequest(w, "invalid session_type")
		return
	}
	if req.TargetID != nil && *req.TargetID <= 0 {
		writeBadRequest(w, "target_id must be positive")
		return
	}
	for _, score := range req.Scores {
		if score.TargetID <= 0 {
			writeBadRequest(w, "score target_id must be positive")
			return
		}
		if !validSessionScore(score.WellbeingScore) || !validSessionScore(score.ProgressScore) {
			writeBadRequest(w, "score values must be between 0 and 4")
			return
		}
	}

	now := time.Now().UTC()

	sess := &models.KeyworkSession{
		ChildID:           childID,
		SessionAt:         sessionTime,
		Notes:             req.Notes,
		SessionType:       sessionType,
		TargetID:          req.TargetID,
		SignedByKeyWorker: req.SignedByKeyWorker,
	}

	if req.SignedByKeyWorker {
		sess.KeyWorkerSignedAt = &now
	}
	if v := strings.TrimSpace(req.PrimaryDomain); v != "" {
		sess.PrimaryDomain = &v
	}
	if v := strings.TrimSpace(req.YoungPersonPresentation); v != "" {
		sess.YoungPersonPresentation = &v
	}
	if v := strings.TrimSpace(req.YoungPersonVoiceNote); v != "" {
		sess.YoungPersonVoiceNote = &v
	}
	if v := strings.TrimSpace(req.KeyWorkerReflection); v != "" {
		sess.KeyWorkerReflection = &v
	}
	scores := make([]models.SessionScore, len(req.Scores))
	for i, in := range req.Scores {
		scores[i] = models.SessionScore{
			TargetID:       in.TargetID,
			WellbeingScore: in.WellbeingScore,
			ProgressScore:  in.ProgressScore,
			Comment:        in.Comment,
		}
	}

	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(
			r.Context(),
			tx,
			childID,
			security.RoleKeyWorker,
			security.RoleSenior,
			security.RoleManager,
		)
		if err != nil {
			return err
		}
		sess.StaffName = principal.DisplayName()

		primaryTargetOK, err := tx.LockSessionTargetBelongsToChild(r.Context(), sess.TargetID, childID)
		if err != nil {
			return err
		}
		if !primaryTargetOK {
			return security.ErrInvalidRelationship
		}

		scoreTargetIDs := make([]int64, len(scores))
		for i := range scores {
			scoreTargetIDs[i] = scores[i].TargetID
		}
		scoreTargetsOK, err := tx.LockSessionScoreTargetsBelongToChild(r.Context(), scoreTargetIDs, childID)
		if err != nil {
			return err
		}
		if !scoreTargetsOK {
			return security.ErrInvalidRelationship
		}

		if err := tx.CreateKeyworkSessionWithScores(r.Context(), sess, scores); err != nil {
			return err
		}
		scoreCount := len(scores)
		signed := sess.SignedByKeyWorker
		bound, err := tx.NewSessionAuditEvent(
			r.Context(),
			principal,
			sess.ID,
			storage.AuditEventSessionCreate,
			security.AuditMetadataFields{
				SessionType: security.SessionType(sessionType),
				ScoreCount:  &scoreCount,
				Signed:      &signed,
				RequestID:   chimw.GetReqID(r.Context()),
			},
			storage.AuditRouteSessions,
		)
		if err != nil {
			return err
		}
		_, err = tx.InsertAuditEvent(r.Context(), bound)
		return err
	})
	if err != nil {
		if errors.Is(err, security.ErrUnauthenticated) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if errors.Is(err, security.ErrForbidden) || errors.Is(err, security.ErrInvalidRelationship) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		writeServerError(w, err)
		return
	}

	resp := map[string]any{
		"session": sess,
		"scores":  scores,
	}
	writeJSON(w, http.StatusCreated, resp)
}

func validSessionScore(score int32) bool {
	return score >= 0 && score <= 4
}

func (h *TherapeuticHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}

	sessions, err := h.Repo.ListKeyworkSessionsByChild(r.Context(), childID)
	if err != nil {
		writeServerError(w, err)
		return
	}

	principal, err := security.RequirePrincipal(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	homeID, err := h.Repo.GetChildHomeID(r.Context(), childID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	manager, err := h.Repo.StaffHasAnyHomeRole(
		r.Context(), principal.StaffUserID(), homeID, security.RoleManager,
	)
	if err != nil {
		writeServerError(w, err)
		return
	}
	for _, session := range sessions {
		session.CanManagerReview = manager && session.SignedByKeyWorker &&
			session.KeyWorkerSignedAt != nil && session.ManagerReviewDecision == nil
	}

	writeJSON(w, http.StatusOK, sessions)
}

func (h *TherapeuticHandler) CreateManagerReview(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("therapeutic repo not initialised"))
		return
	}
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil || childID <= 0 {
		writeBadRequest(w, "invalid child id")
		return
	}
	sessionID, err := strconv.ParseInt(chi.URLParam(r, "sessionID"), 10, 64)
	if err != nil || sessionID <= 0 {
		writeBadRequest(w, "invalid session id")
		return
	}

	var req managerReviewRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeBadRequest(w, "invalid manager review payload")
		return
	}
	decision := models.ManagerReviewDecision(strings.TrimSpace(req.Decision))
	comment := strings.TrimSpace(req.Comment)
	if decision != models.ManagerReviewApproved && decision != models.ManagerReviewReturnedForReview {
		writeBadRequest(w, "decision must be approved or returned_for_review")
		return
	}
	if decision == models.ManagerReviewReturnedForReview && comment == "" {
		writeBadRequest(w, "a comment is required when returning for follow-up")
		return
	}
	if len(comment) > 4000 {
		writeBadRequest(w, "manager review comment is too long")
		return
	}

	review := &models.KeyworkManagerReview{Decision: decision}
	if comment != "" {
		review.Comment = &comment
	}
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(
			r.Context(), tx, childID, security.RoleManager,
		)
		if err != nil {
			return err
		}
		session, err := tx.LockKeyworkSessionForManagerReview(r.Context(), childID, sessionID)
		if err != nil {
			return err
		}
		if !session.SignedByKeyWorker || session.KeyWorkerSignedAt == nil {
			return storage.ErrSessionNotCompleted
		}
		if session.ManagerReviewDecision != nil {
			return storage.ErrSessionAlreadyManagerReviewed
		}
		review.ReviewerName = principal.DisplayName()
		if err := tx.CreateKeyworkManagerReview(
			r.Context(), session, review, principal.StaffUserID(),
		); err != nil {
			return err
		}
		bound, err := tx.NewSessionAuditEvent(
			r.Context(), principal, sessionID,
			storage.AuditEventSessionManagerReview,
			security.AuditMetadataFields{
				ManagerReviewDecision: security.ManagerReviewDecision(decision),
				RequestID:             chimw.GetReqID(r.Context()),
			},
			storage.AuditRouteSessionManagerReview,
		)
		if err != nil {
			return err
		}
		_, err = tx.InsertAuditEvent(r.Context(), bound)
		return err
	})
	if err != nil {
		switch {
		case errors.Is(err, security.ErrUnauthenticated):
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case errors.Is(err, security.ErrForbidden), errors.Is(err, security.ErrInvalidRelationship):
			http.Error(w, "forbidden", http.StatusForbidden)
		case errors.Is(err, storage.ErrSessionNotCompleted), errors.Is(err, storage.ErrSessionAlreadyManagerReviewed):
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		default:
			writeServerError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"manager_review": review})
}

func (h *TherapeuticHandler) Timeline(w http.ResponseWriter, r *http.Request) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}

	items, err := h.Repo.ListTimelineItemsForChild(r.Context(), childID)
	if err != nil {
		writeServerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, items)
}

// GET /v1/children/{childID}/domain_index?window_days=28
func (h *TherapeuticHandler) GetChildDomainIndex(w http.ResponseWriter, r *http.Request) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}

	windowDays := 28
	if q := r.URL.Query().Get("window_days"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			windowDays = v
		}
	}

	idx, err := h.Repo.GetChildDomainIndex(r.Context(), childID, windowDays)
	if err != nil {
		writeServerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, idx)
}

func (h *TherapeuticHandler) CreatePsychologicalReview(
	w http.ResponseWriter,
	r *http.Request,
) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}

	var req psychReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, "invalid json body")
		return
	}

	if req.ReviewFrom == "" || req.ReviewTo == "" {
		writeBadRequest(w, "review_from and review_to are required")
		return
	}

	from, err := time.Parse("2006-01-02", req.ReviewFrom)
	if err != nil {
		writeBadRequest(w, "invalid review_from date")
		return
	}
	to, err := time.Parse("2006-01-02", req.ReviewTo)
	if err != nil {
		writeBadRequest(w, "invalid review_to date")
		return
	}
	if from.After(to) {
		writeBadRequest(w, "review_from must not be after review_to")
		return
	}
	if to.Sub(from) > 365*24*time.Hour {
		writeBadRequest(w, "review period must not exceed 365 days")
		return
	}

	windowDays := req.WindowDays
	if windowDays <= 0 {
		windowDays = 42
	}
	if windowDays > 365 {
		writeBadRequest(w, "window_days must not exceed 365")
		return
	}

	rev := &models.PsychologicalReview{
		ChildID:           childID,
		WindowDays:        windowDays,
		ReviewFrom:        from,
		ReviewTo:          to,
		Formulation:       req.Formulation,
		ProtectiveFactors: req.ProtectiveFactors,
		Recommendations:   req.Recommendations,
	}

	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(
			r.Context(),
			tx,
			childID,
			security.RoleSenior,
			security.RoleManager,
		)
		if err != nil {
			return err
		}
		rev.CreatedBy = principal.DisplayName()
		if err := tx.CreatePsychologicalReview(r.Context(), rev); err != nil {
			return err
		}
		bound, err := tx.NewReviewAuditEvent(
			r.Context(),
			principal,
			rev.ID,
			storage.AuditEventReviewCreate,
			security.AuditMetadataFields{RequestID: chimw.GetReqID(r.Context())},
			storage.AuditRouteReviews,
		)
		if err != nil {
			return err
		}
		_, err = tx.InsertAuditEvent(r.Context(), bound)
		return err
	})
	if err != nil {
		switch {
		case errors.Is(err, security.ErrUnauthenticated):
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case errors.Is(err, security.ErrForbidden), errors.Is(err, security.ErrInvalidRelationship):
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			writeServerError(w, err)
		}
		return
	}

	writeJSON(w, http.StatusCreated, rev)
}

func (h *TherapeuticHandler) ListPsychologicalReviews(
	w http.ResponseWriter,
	r *http.Request,
) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}

	revs, err := h.Repo.ListPsychologicalReviewsByChild(r.Context(), childID)
	if err != nil {
		writeServerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, revs)
}

// -------------------------------
// DEMO SEED (REMOVE AFTER DEMO)
// POST /v1/children/{childID}/demo_seed
// -------------------------------

type demoSeedRequest struct {
	Days int `json:"days"` // default 60
}

// DemoSeedSessions creates ~60 days of sessions for the given child.
// It uses the same DB write path as CreateSessionWithScores, so your graphs work.
func (h *TherapeuticHandler) DemoSeedSessions(w http.ResponseWriter, r *http.Request) {
	// 1) Safety guards
	if os.Getenv("ALLOW_DEMO_SEED") != "true" {
		writeBadRequest(w, "demo seeding disabled (set ALLOW_DEMO_SEED=true)")
		return
	}
	// Simple shared secret header (set env DEMO_SEED_KEY)
	if r.Header.Get("X-Demo-Seed-Key") == "" ||
		r.Header.Get("X-Demo-Seed-Key") != os.Getenv("DEMO_SEED_KEY") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}

	// Parse optional body
	days := 60
	var req demoSeedRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Days > 0 && req.Days <= 180 {
			days = req.Days
		}
	}

	// 2) Load targets for this child (we need real target IDs from child_targets)
	targets, err := h.Repo.ListChildTargets(r.Context(), storage.TargetFilters{
		ChildID: childID,
	})
	if err != nil {
		writeServerError(w, err)
		return
	}
	if len(targets) < 3 {
		writeBadRequest(w, "child needs at least 3 targets before seeding")
		return
	}

	// Optional: prevent accidental double-seeding (keeps your demo clean)
	existing, err := h.Repo.ListKeyworkSessionsByChild(r.Context(), childID)
	if err == nil && len(existing) > 0 {
		writeBadRequest(w, "sessions already exist for this child — refusing to seed twice")
		return
	}

	// Pick first 3 targets (simple). If you want later, match by label/domain.
	t1 := targets[0].ID
	t2 := targets[1].ID
	t3 := targets[2].ID

	// 3) Generate sessions
	rand.Seed(time.Now().UnixNano())

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -(days - 1))

	alertKeywords := []string{"missing", "abscond", "damage", "aggressive"}

	// choose ~6 alert days spread through the window
	alertDayIdx := map[int]bool{}
	for len(alertDayIdx) < 6 && days >= 6 {
		alertDayIdx[rand.Intn(days)] = true
	}

	inserted := 0

	for i := 0; i < days; i++ {
		sessionAt := start.AddDate(0, 0, i)

		// Pattern:
		// - Target 2 has improve -> dip -> recover (nice for graphs)
		// - Target 1 improves gradually
		// - Target 3 improves mildly
		w1, p1 := genScore04(i, days, "improve")
		w2, p2 := genScore04(i, days, "dip_recover")
		w3, p3 := genScore04(i, days, "mild")

		notes := ""
		if alertDayIdx[i] {
			kw := alertKeywords[rand.Intn(len(alertKeywords))]
			notes = "ALERT KEYWORD: " + kw + ". Demo safeguarding note for graph/alert testing."
		} else if i%11 == 0 {
			notes = "Key-working: reviewed routine; agreed small step for tomorrow."
		}

		sess := &models.KeyworkSession{
			ChildID:   childID,
			SessionAt: sessionAt,
			StaffName: "Demo Seeder",
			Notes:     notes,
		}

		scores := []models.SessionScore{
			{
				TargetID:       t1,
				WellbeingScore: w1,
				ProgressScore:  p1,
				Comment:        "",
			},
			{
				TargetID:       t2,
				WellbeingScore: w2,
				ProgressScore:  p2,
				Comment:        "",
			},
			{
				TargetID:       t3,
				WellbeingScore: w3,
				ProgressScore:  p3,
				Comment:        "",
			},
		}

		if err := h.Repo.CreateKeyworkSessionWithScores(r.Context(), sess, scores); err != nil {
			writeServerError(w, err)
			return
		}
		inserted++
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":          true,
		"child_id":    childID,
		"seeded_days": days,
		"inserted":    inserted,
		"target_ids":  []int64{t1, t2, t3},
		"start_date":  start.Format("2006-01-02"),
		"end_date":    end.Format("2006-01-02"),
		"remove_hint": "Remove /v1/children/{childID}/demo_seed route + DemoSeedSessions handler after demo.",
	})
}

// genScore04 generates wellbeing/progress scores (0–4) with different patterns:
// "improve" => gradual improvement
// "dip_recover" => improves, dips mid-way, then recovers strongly
// "mild" => gentle improvement with noise
func genScore04(i, days int, pattern string) (int32, int32) {
	// base 0..4
	clamp := func(x int) int32 {
		if x < 0 {
			x = 0
		}
		if x > 4 {
			x = 4
		}
		return int32(x)
	}

	// noise -1..+1
	noise := func() int { return rand.Intn(3) - 1 }

	third := days / 3
	switch pattern {
	case "dip_recover":
		// first third: improve, middle: dip, last: recover
		if i < third {
			base := 1 + (i * 2 / max(1, third)) // 1->3
			return clamp(base + noise()), clamp(base + noise())
		} else if i < 2*third {
			base := 2 + noise() - 1 // dip around 1–2
			return clamp(base), clamp(base)
		} else {
			base := 2 + ((i - 2*third) * 2 / max(1, days-2*third)) // 2->4
			return clamp(base + noise()), clamp(base + noise())
		}

	case "mild":
		base := 1 + (i * 2 / max(1, days-1)) // 1->3
		return clamp(base + noise()), clamp(base + noise())

	default: // "improve"
		base := 1 + (i * 3 / max(1, days-1)) // 1->4
		return clamp(base + noise()), clamp(base + noise())
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
