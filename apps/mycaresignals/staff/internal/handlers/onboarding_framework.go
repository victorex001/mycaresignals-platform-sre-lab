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

var errInvalidFrameworkPayload = errors.New("invalid onboarding framework payload")

var frameworkInformationStatuses = map[string]struct{}{
	"known_confirmed": {}, "partly_known": {},
	"awaiting_confirmation": {}, "not_known_yet": {},
}

var frameworkPathways = map[string]struct{}{
	"new_placement": {}, "existing_young_person": {},
}

var frameworkSourceTypes = map[string]struct{}{
	"local_authority_information": {}, "existing_care_plan": {},
	"placement_plan": {}, "existing_risk_assessment": {},
	"missing_from_home_plan": {}, "health_plan": {}, "education_pep": {},
	"behaviour_support_plan": {}, "contact_arrangements": {},
	"berri_assessment": {}, "social_worker": {}, "young_person": {},
	"parent_family": {}, "school_education": {}, "health_professional": {},
	"staff_observation": {}, "other_professional_report": {}, "other": {},
}

type frameworkDomainPayload struct {
	DomainKey                  string  `json:"domain_key"`
	CurrentUnderstanding       string  `json:"current_understanding"`
	StrengthsProtectiveFactors string  `json:"strengths_protective_factors"`
	SupportNeeds               string  `json:"support_needs"`
	WhatHelps                  string  `json:"what_helps"`
	CurrentSupport             string  `json:"current_support"`
	YoungPersonView            string  `json:"young_person_view"`
	InformationStatus          string  `json:"information_status"`
	InformationSource          string  `json:"information_source"`
	SourceDetails              string  `json:"source_details"`
	SourceDate                 *string `json:"source_date"`
	Priority                   bool    `json:"priority"`
	PriorityReason             string  `json:"priority_reason"`
}

type frameworkSourcePayload struct {
	SourceType string  `json:"source_type"`
	SourceName string  `json:"source_name"`
	SourceDate *string `json:"source_date"`
	Notes      string  `json:"notes"`
}

type onboardingFrameworkPayload struct {
	OnboardingPathway           string                   `json:"onboarding_pathway"`
	ImmediateInformation        string                   `json:"immediate_information"`
	ImmediateInformationStatus  string                   `json:"immediate_information_status"`
	SummaryOfHistory            string                   `json:"summary_of_history"`
	BackgroundInformationStatus string                   `json:"background_information_status"`
	RecentPlacementPattern      string                   `json:"recent_placement_pattern"`
	CurrentConcerns             string                   `json:"current_concerns"`
	Strengths                   string                   `json:"strengths"`
	ExistingPlans               string                   `json:"existing_plans"`
	ChildsVoice                 string                   `json:"childs_voice"`
	KnownSafetyInformation      string                   `json:"known_safety_information"`
	SafetyInformationStatus     string                   `json:"safety_information_status"`
	Domains                     []frameworkDomainPayload `json:"domains"`
	Sources                     []frameworkSourcePayload `json:"sources"`
	Priorities                  []priorityPayload        `json:"priorities"`
	ReviewPlan                  reviewPlanPayload        `json:"review_plan"`
	Status                      string                   `json:"status"`
}

type frameworkFullResponse struct {
	Baseline                   *models.ChildOnboardingBaseline         `json:"baseline"`
	Domains                    []models.ChildOnboardingDomainState     `json:"domains"`
	Priorities                 []models.ChildPriorityDomain            `json:"priorities"`
	Sources                    []models.ChildOnboardingFrameworkSource `json:"sources"`
	UnresolvedInformationCount int                                     `json:"unresolved_information_count"`
}

func parseOptionalFrameworkDate(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*value))
	if err != nil {
		return nil, fmt.Errorf("%w: date", errInvalidFrameworkPayload)
	}
	return &parsed, nil
}

func validInformationStatus(value string) bool {
	_, ok := frameworkInformationStatuses[value]
	return ok
}

func validateFrameworkPayload(payload onboardingFrameworkPayload) error {
	if _, ok := frameworkPathways[payload.OnboardingPathway]; !ok {
		return fmt.Errorf("%w: onboarding pathway", errInvalidFrameworkPayload)
	}
	if !validInformationStatus(payload.ImmediateInformationStatus) ||
		!validInformationStatus(payload.BackgroundInformationStatus) ||
		!validInformationStatus(payload.SafetyInformationStatus) {
		return fmt.Errorf("%w: information status", errInvalidFrameworkPayload)
	}
	if len(payload.Domains) != len(models.CanonicalProgressDomains) {
		return fmt.Errorf("%w: six domains required", errInvalidFrameworkPayload)
	}
	seen := make(map[string]struct{}, len(payload.Domains))
	selectedPriorityDomains := make(map[string]struct{}, 3)
	priorities := 0
	for _, domain := range payload.Domains {
		if !models.ValidProgressDomain(strings.TrimSpace(domain.DomainKey)) {
			return fmt.Errorf("%w: non-canonical domain", errInvalidFrameworkPayload)
		}
		if _, exists := seen[domain.DomainKey]; exists {
			return fmt.Errorf("%w: duplicate domain", errInvalidFrameworkPayload)
		}
		seen[domain.DomainKey] = struct{}{}
		if !validInformationStatus(domain.InformationStatus) {
			return fmt.Errorf("%w: domain information status", errInvalidFrameworkPayload)
		}
		if domain.InformationSource != "" {
			if _, ok := frameworkSourceTypes[domain.InformationSource]; !ok {
				return fmt.Errorf("%w: domain information source", errInvalidFrameworkPayload)
			}
		}
		if domain.Priority {
			priorities++
			selectedPriorityDomains[domain.DomainKey] = struct{}{}
		}
		if _, err := parseOptionalFrameworkDate(domain.SourceDate); err != nil {
			return err
		}
	}
	if priorities > 3 || len(payload.Priorities) > 3 {
		return fmt.Errorf("%w: maximum three priorities", errInvalidFrameworkPayload)
	}
	prioritySeen := map[string]struct{}{}
	for _, priority := range payload.Priorities {
		if !models.ValidProgressDomain(strings.TrimSpace(priority.DomainKey)) {
			return fmt.Errorf("%w: priority domain", errInvalidFrameworkPayload)
		}
		if _, exists := prioritySeen[priority.DomainKey]; exists {
			return fmt.Errorf("%w: duplicate priority", errInvalidFrameworkPayload)
		}
		prioritySeen[priority.DomainKey] = struct{}{}
		if _, selected := selectedPriorityDomains[priority.DomainKey]; !selected {
			return fmt.Errorf("%w: priority profile mismatch", errInvalidFrameworkPayload)
		}
	}
	if len(prioritySeen) != len(selectedPriorityDomains) {
		return fmt.Errorf("%w: selected priority details required", errInvalidFrameworkPayload)
	}
	if len(payload.Sources) > 20 {
		return fmt.Errorf("%w: too many sources", errInvalidFrameworkPayload)
	}
	for _, source := range payload.Sources {
		if _, ok := frameworkSourceTypes[source.SourceType]; !ok {
			return fmt.Errorf("%w: source type", errInvalidFrameworkPayload)
		}
		if _, err := parseOptionalFrameworkDate(source.SourceDate); err != nil {
			return err
		}
	}
	if _, err := normalizeBaselineSaveStatus(payload.Status); err != nil {
		return err
	}
	return nil
}

// POST /v1/homes/{homeID}/children/{childID}/onboarding/framework
func (h *ChildrenHandler) SaveOnboardingFramework(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		http.Error(w, "repository not initialised", http.StatusInternalServerError)
		return
	}
	homeID, err := strconv.ParseInt(chi.URLParam(r, "homeID"), 10, 64)
	if err != nil || homeID <= 0 {
		http.Error(w, "invalid home id", http.StatusBadRequest)
		return
	}
	childID, err := strconv.ParseInt(chi.URLParam(r, "childID"), 10, 64)
	if err != nil || childID <= 0 {
		http.Error(w, "invalid child id", http.StatusBadRequest)
		return
	}
	var payload onboardingFrameworkPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := validateFrameworkPayload(payload); err != nil {
		http.Error(w, "invalid framework payload", http.StatusBadRequest)
		return
	}
	status, _ := normalizeBaselineSaveStatus(payload.Status)
	nextReview, err := parseOptionalFrameworkDate(payload.ReviewPlan.NextReviewDate)
	if err != nil {
		http.Error(w, "invalid next_review_date", http.StatusBadRequest)
		return
	}

	baseline := &models.ChildOnboardingBaseline{
		ChildID: models.ChildID(childID), SummaryOfHistory: strings.TrimSpace(payload.SummaryOfHistory),
		RecentPlacement: strings.TrimSpace(payload.RecentPlacementPattern), CurrentConcerns: strings.TrimSpace(payload.CurrentConcerns),
		Strengths: strings.TrimSpace(payload.Strengths), ExistingPlans: strings.TrimSpace(payload.ExistingPlans),
		ChildsVoice: strings.TrimSpace(payload.ChildsVoice), KnownSafetyInformation: strings.TrimSpace(payload.KnownSafetyInformation),
		ReviewFrequency: strings.TrimSpace(payload.ReviewPlan.ReviewFrequency), NextReviewDate: nextReview,
		LeadReviewer: strings.TrimSpace(payload.ReviewPlan.LeadReviewer), OtherProfessionals: strings.TrimSpace(payload.ReviewPlan.OtherProfessionals),
		EarlyWarningSignals: strings.TrimSpace(payload.ReviewPlan.EarlyWarning), NotesForTeam: strings.TrimSpace(payload.ReviewPlan.NotesForTeam),
		FrameworkModel: storage.SixDomainFrameworkModel, OnboardingPathway: payload.OnboardingPathway,
		ImmediateInformation: strings.TrimSpace(payload.ImmediateInformation), ImmediateInformationStatus: payload.ImmediateInformationStatus,
		BackgroundInformationStatus: payload.BackgroundInformationStatus, SafetyInformationStatus: payload.SafetyInformationStatus,
	}
	domains := make([]models.ChildOnboardingDomainState, 0, len(payload.Domains))
	for _, item := range payload.Domains {
		sourceDate, _ := parseOptionalFrameworkDate(item.SourceDate)
		domains = append(domains, models.ChildOnboardingDomainState{
			DomainKey: item.DomainKey, Active: item.Priority, Reason: strings.TrimSpace(item.PriorityReason),
			CurrentUnderstanding: strings.TrimSpace(item.CurrentUnderstanding), StrengthsProtectiveFactors: strings.TrimSpace(item.StrengthsProtectiveFactors),
			SupportNeeds: strings.TrimSpace(item.SupportNeeds), WhatHelps: strings.TrimSpace(item.WhatHelps),
			CurrentSupport: strings.TrimSpace(item.CurrentSupport), YoungPersonView: strings.TrimSpace(item.YoungPersonView),
			InformationStatus: item.InformationStatus, InformationSource: item.InformationSource,
			SourceDetails: strings.TrimSpace(item.SourceDetails), SourceDate: sourceDate, PriorityReason: strings.TrimSpace(item.PriorityReason),
		})
	}
	priorities := make([]models.ChildPriorityDomain, 0, len(payload.Priorities))
	for index, item := range payload.Priorities {
		priorities = append(priorities, models.ChildPriorityDomain{DomainKey: item.DomainKey,
			FocusSummary: strings.TrimSpace(item.FocusSummary), HopedForChange: strings.TrimSpace(item.HopedForChange), SortOrder: index + 1})
	}
	sources := make([]models.ChildOnboardingFrameworkSource, 0, len(payload.Sources))
	for _, item := range payload.Sources {
		sourceDate, _ := parseOptionalFrameworkDate(item.SourceDate)
		sources = append(sources, models.ChildOnboardingFrameworkSource{SourceType: item.SourceType,
			SourceName: strings.TrimSpace(item.SourceName), SourceDate: sourceDate, Notes: strings.TrimSpace(item.Notes)})
	}

	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		authoritativeHomeID, principal, err := security.RequireChildHomeRoleForMutation(r.Context(), tx, childID,
			security.RoleKeyWorker, security.RoleSenior, security.RoleManager)
		if err != nil {
			return err
		}
		if authoritativeHomeID != homeID {
			return security.ErrForbidden
		}
		if err := tx.SaveSixDomainFramework(r.Context(), baseline, domains, priorities, sources, status, principal.StaffUserID()); err != nil {
			return err
		}
		domainCount, priorityCount := len(domains), len(priorities)
		bound, err := tx.NewBaselineAuditEvent(r.Context(), principal, baseline.ID, storage.AuditEventBaselineCreate,
			security.AuditMetadataFields{NewStatus: security.AuditStatus(status), DomainCount: &domainCount,
				PriorityCount: &priorityCount, RequestID: chimw.GetReqID(r.Context())}, storage.AuditRouteFrameworkSave)
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
		case errors.Is(err, storage.ErrInvalidBaselineTransition):
			http.Error(w, "submitted framework cannot be changed", http.StatusConflict)
		default:
			http.Error(w, "could not save framework", http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusCreated, baseline)
}

func unresolvedInformationCount(b *models.ChildOnboardingBaseline, domains []models.ChildOnboardingDomainState) int {
	count := 0
	for _, status := range []string{b.ImmediateInformationStatus, b.BackgroundInformationStatus, b.SafetyInformationStatus} {
		if status == "awaiting_confirmation" || status == "not_known_yet" {
			count++
		}
	}
	for _, domain := range domains {
		if domain.InformationStatus == "awaiting_confirmation" || domain.InformationStatus == "not_known_yet" {
			count++
		}
	}
	return count
}

// GET /v1/children/{childID}/onboarding/framework
func (h *ChildrenHandler) GetOnboardingFramework(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Children == nil {
		http.Error(w, "children store not initialised", http.StatusInternalServerError)
		return
	}
	childID, err := strconv.ParseInt(chi.URLParam(r, "childID"), 10, 64)
	if err != nil || childID <= 0 {
		http.Error(w, "invalid child id", http.StatusBadRequest)
		return
	}
	baseline, domains, priorities, sources, err := h.Children.GetLatestSixDomainFramework(r.Context(), models.ChildID(childID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no framework found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load framework", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, frameworkFullResponse{Baseline: baseline, Domains: domains,
		Priorities: priorities, Sources: sources, UnresolvedInformationCount: unresolvedInformationCount(baseline, domains)})
}

// GET /v1/children/{childID}/onboarding/framework/history
func (h *ChildrenHandler) ListOnboardingFrameworkHistory(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Children == nil {
		http.Error(w, "children store not initialised", http.StatusInternalServerError)
		return
	}
	childID, err := strconv.ParseInt(chi.URLParam(r, "childID"), 10, 64)
	if err != nil || childID <= 0 {
		http.Error(w, "invalid child id", http.StatusBadRequest)
		return
	}
	items, err := h.Children.ListSixDomainFrameworkHistory(r.Context(), models.ChildID(childID))
	if err != nil {
		http.Error(w, "failed to load framework history", http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []models.ChildOnboardingFrameworkSummary{}
	}
	writeJSON(w, http.StatusOK, items)
}

// POST /v1/children/{childID}/onboarding/framework/approve
func (h *ChildrenHandler) ApproveOnboardingFramework(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		http.Error(w, "repository not initialised", http.StatusInternalServerError)
		return
	}
	childID, err := strconv.ParseInt(chi.URLParam(r, "childID"), 10, 64)
	if err != nil || childID <= 0 {
		http.Error(w, "invalid child id", http.StatusBadRequest)
		return
	}
	err = h.Repo.WithTx(r.Context(), nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(r.Context(), tx, childID, security.RoleManager)
		if err != nil {
			return err
		}
		baseline, err := tx.ApproveLatestSixDomainFramework(r.Context(), models.ChildID(childID), principal.DisplayName())
		if err != nil {
			return err
		}
		approved := true
		bound, err := tx.NewBaselineAuditEvent(r.Context(), principal, baseline.ID, storage.AuditEventBaselineApprove,
			security.AuditMetadataFields{PreviousStatus: security.AuditStatusSubmitted, NewStatus: security.AuditStatusApproved,
				Approved: &approved, RequestID: chimw.GetReqID(r.Context())}, storage.AuditRouteFrameworkApprove)
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
		case errors.Is(err, storage.ErrInvalidBaselineTransition), errors.Is(err, sql.ErrNoRows):
			http.Error(w, "framework must be submitted before approval", http.StatusBadRequest)
		default:
			http.Error(w, "failed to approve framework", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
