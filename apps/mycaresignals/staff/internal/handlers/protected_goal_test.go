package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
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
	"therapeutic-engine/internal/security"
	"therapeutic-engine/internal/storage"
)

var (
	goalDriverOnce sync.Once
	goalStatesMu   sync.Mutex
	goalStates     = map[string]*protectedGoalDBState{}
	goalDSN        atomic.Int64
)

type protectedGoalDBState struct {
	staffExists            bool
	staffActive            bool
	role                   string
	roleActive             bool
	roleHomeID             int64
	childExists            bool
	childHomeID            int64
	goalExists             bool
	goalChildID            int64
	targetExists           bool
	targetChildID          int64
	riskExists             bool
	riskChildID            int64
	riskStatus             string
	riskResidualLikelihood driver.Value
	riskResidualSeverity   driver.Value
	carePlanExists         bool
	carePlanChildID        int64
	carePlanStatus         string
	carePlanPredecessorID  driver.Value
	predecessorPlanStatus  string
	otherActiveCarePlan    bool
	goalErr                error
	targetErr              error
	reviewErr              error
	riskReviewUpdateErr    error
	carePlanErr            error
	carePlanSectionErr     error
	auditErr               error

	begins, commits, rollbacks                                                                                                                                                                                                                      int
	goalAttempts, targetAttempts, riskAttempts, reviewAttempts, riskReviewUpdateAttempts, carePlanAttempts, carePlanSectionAttempts, carePlanStatusAttempts, carePlanReviewAttempts, carePlanSnapshotAttempts, auditAttempts                        int
	committedGoals, committedTargets, committedRisks, committedReviews, committedRiskReviewUpdates, committedCarePlans, committedCarePlanSections, committedCarePlanStatuses, committedCarePlanReviews, committedCarePlanSnapshots, committedAudits int
	pendingGoals, pendingTargets, pendingRisks, pendingReviews, pendingRiskReviewUpdates, pendingCarePlans, pendingCarePlanSections, pendingCarePlanStatuses, pendingCarePlanReviews, pendingCarePlanSnapshots, pendingAudits                       int
	queryOrder                                                                                                                                                                                                                                      []string
	goalArgs, targetArgs, reviewArgs, riskReviewUpdateArgs, carePlanArgs, carePlanSectionArgs, carePlanStatusArgs, auditArgs                                                                                                                        []driver.NamedValue
	riskReviewUpdateQuery                                                                                                                                                                                                                           string
	goalTransaction, targetTransaction, carePlanTransaction, carePlanSectionTransaction, auditTransaction                                                                                                                                           int64
	targetGoalID                                                                                                                                                                                                                                    driver.Value
}

type protectedGoalDriver struct{}
type protectedGoalConn struct {
	state *protectedGoalDBState
	inTx  bool
	txID  int64
}
type protectedGoalTx struct{ conn *protectedGoalConn }
type protectedGoalRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func openProtectedGoalDB(t *testing.T, state *protectedGoalDBState) *sql.DB {
	t.Helper()
	if state.childHomeID == 0 {
		state.childHomeID = 10
	}
	if state.roleHomeID == 0 {
		state.roleHomeID = state.childHomeID
	}
	if state.goalChildID == 0 {
		state.goalChildID = 100
	}
	if state.targetChildID == 0 {
		state.targetChildID = 100
	}
	if state.riskChildID == 0 {
		state.riskChildID = 100
	}
	if state.riskStatus == "" {
		state.riskStatus = "active"
	}
	if state.carePlanChildID == 0 {
		state.carePlanChildID = 100
	}
	if state.carePlanStatus == "" {
		state.carePlanStatus = "draft"
	}
	if state.predecessorPlanStatus == "" {
		state.predecessorPlanStatus = "active"
	}
	if state.riskResidualLikelihood == nil {
		state.riskResidualLikelihood = int64(2)
	}
	if state.riskResidualSeverity == nil {
		state.riskResidualSeverity = int64(3)
	}
	goalDriverOnce.Do(func() { sql.Register("protected-goal-test", protectedGoalDriver{}) })
	dsn := fmt.Sprintf("protected-goal-%d", goalDSN.Add(1))
	goalStatesMu.Lock()
	goalStates[dsn] = state
	goalStatesMu.Unlock()
	db, err := sql.Open("protected-goal-test", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close(); goalStatesMu.Lock(); delete(goalStates, dsn); goalStatesMu.Unlock() })
	return db
}

func (protectedGoalDriver) Open(name string) (driver.Conn, error) {
	goalStatesMu.Lock()
	defer goalStatesMu.Unlock()
	state := goalStates[name]
	if state == nil {
		return nil, errors.New("unknown protected-goal database")
	}
	return &protectedGoalConn{state: state}, nil
}
func (c *protectedGoalConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported")
}
func (c *protectedGoalConn) Close() error { return nil }
func (c *protectedGoalConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *protectedGoalConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.inTx = true
	c.txID++
	c.state.begins++
	c.state.pendingGoals = 0
	c.state.pendingTargets = 0
	c.state.pendingRisks = 0
	c.state.pendingReviews = 0
	c.state.pendingRiskReviewUpdates = 0
	c.state.pendingCarePlans = 0
	c.state.pendingCarePlanSections = 0
	c.state.pendingCarePlanStatuses = 0
	c.state.pendingCarePlanReviews = 0
	c.state.pendingCarePlanSnapshots = 0
	c.state.pendingAudits = 0
	return &protectedGoalTx{conn: c}, nil
}
func (c *protectedGoalConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if strings.Contains(query, "UPDATE child_risks") && strings.Contains(query, "last_reviewed_at") {
		c.state.queryOrder = append(c.state.queryOrder, "risk_review_update")
		c.state.riskReviewUpdateAttempts++
		c.state.riskReviewUpdateArgs = append([]driver.NamedValue(nil), args...)
		c.state.riskReviewUpdateQuery = query
		if c.state.riskReviewUpdateErr != nil {
			return nil, c.state.riskReviewUpdateErr
		}
		c.state.pendingRiskReviewUpdates++
	}
	return driver.RowsAffected(1), nil
}
func (c *protectedGoalConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	copyArgs := append([]driver.NamedValue(nil), args...)
	switch {
	case strings.Contains(query, "SELECT home_id") && strings.Contains(query, "FROM children") && !strings.Contains(query, "FOR UPDATE"):
		rows := &protectedGoalRows{columns: []string{"home_id"}}
		if c.state.childExists {
			rows.values = [][]driver.Value{{c.state.childHomeID}}
		}
		return rows, nil
	case strings.Contains(query, "FROM staff_home_roles AS shr") && strings.Contains(query, "JOIN staff_users AS su"):
		rows := &protectedGoalRows{columns: []string{"id", "staff_user_id", "home_id", "role", "active", "assigned_at"}}
		if c.state.staffExists && c.state.roleHomeID == c.state.childHomeID && c.state.role != "" {
			rows.values = [][]driver.Value{{int64(1), int64(7), c.state.roleHomeID, c.state.role, c.state.roleActive, time.Unix(10, 0)}}
		}
		return rows, nil
	case strings.Contains(query, "SELECT status") && strings.Contains(query, "FROM staff_users") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "staff_lock")
		rows := &protectedGoalRows{columns: []string{"status"}}
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
		rows := &protectedGoalRows{columns: []string{"home_id", "role", "active"}}
		if c.state.childExists && c.state.roleHomeID == c.state.childHomeID && c.state.role != "" {
			rows.values = [][]driver.Value{{c.state.roleHomeID, c.state.role, c.state.roleActive}}
		}
		return rows, nil
	case strings.Contains(query, "FROM children") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "child_lock")
		rows := &protectedGoalRows{columns: []string{"home_id"}}
		if c.state.childExists {
			rows.values = [][]driver.Value{{c.state.childHomeID}}
		}
		return rows, nil
	case strings.Contains(query, "INSERT INTO child_goals"):
		c.state.queryOrder = append(c.state.queryOrder, "goal_insert")
		c.state.goalAttempts++
		c.state.goalArgs = copyArgs
		c.state.goalTransaction = c.txID
		if !c.inTx {
			return nil, errors.New("goal insert was not transactional")
		}
		if c.state.goalErr != nil {
			return nil, c.state.goalErr
		}
		c.state.pendingGoals++
		return &protectedGoalRows{columns: []string{"id", "created_at", "updated_at"}, values: [][]driver.Value{{int64(300), time.Unix(10, 0), time.Unix(10, 0)}}}, nil
	case strings.Contains(query, "UPDATE child_goals"):
		c.state.queryOrder = append(c.state.queryOrder, "goal_update")
		c.state.goalAttempts++
		c.state.goalArgs = copyArgs
		c.state.goalTransaction = c.txID
		if !c.inTx {
			return nil, errors.New("goal update was not transactional")
		}
		if c.state.goalErr != nil {
			return nil, c.state.goalErr
		}
		c.state.pendingGoals++
		return &protectedGoalRows{columns: []string{"created_at", "updated_at"}, values: [][]driver.Value{{time.Unix(10, 0), time.Unix(20, 0)}}}, nil
	case strings.Contains(query, "SELECT child_id FROM child_goals") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "goal_lock")
		if c.state.pendingGoals > 0 {
			return &protectedGoalRows{columns: []string{"child_id"}, values: [][]driver.Value{{int64(100)}}}, nil
		}
		rows := &protectedGoalRows{columns: []string{"child_id"}}
		if c.state.goalExists {
			rows.values = [][]driver.Value{{c.state.goalChildID}}
		}
		return rows, nil
	case strings.Contains(query, "INSERT INTO child_targets"):
		c.state.queryOrder = append(c.state.queryOrder, "target_insert")
		c.state.targetAttempts++
		c.state.targetArgs = copyArgs
		c.state.targetTransaction = c.txID
		if !c.inTx {
			return nil, errors.New("target insert was not transactional")
		}
		if c.state.targetErr != nil {
			return nil, c.state.targetErr
		}
		c.state.pendingTargets++
		c.state.targetGoalID = copyArgs[1].Value
		return &protectedGoalRows{columns: []string{"id", "created_at", "updated_at"}, values: [][]driver.Value{{int64(400), time.Unix(10, 0), time.Unix(10, 0)}}}, nil
	case strings.Contains(query, "SELECT child_id FROM child_targets") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "target_child_lock")
		rows := &protectedGoalRows{columns: []string{"child_id"}}
		if c.state.targetExists {
			rows.values = [][]driver.Value{{c.state.targetChildID}}
		}
		return rows, nil
	case strings.Contains(query, "UPDATE child_targets"):
		c.state.queryOrder = append(c.state.queryOrder, "target_update")
		c.state.targetAttempts++
		c.state.targetArgs = copyArgs
		c.state.targetTransaction = c.txID
		if !c.inTx {
			return nil, errors.New("target update was not transactional")
		}
		if c.state.targetErr != nil {
			return nil, c.state.targetErr
		}
		c.state.pendingTargets++
		c.state.targetGoalID = copyArgs[2].Value
		return &protectedGoalRows{columns: []string{"created_at", "updated_at"}, values: [][]driver.Value{{time.Unix(10, 0), time.Unix(20, 0)}}}, nil
	case strings.Contains(query, "SELECT child_id, goal_id FROM child_targets") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "target_lock")
		rows := &protectedGoalRows{columns: []string{"child_id", "goal_id"}}
		if c.state.pendingTargets > 0 {
			rows.values = [][]driver.Value{{int64(100), c.state.targetGoalID}}
		}
		return rows, nil
	case strings.Contains(query, "INSERT INTO child_risks"):
		c.state.queryOrder = append(c.state.queryOrder, "risk_insert")
		c.state.riskAttempts++
		if !c.inTx {
			return nil, errors.New("risk insert was not transactional")
		}
		c.state.pendingRisks++
		return &protectedGoalRows{columns: []string{"id", "created_at", "updated_at"}, values: [][]driver.Value{{int64(500), time.Unix(10, 0), time.Unix(10, 0)}}}, nil
	case strings.Contains(query, "SELECT id, child_id, status, residual_likelihood, residual_severity, next_review_date") && strings.Contains(query, "FROM child_risks"):
		c.state.queryOrder = append(c.state.queryOrder, "risk_review_lock")
		rows := &protectedGoalRows{columns: []string{"id", "child_id", "status", "residual_likelihood", "residual_severity", "next_review_date"}}
		if c.state.riskExists && c.state.riskChildID == args[1].Value {
			rows.values = [][]driver.Value{{args[0].Value, c.state.riskChildID, c.state.riskStatus, c.state.riskResidualLikelihood, c.state.riskResidualSeverity, time.Unix(30, 0)}}
		}
		return rows, nil
	case strings.Contains(query, "INSERT INTO child_risk_reviews"):
		c.state.queryOrder = append(c.state.queryOrder, "risk_review_insert")
		c.state.reviewAttempts++
		c.state.reviewArgs = copyArgs
		if c.state.reviewErr != nil {
			return nil, c.state.reviewErr
		}
		c.state.pendingReviews++
		return &protectedGoalRows{columns: []string{"id", "reviewed_at", "created_at"}, values: [][]driver.Value{{int64(700), time.Unix(40, 0), time.Unix(40, 0)}}}, nil
	case strings.Contains(query, "SELECT child_id FROM child_risks") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "risk_lock")
		rows := &protectedGoalRows{columns: []string{"child_id"}}
		if c.state.pendingRisks > 0 || c.state.riskExists {
			rows.values = [][]driver.Value{{c.state.riskChildID}}
		}
		return rows, nil
	case strings.Contains(query, "INSERT INTO child_care_plans") && strings.Contains(query, "predecessor_care_plan_id"):
		c.state.queryOrder = append(c.state.queryOrder, "care_plan_successor_insert")
		c.state.carePlanAttempts++
		c.state.carePlanArgs = copyArgs
		c.state.pendingCarePlans++
		return &protectedGoalRows{columns: []string{"id", "effective_from", "created_at", "updated_at"}, values: [][]driver.Value{{int64(601), time.Unix(50, 0), time.Unix(50, 0), time.Unix(50, 0)}}}, nil
	case strings.Contains(query, "INSERT INTO child_care_plans"):
		c.state.queryOrder = append(c.state.queryOrder, "care_plan_insert")
		c.state.carePlanAttempts++
		c.state.carePlanArgs = copyArgs
		c.state.carePlanTransaction = c.txID
		if c.state.carePlanErr != nil {
			return nil, c.state.carePlanErr
		}
		c.state.pendingCarePlans++
		return &protectedGoalRows{columns: []string{"id", "created_at", "updated_at"}, values: [][]driver.Value{{int64(600), time.Unix(10, 0), time.Unix(10, 0)}}}, nil
	case strings.Contains(query, "INSERT INTO child_care_plan_sections"):
		c.state.queryOrder = append(c.state.queryOrder, "care_plan_section_upsert")
		c.state.carePlanSectionAttempts++
		c.state.carePlanSectionArgs = copyArgs
		c.state.carePlanSectionTransaction = c.txID
		if c.state.carePlanSectionErr != nil {
			return nil, c.state.carePlanSectionErr
		}
		c.state.pendingCarePlanSections++
		return &protectedGoalRows{columns: []string{"id", "created_at", "updated_at"}, values: [][]driver.Value{{int64(610 + c.state.carePlanSectionAttempts), time.Unix(10, 0), time.Unix(10, 0)}}}, nil
	case strings.Contains(query, "SELECT id, child_id, status, effective_from") && strings.Contains(query, "FROM child_care_plans") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "care_plan_lock")
		rows := &protectedGoalRows{columns: []string{"id", "child_id", "status", "effective_from", "review_date", "predecessor_care_plan_id", "created_by_staff_id", "activated_by_staff_id", "activated_at", "archived_by_staff_id", "archived_at", "created_at", "updated_at"}}
		if c.state.carePlanExists && c.state.carePlanChildID == args[1].Value {
			status := c.state.carePlanStatus
			predecessorID := c.state.carePlanPredecessorID
			if c.state.carePlanPredecessorID != nil && args[0].Value == c.state.carePlanPredecessorID {
				status = c.state.predecessorPlanStatus
				predecessorID = nil
			}
			rows.values = [][]driver.Value{{args[0].Value, c.state.carePlanChildID, status, time.Unix(1, 0), nil, predecessorID, int64(7), nil, nil, nil, nil, time.Unix(1, 0), time.Unix(1, 0)}}
		}
		return rows, nil
	case strings.Contains(query, "SELECT id, care_plan_id, section_key") && strings.Contains(query, "FROM child_care_plan_sections") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "care_plan_sections_lock")
		rows := &protectedGoalRows{columns: []string{"id", "care_plan_id", "section_key", "information_status", "current_understanding", "strengths", "staff_support", "young_person_involvement_note", "created_at", "updated_at"}}
		for index, key := range models.CanonicalCarePlanSectionKeys {
			rows.values = append(rows.values, []driver.Value{int64(620 + index), args[0].Value, string(key), "known", "understanding", "strength", "support", "involvement", time.Unix(2, 0), time.Unix(2, 0)})
		}
		return rows, nil
	case strings.Contains(query, "SELECT COALESCE") && strings.Contains(query, "FROM child_care_plan_reviews"):
		c.state.queryOrder = append(c.state.queryOrder, "care_plan_evidence_since")
		return &protectedGoalRows{columns: []string{"evidence_since"}, values: [][]driver.Value{{time.Unix(3, 0)}}}, nil
	case strings.Contains(query, "SELECT") && strings.Contains(query, "active_goal_count"):
		return nil, fmt.Errorf("unexpected named evidence query")
	case strings.Contains(query, "FROM child_goals WHERE child_id") && strings.Contains(query, "FROM child_risk_reviews"):
		c.state.queryOrder = append(c.state.queryOrder, "care_plan_evidence_counts")
		return &protectedGoalRows{columns: []string{"active_goals", "active_targets", "achieved_targets", "paused_targets", "targets_changed", "targets_due", "active_risks", "risk_reviews", "sessions", "not_known", "awaiting"}, values: [][]driver.Value{{int64(2), int64(2), int64(1), int64(1), int64(2), int64(1), int64(1), int64(1), int64(3), int64(0), int64(0)}}}, nil
	case strings.Contains(query, "INSERT INTO child_care_plan_reviews"):
		c.state.queryOrder = append(c.state.queryOrder, "care_plan_review_insert")
		c.state.carePlanReviewAttempts++
		c.state.pendingCarePlanReviews++
		return &protectedGoalRows{columns: []string{"id", "reviewed_at", "created_at"}, values: [][]driver.Value{{int64(800), time.Unix(60, 0), time.Unix(60, 0)}}}, nil
	case strings.Contains(query, "INSERT INTO child_care_plan_review_sections"):
		c.state.queryOrder = append(c.state.queryOrder, "care_plan_snapshot_insert")
		c.state.carePlanSnapshotAttempts++
		c.state.pendingCarePlanSnapshots++
		return &protectedGoalRows{columns: []string{"id", "created_at"}, values: [][]driver.Value{{int64(810 + c.state.carePlanSnapshotAttempts), time.Unix(60, 0)}}}, nil
	case strings.Contains(query, "UPDATE child_care_plans") && strings.Contains(query, "SET review_date"):
		c.state.queryOrder = append(c.state.queryOrder, "care_plan_review_date_update")
		c.state.carePlanStatusAttempts++
		c.state.carePlanStatusArgs = copyArgs
		c.state.pendingCarePlanStatuses++
		return &protectedGoalRows{columns: []string{"updated_at"}, values: [][]driver.Value{{time.Unix(61, 0)}}}, nil
	case strings.Contains(query, "SELECT id") && strings.Contains(query, "FROM child_care_plans") && strings.Contains(query, "status = 'active'"):
		rows := &protectedGoalRows{columns: []string{"id"}}
		if c.state.otherActiveCarePlan {
			rows.values = [][]driver.Value{{int64(601)}}
		}
		return rows, nil
	case strings.Contains(query, "UPDATE child_care_plans"):
		c.state.queryOrder = append(c.state.queryOrder, "care_plan_status_update")
		c.state.carePlanStatusAttempts++
		c.state.carePlanStatusArgs = copyArgs
		c.state.pendingCarePlanStatuses++
		return &protectedGoalRows{columns: []string{"changed_at", "updated_at"}, values: [][]driver.Value{{time.Unix(20, 0), time.Unix(20, 0)}}}, nil
	case strings.Contains(query, "SELECT child_id FROM child_care_plans") && strings.Contains(query, "FOR UPDATE"):
		c.state.queryOrder = append(c.state.queryOrder, "care_plan_audit_lock")
		rows := &protectedGoalRows{columns: []string{"child_id"}}
		if c.state.pendingCarePlans > 0 || c.state.carePlanExists {
			rows.values = [][]driver.Value{{c.state.carePlanChildID}}
		}
		return rows, nil
	case strings.Contains(query, "SELECT cp.child_id") && strings.Contains(query, "FROM child_care_plan_sections"):
		c.state.queryOrder = append(c.state.queryOrder, "care_plan_section_audit_lock")
		return &protectedGoalRows{columns: []string{"child_id"}, values: [][]driver.Value{{c.state.carePlanChildID}}}, nil
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
		return &protectedGoalRows{columns: []string{"id", "created_at"}, values: [][]driver.Value{{int64(900), time.Unix(11, 0)}}}, nil
	default:
		return nil, fmt.Errorf("unexpected protected-goal query: %s", query)
	}
}
func (tx *protectedGoalTx) Commit() error {
	tx.conn.state.commits++
	tx.conn.state.committedGoals += tx.conn.state.pendingGoals
	tx.conn.state.committedTargets += tx.conn.state.pendingTargets
	tx.conn.state.committedRisks += tx.conn.state.pendingRisks
	tx.conn.state.committedReviews += tx.conn.state.pendingReviews
	tx.conn.state.committedRiskReviewUpdates += tx.conn.state.pendingRiskReviewUpdates
	tx.conn.state.committedCarePlans += tx.conn.state.pendingCarePlans
	tx.conn.state.committedCarePlanSections += tx.conn.state.pendingCarePlanSections
	tx.conn.state.committedCarePlanStatuses += tx.conn.state.pendingCarePlanStatuses
	tx.conn.state.committedCarePlanReviews += tx.conn.state.pendingCarePlanReviews
	tx.conn.state.committedCarePlanSnapshots += tx.conn.state.pendingCarePlanSnapshots
	tx.conn.state.committedAudits += tx.conn.state.pendingAudits
	tx.conn.state.pendingGoals = 0
	tx.conn.state.pendingTargets = 0
	tx.conn.state.pendingRisks = 0
	tx.conn.state.pendingReviews = 0
	tx.conn.state.pendingRiskReviewUpdates = 0
	tx.conn.state.pendingCarePlans = 0
	tx.conn.state.pendingCarePlanSections = 0
	tx.conn.state.pendingCarePlanStatuses = 0
	tx.conn.state.pendingCarePlanReviews = 0
	tx.conn.state.pendingCarePlanSnapshots = 0
	tx.conn.state.pendingAudits = 0
	tx.conn.inTx = false
	return nil
}
func (tx *protectedGoalTx) Rollback() error {
	tx.conn.state.rollbacks++
	tx.conn.state.pendingGoals = 0
	tx.conn.state.pendingTargets = 0
	tx.conn.state.pendingRisks = 0
	tx.conn.state.pendingReviews = 0
	tx.conn.state.pendingRiskReviewUpdates = 0
	tx.conn.state.pendingCarePlans = 0
	tx.conn.state.pendingCarePlanSections = 0
	tx.conn.state.pendingCarePlanStatuses = 0
	tx.conn.state.pendingCarePlanReviews = 0
	tx.conn.state.pendingCarePlanSnapshots = 0
	tx.conn.state.pendingAudits = 0
	tx.conn.inTx = false
	return nil
}
func (r *protectedGoalRows) Columns() []string { return r.columns }
func (r *protectedGoalRows) Close() error      { return nil }
func (r *protectedGoalRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

type protectedGoalStaffLookup struct{ staff map[string]*models.StaffUser }

func (f protectedGoalStaffLookup) GetStaffUserByExternalAuthSubject(_ context.Context, subject string) (*models.StaffUser, error) {
	staff := f.staff[subject]
	if staff == nil {
		return nil, sql.ErrNoRows
	}
	return staff, nil
}

func protectedGoalAuthenticator(t *testing.T, staff map[string]*models.StaffUser) security.Authenticator {
	t.Helper()
	t.Setenv("APP_ENV", "development")
	t.Setenv("AUTH_MODE", "dev-jwt")
	t.Setenv("ENABLE_DEV_AUTH", "true")
	t.Setenv("DEV_JWT_ISSUER", "goal-test-issuer")
	t.Setenv("DEV_JWT_AUDIENCE", "goal-test-audience")
	t.Setenv("DEV_JWT_HMAC_SECRET", "01234567890123456789012345678901")
	authenticator, err := security.NewDevJWTAuthenticatorFromEnv(protectedGoalStaffLookup{staff: staff})
	if err != nil {
		t.Fatal(err)
	}
	return authenticator
}

func protectedGoalToken(subject string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"iss":"goal-test-issuer","aud":"goal-test-audience","sub":%q,"exp":%d}`, subject, time.Now().Add(time.Hour).Unix())))
	unsigned := header + "." + payload
	mac := hmac.New(sha256.New, []byte("01234567890123456789012345678901"))
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func protectedGoalHTTPHandler(t *testing.T, state *protectedGoalDBState, staff map[string]*models.StaffUser) http.Handler {
	t.Helper()
	repo := storage.NewRepo(openProtectedGoalDB(t, state))
	handler := NewGoalsHandler(repo)
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.With(appmw.Authenticate(protectedGoalAuthenticator(t, staff))).Post("/v1/children/{childID}/goals", handler.CreateGoal)
	return router
}

func protectedTargetHTTPHandler(t *testing.T, state *protectedGoalDBState, staff map[string]*models.StaffUser) http.Handler {
	t.Helper()
	repo := storage.NewRepo(openProtectedGoalDB(t, state))
	handler := NewGoalsHandler(repo)
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.With(appmw.Authenticate(protectedGoalAuthenticator(t, staff))).Post("/v1/children/{childID}/targets", handler.CreateTarget)
	return router
}

func protectedRiskHTTPHandler(t *testing.T, state *protectedGoalDBState, staff map[string]*models.StaffUser) http.Handler {
	t.Helper()
	repo := storage.NewRepo(openProtectedGoalDB(t, state))
	handler := NewRiskHandler(repo)
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.With(appmw.Authenticate(protectedGoalAuthenticator(t, staff))).Post("/v1/children/{childID}/risks", handler.CreateRisk)
	return router
}

func protectedRiskReviewHTTPHandler(t *testing.T, state *protectedGoalDBState, staff map[string]*models.StaffUser) http.Handler {
	t.Helper()
	repo := storage.NewRepo(openProtectedGoalDB(t, state))
	handler := NewRiskHandler(repo)
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.With(appmw.Authenticate(protectedGoalAuthenticator(t, staff))).Post("/v1/children/{childID}/risks/{riskID}/reviews", handler.CreateRiskReview)
	return router
}

func protectedGoalUpdateHTTPHandler(t *testing.T, state *protectedGoalDBState, staff map[string]*models.StaffUser) http.Handler {
	t.Helper()
	repo := storage.NewRepo(openProtectedGoalDB(t, state))
	handler := NewGoalsHandler(repo)
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.With(appmw.Authenticate(protectedGoalAuthenticator(t, staff))).Patch("/v1/children/{childID}/goals/{goalID}", handler.UpdateGoal)
	return router
}

func protectedTargetUpdateHTTPHandler(t *testing.T, state *protectedGoalDBState, staff map[string]*models.StaffUser) http.Handler {
	t.Helper()
	repo := storage.NewRepo(openProtectedGoalDB(t, state))
	handler := NewGoalsHandler(repo)
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.With(appmw.Authenticate(protectedGoalAuthenticator(t, staff))).Patch("/v1/children/{childID}/targets/{targetID}", handler.UpdateTarget)
	return router
}

func activeGoalStaff(subject string) *models.StaffUser {
	return &models.StaffUser{ID: 7, Email: "database@example.test", DisplayName: "Database Actor", ExternalAuthSubject: &subject, Status: "active"}
}

func goalRequest(handler http.Handler, token, requestID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/children/100/goals", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func targetRequest(handler http.Handler, token, requestID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/children/100/targets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func riskRequest(handler http.Handler, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/children/100/risks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func riskReviewRequest(handler http.Handler, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/children/100/risks/500/reviews", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestCreateRiskUsesSeniorManagerMutationPolicy(t *testing.T) {
	subject := "risk-role-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	body := `{"title":"Synthetic concern","category":"Local test","nature_of_risk":"Synthetic test description","information_status":"reported","management_approach":"controls_only"}`
	for _, test := range []struct {
		role string
		want int
	}{{"key_worker", http.StatusForbidden}, {"senior", http.StatusCreated}, {"manager", http.StatusCreated}} {
		t.Run(test.role, func(t *testing.T) {
			state := &protectedGoalDBState{staffExists: true, staffActive: true, role: test.role, roleActive: true, childExists: true}
			response := riskRequest(protectedRiskHTTPHandler(t, state, staff), token, body)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want == http.StatusForbidden && (state.committedRisks != 0 || state.committedAudits != 0) {
				t.Fatal("unauthorized risk mutation committed")
			}
			if test.want == http.StatusCreated && (state.committedRisks != 1 || state.committedAudits != 1) {
				t.Fatalf("risk commits=%d audit commits=%d", state.committedRisks, state.committedAudits)
			}
		})
	}
}

func TestCreateRiskReviewAuthenticationAuthorizationAndCrossChildPolicy(t *testing.T) {
	subject := "risk-review-role-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	body := `{"reviewed_residual_likelihood":2,"reviewed_residual_severity":3,"review_note":"Synthetic professional review","outcome":"continue"}`
	tests := []struct {
		name, role string
		token      string
		riskChild  int64
		want       int
	}{
		{name: "unauthenticated", role: "senior", want: http.StatusUnauthorized},
		{name: "key worker rejected", role: "key_worker", token: token, want: http.StatusForbidden},
		{name: "senior allowed", role: "senior", token: token, want: http.StatusCreated},
		{name: "manager allowed", role: "manager", token: token, want: http.StatusCreated},
		{name: "cross child rejected", role: "senior", token: token, riskChild: 200, want: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := &protectedGoalDBState{
				staffExists: true, staffActive: true, role: test.role, roleActive: true,
				childExists: true, riskExists: true, riskChildID: test.riskChild,
			}
			response := riskReviewRequest(protectedRiskReviewHTTPHandler(t, state, staff), test.token, body)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want != http.StatusCreated && (state.committedReviews != 0 || state.committedRiskReviewUpdates != 0 || state.committedAudits != 0) {
				t.Fatalf("unauthorized/cross-child review committed: %#v", state)
			}
		})
	}
}

func TestCreateRiskReviewCapturesPreviousRatingAndAuthoritativeReviewer(t *testing.T) {
	subject := "risk-review-authoritative-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	state := &protectedGoalDBState{
		staffExists: true, staffActive: true, role: "senior", roleActive: true,
		childExists: true, riskExists: true, riskResidualLikelihood: int64(2), riskResidualSeverity: int64(3),
	}
	body := `{"reviewer_staff_id":999,"reviewed_residual_likelihood":4,"reviewed_residual_severity":5,"review_note":"Synthetic reviewed evidence","outcome":"modify","next_review_date":"2026-09-01"}`
	response := riskReviewRequest(protectedRiskReviewHTTPHandler(t, state, staff), protectedGoalToken(subject), body)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if state.committedReviews != 1 || state.committedRiskReviewUpdates != 1 || state.committedAudits != 1 {
		t.Fatalf("transaction results: %#v", state)
	}
	if len(state.reviewArgs) != 10 || state.reviewArgs[2].Value != int64(7) || state.reviewArgs[3].Value != int64(2) || state.reviewArgs[4].Value != int64(3) || state.reviewArgs[5].Value != int64(4) || state.reviewArgs[6].Value != int64(5) {
		t.Fatalf("review insert args=%#v", state.reviewArgs)
	}
	if state.riskReviewUpdateArgs[2].Value != "modify" || state.riskReviewUpdateArgs[3].Value != int64(4) || state.riskReviewUpdateArgs[4].Value != int64(5) {
		t.Fatalf("risk review update args=%#v", state.riskReviewUpdateArgs)
	}
	if !strings.Contains(state.riskReviewUpdateQuery, "WHEN $3 IN ('modify', 'close')") {
		t.Fatal("risk review update does not preserve continue semantics")
	}
	var returned models.ChildRiskReview
	if err := json.Unmarshal(response.Body.Bytes(), &returned); err != nil {
		t.Fatal(err)
	}
	if returned.ReviewerStaffID != 7 || returned.ReviewerDisplayName != "Database Actor" || returned.PreviousResidualLikelihood == nil || *returned.PreviousResidualLikelihood != 2 {
		t.Fatalf("review response=%#v", returned)
	}
	metadata, ok := state.auditArgs[10].Value.([]byte)
	if !ok || !strings.Contains(string(metadata), `"risk_review_id":700`) || strings.Contains(string(metadata), "Synthetic reviewed evidence") {
		t.Fatalf("audit metadata=%s", metadata)
	}
}

func TestRiskReviewOutcomeRulesAndRollback(t *testing.T) {
	subject := "risk-review-outcome-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	base := func() *protectedGoalDBState {
		return &protectedGoalDBState{staffExists: true, staffActive: true, role: "manager", roleActive: true, childExists: true, riskExists: true, riskResidualLikelihood: int64(2), riskResidualSeverity: int64(3)}
	}
	t.Run("continue preserves current rating", func(t *testing.T) {
		state := base()
		response := riskReviewRequest(protectedRiskReviewHTTPHandler(t, state, staff), token, `{"reviewed_residual_likelihood":2,"reviewed_residual_severity":3,"review_note":"Continue synthetic plan","outcome":"continue"}`)
		if response.Code != http.StatusCreated || state.riskReviewUpdateArgs[2].Value != "continue" {
			t.Fatalf("status=%d state=%#v body=%s", response.Code, state, response.Body.String())
		}
	})
	t.Run("continue cannot silently change rating", func(t *testing.T) {
		state := base()
		response := riskReviewRequest(protectedRiskReviewHTTPHandler(t, state, staff), token, `{"reviewed_residual_likelihood":4,"reviewed_residual_severity":4,"review_note":"Invalid continue change","outcome":"continue"}`)
		if response.Code != http.StatusBadRequest || state.reviewAttempts != 0 || state.committedReviews != 0 {
			t.Fatalf("status=%d state=%#v", response.Code, state)
		}
	})
	t.Run("close closes through atomic update", func(t *testing.T) {
		state := base()
		response := riskReviewRequest(protectedRiskReviewHTTPHandler(t, state, staff), token, `{"reviewed_residual_likelihood":1,"reviewed_residual_severity":2,"review_note":"Close synthetic risk","outcome":"close","next_review_date":"2026-09-01"}`)
		if response.Code != http.StatusCreated || state.riskReviewUpdateArgs[2].Value != "close" || state.riskReviewUpdateArgs[6].Value != nil {
			t.Fatalf("status=%d update=%#v body=%s", response.Code, state.riskReviewUpdateArgs, response.Body.String())
		}
	})
	t.Run("audit failure rolls back review and risk update", func(t *testing.T) {
		state := base()
		state.auditErr = errors.New("audit unavailable")
		response := riskReviewRequest(protectedRiskReviewHTTPHandler(t, state, staff), token, `{"reviewed_residual_likelihood":4,"reviewed_residual_severity":4,"review_note":"Rollback synthetic review","outcome":"modify"}`)
		if response.Code != http.StatusInternalServerError || state.reviewAttempts != 1 || state.riskReviewUpdateAttempts != 1 || state.committedReviews != 0 || state.committedRiskReviewUpdates != 0 || state.committedAudits != 0 || state.rollbacks != 1 {
			t.Fatalf("status=%d state=%#v", response.Code, state)
		}
	})
}

func goalUpdateRequest(handler http.Handler, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, "/v1/children/100/goals/300", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func targetUpdateRequest(handler http.Handler, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, "/v1/children/100/targets/400", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestCreateGoalAuthenticationFailures(t *testing.T) {
	subject := "active-subject"
	tests := []struct {
		name, token string
		staff       map[string]*models.StaffUser
	}{
		{name: "no token"}, {name: "invalid token", token: "not-a-jwt"},
		{name: "unmapped staff", token: protectedGoalToken("unmapped")},
		{name: "inactive staff", token: protectedGoalToken("inactive"), staff: map[string]*models.StaffUser{"inactive": func() *models.StaffUser { s := activeGoalStaff("inactive"); s.Status = "inactive"; return s }()}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := &protectedGoalDBState{staffExists: true, staffActive: true, role: "senior", roleActive: true, childExists: true}
			staff := test.staff
			if staff == nil {
				staff = map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
			}
			response := goalRequest(protectedGoalHTTPHandler(t, state, staff), test.token, "", `{"title":"Private goal"}`)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401; body=%s", response.Code, response.Body.String())
			}
			if state.begins != 0 || state.goalAttempts != 0 || state.auditAttempts != 0 {
				t.Fatalf("authentication failure touched transaction: %#v", state)
			}
		})
	}
}

func TestCreateGoalRolePolicyAndNonDisclosure(t *testing.T) {
	subject := "role-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	tests := []struct {
		name, role  string
		roleHome    int64
		childExists bool
		want        int
	}{
		{name: "key worker rejected", role: "key_worker", childExists: true, want: http.StatusForbidden},
		{name: "senior accepted", role: "senior", childExists: true, want: http.StatusCreated},
		{name: "manager accepted", role: "manager", childExists: true, want: http.StatusCreated},
		{name: "wrong-home senior rejected", role: "senior", roleHome: 20, childExists: true, want: http.StatusForbidden},
		{name: "wrong-home manager rejected", role: "manager", roleHome: 20, childExists: true, want: http.StatusForbidden},
		{name: "nonexistent child", role: "senior", childExists: false, want: http.StatusForbidden},
	}
	var forbiddenBody string
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := &protectedGoalDBState{staffExists: true, staffActive: true, role: test.role, roleActive: true, roleHomeID: test.roleHome, childExists: test.childExists, childHomeID: 10}
			response := goalRequest(protectedGoalHTTPHandler(t, state, staff), token, "", `{"title":"Goal"}`)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want == http.StatusForbidden {
				if forbiddenBody == "" {
					forbiddenBody = response.Body.String()
				} else if response.Body.String() != forbiddenBody {
					t.Fatalf("forbidden response disclosed relationship: %q vs %q", response.Body.String(), forbiddenBody)
				}
				if state.committedGoals != 0 || state.committedAudits != 0 {
					t.Fatalf("unauthorized write committed: %#v", state)
				}
			} else if state.committedGoals != 1 || state.committedAudits != 1 {
				t.Fatalf("successful writes = goals %d audits %d", state.committedGoals, state.committedAudits)
			}
		})
	}
}

func TestCreateGoalTransactionAndAuditSafety(t *testing.T) {
	subject := "success-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	state := &protectedGoalDBState{staffExists: true, staffActive: true, role: "senior", roleActive: true, childExists: true, childHomeID: 10}
	body := `{"child_id":999,"goal_type":"therapeutic","title":"Narrative Title","description":"Sensitive Description","child_voice":"Sensitive Voice","formulation_summary":"Sensitive Formulation","protective_factors":"Sensitive Factors"}`
	response := goalRequest(protectedGoalHTTPHandler(t, state, staff), protectedGoalToken(subject), "caller-controlled-request-id", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
	}
	if state.committedGoals != 1 || state.committedAudits != 1 || state.goalAttempts != 1 || state.auditAttempts != 1 {
		t.Fatalf("write counts: %#v", state)
	}
	if state.goalTransaction == 0 || state.goalTransaction != state.auditTransaction {
		t.Fatalf("goal tx=%d audit tx=%d", state.goalTransaction, state.auditTransaction)
	}
	wantOrder := []string{"staff_lock", "role_lock", "child_lock", "goal_insert", "goal_lock", "child_lock", "audit_insert"}
	if fmt.Sprint(state.queryOrder) != fmt.Sprint(wantOrder) {
		t.Fatalf("query order = %v, want %v", state.queryOrder, wantOrder)
	}
	if got := state.goalArgs[0].Value; got != int64(100) {
		t.Fatalf("goal child_id = %v, want URL child 100", got)
	}
	metadataBytes, ok := state.auditArgs[10].Value.([]byte)
	if !ok {
		t.Fatalf("metadata type = %T", state.auditArgs[10].Value)
	}
	metadataText := string(metadataBytes)
	for _, forbidden := range []string{"Narrative Title", "Sensitive Description", "Sensitive Voice", "Sensitive Formulation", "Sensitive Factors", "goal_title", "description", "child_voice", "formulation_summary", "protective_factors"} {
		if strings.Contains(metadataText, forbidden) {
			t.Fatalf("audit metadata contains narrative %q: %s", forbidden, metadataText)
		}
	}
	var metadata map[string]any
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	requestID, _ := metadata["request_id"].(string)
	if requestID == "" || requestID == "caller-controlled-request-id" {
		t.Fatalf("untrusted request ID used: %q", requestID)
	}
	if state.auditArgs[2].Value != int64(7) || state.auditArgs[3].Value != "database@example.test" || state.auditArgs[4].Value != "Database Actor" {
		t.Fatalf("audit actor attribution = %#v", state.auditArgs[2:5])
	}
}

func TestCreateGoalWriteFailuresRollback(t *testing.T) {
	subject := "failure-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	t.Run("audit failure rolls back goal", func(t *testing.T) {
		state := &protectedGoalDBState{staffExists: true, staffActive: true, role: "manager", roleActive: true, childExists: true, auditErr: errors.New("audit unavailable")}
		response := goalRequest(protectedGoalHTTPHandler(t, state, staff), token, "", `{"title":"Goal"}`)
		if response.Code != http.StatusInternalServerError || state.goalAttempts != 1 || state.auditAttempts != 1 || state.committedGoals != 0 || state.committedAudits != 0 || state.rollbacks != 1 {
			t.Fatalf("audit failure state: status=%d %#v", response.Code, state)
		}
	})
	t.Run("goal failure creates no audit", func(t *testing.T) {
		state := &protectedGoalDBState{staffExists: true, staffActive: true, role: "manager", roleActive: true, childExists: true, goalErr: errors.New("goal unavailable")}
		response := goalRequest(protectedGoalHTTPHandler(t, state, staff), token, "", `{"title":"Goal"}`)
		if response.Code != http.StatusInternalServerError || state.goalAttempts != 1 || state.auditAttempts != 0 || state.committedGoals != 0 || state.committedAudits != 0 || state.rollbacks != 1 {
			t.Fatalf("goal failure state: status=%d %#v", response.Code, state)
		}
	})
}

func TestCreateTargetAuthenticationAndRolePolicy(t *testing.T) {
	subject := "target-role-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	tests := []struct {
		name, role string
		roleHome   int64
		token      string
		want       int
	}{
		{name: "missing token rejected", role: "senior", want: http.StatusUnauthorized},
		{name: "key worker rejected", role: "key_worker", token: token, want: http.StatusForbidden},
		{name: "senior accepted", role: "senior", token: token, want: http.StatusCreated},
		{name: "manager accepted", role: "manager", token: token, want: http.StatusCreated},
		{name: "wrong home rejected", role: "senior", roleHome: 20, token: token, want: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := &protectedGoalDBState{
				staffExists: true, staffActive: true, role: test.role, roleActive: true,
				roleHomeID: test.roleHome, childExists: true, childHomeID: 10,
			}
			response := targetRequest(protectedTargetHTTPHandler(t, state, staff), test.token, "", `{"domain":"emotional","short_label":"Target"}`)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want != http.StatusCreated && (state.committedTargets != 0 || state.committedAudits != 0) {
				t.Fatalf("unauthorized target write committed: %#v", state)
			}
			if test.want == http.StatusCreated && (state.committedTargets != 1 || state.committedAudits != 1) {
				t.Fatalf("successful writes = targets %d audits %d", state.committedTargets, state.committedAudits)
			}
		})
	}
}

func TestCreateTargetGoalRelationshipIsLockedAndNonDisclosing(t *testing.T) {
	subject := "target-relationship-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	tests := []struct {
		name        string
		goalID      string
		goalExists  bool
		goalChildID int64
		want        int
	}{
		{name: "valid goal accepted", goalID: "300", goalExists: true, goalChildID: 100, want: http.StatusCreated},
		{name: "cross child goal rejected", goalID: "300", goalExists: true, goalChildID: 200, want: http.StatusForbidden},
		{name: "nonexistent goal rejected", goalID: "300", want: http.StatusForbidden},
		{name: "nonpositive goal rejected", goalID: "0", want: http.StatusForbidden},
	}
	var forbiddenBody string
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := &protectedGoalDBState{
				staffExists: true, staffActive: true, role: "senior", roleActive: true,
				childExists: true, childHomeID: 10, goalExists: test.goalExists, goalChildID: test.goalChildID,
			}
			body := fmt.Sprintf(`{"goal_id":%s,"domain":"emotional","short_label":"Target"}`, test.goalID)
			response := targetRequest(protectedTargetHTTPHandler(t, state, staff), token, "", body)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.want, response.Body.String())
			}
			if test.want == http.StatusForbidden {
				if forbiddenBody == "" {
					forbiddenBody = response.Body.String()
				} else if response.Body.String() != forbiddenBody {
					t.Fatalf("relationship response disclosed existence: %q vs %q", response.Body.String(), forbiddenBody)
				}
				if state.targetAttempts != 0 || state.committedTargets != 0 || state.committedAudits != 0 {
					t.Fatalf("relationship failure wrote target: %#v", state)
				}
			}
		})
	}
}

func TestCreateTargetTransactionPayloadAndAuditSafety(t *testing.T) {
	subject := "target-success-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	state := &protectedGoalDBState{
		staffExists: true, staffActive: true, role: "manager", roleActive: true,
		childExists: true, childHomeID: 10, goalExists: true, goalChildID: 100,
	}
	body := `{"child_id":999,"home_id":999,"staff_user_id":999,"goal_id":300,"domain":"emotional","short_label":"Sensitive Label","description":"Sensitive Description","barriers":"  Sensitive Barriers  ","supports_needed":"  Sensitive Supports  ","intervention_strategy":"  Sensitive Strategy  "}`
	response := targetRequest(protectedTargetHTTPHandler(t, state, staff), protectedGoalToken(subject), "caller-controlled-request-id", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
	}
	if state.targetAttempts != 1 || state.auditAttempts != 1 || state.committedTargets != 1 || state.committedAudits != 1 {
		t.Fatalf("write counts: %#v", state)
	}
	if state.targetTransaction == 0 || state.targetTransaction != state.auditTransaction {
		t.Fatalf("target tx=%d audit tx=%d", state.targetTransaction, state.auditTransaction)
	}
	wantOrder := []string{"staff_lock", "role_lock", "child_lock", "goal_lock", "target_insert", "target_lock", "child_lock", "audit_insert"}
	if fmt.Sprint(state.queryOrder) != fmt.Sprint(wantOrder) {
		t.Fatalf("query order = %v, want %v", state.queryOrder, wantOrder)
	}
	if state.targetArgs[0].Value != int64(100) {
		t.Fatalf("target child_id = %v, want URL child 100", state.targetArgs[0].Value)
	}
	if state.targetArgs[1].Value != int64(300) {
		t.Fatalf("target goal_id = %v, want 300", state.targetArgs[1].Value)
	}
	for index, want := range map[int]string{6: "Sensitive Barriers", 7: "Sensitive Supports", 8: "Sensitive Strategy"} {
		if state.targetArgs[index].Value != want {
			t.Fatalf("target arg %d = %v, want %q", index, state.targetArgs[index].Value, want)
		}
	}
	metadataBytes, ok := state.auditArgs[10].Value.([]byte)
	if !ok {
		t.Fatalf("metadata type = %T", state.auditArgs[10].Value)
	}
	metadataText := string(metadataBytes)
	for _, forbidden := range []string{"Sensitive Label", "Sensitive Description", "Sensitive Barriers", "Sensitive Supports", "Sensitive Strategy", "short_label", "description", "barriers", "supports_needed", "intervention_strategy"} {
		if strings.Contains(metadataText, forbidden) {
			t.Fatalf("audit metadata contains narrative %q: %s", forbidden, metadataText)
		}
	}
	var metadata map[string]any
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["goal_id"] != float64(300) {
		t.Fatalf("authoritative audit goal_id = %v, want 300", metadata["goal_id"])
	}
	requestID, _ := metadata["request_id"].(string)
	if requestID == "" || requestID == "caller-controlled-request-id" {
		t.Fatalf("untrusted request ID used: %q", requestID)
	}
	if state.auditArgs[2].Value != int64(7) || state.auditArgs[3].Value != "database@example.test" || state.auditArgs[4].Value != "Database Actor" {
		t.Fatalf("audit actor attribution = %#v", state.auditArgs[2:5])
	}
	if state.auditArgs[0].Value != int64(10) || state.auditArgs[1].Value != int64(100) || state.auditArgs[5].Value != "target.create" || state.auditArgs[6].Value != "target" || state.auditArgs[7].Value != int64(400) {
		t.Fatalf("audit resource binding = %#v", state.auditArgs[:8])
	}
}

func TestCreateTargetWithoutGoalIsAccepted(t *testing.T) {
	subject := "target-no-goal-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	state := &protectedGoalDBState{staffExists: true, staffActive: true, role: "senior", roleActive: true, childExists: true}
	response := targetRequest(protectedTargetHTTPHandler(t, state, staff), protectedGoalToken(subject), "", `{"domain":"emotional","short_label":"Independent target"}`)
	if response.Code != http.StatusCreated || state.committedTargets != 1 || state.committedAudits != 1 {
		t.Fatalf("nil goal target result: status=%d state=%#v", response.Code, state)
	}
	if state.targetArgs[1].Value != nil {
		t.Fatalf("goal_id = %v, want nil", state.targetArgs[1].Value)
	}
	metadataBytes, ok := state.auditArgs[10].Value.([]byte)
	if !ok || strings.Contains(string(metadataBytes), "goal_id") {
		t.Fatalf("nil goal audit metadata = %v", state.auditArgs[10].Value)
	}
}

func TestCreateTargetWriteFailuresRollback(t *testing.T) {
	subject := "target-failure-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	t.Run("audit failure rolls back target", func(t *testing.T) {
		state := &protectedGoalDBState{staffExists: true, staffActive: true, role: "manager", roleActive: true, childExists: true, auditErr: errors.New("audit unavailable")}
		response := targetRequest(protectedTargetHTTPHandler(t, state, staff), token, "", `{"domain":"emotional","short_label":"Target"}`)
		if response.Code != http.StatusInternalServerError || state.targetAttempts != 1 || state.auditAttempts != 1 || state.committedTargets != 0 || state.committedAudits != 0 || state.rollbacks != 1 {
			t.Fatalf("audit failure state: status=%d %#v", response.Code, state)
		}
	})
	t.Run("target failure creates no audit", func(t *testing.T) {
		state := &protectedGoalDBState{staffExists: true, staffActive: true, role: "manager", roleActive: true, childExists: true, targetErr: errors.New("target unavailable")}
		response := targetRequest(protectedTargetHTTPHandler(t, state, staff), token, "", `{"domain":"emotional","short_label":"Target"}`)
		if response.Code != http.StatusInternalServerError || state.targetAttempts != 1 || state.auditAttempts != 0 || state.committedTargets != 0 || state.committedAudits != 0 || state.rollbacks != 1 {
			t.Fatalf("target failure state: status=%d %#v", response.Code, state)
		}
	})
}

func TestUpdateGoalPersistsPlanningFieldsAndAuditsStatus(t *testing.T) {
	subject := "goal-update-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	state := &protectedGoalDBState{
		staffExists: true, staffActive: true, role: "manager", roleActive: true,
		childExists: true, childHomeID: 10, goalExists: true, goalChildID: 100,
	}
	body := `{"goal_type":"care_plan","title":"A meaningful outcome","domain":"Relationships & Connection","description":"Professional rationale","start_date":"2026-08-01","target_date":"2026-12-01","status":"paused","child_voice":"Young person voice"}`
	response := goalUpdateRequest(protectedGoalUpdateHTTPHandler(t, state, staff), protectedGoalToken(subject), body)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
	}
	if state.committedGoals != 1 || state.committedAudits != 1 {
		t.Fatalf("committed updates = goals %d audits %d", state.committedGoals, state.committedAudits)
	}
	if state.goalArgs[0].Value != int64(300) || state.goalArgs[1].Value != int64(100) || state.goalArgs[4].Value != "Relationships & Connection" || state.goalArgs[8].Value != "paused" {
		t.Fatalf("goal update args = %#v", state.goalArgs)
	}
	if state.auditArgs[5].Value != "goal.update" || state.auditArgs[8].Value != "PATCH" {
		t.Fatalf("goal update audit = %#v", state.auditArgs)
	}
	metadataBytes, ok := state.auditArgs[10].Value.([]byte)
	if !ok || !strings.Contains(string(metadataBytes), `"new_status":"paused"`) || strings.Contains(string(metadataBytes), "Professional rationale") {
		t.Fatalf("goal update metadata = %v", state.auditArgs[10].Value)
	}
}

func TestUpdateTargetPersistsReviewAndRejectsCrossChildTarget(t *testing.T) {
	subject := "target-update-subject"
	staff := map[string]*models.StaffUser{subject: activeGoalStaff(subject)}
	token := protectedGoalToken(subject)
	body := `{"goal_id":300,"domain":"Relationships & Connection","short_label":"Share one view during a weekly session","description":"Observable progress","status":"achieved","supports_needed":"Offer time","barriers":"May need space","intervention_strategy":"Use a predictable check-in","review_date":"2026-10-15"}`

	t.Run("valid update", func(t *testing.T) {
		state := &protectedGoalDBState{
			staffExists: true, staffActive: true, role: "senior", roleActive: true,
			childExists: true, childHomeID: 10, goalExists: true, goalChildID: 100,
			targetExists: true, targetChildID: 100,
		}
		response := targetUpdateRequest(protectedTargetUpdateHTTPHandler(t, state, staff), token, body)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
		}
		if state.committedTargets != 1 || state.committedAudits != 1 {
			t.Fatalf("committed updates = targets %d audits %d", state.committedTargets, state.committedAudits)
		}
		if state.targetArgs[0].Value != int64(400) || state.targetArgs[1].Value != int64(100) || state.targetArgs[2].Value != int64(300) || state.targetArgs[10].Value == nil {
			t.Fatalf("target update args = %#v", state.targetArgs)
		}
		if state.auditArgs[5].Value != "target.update" || state.auditArgs[8].Value != "PATCH" {
			t.Fatalf("target update audit = %#v", state.auditArgs)
		}
	})

	t.Run("cross child target", func(t *testing.T) {
		state := &protectedGoalDBState{
			staffExists: true, staffActive: true, role: "senior", roleActive: true,
			childExists: true, childHomeID: 10, targetExists: true, targetChildID: 200,
		}
		response := targetUpdateRequest(protectedTargetUpdateHTTPHandler(t, state, staff), token, body)
		if response.Code != http.StatusForbidden || state.targetAttempts != 0 || state.committedAudits != 0 {
			t.Fatalf("cross-child update: status=%d state=%#v", response.Code, state)
		}
	})
}

var _ driver.Driver = protectedGoalDriver{}
var _ driver.ConnBeginTx = (*protectedGoalConn)(nil)
var _ driver.QueryerContext = (*protectedGoalConn)(nil)
var _ driver.ExecerContext = (*protectedGoalConn)(nil)
