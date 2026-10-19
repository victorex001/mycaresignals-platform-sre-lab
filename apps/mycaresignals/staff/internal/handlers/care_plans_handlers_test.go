package handlers

import (
	"bytes"
	"encoding/json"
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

func protectedCarePlanHTTPHandler(t *testing.T, state *protectedGoalDBState, staff map[string]*models.StaffUser) http.Handler {
	t.Helper()
	repo := storage.NewRepo(openProtectedGoalDB(t, state))
	handler := NewCarePlanHandler(repo)
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.Group(func(r chi.Router) {
		r.Use(appmw.Authenticate(protectedGoalAuthenticator(t, staff)))
		r.Get("/v1/children/{childID}/care-plans/access", handler.GetCarePlanAccess)
		r.Post("/v1/children/{childID}/care-plans", handler.CreateCarePlan)
		r.Put("/v1/children/{childID}/care-plans/{carePlanID}/sections/{sectionKey}", handler.UpdateCarePlanSection)
		r.Post("/v1/children/{childID}/care-plans/{carePlanID}/activate", handler.ActivateCarePlan)
		r.Post("/v1/children/{childID}/care-plans/{carePlanID}/archive", handler.ArchiveCarePlan)
		r.Post("/v1/children/{childID}/care-plans/{carePlanID}/reviews", handler.CreateCarePlanReview)
	})
	return router
}

func TestCarePlanAccessSeparatesEditingFromManagerReview(t *testing.T) {
	subject := "care-plan-access"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	for _, test := range []struct {
		role                 string
		wantEdit, wantReview bool
	}{
		{role: "key_worker", wantEdit: false, wantReview: false},
		{role: "senior", wantEdit: true, wantReview: false},
		{role: "manager", wantEdit: true, wantReview: true},
		{role: "responsible_individual", wantEdit: false, wantReview: false},
	} {
		t.Run(test.role, func(t *testing.T) {
			state := &protectedGoalDBState{
				staffExists: true, staffActive: true, role: test.role, roleActive: true, childExists: true,
			}
			response := carePlanRequest(protectedCarePlanHTTPHandler(t, state, staff), http.MethodGet,
				"/v1/children/100/care-plans/access", protectedGoalToken(subject), "")
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			var body carePlanAccessResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.CanEdit != test.wantEdit || body.CanReview != test.wantReview {
				t.Fatalf("access=%+v want edit=%t review=%t", body, test.wantEdit, test.wantReview)
			}
		})
	}
}

func TestCarePlanReviewRequiresManagerAndAuthoritativeChildScope(t *testing.T) {
	subject := "care-plan-review-auth"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	for _, test := range []struct {
		name        string
		role        string
		planChildID int64
		want        int
	}{
		{name: "senior rejected", role: "senior", planChildID: 100, want: http.StatusForbidden},
		{name: "cross child rejected", role: "manager", planChildID: 200, want: http.StatusForbidden},
		{name: "manager allowed", role: "manager", planChildID: 100, want: http.StatusCreated},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := &protectedGoalDBState{
				staffExists: true, staffActive: true, role: test.role, roleActive: true,
				childExists: true, carePlanExists: true, carePlanChildID: test.planChildID,
				carePlanStatus: "active",
			}
			response := carePlanRequest(protectedCarePlanHTTPHandler(t, state, staff), http.MethodPost,
				"/v1/children/100/care-plans/600/reviews", token,
				`{"review_note":"Professional review note","young_person_involvement_note":"The review was discussed.","outcome":"continue","next_review_date":"2026-10-01"}`)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want == http.StatusCreated {
				if state.committedCarePlanReviews != 1 || state.committedCarePlanSnapshots != len(models.CanonicalCarePlanSectionKeys) || state.committedCarePlanStatuses != 1 {
					t.Fatalf("review writes=%#v", state)
				}
				if state.committedCarePlans != 0 || state.committedAudits != 1 {
					t.Fatalf("continue created successor or missed audit: %#v", state)
				}
			} else if state.committedCarePlanReviews != 0 || state.committedCarePlanSnapshots != 0 {
				t.Fatalf("rejected review committed writes: %#v", state)
			}
		})
	}
}

func TestCarePlanReviseCreatesCopiedSuccessorWithoutChangingActivePlan(t *testing.T) {
	subject := "care-plan-review-revise"
	state := &protectedGoalDBState{
		staffExists: true, staffActive: true, role: "manager", roleActive: true,
		childExists: true, carePlanExists: true, carePlanChildID: 100, carePlanStatus: "active",
	}
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	response := carePlanRequest(protectedCarePlanHTTPHandler(t, state, staff), http.MethodPost,
		"/v1/children/100/care-plans/600/reviews", protectedGoalToken(subject),
		`{"review_note":"The plan needs revision.","outcome":"revise","next_review_date":"2026-11-01"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if state.committedCarePlans != 1 || state.committedCarePlanSections != len(models.CanonicalCarePlanSectionKeys) {
		t.Fatalf("successor writes=%#v", state)
	}
	if state.committedCarePlanReviews != 1 || state.committedCarePlanSnapshots != len(models.CanonicalCarePlanSectionKeys) {
		t.Fatalf("review snapshot writes=%#v", state)
	}
	if state.committedCarePlanStatuses != 0 {
		t.Fatalf("revise mutated active plan status: %#v", state)
	}
	if state.committedAudits != 2 {
		t.Fatalf("audits=%d want successor + review", state.committedAudits)
	}
	for _, expected := range []string{"care_plan_successor_insert", "care_plan_review_insert", "care_plan_snapshot_insert"} {
		if !containsQueryStep(state.queryOrder, expected) {
			t.Fatalf("query order missing %q: %v", expected, state.queryOrder)
		}
	}
}

func TestCarePlanReviewValidationOccursBeforeTransaction(t *testing.T) {
	state := &protectedGoalDBState{}
	staff := map[string]*models.StaffUser{"review-validation": activeGoalStaff("review-validation")}
	for _, body := range []string{
		`{"review_note":"","outcome":"continue","next_review_date":"2026-10-01"}`,
		`{"review_note":"Review","outcome":"continue"}`,
		`{"review_note":"Review","outcome":"automatic_decision"}`,
	} {
		response := carePlanRequest(protectedCarePlanHTTPHandler(t, state, staff), http.MethodPost,
			"/v1/children/100/care-plans/600/reviews", protectedGoalToken("review-validation"), body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	}
	if state.begins != 0 {
		t.Fatalf("invalid payload began %d transactions", state.begins)
	}
}

func TestCarePlanReviewAuditFailureRollsBackReviewSnapshotAndPlanUpdate(t *testing.T) {
	subject := "care-plan-review-rollback"
	state := &protectedGoalDBState{
		staffExists: true, staffActive: true, role: "manager", roleActive: true,
		childExists: true, carePlanExists: true, carePlanChildID: 100, carePlanStatus: "active",
		auditErr: errSyntheticAuditUnavailable,
	}
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	response := carePlanRequest(protectedCarePlanHTTPHandler(t, state, staff), http.MethodPost,
		"/v1/children/100/care-plans/600/reviews", protectedGoalToken(subject),
		`{"review_note":"Synthetic review","outcome":"continue","next_review_date":"2026-10-01"}`)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if state.committedCarePlanReviews != 0 || state.committedCarePlanSnapshots != 0 || state.committedCarePlanStatuses != 0 || state.committedAudits != 0 {
		t.Fatalf("audit failure left partial review writes: %#v", state)
	}
}

func TestCarePlanCloseReviewArchivesOnlyThePlan(t *testing.T) {
	subject := "care-plan-review-close"
	state := &protectedGoalDBState{
		staffExists: true, staffActive: true, role: "manager", roleActive: true,
		childExists: true, carePlanExists: true, carePlanChildID: 100, carePlanStatus: "active",
	}
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	response := carePlanRequest(protectedCarePlanHTTPHandler(t, state, staff), http.MethodPost,
		"/v1/children/100/care-plans/600/reviews", protectedGoalToken(subject),
		`{"review_note":"Synthetic closure review","outcome":"close"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if state.committedCarePlanReviews != 1 || state.committedCarePlanSnapshots != len(models.CanonicalCarePlanSectionKeys) || state.committedCarePlanStatuses != 1 || state.committedAudits != 2 {
		t.Fatalf("close review writes=%#v", state)
	}
	if state.committedGoals != 0 || state.committedTargets != 0 || state.committedRisks != 0 || state.committedReviews != 0 {
		t.Fatalf("close review changed linked records: %#v", state)
	}
}

func containsQueryStep(steps []string, expected string) bool {
	for _, step := range steps {
		if step == expected {
			return true
		}
	}
	return false
}

func carePlanRequest(handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestCreateCarePlanAuthorizationAndCanonicalSections(t *testing.T) {
	subject := "care-plan-create"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	for _, test := range []struct {
		name, role, token string
		want              int
	}{
		{name: "unauthenticated", role: "manager", want: http.StatusUnauthorized},
		{name: "key worker rejected", role: "key_worker", token: token, want: http.StatusForbidden},
		{name: "senior allowed", role: "senior", token: token, want: http.StatusCreated},
		{name: "manager allowed", role: "manager", token: token, want: http.StatusCreated},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := &protectedGoalDBState{
				staffExists: true, staffActive: true, role: test.role, roleActive: true,
				childExists: true, childHomeID: 10,
			}
			response := carePlanRequest(
				protectedCarePlanHTTPHandler(t, state, staff), http.MethodPost,
				"/v1/children/100/care-plans", test.token,
				`{"effective_from":"2026-08-19","review_date":"2026-09-19"}`,
			)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want == http.StatusCreated {
				if state.committedCarePlans != 1 || state.committedCarePlanSections != len(models.CanonicalCarePlanSectionKeys) || state.committedAudits != 1 {
					t.Fatalf("committed care plan writes = %#v", state)
				}
				if state.carePlanArgs[4].Value != int64(7) {
					t.Fatalf("created_by_staff_id = %v, want authoritative staff 7", state.carePlanArgs[4].Value)
				}
				if state.auditArgs[5].Value != "care_plan.create" || state.auditArgs[6].Value != "care_plan" {
					t.Fatalf("care plan audit = %#v", state.auditArgs)
				}
			} else if state.committedCarePlans != 0 || state.committedCarePlanSections != 0 || state.committedAudits != 0 {
				t.Fatalf("unauthorized care plan write committed: %#v", state)
			}
		})
	}
}

func TestCarePlanSectionAcceptsUnknownStatesWithoutNarrative(t *testing.T) {
	subject := "care-plan-section"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	for _, status := range []string{"not_known_yet", "awaiting_confirmation"} {
		t.Run(status, func(t *testing.T) {
			state := &protectedGoalDBState{
				staffExists: true, staffActive: true, role: "senior", roleActive: true,
				childExists: true, carePlanExists: true, carePlanChildID: 100, carePlanStatus: "draft",
			}
			response := carePlanRequest(
				protectedCarePlanHTTPHandler(t, state, staff), http.MethodPut,
				"/v1/children/100/care-plans/600/sections/about_me", token,
				`{"information_status":"`+status+`"}`,
			)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
			}
			if state.committedCarePlanSections != 1 || state.committedAudits != 1 {
				t.Fatalf("section writes = %#v", state)
			}
			for _, index := range []int{3, 4, 5, 6} {
				if state.carePlanSectionArgs[index].Value != nil {
					t.Fatalf("optional narrative arg %d = %v, want nil", index, state.carePlanSectionArgs[index].Value)
				}
			}
			if state.auditArgs[5].Value != "care_plan.section.update" || strings.Contains(string(state.auditArgs[10].Value.([]byte)), "current_understanding") {
				t.Fatalf("section audit = %#v", state.auditArgs)
			}
		})
	}
}

func TestCarePlanSectionRejectsCrossChildAndInvalidStatus(t *testing.T) {
	subject := "care-plan-section-scope"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	t.Run("cross child", func(t *testing.T) {
		state := &protectedGoalDBState{
			staffExists: true, staffActive: true, role: "manager", roleActive: true,
			childExists: true, carePlanExists: true, carePlanChildID: 200,
		}
		response := carePlanRequest(protectedCarePlanHTTPHandler(t, state, staff), http.MethodPut,
			"/v1/children/100/care-plans/600/sections/health", token, `{"information_status":"known"}`)
		if response.Code != http.StatusForbidden || state.committedCarePlanSections != 0 || state.committedAudits != 0 {
			t.Fatalf("cross-child response=%d state=%#v", response.Code, state)
		}
	})
	t.Run("invalid information status", func(t *testing.T) {
		state := &protectedGoalDBState{}
		response := carePlanRequest(protectedCarePlanHTTPHandler(t, state, staff), http.MethodPut,
			"/v1/children/100/care-plans/600/sections/health", token, `{"information_status":"complete"}`)
		if response.Code != http.StatusBadRequest || state.begins != 0 {
			t.Fatalf("invalid status response=%d state=%#v", response.Code, state)
		}
	})
}

func TestCarePlanActivationRequiresManagerAndOneActivePlan(t *testing.T) {
	subject := "care-plan-activation"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	for _, test := range []struct {
		name, role string
		other      bool
		want       int
	}{
		{name: "senior rejected", role: "senior", want: http.StatusForbidden},
		{name: "manager activates", role: "manager", want: http.StatusOK},
		{name: "other active plan rejected", role: "manager", other: true, want: http.StatusConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := &protectedGoalDBState{
				staffExists: true, staffActive: true, role: test.role, roleActive: true,
				childExists: true, carePlanExists: true, carePlanChildID: 100,
				carePlanStatus: "draft", otherActiveCarePlan: test.other,
			}
			response := carePlanRequest(protectedCarePlanHTTPHandler(t, state, staff), http.MethodPost,
				"/v1/children/100/care-plans/600/activate", token, `{}`)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want == http.StatusOK {
				if state.committedCarePlanStatuses != 1 || state.committedAudits != 1 || state.carePlanStatusArgs[2].Value != int64(7) {
					t.Fatalf("activation writes = %#v", state)
				}
				if state.auditArgs[5].Value != "care_plan.activate" {
					t.Fatalf("activation audit = %#v", state.auditArgs)
				}
			} else if state.committedCarePlanStatuses != 0 || state.committedAudits != 0 {
				t.Fatalf("rejected activation committed: %#v", state)
			}
		})
	}
}

func TestSuccessorActivationArchivesPredecessorAndActivatesDraftAtomically(t *testing.T) {
	subject := "care-plan-successor-activation"
	state := &protectedGoalDBState{
		staffExists: true, staffActive: true, role: "manager", roleActive: true,
		childExists: true, carePlanExists: true, carePlanChildID: 100,
		carePlanStatus: "draft", carePlanPredecessorID: int64(599), predecessorPlanStatus: "active",
	}
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	response := carePlanRequest(protectedCarePlanHTTPHandler(t, state, staff), http.MethodPost,
		"/v1/children/100/care-plans/600/activate", protectedGoalToken(subject), `{}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if state.committedCarePlanStatuses != 2 || state.committedAudits != 2 {
		t.Fatalf("successor transition writes=%#v", state)
	}
	if state.commits != 1 || state.rollbacks != 0 {
		t.Fatalf("transaction commits=%d rollbacks=%d", state.commits, state.rollbacks)
	}
}

func TestSuccessorActivationAuditFailureRollsBackBothTransitions(t *testing.T) {
	subject := "care-plan-successor-rollback"
	state := &protectedGoalDBState{
		staffExists: true, staffActive: true, role: "manager", roleActive: true,
		childExists: true, carePlanExists: true, carePlanChildID: 100,
		carePlanStatus: "draft", carePlanPredecessorID: int64(599), predecessorPlanStatus: "active",
		auditErr: errSyntheticAuditUnavailable,
	}
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	response := carePlanRequest(protectedCarePlanHTTPHandler(t, state, staff), http.MethodPost,
		"/v1/children/100/care-plans/600/activate", protectedGoalToken(subject), `{}`)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if state.committedCarePlanStatuses != 0 || state.committedAudits != 0 || state.rollbacks == 0 {
		t.Fatalf("partial successor activation committed: %#v", state)
	}
}

func TestCarePlanAuditFailuresRollbackAllWrites(t *testing.T) {
	subject := "care-plan-rollback"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	state := &protectedGoalDBState{
		staffExists: true, staffActive: true, role: "manager", roleActive: true,
		childExists: true, auditErr: errSyntheticAuditUnavailable,
	}
	response := carePlanRequest(protectedCarePlanHTTPHandler(t, state, staff), http.MethodPost,
		"/v1/children/100/care-plans", protectedGoalToken(subject), `{}`)
	if response.Code != http.StatusInternalServerError || state.rollbacks != 1 {
		t.Fatalf("status=%d rollbacks=%d body=%s", response.Code, state.rollbacks, response.Body.String())
	}
	if state.committedCarePlans != 0 || state.committedCarePlanSections != 0 || state.committedAudits != 0 {
		t.Fatalf("audit failure left partial writes: %#v", state)
	}
}

var errSyntheticAuditUnavailable = &carePlanTestError{"audit unavailable"}

type carePlanTestError struct{ message string }

func (e *carePlanTestError) Error() string { return e.message }
