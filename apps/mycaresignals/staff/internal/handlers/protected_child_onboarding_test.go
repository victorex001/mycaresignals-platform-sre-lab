package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"therapeutic-engine/internal/models"
)

func validChildOnboardingBody() string {
	return `{"home_id":999,"staff_user_id":999,"created_by":"Caller Spoof","external_ref":"Sensitive External Ref","initials":"AB","first_name":"Sensitive First","last_name":"Sensitive Last","date_of_birth":"2012-03-04","legal_status":"Sensitive Legal Status","admission_date":"2026-08-01","gender":"Sensitive Gender","active":false}`
}

func TestChildOnboardingAuthenticationAndRolePolicy(t *testing.T) {
	subject := "child-onboard-role"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	tests := []struct {
		name, role, token string
		roleHome          int64
		homeExists        bool
		want              int
	}{
		{name: "missing authentication", role: "senior", homeExists: true, want: http.StatusUnauthorized},
		{name: "key worker rejected", role: "key_worker", token: token, homeExists: true, want: http.StatusForbidden},
		{name: "senior accepted", role: "senior", token: token, homeExists: true, want: http.StatusOK},
		{name: "manager accepted", role: "manager", token: token, homeExists: true, want: http.StatusOK},
		{name: "wrong home senior", role: "senior", roleHome: 20, token: token, homeExists: true, want: http.StatusForbidden},
		{name: "wrong home manager", role: "manager", roleHome: 20, token: token, homeExists: true, want: http.StatusForbidden},
		{name: "nonexistent home", role: "manager", token: token, want: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedBaselineState(test.role)
			state.roleHomeID = test.roleHome
			state.homeExists = test.homeExists
			response := baselineRequest(
				protectedBaselineHandler(t, state, staff),
				"/v1/homes/10/children/onboard",
				test.token,
				"",
				validChildOnboardingBody(),
			)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want != http.StatusOK && (state.childAttempts != 0 || state.committedChildren != 0 || state.committedAudits != 0) {
				t.Fatalf("unauthorized child onboarding wrote data: %#v", state)
			}
		})
	}
}

func TestChildOnboardingValidation(t *testing.T) {
	subject := "child-onboard-validation"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	tests := []struct{ name, body string }{
		{name: "malformed json", body: `{"initials":`},
		{name: "missing initials", body: `{"date_of_birth":"2012-03-04"}`},
		{name: "blank initials", body: `{"initials":"  ","date_of_birth":"2012-03-04"}`},
		{name: "invalid birth date", body: `{"initials":"AB","date_of_birth":"not-a-date"}`},
		{name: "missing birth date", body: `{"initials":"AB"}`},
		{name: "invalid admission date", body: `{"initials":"AB","date_of_birth":"2012-03-04","admission_date":"not-a-date"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedBaselineState("senior")
			response := baselineRequest(protectedBaselineHandler(t, state, staff), "/v1/homes/10/children/onboard", token, "", test.body)
			if response.Code != http.StatusBadRequest || state.childAttempts != 0 || state.committedChildren != 0 {
				t.Fatalf("validation result: status=%d state=%#v body=%s", response.Code, state, response.Body.String())
			}
		})
	}
}

func TestChildOnboardingAuthoritativeScopeAndPIISafeAudit(t *testing.T) {
	subject := "child-onboard-success"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	state := authorizedBaselineState("manager")
	response := baselineRequest(
		protectedBaselineHandler(t, state, staff),
		"/v1/homes/10/children/onboard",
		protectedGoalToken(subject),
		"caller-controlled-request-id",
		validChildOnboardingBody(),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if state.committedChildren != 1 || state.committedAudits != 1 || state.childAttempts != 1 || state.auditAttempts != 1 {
		t.Fatalf("committed state: %#v", state)
	}
	if state.childTransaction == 0 || state.childTransaction != state.auditTransaction {
		t.Fatalf("child tx=%d audit tx=%d", state.childTransaction, state.auditTransaction)
	}
	wantOrder := []string{"staff_lock", "home_role_lock", "home_lock", "child_insert", "child_lock", "audit_insert"}
	if fmt.Sprint(state.queryOrder) != fmt.Sprint(wantOrder) {
		t.Fatalf("query order=%v want=%v", state.queryOrder, wantOrder)
	}
	if state.childArgs[0].Value != int64(10) {
		t.Fatalf("child home_id=%v, want URL home 10", state.childArgs[0].Value)
	}
	if state.childArgs[9].Value != true {
		t.Fatalf("child active=%v, want server-defined true", state.childArgs[9].Value)
	}

	metadataBytes, ok := state.auditArgs[10].Value.([]byte)
	if !ok {
		t.Fatalf("metadata type=%T", state.auditArgs[10].Value)
	}
	metadataText := string(metadataBytes)
	for _, forbidden := range []string{
		"Sensitive External Ref", "AB", "Sensitive First", "Sensitive Last", "2012-03-04",
		"Sensitive Legal Status", "Sensitive Gender", "Caller Spoof", "external_ref", "initials",
		"first_name", "last_name", "date_of_birth", "legal_status", "gender", "request_body",
	} {
		if strings.Contains(metadataText, forbidden) {
			t.Fatalf("audit metadata contains child PII %q: %s", forbidden, metadataText)
		}
	}
	var metadata map[string]any
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["resource_id"] != float64(100) {
		t.Fatalf("child resource metadata=%#v", metadata)
	}
	if requestID, _ := metadata["request_id"].(string); requestID == "" || requestID == "caller-controlled-request-id" {
		t.Fatalf("untrusted request ID=%q", requestID)
	}
	if state.auditArgs[0].Value != int64(10) || state.auditArgs[1].Value != int64(100) || state.auditArgs[2].Value != int64(7) || state.auditArgs[3].Value != "database@example.test" || state.auditArgs[4].Value != "Database Actor" || state.auditArgs[5].Value != "child.create" || state.auditArgs[6].Value != "child" || state.auditArgs[7].Value != int64(100) {
		t.Fatalf("authoritative child audit binding=%#v", state.auditArgs[:8])
	}
}

func TestChildOnboardingFailuresAreAtomic(t *testing.T) {
	subject := "child-onboard-failure"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	t.Run("child insert failure creates no audit", func(t *testing.T) {
		state := authorizedBaselineState("manager")
		state.childErr = errors.New("child insert unavailable")
		response := baselineRequest(protectedBaselineHandler(t, state, staff), "/v1/homes/10/children/onboard", token, "", validChildOnboardingBody())
		if response.Code != http.StatusInternalServerError || state.rollbacks != 1 || state.childAttempts != 1 || state.auditAttempts != 0 || state.committedChildren != 0 || state.committedAudits != 0 {
			t.Fatalf("insert failure result: status=%d state=%#v", response.Code, state)
		}
	})
	t.Run("audit failure rolls back child", func(t *testing.T) {
		state := authorizedBaselineState("manager")
		state.auditErr = errors.New("audit unavailable")
		response := baselineRequest(protectedBaselineHandler(t, state, staff), "/v1/homes/10/children/onboard", token, "", validChildOnboardingBody())
		if response.Code != http.StatusInternalServerError || state.rollbacks != 1 || state.childAttempts != 1 || state.auditAttempts != 1 || state.committedChildren != 0 || state.committedAudits != 0 {
			t.Fatalf("audit failure result: status=%d state=%#v", response.Code, state)
		}
	})
}
