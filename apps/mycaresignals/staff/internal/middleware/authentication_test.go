package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"therapeutic-engine/internal/security"
)

type unavailableAuthenticator struct{}

func (unavailableAuthenticator) Authenticate(context.Context, string) (*security.Principal, error) {
	return nil, security.ErrAuthenticationUnavailable
}

func TestAuthenticateMapsAuthenticationStorageFailureToInternalServerError(t *testing.T) {
	nextCalled := false
	handler := Authenticate(unavailableAuthenticator{})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))

	request := httptest.NewRequest(http.MethodPost, "/v1/children/100/goals", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if nextCalled {
		t.Fatal("next handler was called after authentication storage failure")
	}
}
