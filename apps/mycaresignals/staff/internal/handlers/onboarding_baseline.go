package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

var errInvalidBaselinePayload = errors.New("invalid onboarding baseline payload")

type domainPayload struct {
	DomainKey string `json:"domain_key"`
	Active    bool   `json:"active"`
	Reason    string `json:"reason"`
}

type priorityPayload struct {
	DomainKey      string `json:"domain_key"`
	FocusSummary   string `json:"focus_summary"`
	HopedForChange string `json:"hoped_for_change"`
	SortOrder      int    `json:"sort_order"`
}

type reviewPlanPayload struct {
	ReviewFrequency    string  `json:"review_frequency"`
	NextReviewDate     *string `json:"next_review_date"`
	LeadReviewer       string  `json:"lead_reviewer"`
	OtherProfessionals string  `json:"other_professionals"`
	EarlyWarning       string  `json:"early_warning_signals"`
	NotesForTeam       string  `json:"notes_for_team"`
}

type onboardingBaselinePayload struct {
	SummaryOfHistory       string `json:"summary_of_history"`
	RecentPlacementPattern string `json:"recent_placement_pattern"`
	CurrentConcerns        string `json:"current_concerns"`
	Strengths              string `json:"strengths"`
	ExistingPlans          string `json:"existing_plans"`
	ChildsVoice            string `json:"childs_voice"`

	RelapseRisks        string `json:"relapse_risks"`
	RelapseEarlySignals string `json:"relapse_early_signals"`
	RelapseSupportPlan  string `json:"relapse_support_plan"`

	Domains    []domainPayload   `json:"domains"`
	Priorities []priorityPayload `json:"priorities"`
	ReviewPlan reviewPlanPayload `json:"review_plan"`

	Status string `json:"status"` // optional: "draft" | "submitted"
}

type baselineFullResponse struct {
	Baseline   *models.ChildOnboardingBaseline     `json:"baseline"`
	Domains    []models.ChildOnboardingDomainState `json:"domains"`
	Priorities []models.ChildPriorityDomain        `json:"priorities"`
}

// POST /v1/homes/{homeID}/children/{childID}/onboarding/baseline
func (h *ChildrenHandler) SaveOnboardingBaseline(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		http.Error(w, "repository not initialised", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// sanity check homeID
	homeIDStr := chi.URLParam(r, "homeID")
	homeID, err := strconv.ParseInt(homeIDStr, 10, 64)
	if err != nil || homeID <= 0 {
		http.Error(w, "invalid home id", http.StatusBadRequest)
		return
	}

	childIDStr := chi.URLParam(r, "childID")
	childID, err := strconv.ParseInt(childIDStr, 10, 64)
	if err != nil || childID <= 0 {
		http.Error(w, "invalid child id", http.StatusBadRequest)
		return
	}

	var payload onboardingBaselinePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	var nextReview *time.Time
	if payload.ReviewPlan.NextReviewDate != nil && *payload.ReviewPlan.NextReviewDate != "" {
		t, err := time.Parse("2006-01-02", *payload.ReviewPlan.NextReviewDate)
		if err != nil {
			http.Error(w, "invalid next_review_date", http.StatusBadRequest)
			return
		}
		nextReview = &t
	}

	baseline := &models.ChildOnboardingBaseline{
		ChildID:             models.ChildID(childID),
		SummaryOfHistory:    payload.SummaryOfHistory,
		RecentPlacement:     payload.RecentPlacementPattern,
		CurrentConcerns:     payload.CurrentConcerns,
		Strengths:           payload.Strengths,
		ExistingPlans:       payload.ExistingPlans,
		ChildsVoice:         payload.ChildsVoice,
		RelapseRisks:        payload.RelapseRisks,
		RelapseEarlySignals: payload.RelapseEarlySignals,
		RelapseSupportPlan:  payload.RelapseSupportPlan,
		ReviewFrequency:     payload.ReviewPlan.ReviewFrequency,
		NextReviewDate:      nextReview,
		LeadReviewer:        payload.ReviewPlan.LeadReviewer,
		OtherProfessionals:  payload.ReviewPlan.OtherProfessionals,
		EarlyWarningSignals: payload.ReviewPlan.EarlyWarning,
		NotesForTeam:        payload.ReviewPlan.NotesForTeam,
	}

	var domains []models.ChildOnboardingDomainState
	for _, d := range payload.Domains {
		domains = append(domains, models.ChildOnboardingDomainState{
			DomainKey: strings.TrimSpace(d.DomainKey),
			Active:    d.Active,
			Reason:    d.Reason,
		})
	}

	var priorities []models.ChildPriorityDomain
	for _, p := range payload.Priorities {
		priorities = append(priorities, models.ChildPriorityDomain{
			DomainKey:      strings.TrimSpace(p.DomainKey),
			FocusSummary:   p.FocusSummary,
			HopedForChange: p.HopedForChange,
			SortOrder:      p.SortOrder,
		})
	}

	err = h.Repo.WithTx(ctx, nil, func(tx *storage.TxRepo) error {
		authoritativeHomeID, principal, err := security.RequireChildHomeRoleForMutation(
			ctx,
			tx,
			childID,
			security.RoleKeyWorker,
			security.RoleSenior,
			security.RoleManager,
		)
		if err != nil {
			return err
		}
		if authoritativeHomeID != homeID {
			return security.ErrForbidden
		}
		status, err := normalizeBaselineSaveStatus(payload.Status)
		if err != nil {
			return err
		}
		if err := validateBaselineChildren(domains, priorities); err != nil {
			return err
		}
		if err := tx.SaveOnboardingBaseline(ctx, baseline, domains, priorities, status); err != nil {
			return err
		}
		domainCount := len(domains)
		priorityCount := len(priorities)
		bound, err := tx.NewBaselineAuditEvent(
			ctx,
			principal,
			baseline.ID,
			storage.AuditEventBaselineCreate,
			security.AuditMetadataFields{
				NewStatus:     security.AuditStatus(status),
				DomainCount:   &domainCount,
				PriorityCount: &priorityCount,
				RequestID:     chimw.GetReqID(ctx),
			},
			storage.AuditRouteBaselineSave,
		)
		if err != nil {
			return err
		}
		_, err = tx.InsertAuditEvent(ctx, bound)
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
		if errors.Is(err, errInvalidBaselinePayload) {
			http.Error(w, "invalid baseline payload", http.StatusBadRequest)
			return
		}
		http.Error(w, "could not save baseline", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(baseline)
}

// GET /v1/children/{childID}/onboarding/baseline
func (h *ChildrenHandler) GetOnboardingBaseline(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Children == nil {
		http.Error(w, "children store not initialised", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	childIDParam := chi.URLParam(r, "childID")
	id64, err := strconv.ParseInt(childIDParam, 10, 64)
	if err != nil {
		http.Error(w, "invalid child id", http.StatusBadRequest)
		return
	}

	baseline, domains, priorities, err := h.Children.GetLatestOnboardingBaseline(ctx, models.ChildID(id64))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no baseline found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load baseline", http.StatusInternalServerError)
		return
	}

	resp := baselineFullResponse{
		Baseline:   baseline,
		Domains:    domains,
		Priorities: priorities,
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---------- Approve latest baseline ----------
// POST /v1/children/{childID}/onboarding/approve
type approveBaselineRequest struct {
	ApprovedBy string `json:"approved_by"`
}

func (h *ChildrenHandler) ApproveOnboardingBaseline(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		http.Error(w, "repository not initialised", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	childIDParam := chi.URLParam(r, "childID")
	id64, err := strconv.ParseInt(childIDParam, 10, 64)
	if err != nil || id64 <= 0 {
		http.Error(w, "invalid child id", http.StatusBadRequest)
		return
	}

	var req approveBaselineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	err = h.Repo.WithTx(ctx, nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(
			ctx,
			tx,
			id64,
			security.RoleManager,
		)
		if err != nil {
			return err
		}
		baseline, err := tx.ApproveLatestOnboardingBaseline(ctx, models.ChildID(id64), principal.DisplayName())
		if err != nil {
			return err
		}
		approved := true
		bound, err := tx.NewBaselineAuditEvent(
			ctx,
			principal,
			baseline.ID,
			storage.AuditEventBaselineApprove,
			security.AuditMetadataFields{
				PreviousStatus: security.AuditStatusSubmitted,
				NewStatus:      security.AuditStatusApproved,
				Approved:       &approved,
				RequestID:      chimw.GetReqID(ctx),
			},
			storage.AuditRouteBaselineApprove,
		)
		if err != nil {
			return err
		}
		_, err = tx.InsertAuditEvent(ctx, bound)
		return err
	})
	if err != nil {
		if errors.Is(err, security.ErrUnauthenticated) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if errors.Is(err, security.ErrForbidden) || errors.Is(err, security.ErrInvalidRelationship) || errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if errors.Is(err, storage.ErrInvalidBaselineTransition) {
			http.Error(w, "invalid baseline transition", http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to approve baseline", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func normalizeBaselineSaveStatus(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "submitted":
		return "submitted", nil
	case "draft":
		return "draft", nil
	default:
		return "", fmt.Errorf("%w: status", errInvalidBaselinePayload)
	}
}

func validateBaselineChildren(domains []models.ChildOnboardingDomainState, priorities []models.ChildPriorityDomain) error {
	domainKeys := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		if domain.DomainKey == "" {
			return fmt.Errorf("%w: empty domain key", errInvalidBaselinePayload)
		}
		if _, exists := domainKeys[domain.DomainKey]; exists {
			return fmt.Errorf("%w: duplicate domain key", errInvalidBaselinePayload)
		}
		domainKeys[domain.DomainKey] = struct{}{}
	}

	priorityKeys := make(map[string]struct{}, len(priorities))
	sortOrders := make(map[int]struct{}, len(priorities))
	for _, priority := range priorities {
		if priority.DomainKey == "" {
			return fmt.Errorf("%w: empty priority domain key", errInvalidBaselinePayload)
		}
		if _, exists := priorityKeys[priority.DomainKey]; exists {
			return fmt.Errorf("%w: duplicate priority domain key", errInvalidBaselinePayload)
		}
		priorityKeys[priority.DomainKey] = struct{}{}
		if _, exists := sortOrders[priority.SortOrder]; exists {
			return fmt.Errorf("%w: duplicate priority sort order", errInvalidBaselinePayload)
		}
		sortOrders[priority.SortOrder] = struct{}{}
	}
	return nil
}
