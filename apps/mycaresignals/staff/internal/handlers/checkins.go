// internal/handlers/checkins.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"therapeutic-engine/internal/storage"
)

// incoming JSON from qr-feedback / frontend
type DomainCheckInRequest struct {
	HomeName      string         `json:"home_name"`                // required
	ChildInitials *string        `json:"child_initials,omitempty"` // optional
	Date          string         `json:"date"`                     // RFC3339
	Context       *string        `json:"context,omitempty"`
	Domains       map[string]int `json:"domains"` // safety, relationships, etc.
	Notes         *string        `json:"notes,omitempty"`
}

func (h *TherapeuticHandler) PostCheckIn(w http.ResponseWriter, r *http.Request) {
	var req DomainCheckInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if req.HomeName == "" || req.Date == "" {
		http.Error(w, "home_name and date are required", http.StatusBadRequest)
		return
	}

	t, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}

	d := storage.DomainCheckIn{
		HomeName:      req.HomeName,
		ChildInitials: req.ChildInitials,
		Date:          t,
		Context:       req.Context,
		Notes:         req.Notes,
	}

	// Map domain scores from the incoming map into your DB struct.
	if v, ok := req.Domains["safety"]; ok {
		d.Safety = &v
	}
	if v, ok := req.Domains["relationships"]; ok {
		d.Relationships = &v
	}
	if v, ok := req.Domains["health"]; ok {
		d.Health = &v
	}
	if v, ok := req.Domains["education"]; ok {
		d.Education = &v
	}
	if v, ok := req.Domains["identity"]; ok {
		d.Identity = &v
	}
	if v, ok := req.Domains["coping"]; ok {
		d.Coping = &v
	}

	id, err := h.Repo.InsertDomainCheckIn(r.Context(), d)
	if err != nil {
		http.Error(w, "could not save", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"checkin_id": id,
		"message":    "check-in recorded",
	})
}

func (h *TherapeuticHandler) ListDomainCheckIns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	homeName := q.Get("home_name")
	alias := q.Get("alias") // child initials

	if homeName == "" || alias == "" {
		http.Error(w, "home_name and alias are required", http.StatusBadRequest)
		return
	}

	limit := 50
	if s := q.Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			limit = v
		}
	}

	items, err := h.Repo.ListDomainCheckInsForChild(r.Context(), homeName, alias, limit)
	if err != nil {
		http.Error(w, "could not load checkins", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	type respItem struct {
		ID            int64     `json:"id"`
		Date          time.Time `json:"date"`
		Context       *string   `json:"context,omitempty"`
		Safety        *int   `json:"safety,omitempty"`
		Relationships *int   `json:"relationships,omitempty"`
		Health        *int   `json:"health,omitempty"`
		Education     *int   `json:"education,omitempty"`
		Identity      *int   `json:"identity,omitempty"`
		Coping        *int   `json:"coping,omitempty"`
		Notes         *string `json:"notes,omitempty"`
	}

	out := make([]respItem, 0, len(items))
	for _, d := range items {
		out = append(out, respItem{
			ID:            d.ID,
			Date:          d.Date,
			Context:       d.Context,
			Safety:        d.Safety,
			Relationships: d.Relationships,
			Health:        d.Health,
			Education:     d.Education,
			Identity:      d.Identity,
			Coping:        d.Coping,
			Notes:         d.Notes,
		})
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"items": out,
	})
}

// --------- NEW: timeline / graph endpoint ----------

type DomainTimelinePoint struct {
	Date    time.Time      `json:"date"`
	Domains map[string]int `json:"domains"`
}

// GET /v1/children/{childID}/checkins?from=2026-01-01&to=2026-12-31
func (h *TherapeuticHandler) GetDomainTimeline(w http.ResponseWriter, r *http.Request) {
	childID, err := strconv.ParseInt(chi.URLParam(r, "childID"), 10, 64)
	if err != nil || childID <= 0 {
		http.Error(w, "invalid child id", http.StatusBadRequest)
		return
	}

	parseDate := func(name string) (*time.Time, error) {
		value := r.URL.Query().Get(name)
		if value == "" {
			return nil, nil
		}
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			return nil, err
		}
		return &parsed, nil
	}
	fromPtr, err := parseDate("from")
	if err != nil {
		http.Error(w, "invalid from date", http.StatusBadRequest)
		return
	}
	toPtr, err := parseDate("to")
	if err != nil {
		http.Error(w, "invalid to date", http.StatusBadRequest)
		return
	}
	if fromPtr != nil && toPtr != nil && fromPtr.After(*toPtr) {
		http.Error(w, "from must not be after to", http.StatusBadRequest)
		return
	}

	rows, err := h.Repo.ListDomainCheckInsForChildID(r.Context(), childID, fromPtr, toPtr)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	points := make([]DomainTimelinePoint, 0, len(rows))
	for _, d := range rows {
		dom := map[string]int{}
		if d.Safety != nil {
			dom["safety"] = *d.Safety
		}
		if d.Relationships != nil {
			dom["relationships"] = *d.Relationships
		}
		if d.Health != nil {
			dom["health"] = *d.Health
		}
		if d.Education != nil {
			dom["education"] = *d.Education
		}
		if d.Identity != nil {
			dom["identity"] = *d.Identity
		}
		if d.Coping != nil {
			dom["coping"] = *d.Coping
		}

		points = append(points, DomainTimelinePoint{
			Date:    d.Date,
			Domains: dom,
		})
	}

	response := map[string]any{
		"child_id": childID,
		"points":   points,
	}
	if len(rows) > 0 {
		response["home_name"] = rows[0].HomeName
		response["alias"] = rows[0].ChildInitials
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
