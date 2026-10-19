package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
	"therapeutic-engine/internal/storage"
)

var (
	errCarePlanArchived      = errors.New("archived care plans cannot be changed")
	errCarePlanNotDraft      = errors.New("only a draft care plan can be activated")
	errOtherActiveCarePlan   = errors.New("the child already has an active care plan")
	errCarePlanInvalidStatus = errors.New("invalid care plan information status")
	errCarePlanNotActive     = errors.New("only the active care plan can be reviewed")
	errPredecessorNotActive  = errors.New("the predecessor is no longer the active care plan")
)

type CarePlanHandler struct {
	Repo *storage.Repo
}

func NewCarePlanHandler(repo *storage.Repo) *CarePlanHandler {
	return &CarePlanHandler{Repo: repo}
}

type createCarePlanPayload struct {
	EffectiveFrom *string `json:"effective_from"`
	ReviewDate    *string `json:"review_date"`
}

type carePlanSectionPayload struct {
	InformationStatus          string `json:"information_status"`
	CurrentUnderstanding       string `json:"current_understanding"`
	Strengths                  string `json:"strengths"`
	StaffSupport               string `json:"staff_support"`
	YoungPersonInvolvementNote string `json:"young_person_involvement_note"`
}

type carePlanReviewPayload struct {
	ReviewNote                 string  `json:"review_note"`
	YoungPersonInvolvementNote string  `json:"young_person_involvement_note"`
	Outcome                    string  `json:"outcome"`
	NextReviewDate             *string `json:"next_review_date"`
}

type carePlanAccessResponse struct {
	CanEdit   bool `json:"can_edit"`
	CanReview bool `json:"can_review"`
}

func (h *CarePlanHandler) GetCarePlanAccess(w http.ResponseWriter, r *http.Request) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
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
	canEdit, err := h.Repo.StaffHasAnyHomeRole(r.Context(), principal.StaffUserID(), homeID,
		security.RoleSenior, security.RoleManager)
	if err != nil {
		writeServerError(w, err)
		return
	}
	canReview, err := h.Repo.StaffHasAnyHomeRole(r.Context(), principal.StaffUserID(), homeID, security.RoleManager)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, carePlanAccessResponse{CanEdit: canEdit, CanReview: canReview})
}

func (h *CarePlanHandler) GetCurrentCarePlan(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("care plan repo not initialised"))
		return
	}
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	plan, err := h.Repo.GetCurrentChildCarePlan(r.Context(), childID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	if plan == nil {
		writeJSON(w, http.StatusOK, map[string]any{"plan": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
}

func (h *CarePlanHandler) GetCarePlan(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("care plan repo not initialised"))
		return
	}
	childID, planID, _, err := carePlanRouteIDs(r)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	plan, err := h.Repo.GetChildCarePlan(r.Context(), childID, planID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	if plan == nil {
		http.Error(w, "care plan not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
}

func (h *CarePlanHandler) ListCarePlans(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("care plan repo not initialised"))
		return
	}
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	plans, err := h.Repo.ListChildCarePlans(r.Context(), childID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	if plans == nil {
		plans = []models.ChildCarePlan{}
	}
	writeJSON(w, http.StatusOK, plans)
}

func (h *CarePlanHandler) ListCarePlanReviews(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("care plan repo not initialised"))
		return
	}
	childID, planID, _, err := carePlanRouteIDs(r)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	reviews, err := h.Repo.ListChildCarePlanReviews(r.Context(), childID, planID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	if reviews == nil {
		reviews = []models.ChildCarePlanReview{}
	}
	writeJSON(w, http.StatusOK, reviews)
}

func (h *CarePlanHandler) GetCarePlanReviewEvidence(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("care plan repo not initialised"))
		return
	}
	childID, planID, _, err := carePlanRouteIDs(r)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	plan, err := h.Repo.GetChildCarePlan(r.Context(), childID, planID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	if plan == nil {
		http.Error(w, "care plan not found", http.StatusNotFound)
		return
	}
	evidence, err := h.Repo.GetCarePlanReviewEvidence(r.Context(), plan)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, evidence)
}

func (h *CarePlanHandler) CreateCarePlanReview(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("care plan repo not initialised"))
		return
	}
	childID, planID, _, err := carePlanRouteIDs(r)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var payload carePlanReviewPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeBadRequest(w, "invalid json body")
		return
	}
	payload.ReviewNote = strings.TrimSpace(payload.ReviewNote)
	if payload.ReviewNote == "" {
		writeBadRequest(w, "review_note is required")
		return
	}
	outcome := models.CarePlanReviewOutcome(strings.TrimSpace(payload.Outcome))
	if outcome != models.CarePlanReviewContinue && outcome != models.CarePlanReviewRevise && outcome != models.CarePlanReviewClose {
		writeBadRequest(w, "outcome must be continue, revise or close")
		return
	}
	nextReviewDate, err := optionalDate(payload.NextReviewDate)
	if err != nil {
		writeBadRequest(w, "next_review_date must be YYYY-MM-DD")
		return
	}
	if outcome == models.CarePlanReviewContinue && nextReviewDate == nil {
		writeBadRequest(w, "next_review_date is required when continuing the current plan")
		return
	}
	if outcome == models.CarePlanReviewClose {
		nextReviewDate = nil
	}
	review := &models.ChildCarePlanReview{
		CarePlanID: planID, ChildID: childID, ReviewNote: payload.ReviewNote,
		YoungPersonInvolvementNote: optionalTrimmed(payload.YoungPersonInvolvementNote),
		Outcome:                    outcome, NextReviewDate: nextReviewDate,
	}
	var successor *models.ChildCarePlan
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(r.Context(), tx, childID, security.RoleManager)
		if err != nil {
			return err
		}
		plan, err := tx.LockChildCarePlan(r.Context(), planID, childID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return security.ErrInvalidRelationship
			}
			return err
		}
		if plan.Status != models.CarePlanStatusActive {
			return errCarePlanNotActive
		}
		sections, err := tx.ListChildCarePlanSectionsForUpdate(r.Context(), plan.ID)
		if err != nil {
			return err
		}
		evidence, err := tx.GetCarePlanReviewEvidence(r.Context(), plan)
		if err != nil {
			return err
		}
		review.ReviewerStaffID = principal.StaffUserID()
		review.ReviewerDisplayName = principal.DisplayName()
		review.SnapshotStatus = plan.Status
		review.SnapshotEffectiveFrom = plan.EffectiveFrom
		review.SnapshotReviewDate = plan.ReviewDate
		review.Evidence = evidence
		if outcome == models.CarePlanReviewRevise {
			successor, err = tx.CreateSuccessorChildCarePlan(r.Context(), plan, sections, principal.StaffUserID(), nextReviewDate)
			if err != nil {
				return err
			}
			review.SuccessorCarePlanID = &successor.ID
		}
		if err := tx.CreateChildCarePlanReview(r.Context(), review, sections); err != nil {
			return err
		}
		if outcome == models.CarePlanReviewContinue {
			if err := tx.UpdateChildCarePlanReviewDate(r.Context(), plan, nextReviewDate); err != nil {
				return err
			}
		}
		if outcome == models.CarePlanReviewClose {
			previous := plan.Status
			if err := tx.ArchiveChildCarePlan(r.Context(), plan, principal.StaffUserID()); err != nil {
				return err
			}
			if err := writeCarePlanAudit(r, tx, principal, plan.ID, storage.AuditEventCarePlanArchive,
				security.AuditMetadataFields{PreviousStatus: security.AuditStatus(previous), NewStatus: security.AuditStatusArchived}, storage.AuditRouteCarePlanReviews); err != nil {
				return err
			}
		}
		if successor != nil {
			if err := writeCarePlanAudit(r, tx, principal, successor.ID, storage.AuditEventCarePlanSuccessorCreate,
				security.AuditMetadataFields{NewStatus: security.AuditStatusDraft}, storage.AuditRouteCarePlanReviews); err != nil {
				return err
			}
		}
		return writeCarePlanAudit(r, tx, principal, plan.ID, storage.AuditEventCarePlanReviewCreate,
			security.AuditMetadataFields{CarePlanReviewID: &review.ID}, storage.AuditRouteCarePlanReviews)
	})
	if err != nil {
		writeCarePlanMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"review": review, "successor_plan": successor})
}

func (h *CarePlanHandler) CreateCarePlan(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("care plan repo not initialised"))
		return
	}
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	var payload createCarePlanPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeBadRequest(w, "invalid json body")
		return
	}
	effectiveFrom, err := optionalDate(payload.EffectiveFrom)
	if err != nil {
		writeBadRequest(w, "effective_from must be YYYY-MM-DD")
		return
	}
	if effectiveFrom == nil {
		today := time.Now().UTC().Truncate(24 * time.Hour)
		effectiveFrom = &today
	}
	reviewDate, err := optionalDate(payload.ReviewDate)
	if err != nil {
		writeBadRequest(w, "review_date must be YYYY-MM-DD")
		return
	}
	plan := &models.ChildCarePlan{
		ChildID: childID, Status: models.CarePlanStatusDraft,
		EffectiveFrom: *effectiveFrom, ReviewDate: reviewDate,
	}
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(
			r.Context(), tx, childID, security.RoleSenior, security.RoleManager,
		)
		if err != nil {
			return err
		}
		plan.CreatedByStaffID = principal.StaffUserID()
		if err := tx.CreateChildCarePlan(r.Context(), plan); err != nil {
			return err
		}
		return writeCarePlanAudit(r, tx, principal, plan.ID, storage.AuditEventCarePlanCreate,
			security.AuditMetadataFields{NewStatus: security.AuditStatusDraft}, storage.AuditRouteCarePlans)
	})
	if err != nil {
		writeCarePlanMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"plan": plan})
}

func (h *CarePlanHandler) UpdateCarePlanSection(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("care plan repo not initialised"))
		return
	}
	childID, planID, key, err := carePlanRouteIDs(r)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var payload carePlanSectionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeBadRequest(w, "invalid json body")
		return
	}
	status := models.CarePlanInformationStatus(strings.TrimSpace(payload.InformationStatus))
	if !validCarePlanInformationStatus(status) {
		writeBadRequest(w, errCarePlanInvalidStatus.Error())
		return
	}
	section := &models.ChildCarePlanSection{
		CarePlanID: planID, SectionKey: key, InformationStatus: status,
		CurrentUnderstanding: optionalTrimmed(payload.CurrentUnderstanding),
		Strengths:            optionalTrimmed(payload.Strengths), StaffSupport: optionalTrimmed(payload.StaffSupport),
		YoungPersonInvolvementNote: optionalTrimmed(payload.YoungPersonInvolvementNote),
	}
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(
			r.Context(), tx, childID, security.RoleSenior, security.RoleManager,
		)
		if err != nil {
			return err
		}
		plan, err := tx.LockChildCarePlan(r.Context(), planID, childID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return security.ErrInvalidRelationship
			}
			return err
		}
		if plan.Status == models.CarePlanStatusArchived {
			return errCarePlanArchived
		}
		if err := tx.UpsertChildCarePlanSection(r.Context(), section); err != nil {
			return err
		}
		return writeCarePlanSectionAudit(r, tx, principal, section.ID, storage.AuditEventCarePlanSectionUpdate)
	})
	if err != nil {
		writeCarePlanMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, section)
}

func (h *CarePlanHandler) ActivateCarePlan(w http.ResponseWriter, r *http.Request) {
	h.changeCarePlanStatus(w, r, models.CarePlanStatusActive)
}

func (h *CarePlanHandler) ArchiveCarePlan(w http.ResponseWriter, r *http.Request) {
	h.changeCarePlanStatus(w, r, models.CarePlanStatusArchived)
}

func (h *CarePlanHandler) changeCarePlanStatus(w http.ResponseWriter, r *http.Request, next models.CarePlanStatus) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("care plan repo not initialised"))
		return
	}
	childID, planID, _, err := carePlanRouteIDs(r)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var plan *models.ChildCarePlan
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(
			r.Context(), tx, childID, security.RoleManager,
		)
		if err != nil {
			return err
		}
		plan, err = tx.LockChildCarePlan(r.Context(), planID, childID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return security.ErrInvalidRelationship
			}
			return err
		}
		previous := plan.Status
		switch next {
		case models.CarePlanStatusActive:
			if plan.Status != models.CarePlanStatusDraft {
				return errCarePlanNotDraft
			}
			if plan.PredecessorCarePlanID != nil {
				predecessor, err := tx.LockChildCarePlan(r.Context(), *plan.PredecessorCarePlanID, childID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return security.ErrInvalidRelationship
					}
					return err
				}
				if predecessor.Status != models.CarePlanStatusActive {
					return errPredecessorNotActive
				}
				if err := tx.ArchiveChildCarePlan(r.Context(), predecessor, principal.StaffUserID()); err != nil {
					return err
				}
				if err := writeCarePlanAudit(r, tx, principal, predecessor.ID, storage.AuditEventCarePlanArchive,
					security.AuditMetadataFields{PreviousStatus: security.AuditStatusActive, NewStatus: security.AuditStatusArchived}, storage.AuditRouteCarePlanActivate); err != nil {
					return err
				}
			} else {
				hasOther, err := tx.HasOtherActiveCarePlan(r.Context(), childID, planID)
				if err != nil {
					return err
				}
				if hasOther {
					return errOtherActiveCarePlan
				}
			}
			if err := tx.ActivateChildCarePlan(r.Context(), plan, principal.StaffUserID()); err != nil {
				return err
			}
			return writeCarePlanAudit(r, tx, principal, plan.ID, storage.AuditEventCarePlanActivate,
				security.AuditMetadataFields{PreviousStatus: security.AuditStatus(previous), NewStatus: security.AuditStatusActive}, storage.AuditRouteCarePlanActivate)
		case models.CarePlanStatusArchived:
			if plan.Status == models.CarePlanStatusArchived {
				return errCarePlanArchived
			}
			if err := tx.ArchiveChildCarePlan(r.Context(), plan, principal.StaffUserID()); err != nil {
				return err
			}
			return writeCarePlanAudit(r, tx, principal, plan.ID, storage.AuditEventCarePlanArchive,
				security.AuditMetadataFields{PreviousStatus: security.AuditStatus(previous), NewStatus: security.AuditStatusArchived}, storage.AuditRouteCarePlanArchive)
		default:
			return errors.New("unsupported care plan transition")
		}
	})
	if err != nil {
		writeCarePlanMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
}

func carePlanRouteIDs(r *http.Request) (int64, int64, models.CarePlanSectionKey, error) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		return 0, 0, "", errors.New("invalid child id")
	}
	planID, err := resourceID(chi.URLParam(r, "carePlanID"))
	if err != nil {
		return 0, 0, "", errors.New("invalid care plan id")
	}
	key := models.CarePlanSectionKey(strings.TrimSpace(chi.URLParam(r, "sectionKey")))
	if key != "" && !validCarePlanSectionKey(key) {
		return 0, 0, "", errors.New("invalid care plan section")
	}
	return childID, planID, key, nil
}

func validCarePlanSectionKey(key models.CarePlanSectionKey) bool {
	for _, allowed := range models.CanonicalCarePlanSectionKeys {
		if key == allowed {
			return true
		}
	}
	return false
}

func validCarePlanInformationStatus(status models.CarePlanInformationStatus) bool {
	switch status {
	case models.CarePlanInformationKnown, models.CarePlanInformationPartlyKnown,
		models.CarePlanInformationNotKnownYet, models.CarePlanInformationAwaitingConfirmation,
		models.CarePlanInformationNotApplicable:
		return true
	default:
		return false
	}
}

func writeCarePlanAudit(r *http.Request, tx *storage.TxRepo, principal security.Principal, planID int64, event storage.AuditEventType, fields security.AuditMetadataFields, route storage.AuditRouteTemplate) error {
	fields.RequestID = chimw.GetReqID(r.Context())
	bound, err := tx.NewCarePlanAuditEvent(r.Context(), principal, planID, event, fields, route)
	if err != nil {
		return err
	}
	_, err = tx.InsertAuditEvent(r.Context(), bound)
	return err
}

func writeCarePlanSectionAudit(r *http.Request, tx *storage.TxRepo, principal security.Principal, sectionID int64, event storage.AuditEventType) error {
	bound, err := tx.NewCarePlanSectionAuditEvent(
		r.Context(), principal, sectionID, event,
		security.AuditMetadataFields{RequestID: chimw.GetReqID(r.Context())},
		storage.AuditRouteCarePlanSections,
	)
	if err != nil {
		return err
	}
	_, err = tx.InsertAuditEvent(r.Context(), bound)
	return err
}

func writeCarePlanMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, security.ErrUnauthenticated):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	case errors.Is(err, security.ErrForbidden), errors.Is(err, security.ErrInvalidRelationship):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, errCarePlanArchived), errors.Is(err, errCarePlanNotDraft), errors.Is(err, errOtherActiveCarePlan),
		errors.Is(err, errCarePlanNotActive), errors.Is(err, errPredecessorNotActive):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		writeServerError(w, err)
	}
}
