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

func validSignalResponseBody() string {
	return `{"child_id":999,"home_id":999,"staff_user_id":999,"actor_name":"Caller Spoof","signal_kind":"missing_education","did_checkin":true,"adjusted_support":true,"informed_manager":true,"informed_network":false,"monitor_only":false,"did_referral":true,"referral_destinations":"Sensitive referral details","actions_summary":"Sensitive actions","future_notes":"Sensitive future notes","external_log_ref":"Sensitive external reference"}`
}

func TestSignalResponseAuthenticationAndRolePolicy(t *testing.T) {
	subject := "signal-role-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	tests := []struct {
		name, role, token string
		roleHome          int64
		childMissing      bool
		want              int
	}{
		{name: "missing authentication", role: "key_worker", want: http.StatusUnauthorized},
		{name: "key worker accepted", role: "key_worker", token: token, want: http.StatusCreated},
		{name: "senior accepted", role: "senior", token: token, want: http.StatusCreated},
		{name: "manager accepted", role: "manager", token: token, want: http.StatusCreated},
		{name: "insufficient role", role: "responsible_individual", token: token, want: http.StatusForbidden},
		{name: "wrong home", role: "manager", roleHome: 20, token: token, want: http.StatusForbidden},
		{name: "nonexistent child", role: "manager", token: token, childMissing: true, want: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedBaselineState(test.role)
			state.roleHomeID = test.roleHome
			if test.childMissing {
				state.childExists = false
			}
			response := baselineRequest(
				protectedBaselineHandler(t, state, staff),
				"/v1/children/100/signal_responses",
				test.token,
				"",
				validSignalResponseBody(),
			)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want != http.StatusCreated && (state.signalAttempts != 0 || state.committedSignals != 0 || state.committedAudits != 0) {
				t.Fatalf("unauthorized signal response write: %#v", state)
			}
		})
	}
}

func TestSignalResponseStructuralValidation(t *testing.T) {
	subject := "signal-validation-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	for _, test := range []struct{ name, body string }{
		{name: "malformed json", body: `{"signal_kind":`},
		{name: "missing signal kind", body: `{}`},
		{name: "blank signal kind", body: `{"signal_kind":"   "}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedBaselineState("key_worker")
			response := baselineRequest(protectedBaselineHandler(t, state, staff), "/v1/children/100/signal_responses", token, "", test.body)
			if response.Code != http.StatusBadRequest || state.signalAttempts != 0 || state.committedSignals != 0 {
				t.Fatalf("validation result: status=%d state=%#v body=%s", response.Code, state, response.Body.String())
			}
		})
	}
}

func TestSignalResponseTransactionScopeAndAuditRedaction(t *testing.T) {
	subject := "signal-success-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	state := authorizedBaselineState("senior")
	response := baselineRequest(
		protectedBaselineHandler(t, state, staff),
		"/v1/children/100/signal_responses",
		protectedGoalToken(subject),
		"caller-controlled-request-id",
		validSignalResponseBody(),
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if state.committedSignals != 1 || state.committedAudits != 1 || state.signalAttempts != 1 || state.auditAttempts != 1 {
		t.Fatalf("committed state: %#v", state)
	}
	if state.signalTransaction == 0 || state.signalTransaction != state.auditTransaction {
		t.Fatalf("signal tx=%d audit tx=%d", state.signalTransaction, state.auditTransaction)
	}
	wantOrder := []string{"staff_lock", "role_lock", "child_lock", "signal_insert", "signal_scope_lock", "child_lock", "audit_insert"}
	if fmt.Sprint(state.queryOrder) != fmt.Sprint(wantOrder) {
		t.Fatalf("query order=%v want=%v", state.queryOrder, wantOrder)
	}
	if state.signalArgs[0].Value != int64(100) {
		t.Fatalf("signal child ID=%v, want URL child 100", state.signalArgs[0].Value)
	}
	if state.signalArgs[1].Value != "missing_education" || state.signalArgs[2].Value != true || state.signalArgs[3].Value != true || state.signalArgs[4].Value != true || state.signalArgs[7].Value != true {
		t.Fatalf("operational signal args=%#v", state.signalArgs[:8])
	}

	metadataBytes, ok := state.auditArgs[10].Value.([]byte)
	if !ok {
		t.Fatalf("metadata type=%T", state.auditArgs[10].Value)
	}
	metadataText := string(metadataBytes)
	for _, forbidden := range []string{
		"missing_education", "Sensitive referral details", "Sensitive actions", "Sensitive future notes",
		"Sensitive external reference", "Caller Spoof", "referral_destinations", "actions_summary",
		"future_notes", "external_log_ref", "request_body",
	} {
		if strings.Contains(metadataText, forbidden) {
			t.Fatalf("audit metadata contains narrative %q: %s", forbidden, metadataText)
		}
	}
	var metadata map[string]any
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["signal_response_id"] != float64(800) || metadata["resource_id"] != float64(800) {
		t.Fatalf("resource metadata=%#v", metadata)
	}
	if requestID, _ := metadata["request_id"].(string); requestID == "" || requestID == "caller-controlled-request-id" {
		t.Fatalf("untrusted request ID=%q", requestID)
	}
	if state.auditArgs[0].Value != int64(10) || state.auditArgs[1].Value != int64(100) || state.auditArgs[2].Value != int64(7) || state.auditArgs[3].Value != "database@example.test" || state.auditArgs[4].Value != "Database Actor" || state.auditArgs[5].Value != "signal_response.create" || state.auditArgs[6].Value != "signal_response" || state.auditArgs[7].Value != int64(800) {
		t.Fatalf("authoritative audit binding=%#v", state.auditArgs[:8])
	}
}

func TestSignalResponseFailuresAreAtomic(t *testing.T) {
	subject := "signal-failure-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	t.Run("signal insert failure creates no audit", func(t *testing.T) {
		state := authorizedBaselineState("manager")
		state.signalErr = errors.New("signal unavailable")
		response := baselineRequest(protectedBaselineHandler(t, state, staff), "/v1/children/100/signal_responses", token, "", validSignalResponseBody())
		if response.Code != http.StatusInternalServerError || state.rollbacks != 1 || state.signalAttempts != 1 || state.auditAttempts != 0 || state.committedSignals != 0 || state.committedAudits != 0 {
			t.Fatalf("insert failure result: status=%d state=%#v", response.Code, state)
		}
	})
	t.Run("audit failure rolls back signal response", func(t *testing.T) {
		state := authorizedBaselineState("manager")
		state.auditErr = errors.New("audit unavailable")
		response := baselineRequest(protectedBaselineHandler(t, state, staff), "/v1/children/100/signal_responses", token, "", validSignalResponseBody())
		if response.Code != http.StatusInternalServerError || state.rollbacks != 1 || state.signalAttempts != 1 || state.auditAttempts != 1 || state.committedSignals != 0 || state.committedAudits != 0 {
			t.Fatalf("audit failure result: status=%d state=%#v", response.Code, state)
		}
	})
}
