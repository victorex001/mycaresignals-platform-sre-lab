package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"therapeutic-engine/internal/handlers"
	"therapeutic-engine/internal/security"
)

type rejectingAuthenticator struct{}

func (rejectingAuthenticator) Authenticate(context.Context, string) (*security.Principal, error) {
	return nil, security.ErrInvalidToken
}

type rejectingAuthorizationStore struct{}

func (rejectingAuthorizationStore) StaffHasAnyHomeRole(context.Context, int64, int64, ...security.Role) (bool, error) {
	return false, nil
}

func (rejectingAuthorizationStore) GetChildHomeID(context.Context, int64) (int64, error) {
	return 0, nil
}

func (rejectingAuthorizationStore) GoalBelongsToChild(context.Context, int64, int64) (bool, error) {
	return false, nil
}

func (rejectingAuthorizationStore) TargetBelongsToChild(context.Context, int64, int64) (bool, error) {
	return false, nil
}

func TestSensitiveReadsAndExistingWritesRequireAuthenticationOnBothMounts(t *testing.T) {
	handler := New(Deps{
		ChildrenHandler:         &handlers.ChildrenHandler{},
		GoalsHandler:            &handlers.GoalsHandler{},
		RiskHandler:             &handlers.RiskHandler{},
		CarePlanHandler:         &handlers.CarePlanHandler{},
		CareOverviewHandler:     &handlers.CareOverviewHandler{},
		ExternalFeedbackHandler: &handlers.ExternalFeedbackHandler{},
		DailyProgressHandler:    &handlers.DailyProgressHandler{},
		DemoStakeholderHandler:  &handlers.DemoStakeholderHandler{},
		MissingFromHomeHandler:  &handlers.MissingFromHomePlanHandler{},
		LocalDevelopmentAccess:  handlers.NewLocalDevelopmentAccessHandler(rejectingDevelopmentManagerStore{}),
		TherapeuticHandler:      &handlers.TherapeuticHandler{},
		Authenticator:           rejectingAuthenticator{},
		AuthorizationStore:      rejectingAuthorizationStore{},
		FeedbackIntakeToken:     "test-service-token",
	})

	protected := []struct {
		method, path string
	}{
		{method: http.MethodGet, path: "/v1/homes/10/children"},
		{method: http.MethodGet, path: "/v1/children/100/goals"},
		{method: http.MethodGet, path: "/v1/children/100/care-overview"},
		{method: http.MethodGet, path: "/v1/children/100/daily-progress"},
		{method: http.MethodGet, path: "/v1/children/100/daily-progress/monthly"},
		{method: http.MethodPost, path: "/v1/children/100/daily-progress"},
		{method: http.MethodGet, path: "/v1/children/100/missing-from-home-plans"},
		{method: http.MethodGet, path: "/v1/children/100/missing-from-home-plans/reuse-candidates"},
		{method: http.MethodGet, path: "/v1/children/100/missing-from-home-plans/700"},
		{method: http.MethodPost, path: "/v1/children/100/missing-from-home-plans"},
		{method: http.MethodPut, path: "/v1/children/100/missing-from-home-plans/700"},
		{method: http.MethodPost, path: "/v1/children/100/missing-from-home-plans/700/submit"},
		{method: http.MethodPost, path: "/v1/children/100/missing-from-home-plans/700/approve"},
		{method: http.MethodPost, path: "/v1/children/100/missing-from-home-plans/700/revise"},
		{method: http.MethodPost, path: "/v1/children/100/missing-from-home-plans/700/archive"},
		{method: http.MethodGet, path: "/v1/children/100/targets"},
		{method: http.MethodGet, path: "/v1/children/100/sessions"},
		{method: http.MethodGet, path: "/v1/children/100/timeline"},
		{method: http.MethodGet, path: "/v1/children/100/domain_index"},
		{method: http.MethodGet, path: "/v1/children/100/checkins"},
		{method: http.MethodGet, path: "/v1/children/100/reviews"},
		{method: http.MethodPost, path: "/v1/children/100/reviews"},
		{method: http.MethodGet, path: "/v1/children/100/signal_responses"},
		{method: http.MethodGet, path: "/v1/children/100/onboarding/baseline"},
		{method: http.MethodGet, path: "/v1/children/100/onboarding/framework"},
		{method: http.MethodGet, path: "/v1/children/100/onboarding/framework/history"},
		{method: http.MethodPost, path: "/v1/children/100/goals"},
		{method: http.MethodPatch, path: "/v1/children/100/goals/300"},
		{method: http.MethodGet, path: "/v1/children/100/risks"},
		{method: http.MethodPost, path: "/v1/children/100/risks"},
		{method: http.MethodPatch, path: "/v1/children/100/risks/500"},
		{method: http.MethodGet, path: "/v1/children/100/risks/500/reviews"},
		{method: http.MethodPost, path: "/v1/children/100/risks/500/reviews"},
		{method: http.MethodGet, path: "/v1/children/100/care-plans"},
		{method: http.MethodGet, path: "/v1/children/100/care-plans/history"},
		{method: http.MethodGet, path: "/v1/children/100/care-plans/access"},
		{method: http.MethodGet, path: "/v1/children/100/care-plans/600"},
		{method: http.MethodGet, path: "/v1/children/100/care-plans/600/reviews"},
		{method: http.MethodGet, path: "/v1/children/100/care-plans/600/review-evidence"},
		{method: http.MethodPost, path: "/v1/children/100/care-plans"},
		{method: http.MethodPost, path: "/v1/children/100/care-plans/600/reviews"},
		{method: http.MethodPut, path: "/v1/children/100/care-plans/600/sections/about_me"},
		{method: http.MethodPost, path: "/v1/children/100/care-plans/600/activate"},
		{method: http.MethodPost, path: "/v1/children/100/care-plans/600/archive"},
		{method: http.MethodPost, path: "/v1/children/100/targets"},
		{method: http.MethodPatch, path: "/v1/children/100/targets/400"},
		{method: http.MethodPost, path: "/v1/children/100/sessions"},
		{method: http.MethodPost, path: "/v1/children/100/sessions/500/manager-review"},
		{method: http.MethodPost, path: "/v1/homes/10/children/100/onboarding/baseline"},
		{method: http.MethodPost, path: "/v1/homes/10/children/100/onboarding/framework"},
		{method: http.MethodPost, path: "/v1/homes/10/children/onboard"},
		{method: http.MethodGet, path: "/v1/children/100/monthly-voice"},
		{method: http.MethodGet, path: "/v1/children/100/monthly-voice/1"},
		{method: http.MethodGet, path: "/v1/children/100/monthly-voice/review-access"},
		{method: http.MethodPost, path: "/v1/children/100/monthly-voice/1/reviews"},
		{method: http.MethodPost, path: "/v1/children/100/onboarding/approve"},
		{method: http.MethodPost, path: "/v1/children/100/onboarding/framework/approve"},
		{method: http.MethodPost, path: "/v1/children/100/signal_responses"},
	}
	for _, prefix := range []string{"", "/api/te"} {
		for _, route := range protected {
			path := prefix + route.path
			t.Run(route.method+" "+path, func(t *testing.T) {
				request := httptest.NewRequest(route.method, path, nil)
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != http.StatusUnauthorized {
					t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusUnauthorized, response.Body.String())
				}
			})
		}
	}

	retiredVisitorRoutes := []struct {
		method, path string
	}{
		{method: http.MethodGet, path: "/v1/homes/10/visitor-feedback/access"},
		{method: http.MethodGet, path: "/v1/homes/10/visitor-feedback/review-access"},
		{method: http.MethodGet, path: "/v1/homes/10/visitor-feedback"},
		{method: http.MethodGet, path: "/v1/homes/10/visitor-feedback/export"},
		{method: http.MethodGet, path: "/v1/homes/10/visitor-feedback/1"},
		{method: http.MethodPost, path: "/v1/homes/10/visitor-feedback/1/reviews"},
	}
	for _, prefix := range []string{"", "/api/te"} {
		for _, route := range retiredVisitorRoutes {
			path := prefix + route.path
			t.Run(route.method+" retired "+path, func(t *testing.T) {
				request := httptest.NewRequest(route.method, path, nil)
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != http.StatusNotFound {
					t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
				}
			})
		}
	}

	publicOrUnresolved := []struct {
		method, path string
		want         int
	}{
		{method: http.MethodGet, path: "/healthz", want: http.StatusOK},
		{method: http.MethodPost, path: "/v1/children/100/demo_seed", want: http.StatusBadRequest},
		{method: http.MethodPost, path: "/api/v1/checkins", want: http.StatusNotFound},
		{method: http.MethodGet, path: "/api/v1/checkins", want: http.StatusNotFound},
	}
	for _, prefix := range []string{"", "/api/te"} {
		for _, route := range publicOrUnresolved {
			path := prefix + route.path
			t.Run(route.method+" unchanged "+path, func(t *testing.T) {
				request := httptest.NewRequest(route.method, path, nil)
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != route.want {
					t.Fatalf("status = %d, want %d; body=%s", response.Code, route.want, response.Body.String())
				}
			})
		}
	}
}

type rejectingDevelopmentManagerStore struct{}

func (rejectingDevelopmentManagerStore) GetLocalDevelopmentManagerSubject(context.Context) (string, error) {
	return "", nil
}
