// internal/storage/repo.go
package storage

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"qr-feedback-api/internal/models"
)

var (
	ErrInvalidIntakeContext = errors.New("invalid or expired intake context")
	ErrMonthlyContextExists = errors.New("active monthly voice context already exists")
	ErrMonthlyVoiceConflict = errors.New("monthly voice context has already been used")
)

type Repo struct {
	db        *sql.DB
	secretKey string
}

func NewRepo(db *sql.DB, secret string) *Repo {
	return &Repo{
		db:        db,
		secretKey: secret,
	}
}

// HashIP returns an HMAC hash of the IP, or empty string if no secret key.
func (r *Repo) HashIP(ip string) string {
	if ip == "" || r.secretKey == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(r.secretKey))
	h.Write([]byte(ip))
	return hex.EncodeToString(h.Sum(nil))
}

func (r *Repo) hashContextToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" || r.secretKey == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(r.secretKey))
	h.Write([]byte("intake-context:" + token))
	return hex.EncodeToString(h.Sum(nil))
}

func (r *Repo) ResolveIntakeContext(ctx context.Context, token, purpose string, now time.Time) (*models.IntakeContext, error) {
	tokenHash := r.hashContextToken(token)
	if tokenHash == "" {
		return nil, ErrInvalidIntakeContext
	}
	const q = `
SELECT id::text, home_external_id, child_external_id, home_label, purpose,
       period_month, expires_at, used_at
FROM intake_contexts
WHERE token_hash = $1
  AND purpose = $2
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > $3)`
	var resolved models.IntakeContext
	var childID sql.NullInt64
	var periodMonth, expiresAt, usedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, q, tokenHash, purpose, now.UTC()).Scan(
		&resolved.ID, &resolved.HomeExternalID, &childID, &resolved.HomeLabel,
		&resolved.Purpose, &periodMonth, &expiresAt, &usedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidIntakeContext
		}
		return nil, fmt.Errorf("resolve intake context: %w", err)
	}
	if childID.Valid {
		value := childID.Int64
		resolved.ChildExternalID = &value
	}
	if expiresAt.Valid {
		value := expiresAt.Time
		resolved.ExpiresAt = &value
	}
	if periodMonth.Valid {
		value := periodMonth.Time
		resolved.PeriodMonth = &value
	}
	if usedAt.Valid {
		value := usedAt.Time
		resolved.UsedAt = &value
	}
	return &resolved, nil
}

func (r *Repo) CreateMonthlyVoiceContext(ctx context.Context, input models.MonthlyVoiceContextInput) (*models.MonthlyVoiceContext, error) {
	if input.HomeExternalID <= 0 || input.ChildExternalID <= 0 || input.PeriodMonth.IsZero() || input.ExpiresAt.IsZero() {
		return nil, ErrInvalidIntakeContext
	}
	period := time.Date(input.PeriodMonth.UTC().Year(), input.PeriodMonth.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	if !input.ExpiresAt.After(time.Now().UTC()) || input.ExpiresAt.After(time.Now().UTC().Add(90*24*time.Hour)) {
		return nil, ErrInvalidIntakeContext
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate monthly context token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	tokenHash := r.hashContextToken(token)
	const q = `
INSERT INTO intake_contexts (
  token_hash, home_external_id, child_external_id, home_label,
  purpose, period_month, expires_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7)
RETURNING id::text`
	var contextID string
	err := r.db.QueryRowContext(ctx, q, tokenHash, input.HomeExternalID, input.ChildExternalID,
		"Verified home", models.IntakePurposeMonthlyVoice, period, input.ExpiresAt.UTC()).Scan(&contextID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "uq_monthly_voice_context_child_period") {
			return nil, ErrMonthlyContextExists
		}
		return nil, fmt.Errorf("create monthly voice context: %w", err)
	}
	return &models.MonthlyVoiceContext{
		ContextID: contextID, Token: token, PeriodMonth: period.Format("2006-01"), ExpiresAt: input.ExpiresAt.UTC(),
	}, nil
}

// Shared helpers used across storage files.

func nullStr(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
