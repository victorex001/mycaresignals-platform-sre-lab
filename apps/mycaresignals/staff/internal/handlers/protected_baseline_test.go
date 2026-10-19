package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	appmw "therapeutic-engine/internal/middleware"
	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/storage"
)

var (
	baselineDriverOnce sync.Once
	baselineStatesMu   sync.Mutex
	baselineStates     = map[string]*protectedBaselineDBState{}
	baselineDSN        atomic.Int64
)

type protectedBaselineDBState struct {
	staffExists bool
	staffActive bool
	role        string
	roleActive  bool
	roleHomeID  int64
	childExists bool
	childHomeID int64
	homeExists  bool

	baselineExists bool
	baselineStatus string
	baselineErr    error
	domainErrAt    int
	priorityErrAt  int
	approvalErr    error
	signalErr      error
	childErr       error
	auditErr       error

	begins, commits, rollbacks                                 int
	baselineAttempts, domainAttempts, priorityAttempts         int
	approvalAttempts, auditAttempts                            int
	signalAttempts                                             int
	childAttempts                                              int
	pendingBaselines, pendingDomains, pendingPriorities        int
	pendingApprovals, pendingAudits                            int
	pendingSignals                                             int
	pendingChildren                                            int
	pendingApprovalStatus                                      string
	committedBaselines, committedDomains, committedPriorities  int
	committedApprovals, committedAudits                        int
	committedSignals                                           int
	committedChildren                                          int
	committedApprovalStatus                                    string
	queryOrder                                                 []string
	baselineArgs, approvalArgs, auditArgs                      []driver.NamedValue
	signalArgs                                                 []driver.NamedValue
	childArgs                                                  []driver.NamedValue
	baselineTransaction, approvalTransaction, auditTransaction int64
	signalTransaction                                          int64
	childTransaction                                           int64
	domainTransaction, priorityTransaction                     int64
	deterministicApprovalOrder                                 bool
	deterministicReadOrder                                     bool
	approvalUsesServerTimes                                    bool
}

type protectedBaselineDriver struct{}
type protectedBaselineConn struct {
	state *protectedBaselineDBState
	inTx  bool
	txID  int64
}
type protectedBaselineTx struct{ conn *protectedBaselineConn }
type protectedBaselineRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func openProtectedBaselineDB(t *testing.T, state *protectedBaselineDBState) *sql.DB {
	t.Helper()
	if state.childHomeID == 0 {
		state.childHomeID = 10
	}
	if state.roleHomeID == 0 {
		state.roleHomeID = state.childHomeID
	}
	baselineDriverOnce.Do(func() { sql.Register("protected-baseline-test", protectedBaselineDriver{}) })
	dsn := fmt.Sprintf("protected-baseline-%d", baselineDSN.Add(1))
	baselineStatesMu.Lock()
	baselineStates[dsn] = state
	baselineStatesMu.Unlock()
	db, err := sql.Open("protected-baseline-test", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
		baselineStatesMu.Lock()
		delete(baselineStates, dsn)
		baselineStatesMu.Unlock()
	})
	return db
}

func (protectedBaselineDriver) Open(name string) (driver.Conn, error) {
	baselineStatesMu.Lock()
	defer baselineStatesMu.Unlock()
	state := baselineStates[name]
	if state == nil {
		return nil, errors.New("unknown protected-baseline database")
	}
	return &protectedBaselineConn{state: state}, nil
}

func (c *protectedBaselineConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported")
}
func (c *protectedBaselineConn) Close() error { return nil }
func (c *protectedBaselineConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *protectedBaselineConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.inTx = true
	c.txID++
	c.state.begins++
	c.state.pendingBaselines, c.state.pendingDomains, c.state.pendingPriorities = 0, 0, 0
	c.state.pendingApprovals, c.state.pendingAudits = 0, 0
	c.state.pendingSignals = 0
	c.state.pendingChildren = 0
	c.state.pendingApprovalStatus = ""
	return &protectedBaselineTx{conn: c}, nil
}
func (c *protectedBaselineConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	switch {
	case strings.Contains(query, "INSERT INTO child_onboarding_domain_states"):
		c.state.queryOrder = append(c.state.queryOrder, "domain_insert")
		c.state.domainAttempts++
		c.state.domainTransaction = c.txID
		if c.state.domainErrAt == c.state.domainAttempts {
			return nil, errors.New("domain insert unavailable")
		}
		c.state.pendingDomains++
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "INSERT INTO child_priority_domains"):
		c.state.queryOrder = append(c.state.queryOrder, "priority_insert")
		c.state.priorityAttempts++
		c.state.priorityTransaction = c.txID
		if c.state.priorityErrAt == c.state.priorityAttempts {
			return nil, errors.New("priority insert unavailable")
		}
		c.state.pendingPriorities++
		return driver.RowsAffected(1), nil
	default:
		return nil, fmt.Errorf("unexpected protected-baseline exec: %s", query)
	}
}
func (c *protectedBaselineConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	copyArgs := append([]driver.NamedValue(nil), args...)
	switch {
	case strings.Contains(query, "SELECT status") && strings.Contains(query, "FROM staff_users") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "staff_lock")
		rows := &protectedBaselineRows{columns: []string{"status"}}
		if c.state.staffExists {
			status := "inactive"
			if c.state.staffActive {
				status = "active"
			}
			rows.values = [][]driver.Value{{status}}
		}
		return rows, nil
	case strings.Contains(query, "FROM staff_home_roles") && strings.Contains(query, "JOIN children") && strings.Contains(query, "FOR UPDATE OF shr"):
		c.state.queryOrder = append(c.state.queryOrder, "role_lock")
		rows := &protectedBaselineRows{columns: []string{"home_id", "role", "active"}}
		if c.state.childExists && c.state.roleHomeID == c.state.childHomeID && c.state.role != "" {
			rows.values = [][]driver.Value{{c.state.roleHomeID, c.state.role, c.state.roleActive}}
		}
		return rows, nil
	case strings.Contains(query, "SELECT role, active") && strings.Contains(query, "FROM staff_home_roles") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "home_role_lock")
		rows := &protectedBaselineRows{columns: []string{"role", "active"}}
		if c.state.roleHomeID == args[1].Value.(int64) && c.state.role != "" {
			rows.values = [][]driver.Value{{c.state.role, c.state.roleActive}}
		}
		return rows, nil
	case strings.Contains(query, "SELECT id FROM homes") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "home_lock")
		rows := &protectedBaselineRows{columns: []string{"id"}}
		if c.state.homeExists {
			rows.values = [][]driver.Value{{args[0].Value}}
		}
		return rows, nil
	case strings.Contains(query, "FROM children") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "child_lock")
		rows := &protectedBaselineRows{columns: []string{"home_id"}}
		if c.state.childExists || c.state.pendingChildren > 0 {
			rows.values = [][]driver.Value{{c.state.childHomeID}}
		}
		return rows, nil
	case strings.Contains(query, "INSERT INTO children"):
		c.state.queryOrder = append(c.state.queryOrder, "child_insert")
		c.state.childAttempts++
		c.state.childArgs = copyArgs
		c.state.childTransaction = c.txID
		if c.state.childErr != nil {
			return nil, c.state.childErr
		}
		c.state.pendingChildren++
		return &protectedBaselineRows{columns: []string{"id", "created_at", "updated_at"}, values: [][]driver.Value{{int64(100), time.Unix(10, 0), time.Unix(10, 0)}}}, nil
	case strings.Contains(query, "INSERT INTO child_onboarding_baselines"):
		c.state.queryOrder = append(c.state.queryOrder, "baseline_insert")
		c.state.baselineAttempts++
		c.state.baselineArgs = copyArgs
		c.state.baselineTransaction = c.txID
		if c.state.baselineErr != nil {
			return nil, c.state.baselineErr
		}
		c.state.pendingBaselines++
		return &protectedBaselineRows{columns: []string{"id", "status", "created_at", "updated_at"}, values: [][]driver.Value{{int64(600), args[16].Value, time.Unix(10, 0), time.Unix(10, 0)}}}, nil
	case strings.Contains(query, "summary_of_history") && strings.Contains(query, "FROM child_onboarding_baselines") && !strings.Contains(query, "FOR UPDATE"):
		c.state.deterministicReadOrder = strings.Contains(query, "ORDER BY created_at DESC, id DESC")
		rows := &protectedBaselineRows{columns: []string{
			"id", "child_id", "summary_of_history", "recent_placement_pattern", "current_concerns",
			"strengths", "existing_plans", "childs_voice", "relapse_risks", "relapse_early_signals",
			"relapse_support_plan", "review_frequency", "next_review_date", "lead_reviewer",
			"other_professionals", "early_warning_signals", "notes_for_team", "status", "approved_by",
			"approved_at", "created_at", "updated_at",
		}}
		if c.state.baselineExists {
			rows.values = [][]driver.Value{{
				int64(600), int64(100), nil, nil, nil, nil, nil, nil, nil, nil, nil,
				nil, nil, nil, nil, nil, nil, "submitted", nil, nil, time.Unix(10, 0), time.Unix(11, 0),
			}}
		}
		return rows, nil
	case strings.Contains(query, "FROM child_onboarding_domain_states"):
		return &protectedBaselineRows{
			columns: []string{"id", "baseline_id", "domain_key", "active", "reason"},
			values:  [][]driver.Value{{int64(1), int64(600), "emotional", true, nil}},
		}, nil
	case strings.Contains(query, "FROM child_priority_domains"):
		return &protectedBaselineRows{
			columns: []string{"id", "baseline_id", "domain_key", "focus_summary", "hoped_for_change", "sort_order"},
			values:  [][]driver.Value{{int64(2), int64(600), "emotional", nil, nil, int64(1)}},
		}, nil
	case strings.Contains(query, "SELECT id, status") && strings.Contains(query, "FROM child_onboarding_baselines") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "baseline_lock")
		c.state.deterministicApprovalOrder = strings.Contains(query, "ORDER BY created_at DESC, id DESC")
		rows := &protectedBaselineRows{columns: []string{"id", "status"}}
		if c.state.baselineExists {
			rows.values = [][]driver.Value{{int64(600), c.state.baselineStatus}}
		}
		return rows, nil
	case strings.Contains(query, "UPDATE child_onboarding_baselines") && strings.Contains(query, "RETURNING status"):
		c.state.queryOrder = append(c.state.queryOrder, "baseline_update")
		c.state.approvalAttempts++
		c.state.approvalArgs = copyArgs
		c.state.approvalTransaction = c.txID
		c.state.approvalUsesServerTimes = strings.Contains(query, "approved_at = now()") && strings.Contains(query, "updated_at = now()")
		if c.state.approvalErr != nil {
			return nil, c.state.approvalErr
		}
		c.state.pendingApprovals++
		c.state.pendingApprovalStatus = "approved"
		return &protectedBaselineRows{columns: []string{"status", "approved_by", "approved_at", "updated_at"}, values: [][]driver.Value{{"approved", args[1].Value, time.Unix(20, 0), time.Unix(21, 0)}}}, nil
	case strings.Contains(query, "SELECT child_id FROM child_onboarding_baselines") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "baseline_scope_lock")
		rows := &protectedBaselineRows{columns: []string{"child_id"}}
		if c.state.pendingBaselines > 0 || c.state.baselineExists {
			rows.values = [][]driver.Value{{int64(100)}}
		}
		return rows, nil
	case strings.Contains(query, "INSERT INTO child_signal_responses"):
		c.state.queryOrder = append(c.state.queryOrder, "signal_insert")
		c.state.signalAttempts++
		c.state.signalArgs = copyArgs
		c.state.signalTransaction = c.txID
		if c.state.signalErr != nil {
			return nil, c.state.signalErr
		}
		c.state.pendingSignals++
		return &protectedBaselineRows{columns: []string{"id", "created_at"}, values: [][]driver.Value{{int64(800), time.Unix(20, 0)}}}, nil
	case strings.Contains(query, "SELECT child_id FROM child_signal_responses") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "signal_scope_lock")
		rows := &protectedBaselineRows{columns: []string{"child_id"}}
		if c.state.pendingSignals > 0 {
			rows.values = [][]driver.Value{{int64(100)}}
		}
		return rows, nil
	case strings.Contains(query, "INSERT INTO audit_events"):
		c.state.queryOrder = append(c.state.queryOrder, "audit_insert")
		c.state.auditAttempts++
		c.state.auditArgs = copyArgs
		c.state.auditTransaction = c.txID
		if c.state.auditErr != nil {
			return nil, c.state.auditErr
		}
		c.state.pendingAudits++
		return &protectedBaselineRows{columns: []string{"id", "created_at"}, values: [][]driver.Value{{int64(900), time.Unix(22, 0)}}}, nil
	default:
		return nil, fmt.Errorf("unexpected protected-baseline query: %s", query)
	}
}

func (tx *protectedBaselineTx) Commit() error {
	state := tx.conn.state
	state.commits++
	state.committedBaselines += state.pendingBaselines
	state.committedDomains += state.pendingDomains
	state.committedPriorities += state.pendingPriorities
	state.committedApprovals += state.pendingApprovals
	if state.pendingApprovals > 0 {
		state.committedApprovalStatus = state.pendingApprovalStatus
	}
	state.committedAudits += state.pendingAudits
	state.committedSignals += state.pendingSignals
	state.committedChildren += state.pendingChildren
	state.pendingBaselines, state.pendingDomains, state.pendingPriorities = 0, 0, 0
	state.pendingApprovals, state.pendingAudits = 0, 0
	state.pendingSignals = 0
	state.pendingChildren = 0
	state.pendingApprovalStatus = ""
	tx.conn.inTx = false
	return nil
}
func (tx *protectedBaselineTx) Rollback() error {
	state := tx.conn.state
	state.rollbacks++
	state.pendingBaselines, state.pendingDomains, state.pendingPriorities = 0, 0, 0
	state.pendingApprovals, state.pendingAudits = 0, 0
	state.pendingSignals = 0
	state.pendingChildren = 0
	state.pendingApprovalStatus = ""
	tx.conn.inTx = false
	return nil
}
func (r *protectedBaselineRows) Columns() []string { return r.columns }
func (r *protectedBaselineRows) Close() error      { return nil }
func (r *protectedBaselineRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func protectedBaselineHandler(t *testing.T, state *protectedBaselineDBState, staff map[string]*models.StaffUser) http.Handler {
	t.Helper()
	handler := &ChildrenHandler{Repo: storage.NewRepo(openProtectedBaselineDB(t, state))}
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	authenticate := appmw.Authenticate(protectedGoalAuthenticator(t, staff))
	router.With(authenticate).Post("/v1/homes/{homeID}/children/{childID}/onboarding/baseline", handler.SaveOnboardingBaseline)
	router.With(authenticate).Post("/v1/children/{childID}/onboarding/approve", handler.ApproveOnboardingBaseline)
	router.With(authenticate).Post("/v1/children/{childID}/signal_responses", handler.LogSignalResponse)
	router.With(authenticate).Post("/v1/homes/{homeID}/children/onboard", handler.OnboardChild)
	return router
}

func baselineRequest(handler http.Handler, methodPath, token, requestID, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, methodPath, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if requestID != "" {
		request.Header.Set("X-Request-Id", requestID)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func validBaselineBody() string {
	return `{"child_id":999,"home_id":999,"status":"submitted","summary_of_history":"Sensitive history","domains":[{"domain_key":"emotional","active":true,"reason":"Sensitive reason"}],"priorities":[{"domain_key":"emotional","focus_summary":"Sensitive focus","hoped_for_change":"Sensitive hope","sort_order":1}]}`
}

func authorizedBaselineState(role string) *protectedBaselineDBState {
	return &protectedBaselineDBState{staffExists: true, staffActive: true, role: role, roleActive: true, childExists: true, childHomeID: 10, homeExists: true}
}

func TestSaveBaselineAuthorizationAndHomeScope(t *testing.T) {
	subject := "baseline-save-role"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	tests := []struct {
		name, role, token, path string
		roleHome                int64
		want                    int
	}{
		{name: "missing token", role: "key_worker", path: "/v1/homes/10/children/100/onboarding/baseline", want: http.StatusUnauthorized},
		{name: "key worker", role: "key_worker", token: token, path: "/v1/homes/10/children/100/onboarding/baseline", want: http.StatusCreated},
		{name: "senior", role: "senior", token: token, path: "/v1/homes/10/children/100/onboarding/baseline", want: http.StatusCreated},
		{name: "manager", role: "manager", token: token, path: "/v1/homes/10/children/100/onboarding/baseline", want: http.StatusCreated},
		{name: "wrong home role", role: "manager", roleHome: 20, token: token, path: "/v1/homes/10/children/100/onboarding/baseline", want: http.StatusForbidden},
		{name: "url home mismatch", role: "manager", token: token, path: "/v1/homes/20/children/100/onboarding/baseline", want: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedBaselineState(test.role)
			state.roleHomeID = test.roleHome
			response := baselineRequest(protectedBaselineHandler(t, state, staff), test.path, test.token, "", validBaselineBody())
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want != http.StatusCreated && (state.committedBaselines != 0 || state.committedAudits != 0) {
				t.Fatalf("unauthorized save committed: %#v", state)
			}
		})
	}
}

func TestSaveBaselineStructuralValidation(t *testing.T) {
	subject := "baseline-validation"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	tests := []struct{ name, body string }{
		{name: "approved status", body: `{"status":"approved"}`},
		{name: "empty domain key", body: `{"domains":[{"domain_key":" "}]}`},
		{name: "duplicate domain", body: `{"domains":[{"domain_key":"emotional"},{"domain_key":"emotional"}]}`},
		{name: "duplicate priority domain", body: `{"priorities":[{"domain_key":"emotional","sort_order":1},{"domain_key":"emotional","sort_order":2}]}`},
		{name: "duplicate priority sort order", body: `{"priorities":[{"domain_key":"emotional","sort_order":1},{"domain_key":"education","sort_order":1}]}`},
		{name: "invalid review date", body: `{"review_plan":{"next_review_date":"not-a-date"}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedBaselineState("senior")
			response := baselineRequest(protectedBaselineHandler(t, state, staff), "/v1/homes/10/children/100/onboarding/baseline", token, "", test.body)
			if response.Code != http.StatusBadRequest || state.baselineAttempts != 0 {
				t.Fatalf("validation result: status=%d state=%#v body=%s", response.Code, state, response.Body.String())
			}
		})
	}
}

func TestSaveBaselineAtomicityAndAudit(t *testing.T) {
	subject := "baseline-save-success"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	t.Run("success", func(t *testing.T) {
		state := authorizedBaselineState("key_worker")
		response := baselineRequest(protectedBaselineHandler(t, state, staff), "/v1/homes/10/children/100/onboarding/baseline", token, "caller-request-id", validBaselineBody())
		if response.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		if state.committedBaselines != 1 || state.committedDomains != 1 || state.committedPriorities != 1 || state.committedAudits != 1 {
			t.Fatalf("committed state: %#v", state)
		}
		if state.baselineTransaction == 0 || state.baselineTransaction != state.domainTransaction || state.baselineTransaction != state.priorityTransaction || state.baselineTransaction != state.auditTransaction {
			t.Fatalf("transaction IDs: %#v", state)
		}
		wantOrder := []string{"staff_lock", "role_lock", "child_lock", "baseline_insert", "domain_insert", "priority_insert", "baseline_scope_lock", "child_lock", "audit_insert"}
		if fmt.Sprint(state.queryOrder) != fmt.Sprint(wantOrder) {
			t.Fatalf("save order = %v, want %v", state.queryOrder, wantOrder)
		}
		if state.baselineArgs[0].Value != int64(100) && state.baselineArgs[0].Value != models.ChildID(100) {
			t.Fatalf("baseline child = %v", state.baselineArgs[0].Value)
		}
		metadataBytes := state.auditArgs[10].Value.([]byte)
		metadataText := string(metadataBytes)
		for _, forbidden := range []string{"Sensitive history", "Sensitive reason", "Sensitive focus", "Sensitive hope", "summary_of_history", "focus_summary", "hoped_for_change"} {
			if strings.Contains(metadataText, forbidden) {
				t.Fatalf("audit contains narrative %q: %s", forbidden, metadataText)
			}
		}
		var metadata map[string]any
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata["new_status"] != "submitted" || metadata["domain_count"] != float64(1) || metadata["priority_count"] != float64(1) {
			t.Fatalf("audit metadata = %#v", metadata)
		}
		if requestID, _ := metadata["request_id"].(string); requestID == "" || requestID == "caller-request-id" {
			t.Fatalf("untrusted request ID = %q", requestID)
		}
	})

	tests := []struct {
		name                                string
		configure                           func(*protectedBaselineDBState)
		wantDomain, wantPriority, wantAudit int
	}{
		{name: "baseline insert failure", configure: func(s *protectedBaselineDBState) { s.baselineErr = errors.New("baseline unavailable") }},
		{name: "domain insert failure", configure: func(s *protectedBaselineDBState) { s.domainErrAt = 1 }, wantDomain: 1},
		{name: "priority insert failure", configure: func(s *protectedBaselineDBState) { s.priorityErrAt = 1 }, wantDomain: 1, wantPriority: 1},
		{name: "audit failure", configure: func(s *protectedBaselineDBState) { s.auditErr = errors.New("audit unavailable") }, wantDomain: 1, wantPriority: 1, wantAudit: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedBaselineState("manager")
			test.configure(state)
			response := baselineRequest(protectedBaselineHandler(t, state, staff), "/v1/homes/10/children/100/onboarding/baseline", token, "", validBaselineBody())
			if response.Code != http.StatusInternalServerError || state.rollbacks != 1 {
				t.Fatalf("failure result: status=%d state=%#v", response.Code, state)
			}
			if state.domainAttempts != test.wantDomain || state.priorityAttempts != test.wantPriority || state.auditAttempts != test.wantAudit {
				t.Fatalf("attempt counts: %#v", state)
			}
			if state.committedBaselines != 0 || state.committedDomains != 0 || state.committedPriorities != 0 || state.committedAudits != 0 {
				t.Fatalf("failed save committed: %#v", state)
			}
		})
	}
}

func TestApproveBaselineAuthorizationStateAndAtomicity(t *testing.T) {
	subject := "baseline-approve"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	for _, test := range []struct {
		name, role string
		roleHome   int64
		status     string
		exists     bool
		want       int
	}{
		{name: "key worker rejected", role: "key_worker", exists: true, status: "submitted", want: http.StatusForbidden},
		{name: "senior rejected", role: "senior", exists: true, status: "submitted", want: http.StatusForbidden},
		{name: "manager accepted", role: "manager", exists: true, status: "submitted", want: http.StatusNoContent},
		{name: "wrong home manager", role: "manager", roleHome: 20, exists: true, status: "submitted", want: http.StatusForbidden},
		{name: "draft transition rejected", role: "manager", exists: true, status: "draft", want: http.StatusBadRequest},
		{name: "already approved rejected", role: "manager", exists: true, status: "approved", want: http.StatusBadRequest},
		{name: "missing baseline non-disclosing", role: "manager", want: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedBaselineState(test.role)
			state.roleHomeID = test.roleHome
			state.baselineExists = test.exists
			state.baselineStatus = test.status
			response := baselineRequest(protectedBaselineHandler(t, state, staff), "/v1/children/100/onboarding/approve", token, "", `{"approved_by":"Caller Spoof"}`)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want == http.StatusNoContent {
				if state.committedApprovals != 1 || state.committedApprovalStatus != "approved" || state.committedAudits != 1 || !state.deterministicApprovalOrder {
					t.Fatalf("approval state: %#v", state)
				}
				if !state.approvalUsesServerTimes {
					t.Fatal("approval did not generate approved_at and updated_at server-side")
				}
				if state.approvalArgs[1].Value != "Database Actor" || state.approvalArgs[1].Value == "Caller Spoof" {
					t.Fatalf("approved_by = %v", state.approvalArgs[1].Value)
				}
				if state.approvalTransaction == 0 || state.approvalTransaction != state.auditTransaction {
					t.Fatalf("approval tx=%d audit tx=%d", state.approvalTransaction, state.auditTransaction)
				}
				wantOrder := []string{"staff_lock", "role_lock", "child_lock", "baseline_lock", "baseline_update", "baseline_scope_lock", "child_lock", "audit_insert"}
				if fmt.Sprint(state.queryOrder) != fmt.Sprint(wantOrder) {
					t.Fatalf("approval order = %v, want %v", state.queryOrder, wantOrder)
				}
				metadataBytes := state.auditArgs[10].Value.([]byte)
				var metadata map[string]any
				if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
					t.Fatal(err)
				}
				if metadata["previous_status"] != "submitted" || metadata["new_status"] != "approved" || metadata["approved"] != true {
					t.Fatalf("approval metadata = %#v", metadata)
				}
				if state.auditArgs[2].Value != int64(7) || state.auditArgs[3].Value != "database@example.test" || state.auditArgs[4].Value != "Database Actor" || state.auditArgs[5].Value != "onboarding_baseline.approve" {
					t.Fatalf("approval audit binding = %#v", state.auditArgs[:8])
				}
			}
		})
	}

	for _, test := range []struct {
		name      string
		configure func(*protectedBaselineDBState)
		wantAudit int
	}{
		{name: "approval update failure", configure: func(s *protectedBaselineDBState) { s.approvalErr = errors.New("update unavailable") }},
		{name: "approval audit failure", configure: func(s *protectedBaselineDBState) { s.auditErr = errors.New("audit unavailable") }, wantAudit: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedBaselineState("manager")
			state.baselineExists, state.baselineStatus = true, "submitted"
			test.configure(state)
			response := baselineRequest(protectedBaselineHandler(t, state, staff), "/v1/children/100/onboarding/approve", token, "", `{"approved_by":"Caller"}`)
			if response.Code != http.StatusInternalServerError || state.rollbacks != 1 || state.auditAttempts != test.wantAudit || state.committedApprovals != 0 || state.committedAudits != 0 {
				t.Fatalf("approval failure result: status=%d state=%#v", response.Code, state)
			}
		})
	}
}

func TestLatestBaselineNullableFieldsAndDeterministicRead(t *testing.T) {
	state := authorizedBaselineState("manager")
	state.baselineExists = true
	store := storage.NewChildrenStore(openProtectedBaselineDB(t, state))
	baseline, domains, priorities, err := store.GetLatestOnboardingBaseline(context.Background(), models.ChildID(100))
	if err != nil {
		t.Fatal(err)
	}
	if !state.deterministicReadOrder {
		t.Fatal("latest baseline read is not deterministically ordered")
	}
	if baseline.SummaryOfHistory != "" || baseline.ApprovedBy != nil || baseline.ApprovedAt != nil || baseline.NextReviewDate != nil {
		t.Fatalf("nullable baseline fields = %#v", baseline)
	}
	if len(domains) != 1 || domains[0].Reason != "" {
		t.Fatalf("nullable domain reason = %#v", domains)
	}
	if len(priorities) != 1 || priorities[0].FocusSummary != "" || priorities[0].HopedForChange != "" {
		t.Fatalf("nullable priority fields = %#v", priorities)
	}
}

var _ driver.Driver = protectedBaselineDriver{}
var _ driver.ConnBeginTx = (*protectedBaselineConn)(nil)
var _ driver.QueryerContext = (*protectedBaselineConn)(nil)
var _ driver.ExecerContext = (*protectedBaselineConn)(nil)
