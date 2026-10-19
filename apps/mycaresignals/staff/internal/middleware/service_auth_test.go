package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFeedbackIntakeServiceTokenFailsClosed(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := RequireFeedbackIntakeToken("synthetic-service-token")(next)

	for _, supplied := range []string{"", "wrong", "synthetic-service-token-extra"} {
		request := httptest.NewRequest(http.MethodPost, "/internal/v1/feedback-intake/receipts", nil)
		request.Header.Set("X-Feedback-Intake-Token", supplied)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("token %q status = %d, want 401", supplied, response.Code)
		}
	}
}

func TestFeedbackIntakeServiceTokenAllowsExactMatch(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := RequireFeedbackIntakeToken("synthetic-service-token")(next)
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/feedback-intake/receipts", nil)
	request.Header.Set("X-Feedback-Intake-Token", "synthetic-service-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
}
