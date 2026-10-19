package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
)

var (
	phase019DriverOnce sync.Once
	phase019StatesMu   sync.Mutex
	phase019States     = map[string]*phase019DBState{}
	phase019DSN        atomic.Int64
)

type phase019RoleRow struct {
	role   string
	active bool
}
type phase019Query struct {
	sql  string
	args []driver.NamedValue
	inTx bool
}
type phase019DBState struct {
	staffExists     bool
	staffActive     bool
	roles           []phase019RoleRow
	queryErr        error
	commitErr       error
	rollbackErr     error
	commits         int
	rollbacks       int
	auditInTx       bool
	authInTx        bool
	homeID          int64
	childID         int64
	queries         []phase019Query
	lockedTargetIDs []int64
	lockedGoalIDs   []int64
	auditArgs       []driver.NamedValue
}

type phase019Driver struct{}
type phase019Conn struct {
	state *phase019DBState
	inTx  bool
}
type phase019Tx struct{ conn *phase019Conn }
type phase019Rows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func openPhase019DB(t *testing.T, state *phase019DBState) *sql.DB {
	t.Helper()
	if state.homeID == 0 {
		state.homeID = 10
	}
	if state.childID == 0 {
		state.childID = 100
	}
	phase019DriverOnce.Do(func() { sql.Register("phase019-test", phase019Driver{}) })
	dsn := fmt.Sprintf("phase019-%d", phase019DSN.Add(1))
	phase019StatesMu.Lock()
	phase019States[dsn] = state
	phase019StatesMu.Unlock()
	db, err := sql.Open("phase019-test", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		phase019StatesMu.Lock()
		delete(phase019States, dsn)
		phase019StatesMu.Unlock()
	})
	return db
}

func (phase019Driver) Open(name string) (driver.Conn, error) {
	phase019StatesMu.Lock()
	defer phase019StatesMu.Unlock()
	state := phase019States[name]
	if state == nil {
		return nil, errors.New("unknown test database")
	}
	return &phase019Conn{state: state}, nil
}
func (c *phase019Conn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported")
}
func (c *phase019Conn) Close() error { return nil }
func (c *phase019Conn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *phase019Conn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.inTx = true
	return &phase019Tx{conn: c}, nil
}
func (c *phase019Conn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}
func (c *phase019Conn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	argsCopy := append([]driver.NamedValue(nil), args...)
	c.state.queries = append(c.state.queries, phase019Query{sql: query, args: argsCopy, inTx: c.inTx})
	if c.state.queryErr != nil {
		return nil, c.state.queryErr
	}
	switch {
	case strings.Contains(query, "SELECT status") && strings.Contains(query, "FROM staff_users") && strings.Contains(query, "FOR UPDATE"):
		c.state.authInTx = c.inTx
		rows := &phase019Rows{columns: []string{"status"}}
		if c.state.staffExists {
			status := "inactive"
			if c.state.staffActive {
				status = "active"
			}
			rows.values = [][]driver.Value{{status}}
		}
		return rows, nil
	case strings.Contains(query, "SELECT role, active") && strings.Contains(query, "FROM staff_home_roles"):
		c.state.authInTx = c.inTx
		rows := &phase019Rows{columns: []string{"role", "active"}}
		for _, role := range c.state.roles {
			rows.values = append(rows.values, []driver.Value{role.role, role.active})
		}
		return rows, nil
	case strings.Contains(query, "FROM staff_home_roles"):
		rows := &phase019Rows{columns: []string{"id", "staff_user_id", "home_id", "role", "active", "assigned_at"}}
		if c.state.staffExists && c.state.staffActive {
			for i, role := range c.state.roles {
				if role.active {
					rows.values = append(rows.values, []driver.Value{int64(i + 1), int64(7), int64(10), role.role, true, time.Unix(1, 0)})
				}
			}
		}
		return rows, nil
	case strings.Contains(query, "FROM staff_users") && strings.Contains(query, "external_auth_subject"):
		if strings.Contains(query, "status = 'active'") {
			return nil, errors.New("authentication lookup incorrectly filtered inactive staff")
		}
		status := "inactive"
		if c.state.staffActive {
			status = "active"
		}
		return &phase019Rows{columns: []string{"id", "email", "display_name", "external_auth_subject", "status", "created_at", "updated_at"}, values: [][]driver.Value{{int64(7), "db@example.test", "DB Name", "subject", status, time.Unix(1, 0), time.Unix(1, 0)}}}, nil
	case strings.Contains(query, "FROM children"):
		return &phase019Rows{columns: []string{"home_id"}, values: [][]driver.Value{{c.state.homeID}}}, nil
	case strings.Contains(query, "SELECT child_id, goal_id FROM child_targets"):
		if len(args) > 0 {
			c.state.lockedTargetIDs = append(c.state.lockedTargetIDs, args[0].Value.(int64))
		}
		return &phase019Rows{columns: []string{"child_id", "goal_id"}, values: [][]driver.Value{{c.state.childID, nil}}}, nil
	case strings.Contains(query, "SELECT child_id FROM child_targets"):
		if len(args) > 0 {
			c.state.lockedTargetIDs = append(c.state.lockedTargetIDs, args[0].Value.(int64))
		}
		return &phase019Rows{columns: []string{"child_id"}, values: [][]driver.Value{{c.state.childID}}}, nil
	case strings.Contains(query, "SELECT child_id FROM child_goals"):
		if len(args) > 0 {
			c.state.lockedGoalIDs = append(c.state.lockedGoalIDs, args[0].Value.(int64))
		}
		return &phase019Rows{columns: []string{"child_id"}, values: [][]driver.Value{{c.state.childID}}}, nil
	case strings.Contains(query, "SELECT child_id FROM keyworking_sessions"), strings.Contains(query, "SELECT child_id FROM child_onboarding_baselines"), strings.Contains(query, "SELECT child_id FROM psychological_reviews"), strings.Contains(query, "SELECT child_id FROM child_signal_responses"), strings.Contains(query, "SELECT child_id FROM child_risks"), strings.Contains(query, "SELECT child_id FROM child_care_plans"):
		return &phase019Rows{columns: []string{"child_id"}, values: [][]driver.Value{{c.state.childID}}}, nil
	case strings.Contains(query, "SELECT cp.child_id") && strings.Contains(query, "FROM child_care_plan_sections"):
		return &phase019Rows{columns: []string{"child_id"}, values: [][]driver.Value{{c.state.childID}}}, nil
	case strings.Contains(query, "INSERT INTO audit_events"):
		c.state.auditInTx = c.inTx
		c.state.auditArgs = argsCopy
		return &phase019Rows{columns: []string{"id", "created_at"}, values: [][]driver.Value{{int64(91), time.Unix(2, 0)}}}, nil
	default:
		return &phase019Rows{columns: []string{"exists"}, values: [][]driver.Value{{true}}}, nil
	}
}
func (tx *phase019Tx) Commit() error {
	tx.conn.state.commits++
	tx.conn.inTx = false
	return tx.conn.state.commitErr
}
func (tx *phase019Tx) Rollback() error {
	tx.conn.state.rollbacks++
	tx.conn.inTx = false
	return tx.conn.state.rollbackErr
}
func (r *phase019Rows) Columns() []string { return r.columns }
func (r *phase019Rows) Close() error      { return nil }
func (r *phase019Rows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func TestExternalSubjectLookupReturnsInactiveStaffForExplicitRejection(t *testing.T) {
	state := &phase019DBState{staffExists: true}
	staff, err := NewRepo(openPhase019DB(t, state)).GetStaffUserByExternalAuthSubject(context.Background(), "subject")
	if err != nil || staff.Status != "inactive" {
		t.Fatalf("staff = %#v, error = %v", staff, err)
	}
}

func TestStaffHasAnyHomeRoleRequiresActiveStaffAndRole(t *testing.T) {
	tests := []struct {
		name                                  string
		exists, staffActive, roleActive, want bool
		queryErr                              error
	}{
		{name: "active staff active role", exists: true, staffActive: true, roleActive: true, want: true},
		{name: "inactive staff active role", exists: true, roleActive: true},
		{name: "active staff inactive role", exists: true, staffActive: true},
		{name: "missing staff", roleActive: true},
		{name: "repository error", exists: true, staffActive: true, roleActive: true, queryErr: errors.New("database unavailable")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := &phase019DBState{staffExists: test.exists, staffActive: test.staffActive, roles: []phase019RoleRow{{role: "key_worker", active: test.roleActive}}, queryErr: test.queryErr}
			got, err := NewRepo(openPhase019DB(t, state)).StaffHasAnyHomeRole(context.Background(), 7, 10, security.RoleKeyWorker)
			if test.queryErr != nil {
				if err == nil {
					t.Fatal("expected repository error")
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("result = (%v, %v), want %v", got, err, test.want)
			}
		})
	}
}

func TestMutationRoleAuthorizationLocksStaffAndRoles(t *testing.T) {
	state := &phase019DBState{staffExists: true, staffActive: true, roles: []phase019RoleRow{{role: "key_worker", active: true}}}
	repo := NewRepo(openPhase019DB(t, state))
	err := repo.WithTx(context.Background(), nil, func(tx *TxRepo) error {
		principal := phase019Principal(t)
		ctx := security.WithPrincipal(context.Background(), principal)
		got, err := security.RequireHomeRoleForMutation(ctx, tx, 10, security.RoleKeyWorker)
		if err != nil || got.StaffUserID() != 7 {
			return fmt.Errorf("locking authorization = (%v, %v)", got.StaffUserID(), err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !state.authInTx {
		t.Fatal("locking authorization did not use transaction")
	}
	var staffLock, rolesLock bool
	for _, query := range state.queries {
		if strings.Contains(query.sql, "FROM staff_users") && strings.Contains(query.sql, "FOR UPDATE") {
			staffLock = query.inTx && len(query.args) == 1 && query.args[0].Value == int64(7)
		}
		if strings.Contains(query.sql, "FROM staff_home_roles") && strings.Contains(query.sql, "FOR UPDATE") {
			rolesLock = query.inTx && len(query.args) == 2 && query.args[0].Value == int64(7) && query.args[1].Value == int64(10)
		}
	}
	if !staffLock || !rolesLock {
		t.Fatalf("staffLock=%v rolesLock=%v", staffLock, rolesLock)
	}
}

func TestMutationRoleAuthorizationFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		state   *phase019DBState
		roles   []security.Role
		wantErr bool
	}{
		{name: "inactive staff", state: &phase019DBState{staffExists: true, roles: []phase019RoleRow{{role: "key_worker", active: true}}}, roles: []security.Role{security.RoleKeyWorker}},
		{name: "inactive role", state: &phase019DBState{staffExists: true, staffActive: true, roles: []phase019RoleRow{{role: "key_worker"}}}, roles: []security.Role{security.RoleKeyWorker}},
		{name: "missing staff", state: &phase019DBState{}, roles: []security.Role{security.RoleKeyWorker}},
		{name: "empty roles", state: &phase019DBState{staffExists: true, staffActive: true}},
		{name: "unknown role", state: &phase019DBState{staffExists: true, staffActive: true, roles: []phase019RoleRow{{role: "owner", active: true}}}, roles: []security.Role{security.Role("owner")}},
		{name: "database error", state: &phase019DBState{staffExists: true, staffActive: true, queryErr: errors.New("database unavailable")}, roles: []security.Role{security.RoleKeyWorker}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := NewRepo(openPhase019DB(t, test.state))
			err := repo.WithTx(context.Background(), nil, func(tx *TxRepo) error {
				allowed, err := tx.StaffHasAnyHomeRoleForMutation(context.Background(), 7, 10, test.roles...)
				if test.wantErr {
					return err
				}
				if err != nil {
					return err
				}
				if allowed {
					return errors.New("unauthorized mutation role accepted")
				}
				return nil
			})
			if test.wantErr && err == nil {
				t.Fatal("database failure did not fail closed")
			}
			if !test.wantErr && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWithTxCompletionPaths(t *testing.T) {
	t.Run("success commits", func(t *testing.T) {
		state := &phase019DBState{}
		err := NewRepo(openPhase019DB(t, state)).WithTx(context.Background(), nil, func(*TxRepo) error { return nil })
		if err != nil || state.commits != 1 || state.rollbacks != 0 {
			t.Fatalf("err=%v commits=%d rollbacks=%d", err, state.commits, state.rollbacks)
		}
	})
	t.Run("callback error rolls back", func(t *testing.T) {
		state := &phase019DBState{}
		callbackErr := errors.New("callback")
		err := NewRepo(openPhase019DB(t, state)).WithTx(context.Background(), nil, func(*TxRepo) error { return callbackErr })
		if !errors.Is(err, callbackErr) || state.rollbacks != 1 || state.commits != 0 {
			t.Fatalf("err=%v commits=%d rollbacks=%d", err, state.commits, state.rollbacks)
		}
	})
	t.Run("rollback error preserves both", func(t *testing.T) {
		rollbackErr, callbackErr := errors.New("rollback"), errors.New("callback")
		state := &phase019DBState{rollbackErr: rollbackErr}
		err := NewRepo(openPhase019DB(t, state)).WithTx(context.Background(), nil, func(*TxRepo) error { return callbackErr })
		if !errors.Is(err, callbackErr) || !errors.Is(err, rollbackErr) {
			t.Fatalf("joined error = %v", err)
		}
	})
	t.Run("panic rolls back and propagates", func(t *testing.T) {
		state := &phase019DBState{}
		panicValue := "panic-value"
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			_ = NewRepo(openPhase019DB(t, state)).WithTx(context.Background(), nil, func(*TxRepo) error { panic(panicValue) })
		}()
		if recovered != panicValue || state.rollbacks != 1 || state.commits != 0 {
			t.Fatalf("recovered=%v commits=%d rollbacks=%d", recovered, state.commits, state.rollbacks)
		}
	})
	t.Run("commit failure propagates", func(t *testing.T) {
		commitErr := errors.New("commit")
		state := &phase019DBState{commitErr: commitErr}
		err := NewRepo(openPhase019DB(t, state)).WithTx(context.Background(), nil, func(*TxRepo) error { return nil })
		if !errors.Is(err, commitErr) {
			t.Fatalf("commit error = %v", err)
		}
	})
}

func TestCompletedTxRepoDoesNotFallBackToDB(t *testing.T) {
	state := &phase019DBState{}
	repo := NewRepo(openPhase019DB(t, state))
	var escaped *TxRepo
	if err := repo.WithTx(context.Background(), nil, func(tx *TxRepo) error { escaped = tx; return nil }); err != nil {
		t.Fatal(err)
	}
	_, err := escaped.GetChildHomeID(context.Background(), 100)
	if !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("completed TxRepo error = %v, want %v", err, sql.ErrTxDone)
	}
}

func TestMutationRelationshipLocksAreDeterministicAndFailClosed(t *testing.T) {
	state := &phase019DBState{childID: 100}
	repo := NewRepo(openPhase019DB(t, state))
	err := repo.WithTx(context.Background(), nil, func(tx *TxRepo) error {
		ok, err := tx.LockSessionScoreTargetsBelongToChild(context.Background(), []int64{9, 2, 9, 5}, 100)
		if err != nil || !ok {
			return fmt.Errorf("score locks = (%v, %v)", ok, err)
		}
		ok, err = tx.LockSessionTargetBelongsToChild(context.Background(), nil, 100)
		if err != nil || !ok {
			return fmt.Errorf("nil target lock = (%v, %v)", ok, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(state.lockedTargetIDs) != "[2 5 9]" {
		t.Fatalf("lock order = %v", state.lockedTargetIDs)
	}
	broken := &phase019DBState{queryErr: errors.New("lookup failed")}
	err = NewRepo(openPhase019DB(t, broken)).WithTx(context.Background(), nil, func(tx *TxRepo) error {
		_, err := tx.LockTargetBelongsToChild(context.Background(), 2, 100)
		return err
	})
	if err == nil {
		t.Fatal("relationship database failure authorized")
	}
	mismatch := &phase019DBState{childID: 200}
	err = NewRepo(openPhase019DB(t, mismatch)).WithTx(context.Background(), nil, func(tx *TxRepo) error {
		belongs, err := tx.LockTargetBelongsToChild(context.Background(), 2, 100)
		if err != nil {
			return err
		}
		if belongs {
			return errors.New("wrong-child target authorized")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRiskPlanningLinksLockDeterministicallyAndRejectCrossChildLinks(t *testing.T) {
	state := &phase019DBState{childID: 100}
	err := NewRepo(openPhase019DB(t, state)).WithTx(context.Background(), nil, func(tx *TxRepo) error {
		ok, err := tx.LockRiskPlanningLinksBelongToChild(context.Background(), 100, []int64{7, 2, 7}, []int64{9, 3, 9})
		if err != nil || !ok {
			return fmt.Errorf("risk link locks = (%v, %v)", ok, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(state.lockedGoalIDs) != "[2 7]" || fmt.Sprint(state.lockedTargetIDs) != "[3 9]" {
		t.Fatalf("goal locks=%v target locks=%v", state.lockedGoalIDs, state.lockedTargetIDs)
	}

	mismatch := &phase019DBState{childID: 200}
	err = NewRepo(openPhase019DB(t, mismatch)).WithTx(context.Background(), nil, func(tx *TxRepo) error {
		ok, err := tx.LockRiskPlanningLinksBelongToChild(context.Background(), 100, []int64{2}, nil)
		if err != nil {
			return err
		}
		if ok {
			return errors.New("cross-child goal link authorized")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestResourceBoundAuditEvents(t *testing.T) {
	principal := phase019Principal(t)
	state := &phase019DBState{staffExists: true, staffActive: true, childID: 100, homeID: 10}
	repo := NewRepo(openPhase019DB(t, state))
	err := repo.WithTx(context.Background(), nil, func(tx *TxRepo) error {
		other := int64(999)
		if _, err := tx.NewGoalAuditEvent(context.Background(), principal, 300, AuditEventGoalCreate, security.AuditMetadataFields{GoalID: &other}, AuditRouteGoals); err == nil {
			return errors.New("goal metadata mismatch accepted")
		}
		if _, err := tx.NewTargetAuditEvent(context.Background(), principal, 400, AuditEventTargetCreate, security.AuditMetadataFields{TargetID: &other}, AuditRouteTargets); err == nil {
			return errors.New("target metadata mismatch accepted")
		}
		if _, err := tx.NewSessionAuditEvent(context.Background(), principal, 500, AuditEventSessionCreate, security.AuditMetadataFields{SessionID: &other}, AuditRouteSessions); err == nil {
			return errors.New("session metadata mismatch accepted")
		}
		if _, err := tx.NewGoalAuditEvent(context.Background(), principal, 300, AuditEventTargetCreate, security.AuditMetadataFields{}, AuditRouteGoals); err == nil {
			return errors.New("unrelated event type accepted")
		}
		if _, err := tx.NewGoalAuditEvent(context.Background(), principal, 300, AuditEventGoalCreate, security.AuditMetadataFields{}, AuditRouteTemplate("/v1/children/100/goals?notes=sensitive")); err == nil {
			return errors.New("raw request path accepted")
		}
		otherRisk := int64(998)
		if _, err := tx.NewRiskAuditEvent(context.Background(), principal, 900, AuditEventRiskUpdate, security.AuditMetadataFields{RiskID: &otherRisk}, AuditRouteRisks); err == nil {
			return errors.New("risk metadata mismatch accepted")
		}

		cases := []struct {
			name   string
			build  func() (BoundAuditEvent, error)
			entity string
			id     int64
		}{
			{"goal", func() (BoundAuditEvent, error) {
				return tx.NewGoalAuditEvent(context.Background(), principal, 300, AuditEventGoalCreate, security.AuditMetadataFields{}, AuditRouteGoals)
			}, "goal", 300},
			{"goal update", func() (BoundAuditEvent, error) {
				return tx.NewGoalAuditEvent(context.Background(), principal, 300, AuditEventGoalUpdate, security.AuditMetadataFields{NewStatus: security.AuditStatusPaused}, AuditRouteGoals)
			}, "goal", 300},
			{"target", func() (BoundAuditEvent, error) {
				return tx.NewTargetAuditEvent(context.Background(), principal, 400, AuditEventTargetCreate, security.AuditMetadataFields{}, AuditRouteTargets)
			}, "target", 400},
			{"target update", func() (BoundAuditEvent, error) {
				return tx.NewTargetAuditEvent(context.Background(), principal, 400, AuditEventTargetUpdate, security.AuditMetadataFields{NewStatus: security.AuditStatusAchieved}, AuditRouteTargets)
			}, "target", 400},
			{"session", func() (BoundAuditEvent, error) {
				return tx.NewSessionAuditEvent(context.Background(), principal, 500, AuditEventSessionCreate, security.AuditMetadataFields{SessionType: security.SessionTypeKeywork}, AuditRouteSessions)
			}, "keyworking_session", 500},
			{"baseline", func() (BoundAuditEvent, error) {
				return tx.NewBaselineAuditEvent(context.Background(), principal, 600, AuditEventBaselineApprove, security.AuditMetadataFields{Approved: boolPtr(true)}, AuditRouteBaselineApprove)
			}, "onboarding_baseline", 600},
			{"review", func() (BoundAuditEvent, error) {
				return tx.NewReviewAuditEvent(context.Background(), principal, 700, AuditEventReviewCreate, security.AuditMetadataFields{}, AuditRouteReviews)
			}, "psychological_review", 700},
			{"signal response", func() (BoundAuditEvent, error) {
				return tx.NewSignalResponseAuditEvent(context.Background(), principal, 800, AuditEventSignalResponseCreate, security.AuditMetadataFields{}, AuditRouteSignalResponses)
			}, "signal_response", 800},
			{"risk", func() (BoundAuditEvent, error) {
				goalID := int64(300)
				return tx.NewRiskAuditEvent(context.Background(), principal, 900, AuditEventRiskGoalLink, security.AuditMetadataFields{GoalID: &goalID}, AuditRouteRisks)
			}, "risk", 900},
			{"risk review", func() (BoundAuditEvent, error) {
				reviewID := int64(901)
				return tx.NewRiskAuditEvent(context.Background(), principal, 900, AuditEventRiskReviewCreate, security.AuditMetadataFields{RiskReviewID: &reviewID}, AuditRouteRiskReviews)
			}, "risk", 900},
			{"care plan", func() (BoundAuditEvent, error) {
				return tx.NewCarePlanAuditEvent(context.Background(), principal, 950, AuditEventCarePlanCreate, security.AuditMetadataFields{NewStatus: security.AuditStatusDraft}, AuditRouteCarePlans)
			}, "care_plan", 950},
			{"care plan section", func() (BoundAuditEvent, error) {
				return tx.NewCarePlanSectionAuditEvent(context.Background(), principal, 951, AuditEventCarePlanSectionUpdate, security.AuditMetadataFields{}, AuditRouteCarePlanSections)
			}, "care_plan_section", 951},
		}
		for _, test := range cases {
			bound, err := test.build()
			if err != nil {
				return fmt.Errorf("%s constructor: %w", test.name, err)
			}
			if bound.event.EntityType != test.entity || bound.event.EntityID == nil || *bound.event.EntityID != test.id || *bound.event.HomeID != 10 || *bound.event.ChildID != 100 || *bound.event.StaffUserID != principal.StaffUserID() || *bound.event.ActorEmail != principal.Email() || strings.Contains(*bound.event.RequestPath, "?") {
				return fmt.Errorf("%s binding incorrect: %#v", test.name, bound.event)
			}
			if strings.HasSuffix(test.name, "update") && (bound.event.RequestMethod == nil || *bound.event.RequestMethod != "PATCH") {
				return fmt.Errorf("%s request method = %v, want PATCH", test.name, bound.event.RequestMethod)
			}
			if test.name == "goal" {
				if _, err := tx.InsertAuditEvent(context.Background(), bound); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !state.auditInTx || len(state.auditArgs) != 11 {
		t.Fatalf("auditInTx=%v args=%d", state.auditInTx, len(state.auditArgs))
	}
	if state.auditArgs[0].Value != int64(10) || state.auditArgs[1].Value != int64(100) || state.auditArgs[2].Value != principal.StaffUserID() {
		t.Fatalf("audit attribution args = %#v", state.auditArgs[:3])
	}
}

type phase019StaffLookup struct{ staff *models.StaffUser }

func (f phase019StaffLookup) GetStaffUserByExternalAuthSubject(context.Context, string) (*models.StaffUser, error) {
	return f.staff, nil
}

func phase019Principal(t *testing.T) security.Principal {
	t.Helper()
	secret := "01234567890123456789012345678901"
	subject := "subject"
	t.Setenv("APP_ENV", "development")
	t.Setenv("AUTH_MODE", "dev-jwt")
	t.Setenv("ENABLE_DEV_AUTH", "true")
	t.Setenv("DEV_JWT_ISSUER", "issuer")
	t.Setenv("DEV_JWT_AUDIENCE", "audience")
	t.Setenv("DEV_JWT_HMAC_SECRET", secret)
	authenticator, err := security.NewDevJWTAuthenticatorFromEnv(phase019StaffLookup{staff: &models.StaffUser{ID: 7, Email: "db@example.test", DisplayName: "DB Name", ExternalAuthSubject: &subject, Status: "active"}})
	if err != nil {
		t.Fatal(err)
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"iss":"issuer","aud":"audience","sub":"subject","exp":%d}`, time.Now().Add(time.Hour).Unix())))
	unsigned := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	token := unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	principal, err := authenticator.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	return *principal
}

func boolPtr(value bool) *bool { return &value }

var _ driver.Driver = phase019Driver{}
var _ driver.ConnBeginTx = (*phase019Conn)(nil)
var _ driver.QueryerContext = (*phase019Conn)(nil)
var _ driver.ExecerContext = (*phase019Conn)(nil)
