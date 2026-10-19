package security

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrForbidden           = errors.New("principal is not authorized")
	ErrInvalidRelationship = errors.New("invalid cross-resource relationship")
	ErrAuthorizationCheck  = errors.New("authorization check failed")
)

type AuthorizationStore interface {
	StaffHasAnyHomeRole(ctx context.Context, staffUserID, homeID int64, allowedRoles ...Role) (bool, error)
	GetChildHomeID(ctx context.Context, childID int64) (int64, error)
	GoalBelongsToChild(ctx context.Context, goalID, childID int64) (bool, error)
	TargetBelongsToChild(ctx context.Context, targetID, childID int64) (bool, error)
}

type MutationAuthorizationStore interface {
	StaffHasAnyHomeRoleForMutation(ctx context.Context, staffUserID, homeID int64, allowedRoles ...Role) (bool, error)
}

type ChildMutationAuthorizationStore interface {
	LockStaffRolesAndChildForMutation(ctx context.Context, staffUserID, childID int64, allowedRoles ...Role) (int64, bool, error)
}

// RequirePrincipal returns only identities previously placed in context by a
// trusted authenticator integration.
func RequirePrincipal(ctx context.Context) (Principal, error) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return Principal{}, ErrUnauthenticated
	}
	return principal, nil
}

// RequireHomeRole checks current database roles rather than carrying stale
// roles inside the Principal.
func RequireHomeRole(
	ctx context.Context,
	store AuthorizationStore,
	homeID int64,
	allowedRoles ...Role,
) (Principal, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return Principal{}, err
	}
	if store == nil || homeID <= 0 || len(allowedRoles) == 0 {
		return Principal{}, ErrForbidden
	}
	for _, role := range allowedRoles {
		if !role.Valid() {
			return Principal{}, ErrForbidden
		}
	}

	allowed, err := store.StaffHasAnyHomeRole(ctx, principal.StaffUserID(), homeID, allowedRoles...)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: %v", ErrAuthorizationCheck, err)
	}
	if !allowed {
		return Principal{}, ErrForbidden
	}
	return principal, nil
}

// RequireHomeRoleForMutation is the concurrency-safe authorization entry point
// for protected writes. The store must lock staff and role rows in the same
// transaction later used for mutation and audit.
func RequireHomeRoleForMutation(ctx context.Context, store MutationAuthorizationStore, homeID int64, allowedRoles ...Role) (Principal, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return Principal{}, err
	}
	if store == nil || homeID <= 0 || len(allowedRoles) == 0 {
		return Principal{}, ErrForbidden
	}
	for _, role := range allowedRoles {
		if !role.Valid() {
			return Principal{}, ErrForbidden
		}
	}
	allowed, err := store.StaffHasAnyHomeRoleForMutation(ctx, principal.StaffUserID(), homeID, allowedRoles...)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: %v", ErrAuthorizationCheck, err)
	}
	if !allowed {
		return Principal{}, ErrForbidden
	}
	return principal, nil
}

// RequireChildHomeRoleForMutation authorizes a child-scoped write while the
// transaction store locks staff, matching roles, and the child in canonical
// order. Missing children and wrong-home role assignments both fail closed as
// forbidden so callers do not disclose cross-home child existence.
func RequireChildHomeRoleForMutation(ctx context.Context, store ChildMutationAuthorizationStore, childID int64, allowedRoles ...Role) (int64, Principal, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return 0, Principal{}, err
	}
	if store == nil || childID <= 0 || len(allowedRoles) == 0 {
		return 0, Principal{}, ErrForbidden
	}
	for _, role := range allowedRoles {
		if !role.Valid() {
			return 0, Principal{}, ErrForbidden
		}
	}
	homeID, allowed, err := store.LockStaffRolesAndChildForMutation(ctx, principal.StaffUserID(), childID, allowedRoles...)
	if err != nil {
		return 0, Principal{}, fmt.Errorf("%w: %v", ErrAuthorizationCheck, err)
	}
	if !allowed || homeID <= 0 {
		return 0, Principal{}, ErrForbidden
	}
	return homeID, principal, nil
}

// RequireChildHomeRole resolves the child's actual home before checking roles.
// Callers can map both forbidden and invalid relationships to a non-disclosing
// public response while retaining the internal distinction.
func RequireChildHomeRole(
	ctx context.Context,
	store AuthorizationStore,
	childID int64,
	allowedRoles ...Role,
) (int64, Principal, error) {
	if _, err := RequirePrincipal(ctx); err != nil {
		return 0, Principal{}, err
	}
	if store == nil || childID <= 0 {
		return 0, Principal{}, ErrInvalidRelationship
	}

	homeID, err := store.GetChildHomeID(ctx, childID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, Principal{}, ErrInvalidRelationship
		}
		return 0, Principal{}, fmt.Errorf("%w: %v", ErrAuthorizationCheck, err)
	}
	principal, err := RequireHomeRole(ctx, store, homeID, allowedRoles...)
	if err != nil {
		return 0, Principal{}, err
	}
	return homeID, principal, nil
}

// VerifyChildHomeMatch prevents a URL home ID from being paired with a child
// that belongs to another home.
func VerifyChildHomeMatch(ctx context.Context, store AuthorizationStore, urlHomeID, childID int64) error {
	if store == nil || urlHomeID <= 0 || childID <= 0 {
		return ErrInvalidRelationship
	}
	actualHomeID, err := store.GetChildHomeID(ctx, childID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidRelationship
		}
		return fmt.Errorf("%w: %v", ErrAuthorizationCheck, err)
	}
	if actualHomeID != urlHomeID {
		return ErrInvalidRelationship
	}
	return nil
}

func VerifyGoalBelongsToChild(ctx context.Context, store AuthorizationStore, goalID, childID int64) error {
	if store == nil || goalID <= 0 || childID <= 0 {
		return ErrInvalidRelationship
	}
	ok, err := store.GoalBelongsToChild(ctx, goalID, childID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAuthorizationCheck, err)
	}
	if !ok {
		return ErrInvalidRelationship
	}
	return nil
}

func VerifyTargetBelongsToChild(ctx context.Context, store AuthorizationStore, targetID, childID int64) error {
	if store == nil || targetID <= 0 || childID <= 0 {
		return ErrInvalidRelationship
	}
	ok, err := store.TargetBelongsToChild(ctx, targetID, childID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAuthorizationCheck, err)
	}
	if !ok {
		return ErrInvalidRelationship
	}
	return nil
}

// VerifySessionTargetBelongsToChild accepts a nil optional session target.
func VerifySessionTargetBelongsToChild(ctx context.Context, store AuthorizationStore, targetID *int64, childID int64) error {
	if childID <= 0 {
		return ErrInvalidRelationship
	}
	if targetID == nil {
		return nil
	}
	return VerifyTargetBelongsToChild(ctx, store, *targetID, childID)
}

// VerifySessionScoreTargetsBelongToChild checks every distinct score target.
func VerifySessionScoreTargetsBelongToChild(ctx context.Context, store AuthorizationStore, targetIDs []int64, childID int64) error {
	if childID <= 0 {
		return ErrInvalidRelationship
	}
	seen := make(map[int64]struct{}, len(targetIDs))
	for _, targetID := range targetIDs {
		if _, ok := seen[targetID]; ok {
			continue
		}
		seen[targetID] = struct{}{}
		if err := VerifyTargetBelongsToChild(ctx, store, targetID, childID); err != nil {
			return err
		}
	}
	return nil
}
