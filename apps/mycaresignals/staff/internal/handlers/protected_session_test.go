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
	sessionDriverOnce sync.Once
	sessionStatesMu   sync.Mutex
	sessionStates     = map[string]*protectedSessionDBState{}
	sessionDSN        atomic.Int64
)

type protectedSessionDBState struct {
	staffExists          bool
	staffActive          bool
	role                 string
	roleActive           bool
	roleHomeID           int64
	childExists          bool
	childHomeID          int64
	targets              map[int64]int64
	sessionErr           error
	scoreErrAt           int
	auditErr             error
	reviewSessionExists  bool
	reviewSessionChildID int64
	reviewSigned         bool
	reviewSignedAt       driver.Value
	reviewDecision       driver.Value
	reviewErr            error
	psychReviewErr       error

	begins, commits, rollbacks                             int
	sessionAttempts, scoreAttempts, auditAttempts          int
	reviewAttempts, psychReviewAttempts                    int
	pendingSessions, pendingScores, pendingAudits          int
	pendingReviews, pendingPsychReviews                    int
	committedSessions, committedScores, committedAudits    int
	committedReviews, committedPsychReviews                int
	queryOrder, lockedTargetOrder                          []string
	sessionArgs, auditArgs                                 []driver.NamedValue
	reviewArgs                                             []driver.NamedValue
	psychReviewArgs                                        []driver.NamedValue
	reviewQuery                                            string
	scoreArgs                                              [][]driver.NamedValue
	sessionTransaction, scoreTransaction, auditTransaction int64
}

type protectedSessionDriver struct{}
type protectedSessionConn struct {
	state *protectedSessionDBState
	inTx  bool
	txID  int64
}
type protectedSessionTx struct{ conn *protectedSessionConn }
type protectedSessionRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func openProtectedSessionDB(t *testing.T, state *protectedSessionDBState) *sql.DB {
	t.Helper()
	if state.childHomeID == 0 {
		state.childHomeID = 10
	}
	if state.roleHomeID == 0 {
		state.roleHomeID = state.childHomeID
	}
	sessionDriverOnce.Do(func() { sql.Register("protected-session-test", protectedSessionDriver{}) })
	dsn := fmt.Sprintf("protected-session-%d", sessionDSN.Add(1))
	sessionStatesMu.Lock()
	sessionStates[dsn] = state
	sessionStatesMu.Unlock()
	db, err := sql.Open("protected-session-test", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
		sessionStatesMu.Lock()
		delete(sessionStates, dsn)
		sessionStatesMu.Unlock()
	})
	return db
}

func (protectedSessionDriver) Open(name string) (driver.Conn, error) {
	sessionStatesMu.Lock()
	defer sessionStatesMu.Unlock()
	state := sessionStates[name]
	if state == nil {
		return nil, errors.New("unknown protected-session database")
	}
	return &protectedSessionConn{state: state}, nil
}

func (c *protectedSessionConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported")
}
func (c *protectedSessionConn) Close() error { return nil }
func (c *protectedSessionConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *protectedSessionConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.inTx = true
	c.txID++
	c.state.begins++
	c.state.pendingSessions = 0
	c.state.pendingScores = 0
	c.state.pendingAudits = 0
	c.state.pendingReviews = 0
	c.state.pendingPsychReviews = 0
	return &protectedSessionTx{conn: c}, nil
}
func (c *protectedSessionConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}
func (c *protectedSessionConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	copyArgs := append([]driver.NamedValue(nil), args...)
	switch {
	case strings.Contains(query, "SELECT status") && strings.Contains(query, "FROM staff_users") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "staff_lock")
		rows := &protectedSessionRows{columns: []string{"status"}}
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
		rows := &protectedSessionRows{columns: []string{"home_id", "role", "active"}}
		if c.state.childExists && c.state.roleHomeID == c.state.childHomeID && c.state.role != "" {
			rows.values = [][]driver.Value{{c.state.roleHomeID, c.state.role, c.state.roleActive}}
		}
		return rows, nil
	case strings.Contains(query, "FROM children") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "child_lock")
		rows := &protectedSessionRows{columns: []string{"home_id"}}
		if c.state.childExists {
			rows.values = [][]driver.Value{{c.state.childHomeID}}
		}
		return rows, nil
	case strings.Contains(query, "SELECT child_id FROM child_targets") && strings.Contains(query, "FOR UPDATE"):
		targetID := args[0].Value.(int64)
		label := fmt.Sprintf("target_lock:%d", targetID)
		c.state.queryOrder = append(c.state.queryOrder, label)
		c.state.lockedTargetOrder = append(c.state.lockedTargetOrder, label)
		rows := &protectedSessionRows{columns: []string{"child_id"}}
		if childID, ok := c.state.targets[targetID]; ok {
			rows.values = [][]driver.Value{{childID}}
		}
		return rows, nil
	case strings.Contains(query, "INSERT INTO keyworking_sessions"):
		c.state.queryOrder = append(c.state.queryOrder, "session_insert")
		c.state.sessionAttempts++
		c.state.sessionArgs = copyArgs
		c.state.sessionTransaction = c.txID
		if !c.inTx {
			return nil, errors.New("session insert was not transactional")
		}
		if c.state.sessionErr != nil {
			return nil, c.state.sessionErr
		}
		c.state.pendingSessions++
		return &protectedSessionRows{columns: []string{"id", "created_at"}, values: [][]driver.Value{{int64(500), time.Unix(10, 0)}}}, nil
	case strings.Contains(query, "INSERT INTO session_target_scores"):
		c.state.queryOrder = append(c.state.queryOrder, "score_insert")
		c.state.scoreAttempts++
		c.state.scoreArgs = append(c.state.scoreArgs, copyArgs)
		c.state.scoreTransaction = c.txID
		if !c.inTx {
			return nil, errors.New("score insert was not transactional")
		}
		if c.state.scoreErrAt == c.state.scoreAttempts {
			return nil, errors.New("score insert unavailable")
		}
		c.state.pendingScores++
		return &protectedSessionRows{columns: []string{"id", "created_at"}, values: [][]driver.Value{{int64(600 + c.state.scoreAttempts), time.Unix(11, 0)}}}, nil
	case strings.Contains(query, "manager_review_decision") && strings.Contains(query, "FROM keyworking_sessions") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "manager_session_lock")
		rows := &protectedSessionRows{columns: []string{"id", "child_id", "signed_by_key_worker", "key_worker_signed_at", "manager_review_decision"}}
		requestedSessionID, _ := args[0].Value.(int64)
		requestedChildID, _ := args[1].Value.(int64)
		if c.state.reviewSessionExists && requestedSessionID == int64(500) && requestedChildID == c.state.reviewSessionChildID {
			rows.values = [][]driver.Value{{int64(500), c.state.reviewSessionChildID, c.state.reviewSigned, c.state.reviewSignedAt, c.state.reviewDecision}}
		}
		return rows, nil
	case strings.Contains(query, "UPDATE keyworking_sessions") && strings.Contains(query, "manager_reviewed_by_staff_id"):
		c.state.queryOrder = append(c.state.queryOrder, "manager_review_update")
		c.state.reviewAttempts++
		c.state.reviewArgs = copyArgs
		c.state.reviewQuery = query
		if !c.inTx {
			return nil, errors.New("manager review update was not transactional")
		}
		if c.state.reviewErr != nil {
			return nil, c.state.reviewErr
		}
		c.state.pendingReviews++
		return &protectedSessionRows{columns: []string{"manager_signed_at"}, values: [][]driver.Value{{time.Unix(20, 0)}}}, nil
	case strings.Contains(query, "INSERT INTO psychological_reviews"):
		c.state.queryOrder = append(c.state.queryOrder, "psych_review_insert")
		c.state.psychReviewAttempts++
		c.state.psychReviewArgs = copyArgs
		if !c.inTx {
			return nil, errors.New("psychological review insert was not transactional")
		}
		if c.state.psychReviewErr != nil {
			return nil, c.state.psychReviewErr
		}
		c.state.pendingPsychReviews++
		return &protectedSessionRows{columns: []string{"id", "created_at"}, values: [][]driver.Value{{int64(700), time.Unix(21, 0)}}}, nil
	case strings.Contains(query, "SELECT child_id FROM keyworking_sessions") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "session_lock")
		rows := &protectedSessionRows{columns: []string{"child_id"}}
		if c.state.pendingSessions > 0 || c.state.pendingReviews > 0 {
			childID := int64(100)
			if c.state.pendingReviews > 0 {
				childID = c.state.reviewSessionChildID
			}
			rows.values = [][]driver.Value{{childID}}
		}
		return rows, nil
	case strings.Contains(query, "SELECT child_id FROM psychological_reviews") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "psych_review_lock")
		rows := &protectedSessionRows{columns: []string{"child_id"}}
		if c.state.pendingPsychReviews > 0 {
			rows.values = [][]driver.Value{{int64(100)}}
		}
		return rows, nil
	case strings.Contains(query, "INSERT INTO audit_events"):
		c.state.queryOrder = append(c.state.queryOrder, "audit_insert")
		c.state.auditAttempts++
		c.state.auditArgs = copyArgs
		c.state.auditTransaction = c.txID
		if !c.inTx {
			return nil, errors.New("audit insert was not transactional")
		}
		if c.state.auditErr != nil {
			return nil, c.state.auditErr
		}
		c.state.pendingAudits++
		return &protectedSessionRows{columns: []string{"id", "created_at"}, values: [][]driver.Value{{int64(900), time.Unix(12, 0)}}}, nil
	default:
		return nil, fmt.Errorf("unexpected protected-session query: %s", query)
	}
}

func (tx *protectedSessionTx) Commit() error {
	state := tx.conn.state
	state.commits++
	state.committedSessions += state.pendingSessions
	state.committedScores += state.pendingScores
	state.committedAudits += state.pendingAudits
	state.committedReviews += state.pendingReviews
	state.committedPsychReviews += state.pendingPsychReviews
	state.pendingSessions, state.pendingScores, state.pendingAudits, state.pendingReviews, state.pendingPsychReviews = 0, 0, 0, 0, 0
	tx.conn.inTx = false
	return nil
}
func (tx *protectedSessionTx) Rollback() error {
	state := tx.conn.state
	state.rollbacks++
	state.pendingSessions, state.pendingScores, state.pendingAudits, state.pendingReviews, state.pendingPsychReviews = 0, 0, 0, 0, 0
	tx.conn.inTx = false
	return nil
}
func (r *protectedSessionRows) Columns() []string { return r.columns }
func (r *protectedSessionRows) Close() error      { return nil }
func (r *protectedSessionRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func protectedSessionHTTPHandler(t *testing.T, state *protectedSessionDBState, staff map[string]*models.StaffUser) http.Handler {
	t.Helper()
	repo := storage.NewRepo(openProtectedSessionDB(t, state))
	handler := NewTherapeuticHandler(repo)
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.With(appmw.Authenticate(protectedGoalAuthenticator(t, staff))).Post("/v1/children/{childID}/sessions", handler.CreateSessionWithScores)
	return router
}

func sessionRequest(handler http.Handler, token, requestID, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/v1/children/100/sessions", bytes.NewBufferString(body))
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

func validSessionBody() string {
	return `{"session_type":"keywork","scores":[{"target_id":10,"wellbeing_score":2,"progress_score":3}]}`
}

func authorizedSessionState(role string) *protectedSessionDBState {
	return &protectedSessionDBState{
		staffExists: true, staffActive: true, role: role, roleActive: true,
		childExists: true, childHomeID: 10, targets: map[int64]int64{10: 100, 11: 100, 12: 100, 20: 100},
	}
}

func TestCreateSessionAuthenticationAndRolePolicy(t *testing.T) {
	subject := "session-role-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	tests := []struct {
		name, role, token string
		roleHome          int64
		want              int
	}{
		{name: "missing token", role: "key_worker", want: http.StatusUnauthorized},
		{name: "invalid token", role: "key_worker", token: "not-a-jwt", want: http.StatusUnauthorized},
		{name: "key worker accepted", role: "key_worker", token: token, want: http.StatusCreated},
		{name: "senior accepted", role: "senior", token: token, want: http.StatusCreated},
		{name: "manager accepted", role: "manager", token: token, want: http.StatusCreated},
		{name: "wrong home rejected", role: "senior", roleHome: 20, token: token, want: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedSessionState(test.role)
			state.roleHomeID = test.roleHome
			response := sessionRequest(protectedSessionHTTPHandler(t, state, staff), test.token, "", validSessionBody())
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want != http.StatusCreated && (state.committedSessions != 0 || state.committedScores != 0 || state.committedAudits != 0) {
				t.Fatalf("unauthorized session committed: %#v", state)
			}
		})
	}
}

func TestCreateSessionPrimaryTargetRelationships(t *testing.T) {
	subject := "session-primary-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	tests := []struct {
		name, targetJSON string
		targets          map[int64]int64
		want             int
	}{
		{name: "nil primary target", targets: map[int64]int64{10: 100}, want: http.StatusCreated},
		{name: "valid primary target", targetJSON: `,"target_id":20`, targets: map[int64]int64{10: 100, 20: 100}, want: http.StatusCreated},
		{name: "cross child primary target", targetJSON: `,"target_id":20`, targets: map[int64]int64{10: 100, 20: 200}, want: http.StatusForbidden},
		{name: "nonexistent primary target", targetJSON: `,"target_id":20`, targets: map[int64]int64{10: 100}, want: http.StatusForbidden},
		{name: "zero primary target", targetJSON: `,"target_id":0`, targets: map[int64]int64{10: 100}, want: http.StatusBadRequest},
		{name: "negative primary target", targetJSON: `,"target_id":-1`, targets: map[int64]int64{10: 100}, want: http.StatusBadRequest},
	}
	var forbiddenBody string
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedSessionState("key_worker")
			state.targets = test.targets
			body := fmt.Sprintf(`{"session_type":"keywork"%s,"scores":[{"target_id":10,"wellbeing_score":2,"progress_score":3}]}`, test.targetJSON)
			response := sessionRequest(protectedSessionHTTPHandler(t, state, staff), token, "", body)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want == http.StatusForbidden {
				if forbiddenBody == "" {
					forbiddenBody = response.Body.String()
				} else if response.Body.String() != forbiddenBody {
					t.Fatalf("primary target response disclosed existence: %q vs %q", response.Body.String(), forbiddenBody)
				}
			}
			if test.want != http.StatusCreated && state.sessionAttempts != 0 {
				t.Fatalf("relationship failure attempted session: %#v", state)
			}
		})
	}
}

func TestCreateSessionScoreTargetsAreValidatedAndLockedDeterministically(t *testing.T) {
	subject := "session-score-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	t.Run("valid duplicate targets are deduplicated and sorted", func(t *testing.T) {
		state := authorizedSessionState("senior")
		body := `{"session_type":"keywork","scores":[{"target_id":12,"wellbeing_score":2,"progress_score":3},{"target_id":10,"wellbeing_score":1,"progress_score":2},{"target_id":12,"wellbeing_score":4,"progress_score":4},{"target_id":11,"wellbeing_score":0,"progress_score":1}]}`
		response := sessionRequest(protectedSessionHTTPHandler(t, state, staff), token, "", body)
		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
		}
		want := []string{"target_lock:10", "target_lock:11", "target_lock:12"}
		if fmt.Sprint(state.lockedTargetOrder) != fmt.Sprint(want) {
			t.Fatalf("target lock order = %v, want %v", state.lockedTargetOrder, want)
		}
		if state.committedScores != 4 {
			t.Fatalf("committed scores = %d, want 4", state.committedScores)
		}
	})

	for _, test := range []struct {
		name    string
		targets map[int64]int64
		want    int
	}{
		{name: "cross child score target", targets: map[int64]int64{10: 100, 11: 200}, want: http.StatusForbidden},
		{name: "nonexistent score target", targets: map[int64]int64{10: 100}, want: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedSessionState("manager")
			state.targets = test.targets
			body := `{"session_type":"keywork","scores":[{"target_id":10,"wellbeing_score":2,"progress_score":3},{"target_id":11,"wellbeing_score":2,"progress_score":3}]}`
			response := sessionRequest(protectedSessionHTTPHandler(t, state, staff), token, "", body)
			if response.Code != test.want || state.sessionAttempts != 0 {
				t.Fatalf("relationship result: status=%d state=%#v", response.Code, state)
			}
		})
	}
}

func TestCreateSessionRequestValidation(t *testing.T) {
	subject := "session-validation-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	tests := []struct {
		name, body string
	}{
		{name: "no scores", body: `{"session_type":"keywork","scores":[]}`},
		{name: "zero score target", body: `{"session_type":"keywork","scores":[{"target_id":0,"wellbeing_score":2,"progress_score":3}]}`},
		{name: "negative score target", body: `{"session_type":"keywork","scores":[{"target_id":-1,"wellbeing_score":2,"progress_score":3}]}`},
		{name: "negative score", body: `{"session_type":"keywork","scores":[{"target_id":10,"wellbeing_score":-1,"progress_score":3}]}`},
		{name: "score above four", body: `{"session_type":"keywork","scores":[{"target_id":10,"wellbeing_score":2,"progress_score":5}]}`},
		{name: "unsupported session type", body: `{"session_type":"review","scores":[{"target_id":10,"wellbeing_score":2,"progress_score":3}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedSessionState("key_worker")
			response := sessionRequest(protectedSessionHTTPHandler(t, state, staff), token, "", test.body)
			if response.Code != http.StatusBadRequest || state.begins != 0 {
				t.Fatalf("validation result: status=%d state=%#v body=%s", response.Code, state, response.Body.String())
			}
		})
	}
}

func TestCreateSessionStructuredPersistenceAttributionAndAudit(t *testing.T) {
	subject := "session-success-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	state := authorizedSessionState("key_worker")
	body := `{"session_at":"2026-08-10T12:30:00Z","session_type":"keywork","staff_name":"Caller Spoof","notes":"Sensitive notes","primary_domain":"Emotional","target_id":20,"young_person_presentation":"Sensitive presentation","young_person_voice_note":"Sensitive voice","key_worker_reflection":"Sensitive reflection","signed_by_key_worker":true,"key_worker_signed_at":"2000-01-01T00:00:00Z","signed_by_manager":true,"manager_signed_at":"2000-01-01T00:00:00Z","manager_name":"Sensitive manager","scores":[{"target_id":10,"wellbeing_score":2,"progress_score":3,"comment":"Sensitive score comment"},{"target_id":11,"wellbeing_score":4,"progress_score":1,"comment":"Other sensitive comment"}]}`
	response := sessionRequest(protectedSessionHTTPHandler(t, state, staff), protectedGoalToken(subject), "caller-controlled-request-id", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
	}
	if state.committedSessions != 1 || state.committedScores != 2 || state.committedAudits != 1 {
		t.Fatalf("committed writes: %#v", state)
	}
	if state.sessionTransaction == 0 || state.sessionTransaction != state.scoreTransaction || state.sessionTransaction != state.auditTransaction {
		t.Fatalf("transaction IDs session=%d score=%d audit=%d", state.sessionTransaction, state.scoreTransaction, state.auditTransaction)
	}
	wantOrder := []string{
		"staff_lock", "role_lock", "child_lock",
		"target_lock:20", "target_lock:10", "target_lock:11",
		"session_insert", "score_insert", "score_insert",
		"session_lock", "child_lock", "audit_insert",
	}
	if fmt.Sprint(state.queryOrder) != fmt.Sprint(wantOrder) {
		t.Fatalf("query order = %v, want %v", state.queryOrder, wantOrder)
	}
	args := state.sessionArgs
	if args[0].Value != int64(100) || args[2].Value != "Database Actor" || args[2].Value == "Caller Spoof" {
		t.Fatalf("authoritative session attribution = %#v", args[:4])
	}
	if args[4].Value != "keywork" || args[5].Value != "Emotional" || args[6].Value != int64(20) || args[7].Value != "Sensitive presentation" || args[8].Value != "Sensitive voice" || args[9].Value != "Sensitive reflection" {
		t.Fatalf("structured session args = %#v", args[4:10])
	}
	if args[10].Value != true {
		t.Fatalf("signed_by_key_worker = %v", args[10].Value)
	}
	signedAt, ok := args[11].Value.(time.Time)
	if !ok || signedAt.Year() == 2000 {
		t.Fatalf("key_worker_signed_at was not server-generated: %v", args[11].Value)
	}
	if args[12].Value != false || args[13].Value != nil || args[14].Value != nil {
		t.Fatalf("caller controlled manager sign-off: signed=%v at=%v name=%v", args[12].Value, args[13].Value, args[14].Value)
	}
	metadataBytes, ok := state.auditArgs[10].Value.([]byte)
	if !ok {
		t.Fatalf("metadata type = %T", state.auditArgs[10].Value)
	}
	metadataText := string(metadataBytes)
	for _, forbidden := range []string{"Sensitive notes", "Sensitive presentation", "Sensitive voice", "Sensitive reflection", "Sensitive manager", "Sensitive score comment", "Other sensitive comment", "manager_name", "comment", "young_person"} {
		if strings.Contains(metadataText, forbidden) {
			t.Fatalf("audit contains narrative %q: %s", forbidden, metadataText)
		}
	}
	var metadata map[string]any
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["session_type"] != "keywork" || metadata["score_count"] != float64(2) || metadata["signed"] != true {
		t.Fatalf("operational metadata = %#v", metadata)
	}
	if requestID, _ := metadata["request_id"].(string); requestID == "" || requestID == "caller-controlled-request-id" {
		t.Fatalf("untrusted request ID used: %q", requestID)
	}
	if state.auditArgs[0].Value != int64(10) || state.auditArgs[1].Value != int64(100) || state.auditArgs[2].Value != int64(7) || state.auditArgs[3].Value != "database@example.test" || state.auditArgs[4].Value != "Database Actor" || state.auditArgs[5].Value != "keyworking_session.create" || state.auditArgs[6].Value != "keyworking_session" || state.auditArgs[7].Value != int64(500) {
		t.Fatalf("audit binding = %#v", state.auditArgs[:8])
	}
}

func TestCreateSessionFailuresAreAtomic(t *testing.T) {
	subject := "session-failure-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	body := `{"session_type":"keywork","scores":[{"target_id":10,"wellbeing_score":2,"progress_score":3},{"target_id":11,"wellbeing_score":3,"progress_score":4}]}`
	tests := []struct {
		name                                                      string
		configure                                                 func(*protectedSessionDBState)
		wantSessionAttempts, wantScoreAttempts, wantAuditAttempts int
	}{
		{name: "session insert failure", configure: func(s *protectedSessionDBState) { s.sessionErr = errors.New("session unavailable") }, wantSessionAttempts: 1},
		{name: "second score failure", configure: func(s *protectedSessionDBState) { s.scoreErrAt = 2 }, wantSessionAttempts: 1, wantScoreAttempts: 2},
		{name: "audit failure", configure: func(s *protectedSessionDBState) { s.auditErr = errors.New("audit unavailable") }, wantSessionAttempts: 1, wantScoreAttempts: 2, wantAuditAttempts: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedSessionState("manager")
			test.configure(state)
			response := sessionRequest(protectedSessionHTTPHandler(t, state, staff), token, "", body)
			if response.Code != http.StatusInternalServerError || state.rollbacks != 1 {
				t.Fatalf("failure result: status=%d state=%#v", response.Code, state)
			}
			if state.sessionAttempts != test.wantSessionAttempts || state.scoreAttempts != test.wantScoreAttempts || state.auditAttempts != test.wantAuditAttempts {
				t.Fatalf("attempts session=%d score=%d audit=%d", state.sessionAttempts, state.scoreAttempts, state.auditAttempts)
			}
			if state.committedSessions != 0 || state.committedScores != 0 || state.committedAudits != 0 {
				t.Fatalf("failed transaction committed data: %#v", state)
			}
		})
	}
}

func protectedManagerReviewHTTPHandler(t *testing.T, state *protectedSessionDBState, staff map[string]*models.StaffUser) http.Handler {
	t.Helper()
	repo := storage.NewRepo(openProtectedSessionDB(t, state))
	handler := NewTherapeuticHandler(repo)
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.With(appmw.Authenticate(protectedGoalAuthenticator(t, staff))).Post(
		"/v1/children/{childID}/sessions/{sessionID}/manager-review",
		handler.CreateManagerReview,
	)
	return router
}

func performManagerReviewRequest(handler http.Handler, token, body string, childID int64) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/v1/children/%d/sessions/500/manager-review", childID),
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

func authorizedManagerReviewState(role string) *protectedSessionDBState {
	state := authorizedSessionState(role)
	state.reviewSessionExists = true
	state.reviewSessionChildID = 100
	state.reviewSigned = true
	state.reviewSignedAt = time.Unix(15, 0)
	return state
}

func TestManagerReviewRequiresManagerRoleAndCompletedSession(t *testing.T) {
	subject := "manager-review-policy-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	for _, test := range []struct {
		name   string
		role   string
		signed bool
		token  string
		want   int
	}{
		{name: "manager can review", role: "manager", signed: true, token: token, want: http.StatusCreated},
		{name: "incomplete session rejected", role: "manager", signed: false, token: token, want: http.StatusConflict},
		{name: "key worker rejected", role: "key_worker", signed: true, token: token, want: http.StatusForbidden},
		{name: "senior rejected", role: "senior", signed: true, token: token, want: http.StatusForbidden},
		{name: "responsible individual rejected", role: "responsible_individual", signed: true, token: token, want: http.StatusForbidden},
		{name: "unauthenticated rejected", role: "manager", signed: true, want: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedManagerReviewState(test.role)
			state.reviewSigned = test.signed
			if !test.signed {
				state.reviewSignedAt = nil
			}
			response := performManagerReviewRequest(
				protectedManagerReviewHTTPHandler(t, state, staff),
				test.token,
				`{"decision":"approved","comment":"Reviewed"}`,
				100,
			)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want != http.StatusCreated && (state.committedReviews != 0 || state.committedAudits != 0) {
				t.Fatalf("rejected manager review committed: %#v", state)
			}
			if test.want == http.StatusCreated && (state.committedReviews != 1 || state.committedAudits != 1 || state.reviewArgs[2].Value != "approved" || !strings.Contains(state.reviewQuery, "signed_by_manager = ($3 = 'approved')")) {
				t.Fatalf("approved manager review did not persist final state: %#v", state)
			}
		})
	}
}

func TestManagerReviewPersistsAuthoritativeDecisionAndAuditWithoutEditingSession(t *testing.T) {
	subject := "manager-review-success-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	state := authorizedManagerReviewState("manager")
	response := performManagerReviewRequest(
		protectedManagerReviewHTTPHandler(t, state, staff),
		protectedGoalToken(subject),
		`{"decision":"returned_for_review","comment":"  Address this in future follow-up.  "}`,
		100,
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
	}
	if state.committedReviews != 1 || state.committedAudits != 1 {
		t.Fatalf("committed writes: %#v", state)
	}
	if len(state.reviewArgs) != 5 || state.reviewArgs[0].Value != int64(500) || state.reviewArgs[1].Value != int64(100) || state.reviewArgs[2].Value != string(models.ManagerReviewReturnedForReview) || state.reviewArgs[3].Value != "Address this in future follow-up." || state.reviewArgs[4].Value != int64(7) {
		t.Fatalf("manager review args = %#v", state.reviewArgs)
	}
	setClause := strings.Split(strings.Split(state.reviewQuery, "SET")[1], "WHERE")[0]
	for _, immutableColumn := range []string{
		"young_person_presentation", "young_person_voice_note", "key_worker_reflection",
		"notes_summary", "target_id", "primary_domain", "session_date", "staff_name",
	} {
		if strings.Contains(setClause, immutableColumn) {
			t.Fatalf("manager review changes original column %q: %s", immutableColumn, setClause)
		}
	}
	if state.auditArgs[2].Value != int64(7) || state.auditArgs[4].Value != "Database Actor" || state.auditArgs[5].Value != "keyworking.manager_review.create" || state.auditArgs[7].Value != int64(500) {
		t.Fatalf("authoritative audit binding = %#v", state.auditArgs[:8])
	}
	metadata, ok := state.auditArgs[10].Value.([]byte)
	if !ok || !strings.Contains(string(metadata), `"manager_review_decision":"returned_for_review"`) {
		t.Fatalf("manager review metadata = %q", metadata)
	}
	for _, forbidden := range []string{"Address this", "comment", "notes", "voice", "reflection"} {
		if strings.Contains(string(metadata), forbidden) {
			t.Fatalf("audit metadata contains narrative %q: %s", forbidden, metadata)
		}
	}
	var payload struct {
		ManagerReview models.KeyworkManagerReview `json:"manager_review"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ManagerReview.ReviewerName != "Database Actor" || payload.ManagerReview.Decision != models.ManagerReviewReturnedForReview || payload.ManagerReview.Comment == nil || *payload.ManagerReview.Comment != "Address this in future follow-up." {
		t.Fatalf("manager review response = %#v", payload.ManagerReview)
	}
}

func TestManagerReviewRelationshipValidationAndSingleFinalReview(t *testing.T) {
	subject := "manager-review-relationship-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	for _, test := range []struct {
		name      string
		configure func(*protectedSessionDBState)
		body      string
		want      int
	}{
		{
			name: "cross child session rejected",
			configure: func(state *protectedSessionDBState) {
				state.reviewSessionChildID = 200
			},
			body: `{"decision":"approved"}`,
			want: http.StatusForbidden,
		},
		{
			name: "duplicate final review rejected",
			configure: func(state *protectedSessionDBState) {
				state.reviewDecision = "approved"
			},
			body: `{"decision":"approved"}`,
			want: http.StatusConflict,
		},
		{
			name:      "return comment required",
			configure: func(*protectedSessionDBState) {},
			body:      `{"decision":"returned_for_review","comment":"   "}`,
			want:      http.StatusBadRequest,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := authorizedManagerReviewState("manager")
			test.configure(state)
			response := performManagerReviewRequest(protectedManagerReviewHTTPHandler(t, state, staff), token, test.body, 100)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.want, response.Body.String())
			}
			if state.committedReviews != 0 || state.committedAudits != 0 {
				t.Fatalf("invalid manager review committed: %#v", state)
			}
		})
	}
}

func TestManagerReviewAuditFailureRollsBackReview(t *testing.T) {
	subject := "manager-review-rollback-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	state := authorizedManagerReviewState("manager")
	state.auditErr = errors.New("audit unavailable")
	response := performManagerReviewRequest(
		protectedManagerReviewHTTPHandler(t, state, staff),
		protectedGoalToken(subject),
		`{"decision":"approved"}`,
		100,
	)
	if response.Code != http.StatusInternalServerError || state.rollbacks != 1 {
		t.Fatalf("failure result: status=%d state=%#v", response.Code, state)
	}
	if state.reviewAttempts != 1 || state.auditAttempts != 1 || state.committedReviews != 0 || state.committedAudits != 0 {
		t.Fatalf("manager review rollback was partial: %#v", state)
	}
}

var _ driver.Driver = protectedSessionDriver{}
var _ driver.ConnBeginTx = (*protectedSessionConn)(nil)
var _ driver.QueryerContext = (*protectedSessionConn)(nil)
var _ driver.ExecerContext = (*protectedSessionConn)(nil)
