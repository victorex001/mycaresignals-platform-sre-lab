package middleware

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"therapeutic-engine/internal/security"
)

// AuthorizeHomeRead permits a read only when the authenticated principal has
// one of the explicitly allowed active roles for the URL's home.
func AuthorizeHomeRead(store security.AuthorizationStore, allowedRoles ...security.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			homeID, err := strconv.ParseInt(chi.URLParam(r, "homeID"), 10, 64)
			if err != nil || homeID <= 0 {
				writeReadAuthorizationError(w, security.ErrForbidden)
				return
			}

			if _, err := security.RequireHomeRole(r.Context(), store, homeID, allowedRoles...); err != nil {
				writeReadAuthorizationError(w, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AuthorizeChildRead resolves the child's authoritative home through the
// authorization store before checking the principal's active home role.
func AuthorizeChildRead(store security.AuthorizationStore, allowedRoles ...security.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			childID, err := strconv.ParseInt(chi.URLParam(r, "childID"), 10, 64)
			if err != nil || childID <= 0 {
				writeReadAuthorizationError(w, security.ErrInvalidRelationship)
				return
			}

			if _, _, err := security.RequireChildHomeRole(r.Context(), store, childID, allowedRoles...); err != nil {
				writeReadAuthorizationError(w, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeReadAuthorizationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, security.ErrUnauthenticated):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	case errors.Is(err, security.ErrForbidden), errors.Is(err, security.ErrInvalidRelationship):
		http.Error(w, "forbidden", http.StatusForbidden)
	default:
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
