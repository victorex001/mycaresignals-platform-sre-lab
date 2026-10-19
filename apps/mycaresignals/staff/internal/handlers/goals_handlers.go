package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
	"therapeutic-engine/internal/storage"
)

type GoalsHandler struct {
	Repo *storage.Repo
}

func NewGoalsHandler(repo *storage.Repo) *GoalsHandler {
	return &GoalsHandler{Repo: repo}
}

type goalPayload struct {
	GoalType           string  `json:"goal_type"`
	Title              string  `json:"title"`
	Domain             string  `json:"domain"`
	Description        string  `json:"description"`
	StartDate          *string `json:"start_date"`
	TargetDate         *string `json:"target_date"`
	Status             string  `json:"status"`
	ChildVoice         string  `json:"child_voice"`
	FormulationSummary string  `json:"formulation_summary"`
	ProtectiveFactors  string  `json:"protective_factors"`
}

type targetPayload struct {
	GoalID               *int64  `json:"goal_id"`
	Domain               string  `json:"domain"`
	ShortLabel           string  `json:"short_label"`
	Description          string  `json:"description"`
	Status               string  `json:"status"`
	Barriers             string  `json:"barriers"`
	SupportsNeeded       string  `json:"supports_needed"`
	InterventionStrategy string  `json:"intervention_strategy"`
	ReviewDate           *string `json:"review_date"`
}

func optionalTrimmed(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func goalStatus(value string) (models.GoalStatus, error) {
	status := models.GoalStatus(strings.TrimSpace(value))
	if status == "" {
		return models.GoalStatusActive, nil
	}
	switch status {
	case models.GoalStatusActive, models.GoalStatusPaused, models.GoalStatusAchieved, models.GoalStatusClosed:
		return status, nil
	default:
		return "", errors.New("invalid status")
	}
}

func optionalDate(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*value))
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func resourceID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid resource id")
	}
	return id, nil
}

// POST /v1/children/{childID}/goals
func (h *GoalsHandler) CreateGoal(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("goals repo not initialised"))
		return
	}

	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}

	var payload goalPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeBadRequest(w, "invalid json body")
		return
	}
	payload.Title = strings.TrimSpace(payload.Title)
	if payload.Title == "" {
		writeBadRequest(w, "title is required")
		return
	}
	status, err := goalStatus(payload.Status)
	if err != nil {
		writeBadRequest(w, "status must be active, paused, achieved or closed")
		return
	}
	startDate, err := optionalDate(payload.StartDate)
	if err != nil {
		writeBadRequest(w, "start_date must be YYYY-MM-DD")
		return
	}
	targetDate, err := optionalDate(payload.TargetDate)
	if err != nil {
		writeBadRequest(w, "target_date must be YYYY-MM-DD")
		return
	}

	goalType := models.GoalTypeCarePlan
	if payload.GoalType != "" {
		goalType = models.GoalType(payload.GoalType)
	}

	g := &models.ChildGoal{
		ChildID:            childID,
		GoalType:           goalType,
		Title:              payload.Title,
		Domain:             optionalTrimmed(payload.Domain),
		Description:        strings.TrimSpace(payload.Description),
		StartDate:          startDate,
		TargetDate:         targetDate,
		Status:             status,
		ChildVoice:         optionalTrimmed(payload.ChildVoice),
		FormulationSummary: optionalTrimmed(payload.FormulationSummary),
		ProtectiveFactors:  optionalTrimmed(payload.ProtectiveFactors),
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
		if err := tx.CreateChildGoal(r.Context(), g); err != nil {
			return err
		}
		bound, err := tx.NewGoalAuditEvent(
			r.Context(),
			principal,
			g.ID,
			storage.AuditEventGoalCreate,
			security.AuditMetadataFields{RequestID: chimw.GetReqID(r.Context())},
			storage.AuditRouteGoals,
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

	writeJSON(w, http.StatusCreated, g)
}

// GET /v1/children/{childID}/goals
func (h *GoalsHandler) ListGoals(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("goals repo not initialised"))
		return
	}

	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}

	var filters storage.GoalFilters
	filters.ChildID = childID

	statusStr := r.URL.Query().Get("status")
	if statusStr != "" {
		s := models.GoalStatus(statusStr)
		filters.Status = &s
	}

	goals, err := h.Repo.ListChildGoals(r.Context(), filters)
	if err != nil {
		writeServerError(w, err)
		return
	}

	// Return [] instead of null
	if goals == nil {
		goals = []models.ChildGoal{}
	}

	writeJSON(w, http.StatusOK, goals)
}

// POST /v1/children/{childID}/targets
func (h *GoalsHandler) CreateTarget(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("goals repo not initialised"))
		return
	}

	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}

	var payload targetPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeBadRequest(w, "invalid json body")
		return
	}
	payload.Domain = strings.TrimSpace(payload.Domain)
	payload.ShortLabel = strings.TrimSpace(payload.ShortLabel)
	if payload.Domain == "" || payload.ShortLabel == "" {
		writeBadRequest(w, "domain and short_label are required")
		return
	}
	status, err := goalStatus(payload.Status)
	if err != nil {
		writeBadRequest(w, "status must be active, paused, achieved or closed")
		return
	}
	reviewDate, err := optionalDate(payload.ReviewDate)
	if err != nil {
		writeBadRequest(w, "review_date must be YYYY-MM-DD")
		return
	}

	t := &models.ChildTarget{
		ChildID:              childID,
		GoalID:               payload.GoalID,
		Domain:               payload.Domain,
		ShortLabel:           payload.ShortLabel,
		Description:          strings.TrimSpace(payload.Description),
		Status:               status,
		Barriers:             optionalTrimmed(payload.Barriers),
		SupportsNeeded:       optionalTrimmed(payload.SupportsNeeded),
		InterventionStrategy: optionalTrimmed(payload.InterventionStrategy),
		ReviewDate:           reviewDate,
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
		if t.GoalID != nil {
			if *t.GoalID <= 0 {
				return security.ErrInvalidRelationship
			}
			belongs, err := tx.LockGoalBelongsToChild(r.Context(), *t.GoalID, childID)
			if err != nil {
				return err
			}
			if !belongs {
				return security.ErrInvalidRelationship
			}
		}
		if err := tx.CreateChildTarget(r.Context(), t); err != nil {
			return err
		}
		bound, err := tx.NewTargetAuditEvent(
			r.Context(),
			principal,
			t.ID,
			storage.AuditEventTargetCreate,
			security.AuditMetadataFields{RequestID: chimw.GetReqID(r.Context())},
			storage.AuditRouteTargets,
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

	writeJSON(w, http.StatusCreated, t)
}

// GET /v1/children/{childID}/targets
func (h *GoalsHandler) ListTargets(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("goals repo not initialised"))
		return
	}

	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}

	var filters storage.TargetFilters
	filters.ChildID = childID

	q := r.URL.Query()

	if goalStr := q.Get("goal_id"); goalStr != "" {
		if goalID, err := strconv.ParseInt(goalStr, 10, 64); err == nil {
			filters.GoalID = &goalID
		}
	}
	if statusStr := q.Get("status"); statusStr != "" {
		s := models.GoalStatus(statusStr)
		filters.Status = &s
	}

	targets, err := h.Repo.ListChildTargets(r.Context(), filters)
	if err != nil {
		writeServerError(w, err)
		return
	}

	if targets == nil {
		targets = []models.ChildTarget{}
	}

	writeJSON(w, http.StatusOK, targets)
}

// PATCH /v1/children/{childID}/goals/{goalID}
func (h *GoalsHandler) UpdateGoal(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("goals repo not initialised"))
		return
	}
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	goalID, err := resourceID(chi.URLParam(r, "goalID"))
	if err != nil {
		writeBadRequest(w, "invalid goal id")
		return
	}
	var payload goalPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeBadRequest(w, "invalid json body")
		return
	}
	payload.Title = strings.TrimSpace(payload.Title)
	if payload.Title == "" {
		writeBadRequest(w, "title is required")
		return
	}
	status, err := goalStatus(payload.Status)
	if err != nil {
		writeBadRequest(w, "status must be active, paused, achieved or closed")
		return
	}
	startDate, err := optionalDate(payload.StartDate)
	if err != nil {
		writeBadRequest(w, "start_date must be YYYY-MM-DD")
		return
	}
	targetDate, err := optionalDate(payload.TargetDate)
	if err != nil {
		writeBadRequest(w, "target_date must be YYYY-MM-DD")
		return
	}
	goalType := models.GoalTypeCarePlan
	if strings.TrimSpace(payload.GoalType) != "" {
		goalType = models.GoalType(strings.TrimSpace(payload.GoalType))
	}
	goal := &models.ChildGoal{
		ID:                 goalID,
		ChildID:            childID,
		GoalType:           goalType,
		Title:              payload.Title,
		Domain:             optionalTrimmed(payload.Domain),
		Description:        strings.TrimSpace(payload.Description),
		StartDate:          startDate,
		TargetDate:         targetDate,
		Status:             status,
		ChildVoice:         optionalTrimmed(payload.ChildVoice),
		FormulationSummary: optionalTrimmed(payload.FormulationSummary),
		ProtectiveFactors:  optionalTrimmed(payload.ProtectiveFactors),
	}

	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(
			r.Context(), tx, childID, security.RoleSenior, security.RoleManager,
		)
		if err != nil {
			return err
		}
		belongs, err := tx.LockGoalBelongsToChild(r.Context(), goalID, childID)
		if err != nil {
			return err
		}
		if !belongs {
			return security.ErrInvalidRelationship
		}
		if err := tx.UpdateChildGoal(r.Context(), goal); err != nil {
			return err
		}
		bound, err := tx.NewGoalAuditEvent(
			r.Context(), principal, goal.ID, storage.AuditEventGoalUpdate,
			security.AuditMetadataFields{
				NewStatus: security.AuditStatus(goal.Status),
				RequestID: chimw.GetReqID(r.Context()),
			},
			storage.AuditRouteGoals,
		)
		if err != nil {
			return err
		}
		_, err = tx.InsertAuditEvent(r.Context(), bound)
		return err
	})
	if err != nil {
		writeGoalMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, goal)
}

// PATCH /v1/children/{childID}/targets/{targetID}
func (h *GoalsHandler) UpdateTarget(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("goals repo not initialised"))
		return
	}
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	targetID, err := resourceID(chi.URLParam(r, "targetID"))
	if err != nil {
		writeBadRequest(w, "invalid target id")
		return
	}
	var payload targetPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeBadRequest(w, "invalid json body")
		return
	}
	payload.Domain = strings.TrimSpace(payload.Domain)
	payload.ShortLabel = strings.TrimSpace(payload.ShortLabel)
	if payload.Domain == "" || payload.ShortLabel == "" {
		writeBadRequest(w, "domain and short_label are required")
		return
	}
	status, err := goalStatus(payload.Status)
	if err != nil {
		writeBadRequest(w, "status must be active, paused, achieved or closed")
		return
	}
	reviewDate, err := optionalDate(payload.ReviewDate)
	if err != nil {
		writeBadRequest(w, "review_date must be YYYY-MM-DD")
		return
	}
	target := &models.ChildTarget{
		ID:                   targetID,
		ChildID:              childID,
		GoalID:               payload.GoalID,
		Domain:               payload.Domain,
		ShortLabel:           payload.ShortLabel,
		Description:          strings.TrimSpace(payload.Description),
		Status:               status,
		Barriers:             optionalTrimmed(payload.Barriers),
		SupportsNeeded:       optionalTrimmed(payload.SupportsNeeded),
		InterventionStrategy: optionalTrimmed(payload.InterventionStrategy),
		ReviewDate:           reviewDate,
	}

	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(
			r.Context(), tx, childID, security.RoleSenior, security.RoleManager,
		)
		if err != nil {
			return err
		}
		belongs, err := tx.LockTargetBelongsToChild(r.Context(), targetID, childID)
		if err != nil {
			return err
		}
		if !belongs {
			return security.ErrInvalidRelationship
		}
		if target.GoalID != nil {
			if *target.GoalID <= 0 {
				return security.ErrInvalidRelationship
			}
			belongs, err = tx.LockGoalBelongsToChild(r.Context(), *target.GoalID, childID)
			if err != nil {
				return err
			}
			if !belongs {
				return security.ErrInvalidRelationship
			}
		}
		if err := tx.UpdateChildTarget(r.Context(), target); err != nil {
			return err
		}
		bound, err := tx.NewTargetAuditEvent(
			r.Context(), principal, target.ID, storage.AuditEventTargetUpdate,
			security.AuditMetadataFields{
				NewStatus: security.AuditStatus(target.Status),
				RequestID: chimw.GetReqID(r.Context()),
			},
			storage.AuditRouteTargets,
		)
		if err != nil {
			return err
		}
		_, err = tx.InsertAuditEvent(r.Context(), bound)
		return err
	})
	if err != nil {
		writeGoalMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, target)
}

func writeGoalMutationError(w http.ResponseWriter, err error) {
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
