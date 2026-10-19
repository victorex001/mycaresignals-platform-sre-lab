package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
)

type readTestStaffLookup struct {
	staff *models.StaffUser
	err   error
}

func (l readTestStaffLookup) GetStaffUserByExternalAuthSubject(context.Context, string) (*models.StaffUser, error) {
	return l.staff, l.err
}

type readTestAuthorizationStore struct {
	role           security.Role
	roleHomeID     int64
	roleActive     bool
	childHomeID    int64
	childExists    bool
	authorizeErr   error
	childLookupErr error
}

func (s readTestAuthorizationStore) StaffHasAnyHomeRole(_ context.Context, _ int64, homeID int64, allowedRoles ...security.Role) (bool, error) {
	if s.authorizeErr != nil {
		return false, s.authorizeErr
	}
	if !s.roleActive || s.roleHomeID != homeID {
		return false, nil
	}
	for _, allowed := range allowedRoles {
		if s.role == allowed {
			return true, nil
		}
	}
	return false, nil
}

func (s readTestAuthorizationStore) GetChildHomeID(context.Context, int64) (int64, error) {
	if s.childLookupErr != nil {
		return 0, s.childLookupErr
	}
	if !s.childExists {
		return 0, sql.ErrNoRows
	}
	return s.childHomeID, nil
}

func (readTestAuthorizationStore) GoalBelongsToChild(context.Context, int64, int64) (bool, error) {
	return false, nil
}

func (readTestAuthorizationStore) TargetBelongsToChild(context.Context, int64, int64) (bool, error) {
	return false, nil
}

var childReadRoles = []security.Role{
	security.RoleKeyWorker,
	security.RoleSenior,
	security.RoleManager,
	security.RoleResponsibleIndividual,
}

func TestAuthorizeHomeReadRolePolicy(t *testing.T) {
	for _, role := range childReadRoles {
		t.Run(string(role), func(t *testing.T) {
			authenticator, token := readTestAuthenticator(t, "active")
			store := readTestAuthorizationStore{role: role, roleHomeID: 10, roleActive: true}
			response := serveProtectedRead(t, authenticator, token, store, true, "/v1/homes/10/children")
			if response.Code != http.StatusOK || response.Body.String() != `{"items":[]}` {
				t.Fatalf("response = (%d, %q), want (200, preserved body)", response.Code, response.Body.String())
			}
		})
	}

	tests := []struct {
		name  string
		token bool
		store readTestAuthorizationStore
		want  int
	}{
		{name: "unauthenticated", store: readTestAuthorizationStore{}, want: http.StatusUnauthorized},
		{name: "wrong home", token: true, store: readTestAuthorizationStore{role: security.RoleManager, roleHomeID: 11, roleActive: true}, want: http.StatusForbidden},
		{name: "inactive role", token: true, store: readTestAuthorizationStore{role: security.RoleManager, roleHomeID: 10}, want: http.StatusForbidden},
		{name: "admin excluded", token: true, store: readTestAuthorizationStore{role: security.RoleAdmin, roleHomeID: 10, roleActive: true}, want: http.StatusForbidden},
		{name: "authorization database error", token: true, store: readTestAuthorizationStore{authorizeErr: errors.New("database unavailable")}, want: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authenticator, token := readTestAuthenticator(t, "active")
			if !test.token {
				token = ""
			}
			response := serveProtectedRead(t, authenticator, token, test.store, true, "/v1/homes/10/children")
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}

func TestAuthorizeChildReadPolicyAndNonDisclosure(t *testing.T) {
	authenticator, token := readTestAuthenticator(t, "active")
	tests := []struct {
		name  string
		store readTestAuthorizationStore
		want  int
	}{
		{name: "permitted own-home role", store: readTestAuthorizationStore{role: security.RoleKeyWorker, roleHomeID: 10, roleActive: true, childHomeID: 10, childExists: true}, want: http.StatusOK},
		{name: "wrong-home staff", store: readTestAuthorizationStore{role: security.RoleKeyWorker, roleHomeID: 11, roleActive: true, childHomeID: 10, childExists: true}, want: http.StatusForbidden},
		{name: "inactive role", store: readTestAuthorizationStore{role: security.RoleKeyWorker, roleHomeID: 10, childHomeID: 10, childExists: true}, want: http.StatusForbidden},
		{name: "admin excluded", store: readTestAuthorizationStore{role: security.RoleAdmin, roleHomeID: 10, roleActive: true, childHomeID: 10, childExists: true}, want: http.StatusForbidden},
		{name: "missing child", store: readTestAuthorizationStore{role: security.RoleKeyWorker, roleHomeID: 10, roleActive: true}, want: http.StatusForbidden},
		{name: "child lookup database error", store: readTestAuthorizationStore{childLookupErr: errors.New("database unavailable")}, want: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := serveProtectedRead(t, authenticator, token, test.store, false, "/v1/children/100/goals")
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
			if test.want == http.StatusOK && response.Body.String() != `{"items":[]}` {
				t.Fatalf("successful response body = %q, want preserved body", response.Body.String())
			}
			if test.want == http.StatusForbidden && response.Body.String() != "forbidden\n" {
				t.Fatalf("forbidden body = %q, want non-disclosing response", response.Body.String())
			}
		})
	}
}

func TestAuthorizeChildReadRejectsInactiveStaff(t *testing.T) {
	authenticator, token := readTestAuthenticator(t, "inactive")
	store := readTestAuthorizationStore{role: security.RoleManager, roleHomeID: 10, roleActive: true, childHomeID: 10, childExists: true}
	response := serveProtectedRead(t, authenticator, token, store, false, "/v1/children/100/goals")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func serveProtectedRead(t *testing.T, authenticator security.Authenticator, token string, store security.AuthorizationStore, homeScoped bool, path string) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	middlewares := []func(http.Handler) http.Handler{Authenticate(authenticator)}
	if homeScoped {
		middlewares = append(middlewares, AuthorizeHomeRead(store, childReadRoles...))
		router.With(middlewares...).Get("/v1/homes/{homeID}/children", preservedReadResponse)
	} else {
		middlewares = append(middlewares, AuthorizeChildRead(store, childReadRoles...))
		router.With(middlewares...).Get("/v1/children/{childID}/goals", preservedReadResponse)
	}
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func preservedReadResponse(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"items":[]}`))
}

func readTestAuthenticator(t *testing.T, status string) (security.Authenticator, string) {
	t.Helper()
	const (
		issuer   = "read-test-issuer"
		audience = "read-test-audience"
		secret   = "read-test-secret-0123456789012345"
		subject  = "read-test-subject"
	)
	t.Setenv("APP_ENV", "development")
	t.Setenv("AUTH_MODE", "dev-jwt")
	t.Setenv("ENABLE_DEV_AUTH", "true")
	t.Setenv("DEV_JWT_ISSUER", issuer)
	t.Setenv("DEV_JWT_AUDIENCE", audience)
	t.Setenv("DEV_JWT_HMAC_SECRET", secret)
	staff := &models.StaffUser{ID: 7, Email: "reader@example.test", DisplayName: "Reader", ExternalAuthSubject: stringPointer(subject), Status: status}
	authenticator, err := security.NewDevJWTAuthenticatorFromEnv(readTestStaffLookup{staff: staff})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := json.Marshal(map[string]any{"iss": issuer, "aud": audience, "sub": subject, "exp": time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	payload := base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(header + "." + payload))
	token := header + "." + payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return authenticator, token
}

func stringPointer(value string) *string {
	return &value
}
