package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"therapeutic-engine/internal/models"
	progressview "therapeutic-engine/internal/progress"
	"therapeutic-engine/internal/security"
	"therapeutic-engine/internal/storage"
)

type DailyProgressStore interface {
	CreateDailyProgress(context.Context, int64, models.DailyProgressInput, string) (*models.ChildDailyProgress, error)
	ListDailyProgress(context.Context, int64, int) ([]models.ChildDailyProgress, error)
	ListDailyProgressMonthly(context.Context, int64, time.Time, time.Time) ([]models.MonthlyPerspectiveComparison, error)
}

type DailyProgressHandler struct {
	Store DailyProgressStore
	Now   func() time.Time
}

func NewDailyProgressHandler(store DailyProgressStore) *DailyProgressHandler {
	return &DailyProgressHandler{Store: store, Now: func() time.Time { return time.Now().UTC() }}
}

type dailyProgressRequest struct {
	ObservedOn         string `json:"observed_on"`
	Domain             string `json:"domain"`
	TargetID           *int64 `json:"target_id"`
	WellbeingScore     *int   `json:"wellbeing_score"`
	ProgressScore      *int   `json:"progress_score"`
	ObservationNote    string `json:"observation_note"`
	YoungPersonComment string `json:"young_person_comment"`
}

func (h *DailyProgressHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeServerError(w, errors.New("daily progress store unavailable"))
		return
	}
	if _, err := security.RequirePrincipal(r.Context()); err != nil {
		writeDailyProgressError(w, err)
		return
	}
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	var request dailyProgressRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeBadRequest(w, "invalid daily progress payload")
		return
	}
	now := h.now()
	input, err := validateDailyProgressRequest(request, now)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	entry, err := h.Store.CreateDailyProgress(r.Context(), childID, input, chimw.GetReqID(r.Context()))
	if err != nil {
		writeDailyProgressError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (h *DailyProgressHandler) List(w http.ResponseWriter, r *http.Request) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	limit := 20
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 || parsed > 100 {
			writeBadRequest(w, "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	items, err := h.Store.ListDailyProgress(r.Context(), childID, limit)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *DailyProgressHandler) Monthly(w http.ResponseWriter, r *http.Request) {
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil {
		writeBadRequest(w, "invalid child id")
		return
	}
	nowMonth := firstOfMonth(h.now())
	fromMonth := nowMonth.AddDate(0, -11, 0)
	toMonth := nowMonth
	if value := strings.TrimSpace(r.URL.Query().Get("from")); value != "" {
		fromMonth, err = time.Parse("2006-01", value)
		if err != nil {
			writeBadRequest(w, "from must be YYYY-MM")
			return
		}
	}
	if value := strings.TrimSpace(r.URL.Query().Get("to")); value != "" {
		toMonth, err = time.Parse("2006-01", value)
		if err != nil {
			writeBadRequest(w, "to must be YYYY-MM")
			return
		}
	}
	if toMonth.Before(fromMonth) || toMonth.AddDate(0, 1, 0).After(fromMonth.AddDate(2, 0, 0)) {
		writeBadRequest(w, "monthly range must be ordered and no longer than 24 months")
		return
	}
	items, err := h.Store.ListDailyProgressMonthly(r.Context(), childID, fromMonth, toMonth.AddDate(0, 1, 0))
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"rule": map[string]any{
			"version":                       progressview.PerspectiveRuleVersion,
			"absolute_difference_threshold": progressview.PerspectiveDifferenceThreshold,
			"consecutive_months":            progressview.PerspectiveConsecutiveMonths,
			"wording":                       progressview.PersistentDifferenceWording,
		},
	})
}

func validateDailyProgressRequest(request dailyProgressRequest, now time.Time) (models.DailyProgressInput, error) {
	observedOn := dayStartUTC(now)
	var err error
	if value := strings.TrimSpace(request.ObservedOn); value != "" {
		observedOn, err = time.Parse("2006-01-02", value)
		if err != nil {
			return models.DailyProgressInput{}, errors.New("observed_on must be YYYY-MM-DD")
		}
	}
	if observedOn.After(dayStartUTC(now)) {
		return models.DailyProgressInput{}, errors.New("observed_on cannot be in the future")
	}
	domain := strings.TrimSpace(request.Domain)
	if request.TargetID == nil && !models.ValidProgressDomain(domain) {
		return models.DailyProgressInput{}, errors.New("choose a valid domain")
	}
	if request.TargetID != nil && *request.TargetID <= 0 {
		return models.DailyProgressInput{}, errors.New("target_id must be positive")
	}
	for _, score := range []*int{request.WellbeingScore, request.ProgressScore} {
		if score != nil && (*score < 0 || *score > 4) {
			return models.DailyProgressInput{}, errors.New("scores must be between 0 and 4")
		}
	}
	note := strings.TrimSpace(request.ObservationNote)
	comment := strings.TrimSpace(request.YoungPersonComment)
	if len(note) > 2000 || len(comment) > 1000 {
		return models.DailyProgressInput{}, errors.New("daily progress text is too long")
	}
	if request.WellbeingScore == nil && request.ProgressScore == nil && note == "" {
		return models.DailyProgressInput{}, errors.New("record a score or concise observation")
	}
	return models.DailyProgressInput{
		ObservedOn: observedOn, Domain: domain, TargetID: request.TargetID,
		WellbeingScore: request.WellbeingScore, ProgressScore: request.ProgressScore,
		ObservationNote: optionalString(note), YoungPersonComment: optionalString(comment),
	}, nil
}

func writeDailyProgressError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, security.ErrUnauthenticated):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	case errors.Is(err, security.ErrForbidden), errors.Is(err, security.ErrInvalidRelationship),
		errors.Is(err, storage.ErrDailyProgressTargetNotActive), errors.Is(err, storage.ErrDailyProgressDomainMismatch):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, storage.ErrDailyProgressInvalidDomain):
		writeBadRequest(w, "choose a valid domain")
	default:
		writeServerError(w, err)
	}
}

func (h *DailyProgressHandler) now() time.Time {
	if h != nil && h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func firstOfMonth(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func dayStartUTC(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
