package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	chimw "github.com/go-chi/chi/v5/middleware"

	"therapeutic-engine/internal/security"
)

// Authenticate protects only routes that explicitly install it. It replaces
// any caller-supplied request ID in the protected request context with a
// server-generated value before authenticating.
func Authenticate(authenticator security.Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authenticator == nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			requestID := security.NewServerRequestID()
			ctx := context.WithValue(r.Context(), chimw.RequestIDKey, requestID)
			parts := strings.Fields(r.Header.Get("Authorization"))
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeAuthenticationRequired(w)
				return
			}
			principal, err := authenticator.Authenticate(ctx, parts[1])
			if err != nil {
				if errors.Is(err, security.ErrUnauthenticated) || errors.Is(err, security.ErrInvalidToken) ||
					errors.Is(err, security.ErrUnknownStaff) || errors.Is(err, security.ErrInactiveStaff) {
					writeAuthenticationRequired(w)
					return
				}
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			if principal == nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			ctx = security.WithPrincipal(ctx, *principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthenticationRequired(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
