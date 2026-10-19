package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeLocalDevelopmentManagerStore struct {
	subject string
	err     error
}

func (f fakeLocalDevelopmentManagerStore) GetLocalDevelopmentManagerSubject(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.subject, nil
}

func TestLocalDevelopmentManagerTokenRequiresLoopbackAndDevConfig(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("AUTH_MODE", "dev-jwt")
	t.Setenv("ENABLE_DEV_AUTH", "true")
	t.Setenv("DEV_JWT_ISSUER", "issuer")
	t.Setenv("DEV_JWT_AUDIENCE", "audience")
	t.Setenv("DEV_JWT_HMAC_SECRET", base64.RawURLEncoding.EncodeToString([]byte("01234567890123456789012345678901")))

	handler := NewLocalDevelopmentAccessHandler(fakeLocalDevelopmentManagerStore{subject: "manager-subject"})

	request := httptest.NewRequest(http.MethodPost, "/internal/dev/local-manager-token", nil)
	request.RemoteAddr = "127.0.0.1:51234"
	response := httptest.NewRecorder()
	handler.IssueManagerToken(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("loopback development status = %d, body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if strings.Count(body.Token, ".") != 2 {
		t.Fatalf("token dot count = %d, want 2", strings.Count(body.Token, "."))
	}

	request = httptest.NewRequest(http.MethodPost, "/internal/dev/local-manager-token", nil)
	request.RemoteAddr = "203.0.113.10:51234"
	response = httptest.NewRecorder()
	handler.IssueManagerToken(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("non-loopback status = %d, want %d", response.Code, http.StatusNotFound)
	}

	t.Setenv("ALLOW_CONTAINER_DEV_AUTH", "true")
	request = httptest.NewRequest(http.MethodPost, "/internal/dev/local-manager-token", nil)
	request.RemoteAddr = "172.20.0.2:51234"
	response = httptest.NewRecorder()
	handler.IssueManagerToken(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("container development status = %d, body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/internal/dev/local-manager-token", nil)
	request.RemoteAddr = "203.0.113.10:51234"
	response = httptest.NewRecorder()
	handler.IssueManagerToken(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("public address with container development enabled status = %d, want %d", response.Code, http.StatusNotFound)
	}

	t.Setenv("APP_ENV", "production")
	request = httptest.NewRequest(http.MethodPost, "/internal/dev/local-manager-token", nil)
	request.RemoteAddr = "127.0.0.1:51234"
	response = httptest.NewRecorder()
	handler.IssueManagerToken(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("production status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
