package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	appmw "therapeutic-engine/internal/middleware"
	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/storage"
)

func protectedPsychologicalReviewHTTPHandler(
	t *testing.T,
	state *protectedSessionDBState,
	staff map[string]*models.StaffUser,
) http.Handler {
	t.Helper()
	repo := storage.NewRepo(openProtectedSessionDB(t, state))
	handler := NewTherapeuticHandler(repo)
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.With(appmw.Authenticate(protectedGoalAuthenticator(t, staff))).Post(
		"/v1/children/{childID}/reviews",
		handler.CreatePsychologicalReview,
	)
	return router
}

func psychologicalReviewRequest(handler http.Handler, token, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/children/100/reviews",
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func validPsychologicalReviewBody() string {
	return `{"window_days":42,"review_from":"2026-08-01","review_to":"2026-09-11","formulation":"Sensitive formulation","protective_factors":"Sensitive factors","recommendations":"Sensitive recommendations","created_by":"Caller Spoof"}`
}

func TestPsychologicalReviewRequiresAuthenticationAndPrivilegedRole(t *testing.T) {
	subject := "psych-review-policy-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	for _, test := range []struct {
		name  string
		role  string
		token string
		want  int
	}{
		{name: "senior accepted", role: "senior", token: token, want: http.StatusCreated},
		{name: "manager accepted", role: "manager", token: token, want: http.StatusCreated},
		{name: "key worker rejected", role: "key_worker", token: token, want: http.StatusForbidden},
		{name: "responsible individual rejected", role: "responsible_individual", token: token, want: http.StatusForbidden},
		{name: "missing token rejected", role: "manager", want: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedSessionState(test.role)
			response := psychologicalReviewRequest(
				protectedPsychologicalReviewHTTPHandler(t, state, staff),
				test.token,
				validPsychologicalReviewBody(),
			)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want != http.StatusCreated && (state.committedPsychReviews != 0 || state.committedAudits != 0) {
				t.Fatalf("rejected review committed: %#v", state)
			}
		})
	}
}

func TestPsychologicalReviewUsesAuthoritativeActorAndAtomicAudit(t *testing.T) {
	subject := "psych-review-success-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	state := authorizedSessionState("senior")
	response := psychologicalReviewRequest(
		protectedPsychologicalReviewHTTPHandler(t, state, staff),
		protectedGoalToken(subject),
		validPsychologicalReviewBody(),
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
	}
	if state.committedPsychReviews != 1 || state.committedAudits != 1 {
		t.Fatalf("committed writes: %#v", state)
	}
	if got := state.psychReviewArgs[7].Value; got != "Database Actor" || got == "Caller Spoof" {
		t.Fatalf("created_by = %v, want authoritative actor", got)
	}
	if state.auditArgs[5].Value != "psychological_review.create" || state.auditArgs[7].Value != int64(700) {
		t.Fatalf("audit binding = %#v", state.auditArgs[:8])
	}
	metadata, ok := state.auditArgs[10].Value.([]byte)
	if !ok {
		t.Fatalf("metadata type = %T", state.auditArgs[10].Value)
	}
	for _, forbidden := range []string{"Sensitive formulation", "Sensitive factors", "Sensitive recommendations", "Caller Spoof"} {
		if strings.Contains(string(metadata), forbidden) {
			t.Fatalf("audit metadata contains narrative %q: %s", forbidden, metadata)
		}
	}
	var payload models.PsychologicalReview
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.CreatedBy != "Database Actor" {
		t.Fatalf("response actor = %q", payload.CreatedBy)
	}
}

func TestPsychologicalReviewValidationRunsBeforeTransaction(t *testing.T) {
	subject := "psych-review-validation-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	for _, body := range []string{
		`{"review_from":"2026-09-12","review_to":"2026-09-11"}`,
		`{"window_days":366,"review_from":"2026-08-01","review_to":"2026-09-11"}`,
		`{"review_from":"not-a-date","review_to":"2026-09-11"}`,
	} {
		state := authorizedSessionState("manager")
		response := psychologicalReviewRequest(
			protectedPsychologicalReviewHTTPHandler(t, state, staff),
			token,
			body,
		)
		if response.Code != http.StatusBadRequest || state.begins != 0 {
			t.Fatalf("validation result: status=%d state=%#v body=%s", response.Code, state, response.Body.String())
		}
	}
}

func TestPsychologicalReviewFailuresRollBackMutationAndAudit(t *testing.T) {
	subject := "psych-review-failure-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	for _, test := range []struct {
		name      string
		configure func(*protectedSessionDBState)
	}{
		{name: "review insert", configure: func(state *protectedSessionDBState) { state.psychReviewErr = errors.New("review unavailable") }},
		{name: "audit insert", configure: func(state *protectedSessionDBState) { state.auditErr = errors.New("audit unavailable") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedSessionState("manager")
			test.configure(state)
			response := psychologicalReviewRequest(
				protectedPsychologicalReviewHTTPHandler(t, state, staff),
				token,
				validPsychologicalReviewBody(),
			)
			if response.Code != http.StatusInternalServerError || state.rollbacks != 1 {
				t.Fatalf("failure result: status=%d state=%#v", response.Code, state)
			}
			if state.committedPsychReviews != 0 || state.committedAudits != 0 {
				t.Fatalf("failed review committed: %#v", state)
			}
		})
	}
}
