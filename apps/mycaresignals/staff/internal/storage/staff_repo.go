package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
)

func (r *Repo) GetStaffUserByExternalAuthSubject(ctx context.Context, externalSubject string) (*models.StaffUser, error) {
	externalSubject = strings.TrimSpace(externalSubject)
	if externalSubject == "" {
		return nil, sql.ErrNoRows
	}
	const q = `
SELECT id, email, display_name, external_auth_subject, status, created_at, updated_at
FROM staff_users
WHERE external_auth_subject = $1`
	return r.scanStaffUser(ctx, q, externalSubject)
}

func (r *Repo) GetStaffUserByID(ctx context.Context, id int64) (*models.StaffUser, error) {
	if id <= 0 {
		return nil, sql.ErrNoRows
	}
	const q = `
SELECT id, email, display_name, external_auth_subject, status, created_at, updated_at
FROM staff_users
WHERE id = $1
  AND status = 'active'`
	return r.scanStaffUser(ctx, q, id)
}

func (r *Repo) GetLocalDevelopmentManagerSubject(ctx context.Context) (string, error) {
	const q = `
SELECT su.external_auth_subject
FROM staff_users AS su
JOIN staff_home_roles AS shr ON shr.staff_user_id = su.id
WHERE su.status = 'active'
  AND su.external_auth_subject IS NOT NULL
  AND trim(su.external_auth_subject) <> ''
  AND shr.active = TRUE
  AND shr.role = $1
ORDER BY su.id
LIMIT 1`
	var subject string
	if err := r.db.QueryRowContext(ctx, q, string(security.RoleManager)).Scan(&subject); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", sql.ErrNoRows
		}
		return "", fmt.Errorf("get local development manager subject: %w", err)
	}
	return strings.TrimSpace(subject), nil
}

func (r *Repo) scanStaffUser(ctx context.Context, q string, value any) (*models.StaffUser, error) {
	var staff models.StaffUser
	var externalAuthSubject sql.NullString
	if err := r.db.QueryRowContext(ctx, q, value).Scan(
		&staff.ID,
		&staff.Email,
		&staff.DisplayName,
		&externalAuthSubject,
		&staff.Status,
		&staff.CreatedAt,
		&staff.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("get staff user: %w", err)
	}
	if externalAuthSubject.Valid {
		subject := externalAuthSubject.String
		staff.ExternalAuthSubject = &subject
	}
	return &staff, nil
}

func (r *Repo) ListStaffHomeRoles(ctx context.Context, staffUserID, homeID int64) ([]models.StaffHomeRole, error) {
	return listStaffHomeRoles(ctx, r.db, staffUserID, homeID)
}

func (r *TxRepo) ListStaffHomeRoles(ctx context.Context, staffUserID, homeID int64) ([]models.StaffHomeRole, error) {
	return listStaffHomeRoles(ctx, r.q, staffUserID, homeID)
}

func listStaffHomeRoles(ctx context.Context, qtx DBTX, staffUserID, homeID int64) ([]models.StaffHomeRole, error) {
	const q = `
SELECT shr.id, shr.staff_user_id, shr.home_id, shr.role, shr.active, shr.assigned_at
FROM staff_home_roles AS shr
JOIN staff_users AS su ON su.id = shr.staff_user_id
WHERE shr.staff_user_id = $1
  AND shr.home_id = $2
  AND shr.active = TRUE
  AND su.status = 'active'
ORDER BY shr.role`

	rows, err := qtx.QueryContext(ctx, q, staffUserID, homeID)
	if err != nil {
		return nil, fmt.Errorf("list staff home roles: %w", err)
	}
	defer rows.Close()

	roles := make([]models.StaffHomeRole, 0)
	for rows.Next() {
		var role models.StaffHomeRole
		if err := rows.Scan(
			&role.ID,
			&role.StaffUserID,
			&role.HomeID,
			&role.Role,
			&role.Active,
			&role.AssignedAt,
		); err != nil {
			return nil, fmt.Errorf("scan staff home role: %w", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate staff home roles: %w", err)
	}
	return roles, nil
}

func (r *Repo) StaffHasAnyHomeRole(ctx context.Context, staffUserID, homeID int64, allowedRoles ...security.Role) (bool, error) {
	return staffHasAnyHomeRole(ctx, r.db, staffUserID, homeID, allowedRoles...)
}

func (r *TxRepo) StaffHasAnyHomeRole(ctx context.Context, staffUserID, homeID int64, allowedRoles ...security.Role) (bool, error) {
	return staffHasAnyHomeRole(ctx, r.q, staffUserID, homeID, allowedRoles...)
}

func staffHasAnyHomeRole(ctx context.Context, qtx DBTX, staffUserID, homeID int64, allowedRoles ...security.Role) (bool, error) {
	if staffUserID <= 0 || homeID <= 0 || len(allowedRoles) == 0 {
		return false, nil
	}
	for _, role := range allowedRoles {
		if !role.Valid() {
			return false, nil
		}
	}

	roles, err := listStaffHomeRoles(ctx, qtx, staffUserID, homeID)
	if err != nil {
		return false, err
	}
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		allowed[string(role)] = struct{}{}
	}
	for _, role := range roles {
		if role.Active {
			if _, ok := allowed[role.Role]; ok {
				return true, nil
			}
		}
	}
	return false, nil
}

// StaffHasAnyHomeRoleForMutation locks the staff row and all of that staff
// member's role assignments for the target home. Protected mutations must use
// this method inside the same transaction as the business write and audit.
func (r *TxRepo) StaffHasAnyHomeRoleForMutation(ctx context.Context, staffUserID, homeID int64, allowedRoles ...security.Role) (bool, error) {
	if staffUserID <= 0 || homeID <= 0 || len(allowedRoles) == 0 {
		return false, nil
	}
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		if !role.Valid() {
			return false, nil
		}
		allowed[string(role)] = struct{}{}
	}

	const staffQuery = `
SELECT status
FROM staff_users
WHERE id = $1
FOR UPDATE`
	var status string
	if err := r.q.QueryRowContext(ctx, staffQuery, staffUserID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("lock staff authorization row: %w", err)
	}
	if status != "active" {
		return false, nil
	}

	const rolesQuery = `
SELECT role, active
FROM staff_home_roles
WHERE staff_user_id = $1
  AND home_id = $2
ORDER BY role
FOR UPDATE`
	rows, err := r.q.QueryContext(ctx, rolesQuery, staffUserID, homeID)
	if err != nil {
		return false, fmt.Errorf("lock staff home roles: %w", err)
	}
	defer rows.Close()

	matched := false
	for rows.Next() {
		var role string
		var active bool
		if err := rows.Scan(&role, &active); err != nil {
			return false, fmt.Errorf("scan locked staff home role: %w", err)
		}
		if active {
			if _, ok := allowed[role]; ok {
				matched = true
			}
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate locked staff home roles: %w", err)
	}
	return matched, nil
}

// LockHomeID verifies and locks the authoritative home row after home-scoped
// mutation authorization. The current homes schema has no active flag.
func (r *TxRepo) LockHomeID(ctx context.Context, homeID int64) (bool, error) {
	if homeID <= 0 {
		return false, nil
	}
	const q = `SELECT id FROM homes WHERE id = $1 FOR UPDATE`
	var lockedID int64
	if err := r.q.QueryRowContext(ctx, q, homeID).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("lock home scope: %w", err)
	}
	return lockedID == homeID, nil
}

// LockStaffRolesAndChildForMutation is the child-scoped mutation authorization
// path. Lock order is staff -> matching home-role rows -> child. The child home
// is re-read under lock before authorization succeeds.
func (r *TxRepo) LockStaffRolesAndChildForMutation(ctx context.Context, staffUserID, childID int64, allowedRoles ...security.Role) (int64, bool, error) {
	if staffUserID <= 0 || childID <= 0 || len(allowedRoles) == 0 {
		return 0, false, nil
	}
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		if !role.Valid() {
			return 0, false, nil
		}
		allowed[string(role)] = struct{}{}
	}

	const staffQuery = `
SELECT status
FROM staff_users
WHERE id = $1
FOR UPDATE`
	var status string
	if err := r.q.QueryRowContext(ctx, staffQuery, staffUserID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("lock staff authorization row: %w", err)
	}
	if status != "active" {
		return 0, false, nil
	}

	const rolesQuery = `
SELECT shr.home_id, shr.role, shr.active
FROM staff_home_roles AS shr
JOIN children AS c ON c.home_id = shr.home_id
WHERE shr.staff_user_id = $1
  AND c.id = $2
ORDER BY shr.home_id, shr.role
FOR UPDATE OF shr`
	rows, err := r.q.QueryContext(ctx, rolesQuery, staffUserID, childID)
	if err != nil {
		return 0, false, fmt.Errorf("lock child home roles: %w", err)
	}
	defer rows.Close()

	var authorizedHomeID int64
	for rows.Next() {
		var homeID int64
		var role string
		var active bool
		if err := rows.Scan(&homeID, &role, &active); err != nil {
			return 0, false, fmt.Errorf("scan locked child home role: %w", err)
		}
		if active {
			if _, ok := allowed[role]; ok {
				authorizedHomeID = homeID
			}
		}
	}
	if err := rows.Err(); err != nil {
		return 0, false, fmt.Errorf("iterate locked child home roles: %w", err)
	}
	if authorizedHomeID <= 0 {
		return 0, false, nil
	}

	lockedHomeID, err := r.LockChildHomeID(ctx, childID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if lockedHomeID != authorizedHomeID {
		return 0, false, nil
	}
	return lockedHomeID, true, nil
}

func (r *Repo) GetChildHomeID(ctx context.Context, childID int64) (int64, error) {
	return getChildHomeID(ctx, r.db, childID)
}

func (r *TxRepo) GetChildHomeID(ctx context.Context, childID int64) (int64, error) {
	return getChildHomeID(ctx, r.q, childID)
}

func getChildHomeID(ctx context.Context, qtx DBTX, childID int64) (int64, error) {
	const q = `
SELECT home_id
FROM children
WHERE id = $1
  AND active = TRUE`

	var homeID int64
	if err := qtx.QueryRowContext(ctx, q, childID).Scan(&homeID); err != nil {
		return 0, fmt.Errorf("get child home id: %w", err)
	}
	return homeID, nil
}

// LockChildHomeID resolves and locks the authoritative child row for a future
// transaction whose correctness depends on the child/home relationship staying
// stable until commit. Read-only checks should continue using GetChildHomeID.
func (r *TxRepo) LockChildHomeID(ctx context.Context, childID int64) (int64, error) {
	if childID <= 0 {
		return 0, security.ErrInvalidRelationship
	}
	const q = `
SELECT home_id
FROM children
WHERE id = $1
  AND active = TRUE
FOR UPDATE`
	var homeID int64
	if err := r.q.QueryRowContext(ctx, q, childID).Scan(&homeID); err != nil {
		return 0, fmt.Errorf("lock child home scope: %w", err)
	}
	return homeID, nil
}

func (r *Repo) GoalBelongsToChild(ctx context.Context, goalID, childID int64) (bool, error) {
	return goalBelongsToChild(ctx, r.db, goalID, childID)
}

func (r *TxRepo) GoalBelongsToChild(ctx context.Context, goalID, childID int64) (bool, error) {
	return goalBelongsToChild(ctx, r.q, goalID, childID)
}

func goalBelongsToChild(ctx context.Context, qtx DBTX, goalID, childID int64) (bool, error) {
	const q = `SELECT EXISTS (
  SELECT 1 FROM child_goals WHERE id = $1 AND child_id = $2
)`
	var belongs bool
	if err := qtx.QueryRowContext(ctx, q, goalID, childID).Scan(&belongs); err != nil {
		return false, fmt.Errorf("check goal child ownership: %w", err)
	}
	return belongs, nil
}

// LockGoalBelongsToChild locks a goal before a future mutation when the
// mutation itself cannot be expressed with child-qualified SQL.
func (r *TxRepo) LockGoalBelongsToChild(ctx context.Context, goalID, childID int64) (bool, error) {
	return lockResourceBelongsToChild(ctx, r.q, "child_goals", goalID, childID)
}

func (r *Repo) TargetBelongsToChild(ctx context.Context, targetID, childID int64) (bool, error) {
	return targetBelongsToChild(ctx, r.db, targetID, childID)
}

func (r *TxRepo) TargetBelongsToChild(ctx context.Context, targetID, childID int64) (bool, error) {
	return targetBelongsToChild(ctx, r.q, targetID, childID)
}

func targetBelongsToChild(ctx context.Context, qtx DBTX, targetID, childID int64) (bool, error) {
	const q = `SELECT EXISTS (
  SELECT 1 FROM child_targets WHERE id = $1 AND child_id = $2
)`
	var belongs bool
	if err := qtx.QueryRowContext(ctx, q, targetID, childID).Scan(&belongs); err != nil {
		return false, fmt.Errorf("check target child ownership: %w", err)
	}
	return belongs, nil
}

// LockTargetBelongsToChild locks a target for session-target or score-target
// operations that cannot enforce child ownership directly in their write SQL.
func (r *TxRepo) LockTargetBelongsToChild(ctx context.Context, targetID, childID int64) (bool, error) {
	return lockResourceBelongsToChild(ctx, r.q, "child_targets", targetID, childID)
}

// LockSessionTargetBelongsToChild is the mutation-safe counterpart of the
// read-only optional session-target helper.
func (r *TxRepo) LockSessionTargetBelongsToChild(ctx context.Context, targetID *int64, childID int64) (bool, error) {
	if childID <= 0 {
		return false, security.ErrInvalidRelationship
	}
	if targetID == nil {
		return true, nil
	}
	return r.LockTargetBelongsToChild(ctx, *targetID, childID)
}

// LockSessionScoreTargetsBelongToChild deduplicates and sorts target IDs before
// locking, giving all mutation callers a deterministic lock order.
func (r *TxRepo) LockSessionScoreTargetsBelongToChild(ctx context.Context, targetIDs []int64, childID int64) (bool, error) {
	if childID <= 0 {
		return false, security.ErrInvalidRelationship
	}
	unique := make(map[int64]struct{}, len(targetIDs))
	for _, targetID := range targetIDs {
		if targetID <= 0 {
			return false, security.ErrInvalidRelationship
		}
		unique[targetID] = struct{}{}
	}
	ordered := make([]int64, 0, len(unique))
	for targetID := range unique {
		ordered = append(ordered, targetID)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for _, targetID := range ordered {
		belongs, err := r.LockTargetBelongsToChild(ctx, targetID, childID)
		if err != nil || !belongs {
			return false, err
		}
	}
	return true, nil
}

func lockResourceBelongsToChild(ctx context.Context, qtx DBTX, table string, resourceID, childID int64) (bool, error) {
	if resourceID <= 0 || childID <= 0 {
		return false, nil
	}
	var q string
	switch table {
	case "child_goals":
		q = `SELECT child_id FROM child_goals WHERE id = $1 FOR UPDATE`
	case "child_targets":
		q = `SELECT child_id FROM child_targets WHERE id = $1 FOR UPDATE`
	case "child_risks":
		q = `SELECT child_id FROM child_risks WHERE id = $1 FOR UPDATE`
	default:
		return false, errors.New("unsupported relationship lock")
	}
	var actualChildID int64
	if err := qtx.QueryRowContext(ctx, q, resourceID).Scan(&actualChildID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("lock resource child scope: %w", err)
	}
	return actualChildID == childID, nil
}

// ChildAuditScope can only be resolved from a transaction using authoritative
// child/home data and a verified Principal. Its fields are intentionally private.
type ChildAuditScope struct {
	homeID      int64
	childID     int64
	staffUserID int64
	actorEmail  string
	actorName   string
}

type resourceAuditScope struct {
	ChildAuditScope
	entityType string
	entityID   int64
}

// ResolveChildAuditScope derives audit attribution for a child resource without
// trusting home or actor fields from request JSON. The child row is locked so
// its home remains stable for the transaction.
func (r *TxRepo) ResolveChildAuditScope(ctx context.Context, principal security.Principal, childID int64) (ChildAuditScope, error) {
	if principal.StaffUserID() <= 0 || principal.ExternalSubject() == "" || childID <= 0 {
		return ChildAuditScope{}, security.ErrUnauthenticated
	}
	homeID, err := r.LockChildHomeID(ctx, childID)
	if err != nil {
		return ChildAuditScope{}, fmt.Errorf("resolve child audit scope: %w", err)
	}
	return ChildAuditScope{
		homeID: homeID, childID: childID, staffUserID: principal.StaffUserID(),
		actorEmail: principal.Email(), actorName: principal.DisplayName(),
	}, nil
}

// ResolveGoalAuditScope derives child/home scope from the protected goal row.
func (r *TxRepo) resolveGoalAuditScope(ctx context.Context, principal security.Principal, goalID int64) (resourceAuditScope, error) {
	return r.resolveResourceAuditScope(ctx, principal, "child_goals", goalID)
}

// ResolveTargetAuditScope derives child/home and optional goal scope from the
// protected target row. The goal relationship is read authoritatively rather
// than accepted from request metadata.
func (r *TxRepo) resolveTargetAuditScope(ctx context.Context, principal security.Principal, targetID int64) (resourceAuditScope, *int64, error) {
	if targetID <= 0 {
		return resourceAuditScope{}, nil, security.ErrInvalidRelationship
	}
	const q = `SELECT child_id, goal_id FROM child_targets WHERE id = $1 FOR UPDATE`
	var childID int64
	var goalID sql.NullInt64
	if err := r.q.QueryRowContext(ctx, q, targetID).Scan(&childID, &goalID); err != nil {
		return resourceAuditScope{}, nil, fmt.Errorf("resolve protected target audit scope: %w", err)
	}
	childScope, err := r.ResolveChildAuditScope(ctx, principal, childID)
	if err != nil {
		return resourceAuditScope{}, nil, err
	}
	scope := resourceAuditScope{ChildAuditScope: childScope, entityType: "target", entityID: targetID}
	if !goalID.Valid {
		return scope, nil, nil
	}
	resolvedGoalID := goalID.Int64
	return scope, &resolvedGoalID, nil
}

// ResolveSessionAuditScope derives child/home scope from the protected session row.
func (r *TxRepo) resolveSessionAuditScope(ctx context.Context, principal security.Principal, sessionID int64) (resourceAuditScope, error) {
	return r.resolveResourceAuditScope(ctx, principal, "keyworking_sessions", sessionID)
}

func (r *TxRepo) resolveResourceAuditScope(ctx context.Context, principal security.Principal, table string, resourceID int64) (resourceAuditScope, error) {
	if resourceID <= 0 {
		return resourceAuditScope{}, security.ErrInvalidRelationship
	}
	var q string
	var entityType string
	switch table {
	case "child_goals":
		q = `SELECT child_id FROM child_goals WHERE id = $1 FOR UPDATE`
		entityType = "goal"
	case "child_targets":
		q = `SELECT child_id FROM child_targets WHERE id = $1 FOR UPDATE`
		entityType = "target"
	case "keyworking_sessions":
		q = `SELECT child_id FROM keyworking_sessions WHERE id = $1 FOR UPDATE`
		entityType = "keyworking_session"
	case "child_onboarding_baselines":
		q = `SELECT child_id FROM child_onboarding_baselines WHERE id = $1 FOR UPDATE`
		entityType = "onboarding_baseline"
	case "psychological_reviews":
		q = `SELECT child_id FROM psychological_reviews WHERE id = $1 FOR UPDATE`
		entityType = "psychological_review"
	case "child_signal_responses":
		q = `SELECT child_id FROM child_signal_responses WHERE id = $1 FOR UPDATE`
		entityType = "signal_response"
	case "child_risks":
		q = `SELECT child_id FROM child_risks WHERE id = $1 FOR UPDATE`
		entityType = "risk"
	case "child_care_plans":
		q = `SELECT child_id FROM child_care_plans WHERE id = $1 FOR UPDATE`
		entityType = "care_plan"
	case "child_care_plan_sections":
		q = `SELECT cp.child_id
FROM child_care_plan_sections s
JOIN child_care_plans cp ON cp.id = s.care_plan_id
WHERE s.id = $1
FOR UPDATE OF s, cp`
		entityType = "care_plan_section"
	case "child_daily_progress":
		q = `SELECT child_id FROM child_daily_progress WHERE id = $1 FOR UPDATE`
		entityType = "daily_progress"
	default:
		return resourceAuditScope{}, security.ErrInvalidRelationship
	}
	var childID int64
	if err := r.q.QueryRowContext(ctx, q, resourceID).Scan(&childID); err != nil {
		return resourceAuditScope{}, fmt.Errorf("resolve protected resource audit scope: %w", err)
	}
	childScope, err := r.ResolveChildAuditScope(ctx, principal, childID)
	if err != nil {
		return resourceAuditScope{}, err
	}
	return resourceAuditScope{ChildAuditScope: childScope, entityType: entityType, entityID: resourceID}, nil
}

type AuditEventType string

const (
	AuditEventChildCreate             AuditEventType = "child.create"
	AuditEventChildView               AuditEventType = "child.view"
	AuditEventGoalCreate              AuditEventType = "goal.create"
	AuditEventGoalView                AuditEventType = "goal.view"
	AuditEventGoalUpdate              AuditEventType = "goal.update"
	AuditEventTargetCreate            AuditEventType = "target.create"
	AuditEventTargetView              AuditEventType = "target.view"
	AuditEventTargetUpdate            AuditEventType = "target.update"
	AuditEventSessionCreate           AuditEventType = "keyworking_session.create"
	AuditEventSessionView             AuditEventType = "keyworking_session.view"
	AuditEventSessionSign             AuditEventType = "keyworking_session.sign"
	AuditEventSessionManagerReview    AuditEventType = "keyworking.manager_review.create"
	AuditEventBaselineCreate          AuditEventType = "onboarding_baseline.create"
	AuditEventBaselineView            AuditEventType = "onboarding_baseline.view"
	AuditEventBaselineApprove         AuditEventType = "onboarding_baseline.approve"
	AuditEventReviewCreate            AuditEventType = "psychological_review.create"
	AuditEventReviewView              AuditEventType = "psychological_review.view"
	AuditEventSignalResponseCreate    AuditEventType = "signal_response.create"
	AuditEventSignalResponseView      AuditEventType = "signal_response.view"
	AuditEventRiskCreate              AuditEventType = "risk.create"
	AuditEventRiskView                AuditEventType = "risk.view"
	AuditEventRiskUpdate              AuditEventType = "risk.update"
	AuditEventRiskGoalLink            AuditEventType = "risk.goal.link"
	AuditEventRiskGoalUnlink          AuditEventType = "risk.goal.unlink"
	AuditEventRiskTargetLink          AuditEventType = "risk.target.link"
	AuditEventRiskTargetUnlink        AuditEventType = "risk.target.unlink"
	AuditEventRiskReviewCreate        AuditEventType = "risk.review.create"
	AuditEventCarePlanCreate          AuditEventType = "care_plan.create"
	AuditEventCarePlanSectionUpdate   AuditEventType = "care_plan.section.update"
	AuditEventCarePlanActivate        AuditEventType = "care_plan.activate"
	AuditEventCarePlanArchive         AuditEventType = "care_plan.archive"
	AuditEventCarePlanReviewCreate    AuditEventType = "care_plan.review.create"
	AuditEventCarePlanSuccessorCreate AuditEventType = "care_plan.successor.create"
	AuditEventFeedbackReviewCreate    AuditEventType = "external_feedback.review.create"
	AuditEventDailyProgressCreate     AuditEventType = "daily_progress.create"
)

type AuditRouteTemplate string

const (
	AuditRouteChildOnboard         AuditRouteTemplate = "/v1/homes/{homeID}/children/onboard"
	AuditRouteChildList            AuditRouteTemplate = "/v1/homes/{homeID}/children"
	AuditRouteGoals                AuditRouteTemplate = "/v1/children/{childID}/goals"
	AuditRouteTargets              AuditRouteTemplate = "/v1/children/{childID}/targets"
	AuditRouteSessions             AuditRouteTemplate = "/v1/children/{childID}/sessions"
	AuditRouteSessionManagerReview AuditRouteTemplate = "/v1/children/{childID}/sessions/{sessionID}/manager-review"
	AuditRouteBaselineSave         AuditRouteTemplate = "/v1/homes/{homeID}/children/{childID}/onboarding/baseline"
	AuditRouteBaseline             AuditRouteTemplate = "/v1/children/{childID}/onboarding/baseline"
	AuditRouteBaselineApprove      AuditRouteTemplate = "/v1/children/{childID}/onboarding/approve"
	AuditRouteFrameworkSave        AuditRouteTemplate = "/v1/homes/{homeID}/children/{childID}/onboarding/framework"
	AuditRouteFramework            AuditRouteTemplate = "/v1/children/{childID}/onboarding/framework"
	AuditRouteFrameworkApprove     AuditRouteTemplate = "/v1/children/{childID}/onboarding/framework/approve"
	AuditRouteReviews              AuditRouteTemplate = "/v1/children/{childID}/reviews"
	AuditRouteSignalResponses      AuditRouteTemplate = "/v1/children/{childID}/signal_responses"
	AuditRouteRisks                AuditRouteTemplate = "/v1/children/{childID}/risks"
	AuditRouteRiskReviews          AuditRouteTemplate = "/v1/children/{childID}/risks/{riskID}/reviews"
	AuditRouteCarePlans            AuditRouteTemplate = "/v1/children/{childID}/care-plans"
	AuditRouteCarePlanSections     AuditRouteTemplate = "/v1/children/{childID}/care-plans/{carePlanID}/sections/{sectionKey}"
	AuditRouteCarePlanActivate     AuditRouteTemplate = "/v1/children/{childID}/care-plans/{carePlanID}/activate"
	AuditRouteCarePlanArchive      AuditRouteTemplate = "/v1/children/{childID}/care-plans/{carePlanID}/archive"
	AuditRouteCarePlanReviews      AuditRouteTemplate = "/v1/children/{childID}/care-plans/{carePlanID}/reviews"
	AuditRouteMonthlyVoiceReviews  AuditRouteTemplate = "/v1/children/{childID}/monthly-voice/{receiptID}/reviews"
	AuditRouteDailyProgress        AuditRouteTemplate = "/v1/children/{childID}/daily-progress"
)

// BoundAuditEvent can only be produced by the resource-specific constructors.
type BoundAuditEvent struct {
	event    models.AuditEvent
	metadata security.AuditMetadata
}

func (r *TxRepo) NewChildAuditEvent(ctx context.Context, principal security.Principal, childID int64, eventType AuditEventType, fields security.AuditMetadataFields, route AuditRouteTemplate) (BoundAuditEvent, error) {
	scope, err := r.ResolveChildAuditScope(ctx, principal, childID)
	if err != nil {
		return BoundAuditEvent{}, err
	}
	return newBoundAuditEvent(resourceAuditScope{ChildAuditScope: scope, entityType: "child", entityID: childID}, eventType, fields, route)
}

func (r *TxRepo) NewGoalAuditEvent(ctx context.Context, principal security.Principal, goalID int64, eventType AuditEventType, fields security.AuditMetadataFields, route AuditRouteTemplate) (BoundAuditEvent, error) {
	scope, err := r.resolveGoalAuditScope(ctx, principal, goalID)
	if err != nil {
		return BoundAuditEvent{}, err
	}
	return newBoundAuditEvent(scope, eventType, fields, route)
}

func (r *TxRepo) NewTargetAuditEvent(ctx context.Context, principal security.Principal, targetID int64, eventType AuditEventType, fields security.AuditMetadataFields, route AuditRouteTemplate) (BoundAuditEvent, error) {
	scope, authoritativeGoalID, err := r.resolveTargetAuditScope(ctx, principal, targetID)
	if err != nil {
		return BoundAuditEvent{}, err
	}
	if fields.GoalID != nil && (authoritativeGoalID == nil || *fields.GoalID != *authoritativeGoalID) {
		return BoundAuditEvent{}, errors.New("audit metadata goal ID does not match authoritative target relationship")
	}
	fields.GoalID = authoritativeGoalID
	return newBoundAuditEvent(scope, eventType, fields, route)
}

func (r *TxRepo) NewSessionAuditEvent(ctx context.Context, principal security.Principal, sessionID int64, eventType AuditEventType, fields security.AuditMetadataFields, route AuditRouteTemplate) (BoundAuditEvent, error) {
	scope, err := r.resolveSessionAuditScope(ctx, principal, sessionID)
	if err != nil {
		return BoundAuditEvent{}, err
	}
	return newBoundAuditEvent(scope, eventType, fields, route)
}

func (r *TxRepo) NewBaselineAuditEvent(ctx context.Context, principal security.Principal, baselineID int64, eventType AuditEventType, fields security.AuditMetadataFields, route AuditRouteTemplate) (BoundAuditEvent, error) {
	scope, err := r.resolveResourceAuditScope(ctx, principal, "child_onboarding_baselines", baselineID)
	if err != nil {
		return BoundAuditEvent{}, err
	}
	return newBoundAuditEvent(scope, eventType, fields, route)
}

func (r *TxRepo) NewReviewAuditEvent(ctx context.Context, principal security.Principal, reviewID int64, eventType AuditEventType, fields security.AuditMetadataFields, route AuditRouteTemplate) (BoundAuditEvent, error) {
	scope, err := r.resolveResourceAuditScope(ctx, principal, "psychological_reviews", reviewID)
	if err != nil {
		return BoundAuditEvent{}, err
	}
	return newBoundAuditEvent(scope, eventType, fields, route)
}

func (r *TxRepo) NewSignalResponseAuditEvent(ctx context.Context, principal security.Principal, responseID int64, eventType AuditEventType, fields security.AuditMetadataFields, route AuditRouteTemplate) (BoundAuditEvent, error) {
	scope, err := r.resolveResourceAuditScope(ctx, principal, "child_signal_responses", responseID)
	if err != nil {
		return BoundAuditEvent{}, err
	}
	return newBoundAuditEvent(scope, eventType, fields, route)
}

func (r *TxRepo) NewRiskAuditEvent(ctx context.Context, principal security.Principal, riskID int64, eventType AuditEventType, fields security.AuditMetadataFields, route AuditRouteTemplate) (BoundAuditEvent, error) {
	scope, err := r.resolveResourceAuditScope(ctx, principal, "child_risks", riskID)
	if err != nil {
		return BoundAuditEvent{}, err
	}
	return newBoundAuditEvent(scope, eventType, fields, route)
}

func (r *TxRepo) NewCarePlanAuditEvent(ctx context.Context, principal security.Principal, planID int64, eventType AuditEventType, fields security.AuditMetadataFields, route AuditRouteTemplate) (BoundAuditEvent, error) {
	scope, err := r.resolveResourceAuditScope(ctx, principal, "child_care_plans", planID)
	if err != nil {
		return BoundAuditEvent{}, err
	}
	return newBoundAuditEvent(scope, eventType, fields, route)
}

func (r *TxRepo) NewCarePlanSectionAuditEvent(ctx context.Context, principal security.Principal, sectionID int64, eventType AuditEventType, fields security.AuditMetadataFields, route AuditRouteTemplate) (BoundAuditEvent, error) {
	scope, err := r.resolveResourceAuditScope(ctx, principal, "child_care_plan_sections", sectionID)
	if err != nil {
		return BoundAuditEvent{}, err
	}
	return newBoundAuditEvent(scope, eventType, fields, route)
}

func (r *TxRepo) NewDailyProgressAuditEvent(ctx context.Context, principal security.Principal, progressID int64, eventType AuditEventType, fields security.AuditMetadataFields, route AuditRouteTemplate) (BoundAuditEvent, error) {
	const q = `SELECT child_id, target_id FROM child_daily_progress WHERE id = $1 FOR UPDATE`
	var childID int64
	var targetID sql.NullInt64
	if err := r.q.QueryRowContext(ctx, q, progressID).Scan(&childID, &targetID); err != nil {
		return BoundAuditEvent{}, fmt.Errorf("resolve daily progress audit scope: %w", err)
	}
	childScope, err := r.ResolveChildAuditScope(ctx, principal, childID)
	if err != nil {
		return BoundAuditEvent{}, err
	}
	authoritativeTargetID := int64FromNull(targetID)
	if fields.TargetID != nil && (authoritativeTargetID == nil || *fields.TargetID != *authoritativeTargetID) {
		return BoundAuditEvent{}, errors.New("audit metadata target ID does not match authoritative daily progress relationship")
	}
	fields.TargetID = authoritativeTargetID
	return newBoundAuditEvent(
		resourceAuditScope{ChildAuditScope: childScope, entityType: "daily_progress", entityID: progressID},
		eventType, fields, route,
	)
}

func newBoundAuditEvent(scope resourceAuditScope, eventType AuditEventType, fields security.AuditMetadataFields, route AuditRouteTemplate) (BoundAuditEvent, error) {
	if scope.homeID <= 0 || scope.childID <= 0 || scope.staffUserID <= 0 || scope.entityID <= 0 || !auditEventMatchesEntity(eventType, scope.entityType) || !auditRouteMatchesEvent(route, eventType) {
		return BoundAuditEvent{}, errors.New("audit event does not match authoritative resource scope")
	}
	metadata, err := bindAuditMetadata(fields, scope.entityType, scope.entityID)
	if err != nil {
		return BoundAuditEvent{}, err
	}
	entityID := scope.entityID
	method := "POST"
	if strings.HasSuffix(string(eventType), ".view") {
		method = "GET"
	} else if strings.HasSuffix(string(eventType), ".update") {
		method = "PATCH"
	} else if strings.HasSuffix(string(eventType), ".unlink") {
		method = "DELETE"
	}
	event := models.AuditEvent{
		HomeID: &scope.homeID, ChildID: &scope.childID, StaffUserID: &scope.staffUserID,
		ActorEmail: &scope.actorEmail, ActorName: &scope.actorName,
		EventType: string(eventType), EntityType: scope.entityType, EntityID: &entityID,
		RequestMethod: &method, RequestPath: nullableNonEmptyString(string(route)),
	}
	return BoundAuditEvent{event: event, metadata: metadata}, nil
}

func (r *TxRepo) InsertAuditEvent(ctx context.Context, bound BoundAuditEvent) (*models.AuditEvent, error) {
	event := bound.event
	if event.EntityID == nil || event.HomeID == nil || event.ChildID == nil || event.StaffUserID == nil {
		return nil, errors.New("bound audit event is required")
	}
	if err := createAuditEvent(ctx, r.q, &event, bound.metadata); err != nil {
		return nil, err
	}
	return &event, nil
}

func auditEventMatchesEntity(eventType AuditEventType, entityType string) bool {
	allowed := map[string]map[AuditEventType]struct{}{
		"child":                {AuditEventChildCreate: {}, AuditEventChildView: {}},
		"goal":                 {AuditEventGoalCreate: {}, AuditEventGoalView: {}, AuditEventGoalUpdate: {}},
		"target":               {AuditEventTargetCreate: {}, AuditEventTargetView: {}, AuditEventTargetUpdate: {}},
		"keyworking_session":   {AuditEventSessionCreate: {}, AuditEventSessionView: {}, AuditEventSessionSign: {}, AuditEventSessionManagerReview: {}},
		"onboarding_baseline":  {AuditEventBaselineCreate: {}, AuditEventBaselineView: {}, AuditEventBaselineApprove: {}},
		"psychological_review": {AuditEventReviewCreate: {}, AuditEventReviewView: {}},
		"signal_response":      {AuditEventSignalResponseCreate: {}, AuditEventSignalResponseView: {}},
		"risk": {
			AuditEventRiskCreate: {}, AuditEventRiskView: {}, AuditEventRiskUpdate: {},
			AuditEventRiskGoalLink: {}, AuditEventRiskGoalUnlink: {},
			AuditEventRiskTargetLink: {}, AuditEventRiskTargetUnlink: {},
			AuditEventRiskReviewCreate: {},
		},
		"care_plan": {
			AuditEventCarePlanCreate: {}, AuditEventCarePlanActivate: {}, AuditEventCarePlanArchive: {},
			AuditEventCarePlanReviewCreate: {}, AuditEventCarePlanSuccessorCreate: {},
		},
		"care_plan_section": {AuditEventCarePlanSectionUpdate: {}},
		"daily_progress":    {AuditEventDailyProgressCreate: {}},
	}
	_, ok := allowed[entityType][eventType]
	return ok
}

func auditRouteMatchesEvent(route AuditRouteTemplate, eventType AuditEventType) bool {
	allowed := map[AuditEventType]map[AuditRouteTemplate]struct{}{
		AuditEventChildCreate: {AuditRouteChildOnboard: {}}, AuditEventChildView: {AuditRouteChildList: {}},
		AuditEventGoalCreate: {AuditRouteGoals: {}}, AuditEventGoalView: {AuditRouteGoals: {}}, AuditEventGoalUpdate: {AuditRouteGoals: {}},
		AuditEventTargetCreate: {AuditRouteTargets: {}}, AuditEventTargetView: {AuditRouteTargets: {}}, AuditEventTargetUpdate: {AuditRouteTargets: {}},
		AuditEventSessionCreate: {AuditRouteSessions: {}}, AuditEventSessionView: {AuditRouteSessions: {}}, AuditEventSessionSign: {AuditRouteSessions: {}},
		AuditEventSessionManagerReview: {AuditRouteSessionManagerReview: {}},
		AuditEventBaselineCreate:       {AuditRouteBaselineSave: {}, AuditRouteFrameworkSave: {}}, AuditEventBaselineView: {AuditRouteBaseline: {}, AuditRouteFramework: {}}, AuditEventBaselineApprove: {AuditRouteBaselineApprove: {}, AuditRouteFrameworkApprove: {}},
		AuditEventReviewCreate: {AuditRouteReviews: {}}, AuditEventReviewView: {AuditRouteReviews: {}},
		AuditEventSignalResponseCreate: {AuditRouteSignalResponses: {}}, AuditEventSignalResponseView: {AuditRouteSignalResponses: {}},
		AuditEventRiskCreate: {AuditRouteRisks: {}}, AuditEventRiskView: {AuditRouteRisks: {}}, AuditEventRiskUpdate: {AuditRouteRisks: {}},
		AuditEventRiskGoalLink: {AuditRouteRisks: {}}, AuditEventRiskGoalUnlink: {AuditRouteRisks: {}},
		AuditEventRiskTargetLink: {AuditRouteRisks: {}}, AuditEventRiskTargetUnlink: {AuditRouteRisks: {}},
		AuditEventRiskReviewCreate:        {AuditRouteRiskReviews: {}},
		AuditEventCarePlanCreate:          {AuditRouteCarePlans: {}},
		AuditEventCarePlanSectionUpdate:   {AuditRouteCarePlanSections: {}},
		AuditEventCarePlanActivate:        {AuditRouteCarePlanActivate: {}},
		AuditEventCarePlanArchive:         {AuditRouteCarePlanArchive: {}, AuditRouteCarePlanActivate: {}, AuditRouteCarePlanReviews: {}},
		AuditEventCarePlanReviewCreate:    {AuditRouteCarePlanReviews: {}},
		AuditEventCarePlanSuccessorCreate: {AuditRouteCarePlanReviews: {}},
		AuditEventDailyProgressCreate:     {AuditRouteDailyProgress: {}},
	}
	_, ok := allowed[eventType][route]
	return ok
}

func bindAuditMetadata(fields security.AuditMetadataFields, entityType string, entityID int64) (security.AuditMetadata, error) {
	if fields.ResourceID != nil && *fields.ResourceID != entityID {
		return security.AuditMetadata{}, errors.New("audit metadata resource_id does not match entity")
	}
	if fields.ParentID != nil {
		return security.AuditMetadata{}, errors.New("audit metadata parent_id is not valid for this resource event")
	}
	if fields.RiskReviewID != nil && entityType != "risk" {
		return security.AuditMetadata{}, errors.New("audit metadata risk review ID is only valid for risk events")
	}
	if fields.CarePlanID != nil && entityType != "care_plan" {
		return security.AuditMetadata{}, errors.New("audit metadata care plan ID is only valid for care plan events")
	}
	if fields.CarePlanSectionID != nil && entityType != "care_plan_section" {
		return security.AuditMetadata{}, errors.New("audit metadata care plan section ID is only valid for care plan section events")
	}
	if fields.CarePlanReviewID != nil && entityType != "care_plan" {
		return security.AuditMetadata{}, errors.New("audit metadata care plan review ID is only valid for care plan events")
	}
	resourceFields := map[string]**int64{
		"goal": &fields.GoalID, "target": &fields.TargetID, "keyworking_session": &fields.SessionID,
		"risk":                &fields.RiskID,
		"onboarding_baseline": &fields.BaselineID, "psychological_review": &fields.ReviewID,
		"signal_response": &fields.SignalResponseID,
		"care_plan":       &fields.CarePlanID, "care_plan_section": &fields.CarePlanSectionID,
	}
	for resourceType, pointer := range resourceFields {
		if resourceType == entityType {
			if *pointer != nil && **pointer != entityID {
				return security.AuditMetadata{}, errors.New("audit metadata entity ID does not match authoritative entity")
			}
			continue
		}
		if (entityType == "target" && resourceType == "goal") ||
			(entityType == "daily_progress" && resourceType == "target") ||
			(entityType == "risk" && (resourceType == "goal" || resourceType == "target")) {
			continue
		}
		if *pointer != nil {
			return security.AuditMetadata{}, errors.New("audit metadata contains an unrelated resource ID")
		}
	}
	fields.ResourceID = &entityID
	if pointer := resourceFields[entityType]; pointer != nil {
		*pointer = &entityID
	}
	return security.NewAuditMetadata(fields)
}

func createAuditEvent(ctx context.Context, qtx DBTX, event *models.AuditEvent, metadata security.AuditMetadata) error {
	if event == nil || strings.TrimSpace(event.EventType) == "" || strings.TrimSpace(event.EntityType) == "" {
		return errors.New("audit event type and entity type are required")
	}
	event.Metadata = metadata.JSON()

	const q = `
INSERT INTO audit_events (
  home_id, child_id, staff_user_id,
  actor_email, actor_name,
  event_type, entity_type, entity_id,
  request_method, request_path,
  metadata
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING id, created_at`

	if err := qtx.QueryRowContext(ctx, q, auditInsertArgs(event)...).Scan(&event.ID, &event.CreatedAt); err != nil {
		return fmt.Errorf("create audit event: %w", err)
	}
	return nil
}

func nullableNonEmptyString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func auditInsertArgs(event *models.AuditEvent) []any {
	return []any{
		nullableInt64Ptr(event.HomeID),
		nullableInt64Ptr(event.ChildID),
		nullableInt64Ptr(event.StaffUserID),
		nullableStringPtr(event.ActorEmail),
		nullableStringPtr(event.ActorName),
		event.EventType,
		event.EntityType,
		nullableInt64Ptr(event.EntityID),
		nullableStringPtr(event.RequestMethod),
		nullableStringPtr(event.RequestPath),
		event.Metadata,
	}
}

func (r *Repo) ListAuditEventsByChild(ctx context.Context, childID int64, limit int) ([]models.AuditEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	const q = `
SELECT id, home_id, child_id, staff_user_id,
       actor_email, actor_name,
       event_type, entity_type, entity_id,
       request_method, request_path, metadata, created_at
FROM audit_events
WHERE child_id = $1
ORDER BY created_at DESC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, q, childID, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit events by child: %w", err)
	}
	defer rows.Close()

	events := make([]models.AuditEvent, 0)
	for rows.Next() {
		var event models.AuditEvent
		if err := rows.Scan(
			&event.ID,
			&event.HomeID,
			&event.ChildID,
			&event.StaffUserID,
			&event.ActorEmail,
			&event.ActorName,
			&event.EventType,
			&event.EntityType,
			&event.EntityID,
			&event.RequestMethod,
			&event.RequestPath,
			&event.Metadata,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit events: %w", err)
	}
	return events, nil
}
