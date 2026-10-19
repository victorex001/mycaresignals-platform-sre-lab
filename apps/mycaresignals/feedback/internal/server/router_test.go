package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"qr-feedback-api/internal/config"
)

func TestPublicFeedbackListIsNotRouted(t *testing.T) {
	router := NewRouter(config.Config{AdminToken: "test-admin", SecretKey: "test-secret", RateLimitRPS: 100, RateLimitBurst: 100}, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/feedback", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("public GET status = %d, want 404", response.Code)
	}
}

func TestAdminFeedbackListStillRequiresAuthentication(t *testing.T) {
	router := NewRouter(config.Config{AdminToken: "test-admin", SecretKey: "test-secret", RateLimitRPS: 100, RateLimitBurst: 100}, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/feedback", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("admin GET status = %d, want 401", response.Code)
	}
}

func TestManagerWorkspaceAPIsAreNotPublic(t *testing.T) {
	router := NewRouter(config.Config{AdminToken: "test-admin", SecretKey: "test-secret", RateLimitRPS: 100, RateLimitBurst: 100}, nil)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/feedback/10000000-0000-4000-8000-000000000001"},
		{http.MethodPost, "/api/v1/admin/feedback/10000000-0000-4000-8000-000000000001/reviews"},
		{http.MethodGet, "/api/v1/admin/feedback/monthly-report?month=2026-08"},
		{http.MethodGet, "/api/v1/admin/feedback/export?month=2026-08&source=Parent"},
	} {
		request := httptest.NewRequest(route.method, route.path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", route.path, response.Code)
		}
	}
	request := httptest.NewRequest("POST", "/api/v1/feedback/10000000-0000-4000-8000-000000000001/reviews", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("public review status = %d, want 404", response.Code)
	}
}
