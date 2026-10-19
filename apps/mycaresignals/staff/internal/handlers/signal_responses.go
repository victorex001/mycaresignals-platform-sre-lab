// internal/handlers/signal_responses.go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
	"therapeutic-engine/internal/storage"
)

var errInvalidSignalResponsePayload = errors.New("invalid signal response payload")

type logSignalResponsePayload struct {
	SignalKind string `json:"signal_kind"`

	DidCheckIn      bool `json:"did_checkin"`
	AdjustedSupport bool `json:"adjusted_support"`
	InformedManager bool `json:"informed_manager"`
	InformedNetwork bool `json:"informed_network"`
	MonitorOnly     bool `json:"monitor_only"`

	// Referrals / multi-agency
	DidReferral          bool   `json:"did_referral"`
	ReferralDestinations string `json:"referral_destinations"`

	ActionsSummary string `json:"actions_summary"`
	FutureNotes    string `json:"future_notes"`

	// Where the full record lives (e.g. “ClearCare keywork 18/01/26 14:30”)
	ExternalLogRef string `json:"external_log_ref"`
}

// POST /v1/children/{childID}/signal_responses
func (h *ChildrenHandler) LogSignalResponse(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		http.Error(w, "repository not initialised", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	childIDStr := chi.URLParam(r, "childID")
	childID, err := strconv.ParseInt(childIDStr, 10, 64)
	if err != nil || childID <= 0 {
		http.Error(w, "invalid child id", http.StatusBadRequest)
		return
	}

	var payload logSignalResponsePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	resp := &models.ChildSignalResponse{
		ChildID:              models.ChildID(childID),
		SignalKind:           strings.TrimSpace(payload.SignalKind),
		DidCheckIn:           payload.DidCheckIn,
		AdjustedSupport:      payload.AdjustedSupport,
		InformedManager:      payload.InformedManager,
		InformedNetwork:      payload.InformedNetwork,
		MonitorOnly:          payload.MonitorOnly,
		DidReferral:          payload.DidReferral,
		ReferralDestinations: strings.TrimSpace(payload.ReferralDestinations),
		ActionsSummary:       strings.TrimSpace(payload.ActionsSummary),
		FutureNotes:          strings.TrimSpace(payload.FutureNotes),
		ExternalLogRef:       strings.TrimSpace(payload.ExternalLogRef),
	}

	err = h.Repo.WithTx(ctx, nil, func(tx *storage.TxRepo) error {
		_, principal, err := security.RequireChildHomeRoleForMutation(
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
		if strings.TrimSpace(resp.SignalKind) == "" {
			return errInvalidSignalResponsePayload
		}
		if err := tx.CreateSignalResponse(ctx, resp); err != nil {
			return err
		}
		bound, err := tx.NewSignalResponseAuditEvent(
			ctx,
			principal,
			resp.ID,
			storage.AuditEventSignalResponseCreate,
			security.AuditMetadataFields{RequestID: chimw.GetReqID(ctx)},
			storage.AuditRouteSignalResponses,
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
		if errors.Is(err, errInvalidSignalResponsePayload) {
			http.Error(w, "signal_kind is required", http.StatusBadRequest)
			return
		}
		http.Error(w, "could not save signal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// GET /v1/children/{childID}/signal_responses?limit=3
func (h *ChildrenHandler) ListSignalResponses(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Children == nil {
		http.Error(w, "children store not initialised", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	childIDStr := chi.URLParam(r, "childID")
	childID, err := strconv.ParseInt(childIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid child id", http.StatusBadRequest)
		return
	}

	limit := 20
	if ls := r.URL.Query().Get("limit"); ls != "" {
		if v, err := strconv.Atoi(ls); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	resps, err := h.Children.ListSignalResponses(ctx, models.ChildID(childID), limit)
	if err != nil {
		http.Error(w, "failed to load signal responses", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resps)
}
