package security

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeAuthorizationStore struct {
	roles          map[int64]map[int64][]string
	childHomes     map[int64]int64
	goalChildren   map[int64]int64
	targetChildren map[int64]int64
	err            error
	targetCalls    int
}

func (f *fakeAuthorizationStore) StaffHasAnyHomeRole(_ context.Context, staffUserID, homeID int64, allowedRoles ...Role) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	for _, have := range f.roles[staffUserID][homeID] {
		for _, allowed := range allowedRoles {
			if have == string(allowed) {
				return true, nil
			}
		}
	}
	return false, nil
}
func (f *fakeAuthorizationStore) GetChildHomeID(_ context.Context, childID int64) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	homeID, ok := f.childHomes[childID]
	if !ok {
		return 0, ErrInvalidRelationship
	}
	return homeID, nil
}
func (f *fakeAuthorizationStore) GoalBelongsToChild(_ context.Context, goalID, childID int64) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.goalChildren[goalID] == childID, nil
}
func (f *fakeAuthorizationStore) TargetBelongsToChild(_ context.Context, targetID, childID int64) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	f.targetCalls++
	return f.targetChildren[targetID] == childID, nil
}

func TestPrincipalIsImmutableAndZeroValueIsInvalid(t *testing.T) {
	if _, ok := PrincipalFromContext(context.Background()); ok {
		t.Fatal("missing context authenticated")
	}
	if _, err := RequirePrincipal(WithPrincipal(context.Background(), Principal{})); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("zero Principal error = %v", err)
	}
	typ := reflect.TypeOf(Principal{})
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).IsExported() {
			t.Fatalf("Principal field %s is exported", typ.Field(i).Name)
		}
	}

	original, ok := newPrincipal(7, "verified-subject", "db@example.test", "DB Name", "test")
	if !ok {
		t.Fatal("newPrincipal() rejected valid identity")
	}
	ctx := WithPrincipal(context.Background(), original)
	original.staffUserID = 99
	stored, ok := PrincipalFromContext(ctx)
	if !ok || stored.StaffUserID() != 7 {
		t.Fatalf("stored principal mutated: %#v", stored)
	}
	stored.staffUserID = 88
	again, _ := PrincipalFromContext(ctx)
	if again.StaffUserID() != 7 {
		t.Fatalf("context principal mutated through returned copy: %#v", again)
	}
}

func TestAuthorizationFoundation(t *testing.T) {
	principal, _ := newPrincipal(7, "verified-subject", "", "", "test")
	authenticated := WithPrincipal(context.Background(), principal)
	store := &fakeAuthorizationStore{
		roles:      map[int64]map[int64][]string{7: {10: {"key_worker"}}},
		childHomes: map[int64]int64{100: 10, 200: 20}, goalChildren: map[int64]int64{300: 100, 301: 200},
		targetChildren: map[int64]int64{400: 100, 401: 100, 402: 200},
	}

	t.Run("unauthenticated", func(t *testing.T) {
		_, err := RequireHomeRole(context.Background(), store, 10, RoleKeyWorker)
		requireErrorIs(t, err, ErrUnauthenticated)
	})
	t.Run("correct role", func(t *testing.T) {
		got, err := RequireHomeRole(authenticated, store, 10, RoleKeyWorker)
		if err != nil || got.StaffUserID() != 7 {
			t.Fatalf("got (%#v, %v)", got, err)
		}
	})
	t.Run("wrong role", func(t *testing.T) {
		_, err := RequireHomeRole(authenticated, store, 10, RoleManager)
		requireErrorIs(t, err, ErrForbidden)
	})
	t.Run("repository failure", func(t *testing.T) {
		broken := &fakeAuthorizationStore{err: errors.New("db unavailable")}
		_, err := RequireHomeRole(authenticated, broken, 10, RoleKeyWorker)
		requireErrorIs(t, err, ErrAuthorizationCheck)
	})
	t.Run("empty and unknown roles fail closed", func(t *testing.T) {
		if _, err := RequireHomeRole(authenticated, store, 10); !errors.Is(err, ErrForbidden) {
			t.Fatalf("empty roles error = %v", err)
		}
		if _, err := RequireHomeRole(authenticated, store, 10, Role("owner")); !errors.Is(err, ErrForbidden) {
			t.Fatalf("unknown role error = %v", err)
		}
	})
	t.Run("child mismatch", func(t *testing.T) {
		requireErrorIs(t, VerifyChildHomeMatch(authenticated, store, 20, 100), ErrInvalidRelationship)
	})
	t.Run("goal mismatch", func(t *testing.T) {
		requireErrorIs(t, VerifyGoalBelongsToChild(authenticated, store, 301, 100), ErrInvalidRelationship)
	})
	t.Run("target mismatch", func(t *testing.T) {
		requireErrorIs(t, VerifyTargetBelongsToChild(authenticated, store, 402, 100), ErrInvalidRelationship)
	})
	t.Run("duplicate score targets deduplicated", func(t *testing.T) {
		store.targetCalls = 0
		if err := VerifySessionScoreTargetsBelongToChild(authenticated, store, []int64{400, 400, 401}, 100); err != nil {
			t.Fatal(err)
		}
		if store.targetCalls != 2 {
			t.Fatalf("calls = %d, want 2", store.targetCalls)
		}
	})
	t.Run("empty score targets explicit success", func(t *testing.T) {
		if err := VerifySessionScoreTargetsBelongToChild(authenticated, store, nil, 100); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("optional nil target", func(t *testing.T) {
		if err := VerifySessionTargetBelongsToChild(authenticated, store, nil, 100); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("invalid child rejected with nil target or empty scores", func(t *testing.T) {
		for _, childID := range []int64{0, -1} {
			requireErrorIs(t, VerifySessionTargetBelongsToChild(authenticated, store, nil, childID), ErrInvalidRelationship)
			requireErrorIs(t, VerifySessionScoreTargetsBelongToChild(authenticated, store, nil, childID), ErrInvalidRelationship)
		}
	})
	t.Run("zero and negative IDs", func(t *testing.T) {
		for _, id := range []int64{0, -1} {
			requireErrorIs(t, VerifyTargetBelongsToChild(authenticated, store, id, 100), ErrInvalidRelationship)
			requireErrorIs(t, VerifyGoalBelongsToChild(authenticated, store, id, 100), ErrInvalidRelationship)
		}
	})
}

func requireErrorIs(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("error = %v, want %v", got, want)
	}
}
