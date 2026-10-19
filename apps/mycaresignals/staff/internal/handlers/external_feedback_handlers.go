package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
	"therapeutic-engine/internal/storage"
)

const maxFeedbackIntakeBody = 96 * 1024

var (
	uuidPattern   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	ruleIDPattern = regexp.MustCompile(`^[a-z0-9_]{1,80}$`)
)

type ExternalFeedbackStore interface {
	IngestExternalFeedback(context.Context, models.ExternalFeedbackIngest) (*models.ExternalFeedbackReceipt, error)
	ListChildMonthlyVoiceReceipts(context.Context, int64, string) ([]models.ExternalFeedbackReceipt, error)
	GetChildMonthlyVoiceReceipt(context.Context, int64, int64) (*models.ExternalFeedbackReceipt, error)
	ReviewExternalFeedback(context.Context, int64, int64, security.Principal, models.ExternalFeedbackReviewInput, string) (*models.ExternalFeedbackReview, error)
}

type ExternalFeedbackHandler struct{ Store ExternalFeedbackStore }

func NewExternalFeedbackHandler(store ExternalFeedbackStore) *ExternalFeedbackHandler {
	return &ExternalFeedbackHandler{Store: store}
}

func (h *ExternalFeedbackHandler) IngestReceipt(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeServerError(w, errors.New("feedback store unavailable"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFeedbackIntakeBody)
	var input models.ExternalFeedbackIngest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || !validFeedbackIngest(input) {
		writeBadRequest(w, "invalid feedback receipt")
		return
	}
	receipt, err := h.Store.IngestExternalFeedback(r.Context(), input)
	if err != nil {
		writeFeedbackError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, receipt)
}

func validFeedbackIngest(input models.ExternalFeedbackIngest) bool {
	if !uuidPattern.MatchString(input.ReceiptID) || input.HomeID <= 0 || input.ReceivedAt.IsZero() || input.ReceivedAt.After(time.Now().UTC().Add(10*time.Minute)) {
		return false
	}
	if input.ChildID != nil && *input.ChildID <= 0 {
		return false
	}
	if input.IntakeType != "young_person_voice" || input.SourceType != "YoungPersonVoice" {
		return false
	}
	if input.ReviewPriority != "routine" && input.ReviewPriority != "priority" && input.ReviewPriority != "urgent" {
		return false
	}
	if strings.TrimSpace(input.RuleVersion) == "" || len(input.RuleVersion) > 80 || len(input.MatchedRuleIDs) > 20 || len(input.RawPayload) == 0 || !json.Valid(input.RawPayload) {
		return false
	}
	for _, ruleID := range input.MatchedRuleIDs {
		if !ruleIDPattern.MatchString(ruleID) {
			return false
		}
	}
	return validVerifiedMonthlyVoice(input)
}

func validVerifiedMonthlyVoice(input models.ExternalFeedbackIngest) bool {
	if input.SourceType != "YoungPersonVoice" || input.ChildID == nil ||
		!uuidPattern.MatchString(input.IntakeContextID) ||
		input.IntakePurpose != models.YoungPersonMonthlyVoiceSource ||
		!models.ValidMonthlyVoiceRatings(input.DomainRatings) {
		return false
	}
	period, err := time.Parse("2006-01", input.PeriodMonth)
	if err != nil || period.Format("2006-01") != input.PeriodMonth {
		return false
	}
	receivedMonth := time.Date(input.ReceivedAt.UTC().Year(), input.ReceivedAt.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	if period.After(receivedMonth) {
		return false
	}
	preference := strings.TrimSpace(input.FollowUpPreference)
	if len(preference) > 500 || (!input.FollowUpRequested && preference != "") {
		return false
	}
	if input.FollowUpRequested && (!input.RequiresHumanReview || !containsString(input.MatchedRuleIDs, "follow_up_requested")) {
		return false
	}
	var raw struct {
		DomainRatings      map[string]int `json:"domain_ratings"`
		FollowUpRequested  *bool          `json:"q9_want_talk"`
		FollowUpPreference string         `json:"q9_follow_up_preference"`
		ContextToken       string         `json:"context_token"`
	}
	if json.Unmarshal(input.RawPayload, &raw) != nil || raw.FollowUpRequested == nil || raw.ContextToken != "" ||
		!equalRatings(raw.DomainRatings, input.DomainRatings) || *raw.FollowUpRequested != input.FollowUpRequested ||
		strings.TrimSpace(raw.FollowUpPreference) != preference {
		return false
	}
	return true
}

func equalRatings(left, right map[string]int) bool {
	if !models.ValidMonthlyVoiceRatings(left) || !models.ValidMonthlyVoiceRatings(right) || len(left) != len(right) {
		return false
	}
	for key, value := range left {
		other, ok := right[key]
		if !ok || other != value {
			return false
		}
	}
	return true
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func (h *ExternalFeedbackHandler) MonthlyVoiceReviewAccess(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *ExternalFeedbackHandler) ListChildMonthlyVoice(w http.ResponseWriter, r *http.Request) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil || childID <= 0 {
		writeBadRequest(w, "invalid child id")
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if !validFeedbackStatus(status) {
		writeBadRequest(w, "invalid triage status")
		return
	}
	items, err := h.Store.ListChildMonthlyVoiceReceipts(r.Context(), childID, status)
	if err != nil {
		writeServerError(w, err)
		return
	}
	stripMonthlyVoiceListDetails(items)
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *ExternalFeedbackHandler) GetChildMonthlyVoice(w http.ResponseWriter, r *http.Request) {
	childID, receiptID, err := childFeedbackRouteIDs(r)
	if err != nil {
		writeBadRequest(w, "invalid monthly voice receipt")
		return
	}
	item, err := h.Store.GetChildMonthlyVoiceReceipt(r.Context(), childID, receiptID)
	if err != nil {
		writeFeedbackError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *ExternalFeedbackHandler) ReviewChildMonthlyVoice(w http.ResponseWriter, r *http.Request) {
	childID, receiptID, err := childFeedbackRouteIDs(r)
	if err != nil {
		writeBadRequest(w, "invalid monthly voice receipt")
		return
	}
	item, err := h.Store.GetChildMonthlyVoiceReceipt(r.Context(), childID, receiptID)
	if err != nil {
		writeFeedbackError(w, err)
		return
	}
	h.reviewReceipt(w, r, item.HomeID, receiptID)
}

func (h *ExternalFeedbackHandler) reviewReceipt(w http.ResponseWriter, r *http.Request, homeID, receiptID int64) {
	principal, err := security.RequirePrincipal(r.Context())
	if err != nil {
		writeFeedbackError(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFeedbackIntakeBody)
	var input models.ExternalFeedbackReviewInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || !validFeedbackReview(input) {
		writeBadRequest(w, "invalid feedback review")
		return
	}
	review, err := h.Store.ReviewExternalFeedback(r.Context(), homeID, receiptID, principal, input, chimw.GetReqID(r.Context()))
	if err != nil {
		writeFeedbackError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, review)
}

func validFeedbackStatus(status string) bool {
	return status == "" || status == "new" || status == "acknowledged" || status == "follow_up" || status == "reviewed"
}

func stripMonthlyVoiceListDetails(items []models.ExternalFeedbackReceipt) {
	for i := range items {
		items[i].RawPayload = nil
		items[i].DomainRatings = nil
		items[i].MatchedRuleIDs = nil
		items[i].IntakeContextID = nil
	}
}

func childFeedbackRouteIDs(r *http.Request) (int64, int64, error) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil || childID <= 0 {
		return 0, 0, errors.New("invalid child")
	}
	receiptID, err := strconv.ParseInt(chi.URLParam(r, "receiptID"), 10, 64)
	if err != nil || receiptID <= 0 {
		return 0, 0, errors.New("invalid receipt")
	}
	return childID, receiptID, nil
}

func validFeedbackReview(input models.ExternalFeedbackReviewInput) bool {
	input.Rationale = strings.TrimSpace(input.Rationale)
	if !input.Outcome.Valid() || len(input.Rationale) < 3 || len(input.Rationale) > 2000 {
		return false
	}
	if input.Outcome == models.FeedbackOutcomeLinkAsEvidence || input.ChildID != nil || input.Domain != nil ||
		input.GoalID != nil || input.TargetID != nil || input.RiskID != nil {
		return false
	}
	return true
}

func writeFeedbackError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, security.ErrUnauthenticated):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	case errors.Is(err, security.ErrForbidden), errors.Is(err, security.ErrInvalidRelationship), errors.Is(err, storage.ErrFeedbackReceiptConflict):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, storage.ErrFeedbackReceiptNotFound), errors.Is(err, sql.ErrNoRows):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "feedback receipt not found"})
	default:
		writeServerError(w, err)
	}
}
