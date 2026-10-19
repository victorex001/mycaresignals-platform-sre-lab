package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/go-chi/chi/v5"

	appmw "therapeutic-engine/internal/middleware"
	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/storage"
)

type demoAuthorizationCognito struct{}

func (demoAuthorizationCognito) AdminAddUserToGroup(context.Context, *cognitoidentityprovider.AdminAddUserToGroupInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminAddUserToGroupOutput, error) {
	return &cognitoidentityprovider.AdminAddUserToGroupOutput{}, nil
}

func (demoAuthorizationCognito) AdminCreateUser(context.Context, *cognitoidentityprovider.AdminCreateUserInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminCreateUserOutput, error) {
	return &cognitoidentityprovider.AdminCreateUserOutput{}, nil
}

func (demoAuthorizationCognito) AdminGetUser(context.Context, *cognitoidentityprovider.AdminGetUserInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminGetUserOutput, error) {
	return &cognitoidentityprovider.AdminGetUserOutput{}, nil
}

func (demoAuthorizationCognito) AdminListGroupsForUser(context.Context, *cognitoidentityprovider.AdminListGroupsForUserInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminListGroupsForUserOutput, error) {
	return &cognitoidentityprovider.AdminListGroupsForUserOutput{}, nil
}

func demoAuthorizationHandler(t *testing.T, role string) http.Handler {
	t.Helper()
	subject := "demo-authorization-user"
	state := &protectedGoalDBState{
		staffExists: true,
		staffActive: true,
		role:        role,
		roleActive:  true,
		roleHomeID:  defaultDemoHomeID,
		childHomeID: defaultDemoHomeID,
	}
	repo := storage.NewRepo(openProtectedGoalDB(t, state))
	handler := &DemoStakeholderHandler{
		Repo:        repo,
		Cognito:     demoAuthorizationCognito{},
		UserPoolID:  "demo-test-pool",
		DemoHomeID:  defaultDemoHomeID,
		Environment: "demo",
	}
	router := chi.NewRouter()
	authenticator := protectedGoalAuthenticator(t, map[string]*models.StaffUser{subject: activeGoalStaff(subject)})
	router.With(appmw.Authenticate(authenticator)).Get("/v1/demo/stakeholders/access", handler.Access)
	router.With(appmw.Authenticate(authenticator)).Get("/v1/demo/stakeholders/invites", handler.ListInvites)
	router.With(appmw.Authenticate(authenticator)).Get("/v1/demo/stakeholders/product-feedback", handler.ListProductFeedback)
	return router
}

func demoAuthorizationRequest(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+protectedGoalToken("demo-authorization-user"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestDemoViewerCannotRetrieveAdminCollections(t *testing.T) {
	handler := demoAuthorizationHandler(t, "demo_viewer")
	for _, path := range []string{
		"/v1/demo/stakeholders/invites",
		"/v1/demo/stakeholders/product-feedback",
	} {
		response := demoAuthorizationRequest(t, handler, path)
		if response.Code != http.StatusForbidden {
			t.Fatalf("GET %s status = %d, want %d", path, response.Code, http.StatusForbidden)
		}
	}
}

func TestDemoAccessSeparatesAdminAndViewerCapabilities(t *testing.T) {
	tests := []struct {
		name      string
		role      string
		isAdmin   bool
		canInvite bool
		canSubmit bool
		canView   bool
	}{
		{name: "admin", role: "admin", isAdmin: true, canInvite: true, canSubmit: true, canView: true},
		{name: "viewer", role: "demo_viewer", isAdmin: false, canInvite: false, canSubmit: true, canView: false},
		{name: "manager", role: "manager", isAdmin: false, canInvite: false, canSubmit: false, canView: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := demoAuthorizationRequest(t, demoAuthorizationHandler(t, test.role), "/v1/demo/stakeholders/access")
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
			}
			var payload map[string]bool
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload["is_admin"] != test.isAdmin || payload["can_invite"] != test.canInvite || payload["can_submit_feedback"] != test.canSubmit || payload["can_view_feedback"] != test.canView {
				t.Fatalf("access = %#v", payload)
			}
		})
	}
}
