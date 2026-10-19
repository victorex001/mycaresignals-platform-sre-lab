package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"therapeutic-engine/internal/security"
)

const localManagerTokenLifetime = time.Hour

type LocalDevelopmentManagerStore interface {
	GetLocalDevelopmentManagerSubject(ctx context.Context) (string, error)
}

type LocalDevelopmentAccessHandler struct {
	Store LocalDevelopmentManagerStore
}

func NewLocalDevelopmentAccessHandler(store LocalDevelopmentManagerStore) *LocalDevelopmentAccessHandler {
	return &LocalDevelopmentAccessHandler{Store: store}
}

func (h *LocalDevelopmentAccessHandler) IssueManagerToken(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeServerError(w, errors.New("local development access store not initialised"))
		return
	}
	if !isLoopbackRequest(r) && !isContainerDevelopmentRequest(r) {
		http.NotFound(w, r)
		return
	}
	subject, err := h.Store.GetLocalDevelopmentManagerSubject(r.Context())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeServerError(w, err)
		return
	}
	token, err := security.IssueDevJWTForSubjectFromEnv(subject, localManagerTokenLifetime)
	if err != nil {
		if errors.Is(err, security.ErrDevAuthConfiguration) || errors.Is(err, security.ErrDevAuthForbidden) {
			http.NotFound(w, r)
			return
		}
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_in": int(localManagerTokenLifetime.Seconds()),
	})
}

func isLoopbackRequest(r *http.Request) bool {
	ip := requestIP(r)
	return ip != nil && ip.IsLoopback()
}

func isContainerDevelopmentRequest(r *http.Request) bool {
	if strings.TrimSpace(os.Getenv("APP_ENV")) != "development" ||
		!strings.EqualFold(strings.TrimSpace(os.Getenv("ALLOW_CONTAINER_DEV_AUTH")), "true") {
		return false
	}
	ip := requestIP(r)
	return ip != nil && ip.IsPrivate()
}

func requestIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	return net.ParseIP(host)
}
