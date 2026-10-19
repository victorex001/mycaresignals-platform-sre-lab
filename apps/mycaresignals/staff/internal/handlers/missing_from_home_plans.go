package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
	"therapeutic-engine/internal/storage"
)

type MissingFromHomePlanHandler struct {
	Repo *storage.Repo
}

func NewMissingFromHomePlanHandler(repo *storage.Repo) *MissingFromHomePlanHandler {
	return &MissingFromHomePlanHandler{Repo: repo}
}

type mfhPlanSavePayload struct {
	Content         models.MissingFromHomePlanContent `json:"content"`
	SourceDecisions []models.MFHSourceDecision        `json:"source_decisions"`
	ReviewDate      *string                           `json:"review_date"`
}

type mfhWorkspaceResponse struct {
	Plan            *models.MissingFromHomePlan  `json:"plan"`
	History         []models.MissingFromHomePlan `json:"history"`
	ReuseCandidates []models.MFHReuseCandidate   `json:"reuse_candidates"`
	CanEdit         bool                         `json:"can_edit"`
	CanApprove      bool                         `json:"can_approve"`
}

func validMFHInformationStatus(status models.MFHInformationStatus) bool {
	switch status {
	case models.MFHInformationKnownConfirmed, models.MFHInformationPartlyKnown,
		models.MFHInformationAwaitingConfirmation, models.MFHInformationNotKnownYet:
		return true
	default:
		return false
	}
}

func validMFHHistoryType(value string) bool {
	switch value {
	case "none_known", "previous_episodes", "frequent_repeated", "awaiting_information":
		return true
	default:
		return false
	}
}

func validMFHPersonPlaceKind(value string) bool {
	switch value {
	case "family_contact", "friend_peer", "previous_area", "known_location", "travel_route", "other_place":
		return true
	default:
		return false
	}
}

func mfhTextValid(value string, maximum int) bool {
	return len(value) <= maximum
}

func validateMFHContent(content models.MissingFromHomePlanContent, forSubmission bool) error {
	texts := []string{
		content.CurrentSituation.SupervisionArrangement, content.CurrentSituation.FreeTimeArrangement,
		content.CurrentSituation.ConfirmedLegalContext, content.CurrentSituation.CurrentRiskContext,
		content.MissingHistory.Patterns, content.MissingHistory.Circumstances,
		content.MissingHistory.WhatHelpedReturn, content.MissingHistory.Source,
		content.Operational.ImmediateChecks, content.Operational.ContactAttempts,
		content.Operational.LocalSearchExpectations, content.Operational.PolicePolicyWording,
		content.Operational.InformationForPolice, content.Operational.SafeguardingConsiderations,
		content.ReturnFollowUp.ImmediateWelfareResponse, content.ReturnFollowUp.MedicalSafeguardingCheck,
		content.ReturnFollowUp.CalmWelcomeRecoveryApproach, content.ReturnFollowUp.ReturnConversation,
		content.ReturnFollowUp.IndependentReturnInterview, content.ReturnFollowUp.ProfessionalNotifications,
		content.ReturnFollowUp.KeyWorkingFollowUp,
	}
	for _, value := range texts {
		if !mfhTextValid(value, 6000) {
			return errors.New("Missing From Home plan text is too long")
		}
	}
	if len(content.PeoplePlaces) > 50 || len(content.Triggers) > 50 || len(content.Notifications) > 50 {
		return errors.New("Missing From Home plan contains too many repeated items")
	}
	if content.MissingHistory.InformationStatus != "" && !validMFHInformationStatus(content.MissingHistory.InformationStatus) {
		return errors.New("invalid missing history information status")
	}
	if content.MissingHistory.HistoryType != "" && !validMFHHistoryType(content.MissingHistory.HistoryType) {
		return errors.New("invalid missing history type")
	}
	for _, item := range content.PeoplePlaces {
		if !validMFHPersonPlaceKind(item.Kind) || !validMFHInformationStatus(item.InformationStatus) ||
			!mfhTextValid(item.NameOrLabel, 500) || !mfhTextValid(item.Details, 3000) ||
			!mfhTextValid(item.ContactOrRoute, 2000) || !mfhTextValid(item.Source, 1000) {
			return errors.New("invalid people and places item")
		}
	}
	for _, item := range content.Triggers {
		if !validMFHInformationStatus(item.InformationStatus) || !mfhTextValid(item.Description, 3000) || !mfhTextValid(item.Source, 1000) {
			return errors.New("invalid trigger item")
		}
	}
	for _, item := range content.Notifications {
		if !mfhTextValid(item.Role, 500) || !mfhTextValid(item.ContactName, 500) ||
			!mfhTextValid(item.ContactDetails, 2000) || !mfhTextValid(item.WhenToNotify, 2000) || !mfhTextValid(item.Source, 1000) {
			return errors.New("invalid notification item")
		}
	}
	if !forSubmission {
		return nil
	}
	if strings.TrimSpace(content.CurrentSituation.SupervisionArrangement) == "" ||
		!validMFHHistoryType(content.MissingHistory.HistoryType) ||
		!validMFHInformationStatus(content.MissingHistory.InformationStatus) ||
		strings.TrimSpace(content.Operational.ImmediateChecks) == "" ||
		strings.TrimSpace(content.Operational.ContactAttempts) == "" ||
		strings.TrimSpace(content.Operational.LocalSearchExpectations) == "" ||
		strings.TrimSpace(content.Operational.PolicePolicyWording) == "" ||
		strings.TrimSpace(content.Operational.InformationForPolice) == "" ||
		strings.TrimSpace(content.ReturnFollowUp.ImmediateWelfareResponse) == "" ||
		strings.TrimSpace(content.ReturnFollowUp.CalmWelcomeRecoveryApproach) == "" ||
		strings.TrimSpace(content.ReturnFollowUp.ProfessionalNotifications) == "" {
		return errors.New("complete the required Missing From Home planning sections before submission")
	}
	return nil
}

func (h *MissingFromHomePlanHandler) workspacePermissions(r *http.Request, childID int64) (bool, bool, error) {
	principal, err := security.RequirePrincipal(r.Context())
	if err != nil {
		return false, false, err
	}
	homeID, err := h.Repo.GetChildHomeID(r.Context(), childID)
	if err != nil {
		return false, false, err
	}
	canEdit, err := h.Repo.StaffHasAnyHomeRole(r.Context(), principal.StaffUserID(), homeID,
		security.RoleKeyWorker, security.RoleSenior, security.RoleManager)
	if err != nil {
		return false, false, err
	}
	canApprove, err := h.Repo.StaffHasAnyHomeRole(r.Context(), principal.StaffUserID(), homeID, security.RoleManager)
	return canEdit, canApprove, err
}

func (h *MissingFromHomePlanHandler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	plan, err := h.Repo.GetCurrentMFHPlan(r.Context(), childID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	history, err := h.Repo.ListMFHPlans(r.Context(), childID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	candidates, err := h.Repo.ListMFHReuseCandidates(r.Context(), childID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	canEdit, canApprove, err := h.workspacePermissions(r, childID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mfhWorkspaceResponse{Plan: plan, History: history, ReuseCandidates: candidates, CanEdit: canEdit, CanApprove: canApprove})
}

func (h *MissingFromHomePlanHandler) GetPlan(w http.ResponseWriter, r *http.Request) {
	childID, planID, err := mfhRouteIDs(r)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	plan, err := h.Repo.GetMFHPlan(r.Context(), childID, planID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	if plan == nil {
		http.Error(w, "Missing From Home plan not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
}

func (h *MissingFromHomePlanHandler) ListReuseCandidates(w http.ResponseWriter, r *http.Request) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	items, err := h.Repo.ListMFHReuseCandidates(r.Context(), childID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *MissingFromHomePlanHandler) CreateDraft(w http.ResponseWriter, r *http.Request) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	var plan *models.MissingFromHomePlan
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(r.Context(), tx, childID,
			security.RoleKeyWorker, security.RoleSenior, security.RoleManager)
		if err != nil {
			return err
		}
		plan, err = tx.CreateMFHDraft(r.Context(), childID, principal.StaffUserID())
		return err
	})
	if writeMFHError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"plan": plan})
}

func (h *MissingFromHomePlanHandler) SaveDraft(w http.ResponseWriter, r *http.Request) {
	childID, planID, err := mfhRouteIDs(r)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var payload mfhPlanSavePayload
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeBadRequest(w, "invalid Missing From Home plan payload")
		return
	}
	if err := validateMFHContent(payload.Content, false); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	reviewDate, err := optionalDate(payload.ReviewDate)
	if err != nil {
		writeBadRequest(w, "review_date must be YYYY-MM-DD")
		return
	}
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, _, err := security.RequireChildHomeRoleForMutation(r.Context(), tx, childID,
			security.RoleKeyWorker, security.RoleSenior, security.RoleManager)
		if err != nil {
			return err
		}
		plan, err := tx.LockMFHPlan(r.Context(), childID, planID)
		if err != nil {
			return err
		}
		if plan.Status != models.MFHPlanDraft {
			return storage.ErrMFHPlanNotDraft
		}
		snapshots, err := tx.MaterializeMFHSourceDecisions(r.Context(), childID, payload.SourceDecisions)
		if err != nil {
			return err
		}
		plan.Content, plan.SourceSnapshots, plan.ReviewDate = payload.Content, snapshots, reviewDate
		return tx.UpdateMFHDraft(r.Context(), plan)
	})
	if writeMFHError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *MissingFromHomePlanHandler) Submit(w http.ResponseWriter, r *http.Request) {
	childID, planID, err := mfhRouteIDs(r)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(r.Context(), tx, childID,
			security.RoleKeyWorker, security.RoleSenior, security.RoleManager)
		if err != nil {
			return err
		}
		plan, err := tx.LockMFHPlan(r.Context(), childID, planID)
		if err != nil {
			return err
		}
		if err := validateMFHContent(plan.Content, true); err != nil {
			return err
		}
		if plan.ReviewDate == nil {
			return errors.New("review date is required before submission")
		}
		return tx.SubmitMFHPlan(r.Context(), plan, principal.StaffUserID())
	})
	if writeMFHError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *MissingFromHomePlanHandler) Approve(w http.ResponseWriter, r *http.Request) {
	childID, planID, err := mfhRouteIDs(r)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(r.Context(), tx, childID, security.RoleManager)
		if err != nil {
			return err
		}
		plan, err := tx.LockMFHPlan(r.Context(), childID, planID)
		if err != nil {
			return err
		}
		return tx.ApproveMFHPlan(r.Context(), plan, principal.StaffUserID())
	})
	if writeMFHError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *MissingFromHomePlanHandler) Revise(w http.ResponseWriter, r *http.Request) {
	childID, planID, err := mfhRouteIDs(r)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var successor *models.MissingFromHomePlan
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(r.Context(), tx, childID,
			security.RoleKeyWorker, security.RoleSenior, security.RoleManager)
		if err != nil {
			return err
		}
		plan, err := tx.LockMFHPlan(r.Context(), childID, planID)
		if err != nil {
			return err
		}
		successor, err = tx.CreateMFHRevision(r.Context(), plan, principal.StaffUserID())
		return err
	})
	if writeMFHError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"plan": successor})
}

func (h *MissingFromHomePlanHandler) Archive(w http.ResponseWriter, r *http.Request) {
	childID, planID, err := mfhRouteIDs(r)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(r.Context(), tx, childID, security.RoleManager)
		if err != nil {
			return err
		}
		plan, err := tx.LockMFHPlan(r.Context(), childID, planID)
		if err != nil {
			return err
		}
		return tx.ArchiveMFHPlan(r.Context(), plan, principal.StaffUserID())
	})
	if writeMFHError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func mfhRouteIDs(r *http.Request) (int64, int64, error) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		return 0, 0, errors.New("invalid child id")
	}
	planID, err := parseChildID(chi.URLParam(r, "planID"))
	if err != nil {
		return 0, 0, errors.New("invalid plan id")
	}
	return childID, planID, nil
}

func writeMFHError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, security.ErrUnauthenticated):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	case errors.Is(err, security.ErrForbidden), errors.Is(err, security.ErrInvalidRelationship), errors.Is(err, sql.ErrNoRows):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, storage.ErrMFHOpenPlanExists), errors.Is(err, storage.ErrMFHPlanNotDraft),
		errors.Is(err, storage.ErrMFHPlanNotSubmitted), errors.Is(err, storage.ErrMFHPlanNotApproved):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, storage.ErrMFHUnknownSource):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "complete the") {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, "Missing From Home plan operation failed", http.StatusInternalServerError)
		}
	}
	return true
}
