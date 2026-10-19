package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"therapeutic-engine/internal/models"
)

type CareOverviewStore interface {
	GetChildCareOverview(context.Context, int64, time.Time) (*models.ChildCareOverview, error)
}

type CareOverviewHandler struct {
	Store CareOverviewStore
	Now   func() time.Time
}

func NewCareOverviewHandler(store CareOverviewStore) *CareOverviewHandler {
	return &CareOverviewHandler{Store: store, Now: func() time.Time { return time.Now().UTC() }}
}

func (h *CareOverviewHandler) GetChildCareOverview(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeServerError(w, errors.New("care overview store not initialised"))
		return
	}
	childID, err := parseChildID(chi.URLParam(r, "childID"))
	if err != nil || childID <= 0 {
		writeBadRequest(w, "invalid child id")
		return
	}
	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now().UTC()
	}
	overview, err := h.Store.GetChildCareOverview(r.Context(), childID, now)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}
