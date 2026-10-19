package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func RequireFeedbackIntakeToken(expected string) func(http.Handler) http.Handler {
	expected = strings.TrimSpace(expected)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := strings.TrimSpace(r.Header.Get("X-Feedback-Intake-Token"))
			if expected == "" || len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
