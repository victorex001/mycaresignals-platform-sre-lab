package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
	"therapeutic-engine/internal/storage"
)

type RiskHandler struct {
	Repo *storage.Repo
}

func NewRiskHandler(repo *storage.Repo) *RiskHandler {
	return &RiskHandler{Repo: repo}
}

type riskPayload struct {
	Title               string  `json:"title"`
	Category            string  `json:"category"`
	Domain              string  `json:"domain"`
	Status              string  `json:"status"`
	NatureOfRisk        string  `json:"nature_of_risk"`
	WhoIsAtRisk         string  `json:"who_is_at_risk"`
	InformationStatus   string  `json:"information_status"`
	SourceNote          string  `json:"source_note"`
	InitialLikelihood   *int    `json:"initial_likelihood"`
	InitialSeverity     *int    `json:"initial_severity"`
	ResidualLikelihood  *int    `json:"residual_likelihood"`
	ResidualSeverity    *int    `json:"residual_severity"`
	ManagementApproach  string  `json:"management_approach"`
	ProactiveStrategies string  `json:"proactive_strategies"`
	ReactiveStrategies  string  `json:"reactive_strategies"`
	EffectiveFrom       *string `json:"effective_from"`
	NextReviewDate      *string `json:"next_review_date"`
	GoalIDs             []int64 `json:"goal_ids"`
	TargetIDs           []int64 `json:"target_ids"`
}

type riskReviewPayload struct {
	ReviewedResidualLikelihood *int    `json:"reviewed_residual_likelihood"`
	ReviewedResidualSeverity   *int    `json:"reviewed_residual_severity"`
	ReviewNote                 string  `json:"review_note"`
	Outcome                    string  `json:"outcome"`
	NextReviewDate             *string `json:"next_review_date"`
}

var errContinueChangesRating = errors.New("continue outcome must preserve the current residual rating; choose modify to change it")

func (h *RiskHandler) ListRisks(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("risk repo not initialised"))
		return
	}
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	risks, err := h.Repo.ListChildRisks(r.Context(), childID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	if risks == nil {
		risks = []models.ChildRisk{}
	}
	writeJSON(w, http.StatusOK, risks)
}

func (h *RiskHandler) ListRiskReviews(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("risk repo not initialised"))
		return
	}
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	riskID, err := resourceID(chi.URLParam(r, "riskID"))
	if err != nil {
		writeBadRequest(w, "invalid risk id")
		return
	}
	reviews, err := h.Repo.ListChildRiskReviews(r.Context(), childID, riskID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	if reviews == nil {
		reviews = []models.ChildRiskReview{}
	}
	writeJSON(w, http.StatusOK, reviews)
}

func (h *RiskHandler) CreateRiskReview(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("risk repo not initialised"))
		return
	}
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	riskID, err := resourceID(chi.URLParam(r, "riskID"))
	if err != nil {
		writeBadRequest(w, "invalid risk id")
		return
	}
	var payload riskReviewPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeBadRequest(w, "invalid json body")
		return
	}
	payload.ReviewNote = strings.TrimSpace(payload.ReviewNote)
	if payload.ReviewNote == "" {
		writeBadRequest(w, "review_note is required")
		return
	}
	if err := validateRiskRatingPair(payload.ReviewedResidualLikelihood, payload.ReviewedResidualSeverity, "reviewed residual"); err != nil || payload.ReviewedResidualLikelihood == nil {
		writeBadRequest(w, "reviewed residual likelihood and severity must both be between 1 and 5")
		return
	}
	outcome := models.RiskReviewOutcome(strings.TrimSpace(payload.Outcome))
	if outcome != models.RiskReviewContinue && outcome != models.RiskReviewModify && outcome != models.RiskReviewClose {
		writeBadRequest(w, "outcome must be continue, modify or close")
		return
	}
	nextReviewDate, err := optionalDate(payload.NextReviewDate)
	if err != nil {
		writeBadRequest(w, "next_review_date must be YYYY-MM-DD")
		return
	}
	if outcome == models.RiskReviewClose {
		nextReviewDate = nil
	}
	review := &models.ChildRiskReview{
		RiskID: riskID, ChildID: childID,
		ReviewedResidualLikelihood: *payload.ReviewedResidualLikelihood,
		ReviewedResidualSeverity:   *payload.ReviewedResidualSeverity,
		ReviewNote:                 payload.ReviewNote, Outcome: outcome, NextReviewDate: nextReviewDate,
	}
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(
			r.Context(), tx, childID, security.RoleSenior, security.RoleManager,
		)
		if err != nil {
			return err
		}
		current, err := tx.LockChildRiskForReview(r.Context(), riskID, childID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return security.ErrInvalidRelationship
			}
			return err
		}
		if outcome == models.RiskReviewContinue && !sameRiskRating(current.ResidualLikelihood, current.ResidualSeverity, review.ReviewedResidualLikelihood, review.ReviewedResidualSeverity) {
			return errContinueChangesRating
		}
		review.ReviewerStaffID = principal.StaffUserID()
		review.ReviewerDisplayName = principal.DisplayName()
		review.PreviousResidualLikelihood = current.ResidualLikelihood
		review.PreviousResidualSeverity = current.ResidualSeverity
		if err := tx.CreateChildRiskReview(r.Context(), review); err != nil {
			return err
		}
		if err := tx.UpdateRiskFromReview(r.Context(), review); err != nil {
			return err
		}
		fields := security.AuditMetadataFields{RiskReviewID: &review.ID}
		if outcome == models.RiskReviewClose {
			fields.NewStatus = security.AuditStatusClosed
		}
		return writeRiskAudit(r, tx, principal, riskID, storage.AuditEventRiskReviewCreate, fields, storage.AuditRouteRiskReviews)
	})
	if err != nil {
		if errors.Is(err, errContinueChangesRating) {
			writeBadRequest(w, err.Error())
			return
		}
		writeRiskMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, review)
}

func sameRiskRating(currentLikelihood, currentSeverity *int, reviewedLikelihood, reviewedSeverity int) bool {
	return currentLikelihood != nil && currentSeverity != nil &&
		*currentLikelihood == reviewedLikelihood && *currentSeverity == reviewedSeverity
}

func (h *RiskHandler) CreateRisk(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("risk repo not initialised"))
		return
	}
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	risk, err := decodeRiskPayload(r, childID, 0)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(
			r.Context(), tx, childID, security.RoleSenior, security.RoleManager,
		)
		if err != nil {
			return err
		}
		valid, err := tx.LockRiskPlanningLinksBelongToChild(r.Context(), childID, risk.GoalIDs, risk.TargetIDs)
		if err != nil {
			return err
		}
		if !valid {
			return security.ErrInvalidRelationship
		}
		if err := tx.CreateChildRisk(r.Context(), risk); err != nil {
			return err
		}
		if err := writeRiskAudit(r, tx, principal, risk.ID, storage.AuditEventRiskCreate, security.AuditMetadataFields{
			NewStatus: security.AuditStatus(risk.Status),
		}); err != nil {
			return err
		}
		for _, goalID := range risk.GoalIDs {
			if err := tx.AddRiskGoalLink(r.Context(), risk.ID, childID, goalID); err != nil {
				return err
			}
			if err := writeRiskAudit(r, tx, principal, risk.ID, storage.AuditEventRiskGoalLink, security.AuditMetadataFields{GoalID: &goalID}); err != nil {
				return err
			}
		}
		for _, targetID := range risk.TargetIDs {
			if err := tx.AddRiskTargetLink(r.Context(), risk.ID, childID, targetID); err != nil {
				return err
			}
			if err := writeRiskAudit(r, tx, principal, risk.ID, storage.AuditEventRiskTargetLink, security.AuditMetadataFields{TargetID: &targetID}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		writeRiskMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, risk)
}

func (h *RiskHandler) UpdateRisk(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("risk repo not initialised"))
		return
	}
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	riskID, err := resourceID(chi.URLParam(r, "riskID"))
	if err != nil {
		writeBadRequest(w, "invalid risk id")
		return
	}
	risk, err := decodeRiskPayload(r, childID, riskID)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(
			r.Context(), tx, childID, security.RoleSenior, security.RoleManager,
		)
		if err != nil {
			return err
		}
		belongs, err := tx.LockRiskBelongsToChild(r.Context(), riskID, childID)
		if err != nil {
			return err
		}
		if !belongs {
			return security.ErrInvalidRelationship
		}
		valid, err := tx.LockRiskPlanningLinksBelongToChild(r.Context(), childID, risk.GoalIDs, risk.TargetIDs)
		if err != nil {
			return err
		}
		if !valid {
			return security.ErrInvalidRelationship
		}
		oldGoals, oldTargets, err := tx.ListRiskPlanningLinkIDs(r.Context(), riskID)
		if err != nil {
			return err
		}
		if err := tx.UpdateChildRisk(r.Context(), risk); err != nil {
			return err
		}
		if err := writeRiskAudit(r, tx, principal, risk.ID, storage.AuditEventRiskUpdate, security.AuditMetadataFields{
			NewStatus: security.AuditStatus(risk.Status),
		}); err != nil {
			return err
		}
		if err := syncRiskGoalLinks(r, tx, principal, risk, oldGoals); err != nil {
			return err
		}
		return syncRiskTargetLinks(r, tx, principal, risk, oldTargets)
	})
	if err != nil {
		writeRiskMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, risk)
}

func decodeRiskPayload(r *http.Request, childID, riskID int64) (*models.ChildRisk, error) {
	var payload riskPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, errors.New("invalid json body")
	}
	payload.Title = strings.TrimSpace(payload.Title)
	payload.Category = strings.TrimSpace(payload.Category)
	payload.NatureOfRisk = strings.TrimSpace(payload.NatureOfRisk)
	if payload.Title == "" || payload.Category == "" || payload.NatureOfRisk == "" {
		return nil, errors.New("title, category and nature_of_risk are required")
	}
	status := models.RiskStatus(strings.TrimSpace(payload.Status))
	if status == "" {
		status = models.RiskStatusActive
	}
	if status != models.RiskStatusActive && status != models.RiskStatusPaused && status != models.RiskStatusClosed {
		return nil, errors.New("status must be active, paused or closed")
	}
	informationStatus := models.RiskInformationStatus(strings.TrimSpace(payload.InformationStatus))
	switch informationStatus {
	case models.RiskInformationCurrentConfirmed, models.RiskInformationReported,
		models.RiskInformationHistorical, models.RiskInformationAllegationNotEstablished,
		models.RiskInformationAwaitingConfirmation:
	default:
		return nil, errors.New("invalid information_status")
	}
	management := models.RiskManagementApproach(strings.TrimSpace(payload.ManagementApproach))
	if management != models.RiskControlsOnly && management != models.RiskControlsAndDevelopmentalWork {
		return nil, errors.New("invalid management_approach")
	}
	if err := validateRiskRatingPair(payload.InitialLikelihood, payload.InitialSeverity, "initial"); err != nil {
		return nil, err
	}
	if err := validateRiskRatingPair(payload.ResidualLikelihood, payload.ResidualSeverity, "residual"); err != nil {
		return nil, err
	}
	effectiveFrom, err := optionalDate(payload.EffectiveFrom)
	if err != nil {
		return nil, errors.New("effective_from must be YYYY-MM-DD")
	}
	nextReview, err := optionalDate(payload.NextReviewDate)
	if err != nil {
		return nil, errors.New("next_review_date must be YYYY-MM-DD")
	}
	goalIDs, err := validateRiskIDs(payload.GoalIDs)
	if err != nil {
		return nil, errors.New("goal_ids must contain positive identifiers")
	}
	targetIDs, err := validateRiskIDs(payload.TargetIDs)
	if err != nil {
		return nil, errors.New("target_ids must contain positive identifiers")
	}
	return &models.ChildRisk{
		ID: riskID, ChildID: childID, Title: payload.Title, Category: payload.Category,
		Domain: optionalTrimmed(payload.Domain), Status: status, NatureOfRisk: payload.NatureOfRisk,
		WhoIsAtRisk: optionalTrimmed(payload.WhoIsAtRisk), InformationStatus: informationStatus,
		SourceNote: optionalTrimmed(payload.SourceNote), InitialLikelihood: payload.InitialLikelihood,
		InitialSeverity: payload.InitialSeverity, ResidualLikelihood: payload.ResidualLikelihood,
		ResidualSeverity: payload.ResidualSeverity, ManagementApproach: management,
		ProactiveStrategies: optionalTrimmed(payload.ProactiveStrategies),
		ReactiveStrategies:  optionalTrimmed(payload.ReactiveStrategies), EffectiveFrom: effectiveFrom,
		NextReviewDate: nextReview, GoalIDs: goalIDs, TargetIDs: targetIDs,
	}, nil
}

func validateRiskRatingPair(likelihood, severity *int, label string) error {
	if (likelihood == nil) != (severity == nil) {
		return errors.New(label + " likelihood and severity must both be set or both be unknown")
	}
	for _, rating := range []*int{likelihood, severity} {
		if rating != nil && (*rating < 1 || *rating > 5) {
			return errors.New(label + " likelihood and severity must be between 1 and 5")
		}
	}
	return nil
}

func validateRiskIDs(ids []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, security.ErrInvalidRelationship
		}
		seen[id] = struct{}{}
	}
	out := make([]int64, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func syncRiskGoalLinks(r *http.Request, tx *storage.TxRepo, principal security.Principal, risk *models.ChildRisk, oldIDs []int64) error {
	for _, id := range removedIDs(oldIDs, risk.GoalIDs) {
		if err := tx.DeleteRiskGoalLink(r.Context(), risk.ID, id); err != nil {
			return err
		}
		if err := writeRiskAudit(r, tx, principal, risk.ID, storage.AuditEventRiskGoalUnlink, security.AuditMetadataFields{GoalID: &id}); err != nil {
			return err
		}
	}
	for _, id := range removedIDs(risk.GoalIDs, oldIDs) {
		if err := tx.AddRiskGoalLink(r.Context(), risk.ID, risk.ChildID, id); err != nil {
			return err
		}
		if err := writeRiskAudit(r, tx, principal, risk.ID, storage.AuditEventRiskGoalLink, security.AuditMetadataFields{GoalID: &id}); err != nil {
			return err
		}
	}
	return nil
}

func syncRiskTargetLinks(r *http.Request, tx *storage.TxRepo, principal security.Principal, risk *models.ChildRisk, oldIDs []int64) error {
	for _, id := range removedIDs(oldIDs, risk.TargetIDs) {
		if err := tx.DeleteRiskTargetLink(r.Context(), risk.ID, id); err != nil {
			return err
		}
		if err := writeRiskAudit(r, tx, principal, risk.ID, storage.AuditEventRiskTargetUnlink, security.AuditMetadataFields{TargetID: &id}); err != nil {
			return err
		}
	}
	for _, id := range removedIDs(risk.TargetIDs, oldIDs) {
		if err := tx.AddRiskTargetLink(r.Context(), risk.ID, risk.ChildID, id); err != nil {
			return err
		}
		if err := writeRiskAudit(r, tx, principal, risk.ID, storage.AuditEventRiskTargetLink, security.AuditMetadataFields{TargetID: &id}); err != nil {
			return err
		}
	}
	return nil
}

func removedIDs(from, keep []int64) []int64 {
	kept := make(map[int64]struct{}, len(keep))
	for _, id := range keep {
		kept[id] = struct{}{}
	}
	out := make([]int64, 0)
	for _, id := range from {
		if _, ok := kept[id]; !ok {
			out = append(out, id)
		}
	}
	return out
}

func writeRiskAudit(r *http.Request, tx *storage.TxRepo, principal security.Principal, riskID int64, event storage.AuditEventType, fields security.AuditMetadataFields, routes ...storage.AuditRouteTemplate) error {
	fields.RequestID = chimw.GetReqID(r.Context())
	route := storage.AuditRouteRisks
	if len(routes) == 1 {
		route = routes[0]
	}
	bound, err := tx.NewRiskAuditEvent(r.Context(), principal, riskID, event, fields, route)
	if err != nil {
		return err
	}
	_, err = tx.InsertAuditEvent(r.Context(), bound)
	return err
}

func writeRiskMutationError(w http.ResponseWriter, err error) {
	if errors.Is(err, security.ErrUnauthenticated) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if errors.Is(err, security.ErrForbidden) || errors.Is(err, security.ErrInvalidRelationship) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeServerError(w, err)
}

func riskScore(likelihood, severity *int) (int, string, bool) {
	if likelihood == nil || severity == nil {
		return 0, "", false
	}
	score := *likelihood * *severity
	band := "high"
	if score <= 4 {
		band = "low"
	} else if score <= 9 {
		band = "medium"
	}
	return score, band, true
}
