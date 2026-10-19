package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"therapeutic-engine/internal/models"
)

type careOverviewHandlerStore struct {
	childID int64
	now     time.Time
}

func (s *careOverviewHandlerStore) GetChildCareOverview(_ context.Context, childID int64, now time.Time) (*models.ChildCareOverview, error) {
	s.childID = childID
	s.now = now
	return &models.ChildCareOverview{Baseline: models.CareOverviewBaseline{Status: "not_started"}}, nil
}

func TestCareOverviewHandlerPassesOnlyRoutedChild(t *testing.T) {
	store := &careOverviewHandlerStore{}
	now := time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC)
	handler := NewCareOverviewHandler(store)
	handler.Now = func() time.Time { return now }
	router := chi.NewRouter()
	router.Get("/v1/children/{childID}/care-overview", handler.GetChildCareOverview)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/children/123/care-overview", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if store.childID != 123 || !store.now.Equal(now) {
		t.Fatalf("store received child/time %d/%s", store.childID, store.now)
	}
}

func TestCareOverviewHandlerRejectsInvalidChildID(t *testing.T) {
	handler := NewCareOverviewHandler(&careOverviewHandlerStore{})
	router := chi.NewRouter()
	router.Get("/v1/children/{childID}/care-overview", handler.GetChildCareOverview)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/children/not-a-child/care-overview", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}
