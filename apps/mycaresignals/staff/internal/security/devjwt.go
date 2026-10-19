package security

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"therapeutic-engine/internal/models"
)

const (
	devJWTMode             = string(AuthModeDevJWT)
	developmentMode        = string(AuthModeDevelopment)
	minimumDevJWTSecretLen = 32
	MaxDevJWTCredentialLen = 12 * 1024
)

type StaffByExternalSubjectLookup interface {
	GetStaffUserByExternalAuthSubject(ctx context.Context, externalSubject string) (*models.StaffUser, error)
}

type devJWTConfig struct {
	issuer   string
	audience string
	secret   []byte
	now      func() time.Time
}

type DevJWTAuthenticator struct {
	lookup StaffByExternalSubjectLookup
	config devJWTConfig
}

type devJWTEnvironment struct {
	appEnv   string
	authMode string
	enabled  string
	issuer   string
	audience string
	secret   string
}

// ValidateAuthenticationModeFromEnv enforces the development authentication
// safety gates at process startup without enabling authentication middleware.
func ValidateAuthenticationModeFromEnv() error {
	return validateAuthenticationConfiguration(os.Getenv, false)
}

// ValidateProtectedAuthenticationModeFromEnv is dormant preparation for route
// enforcement. It does not enable middleware; it only requires a recognized,
// non-none, fully configured authentication mode.
func ValidateProtectedAuthenticationModeFromEnv() error {
	return validateAuthenticationConfiguration(os.Getenv, true)
}

func validateAuthenticationConfiguration(getenv func(string) string, protectedRouting bool) error {
	rawMode := strings.TrimSpace(getenv("AUTH_MODE"))
	mode := AuthMode(rawMode)
	if mode == "" {
		mode = AuthModeNone
	}
	switch mode {
	case AuthModeNone:
		if strings.EqualFold(strings.TrimSpace(getenv("ENABLE_DEV_AUTH")), "true") {
			_, _, err := loadDevJWTEnvironment(getenv)
			return err
		}
		if protectedRouting {
			return ErrProtectedAuthRequired
		}
		return nil
	case AuthModeDevelopment, AuthModeDevJWT:
		_, _, err := loadDevJWTEnvironment(getenv)
		return err
	case AuthModeCognito:
		return validateCognitoEnvironment(getenv)
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedAuthMode, rawMode)
	}
}

// NewAuthenticatorFromEnv selects exactly one authenticator for the configured
// mode. Production Cognito mode never falls back to development JWTs.
func NewAuthenticatorFromEnv(lookup StaffByExternalSubjectLookup) (Authenticator, error) {
	mode := AuthMode(strings.TrimSpace(os.Getenv("AUTH_MODE")))
	switch mode {
	case AuthModeDevelopment, AuthModeDevJWT:
		return NewDevJWTAuthenticatorFromEnv(lookup)
	case AuthModeCognito:
		return NewCognitoAuthenticatorFromEnv(lookup)
	default:
		if mode == "" {
			mode = AuthModeNone
		}
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAuthMode, mode)
	}
}

// NewDevJWTAuthenticatorFromEnv creates the development authenticator only
// when every development-only safety gate and cryptographic setting is valid.
func NewDevJWTAuthenticatorFromEnv(lookup StaffByExternalSubjectLookup) (*DevJWTAuthenticator, error) {
	env, selected, err := loadDevJWTEnvironment(os.Getenv)
	if !selected || err != nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: dev-jwt mode is not selected", ErrDevAuthConfiguration)
	}

	return newDevJWTAuthenticator(devJWTConfig{
		issuer:   env.issuer,
		audience: env.audience,
		secret:   []byte(env.secret),
		now:      time.Now,
	}, lookup)
}

func loadDevJWTEnvironment(getenv func(string) string) (devJWTEnvironment, bool, error) {
	env := devJWTEnvironment{
		appEnv:   strings.TrimSpace(getenv("APP_ENV")),
		authMode: strings.TrimSpace(getenv("AUTH_MODE")),
		enabled:  strings.TrimSpace(getenv("ENABLE_DEV_AUTH")),
		issuer:   strings.TrimSpace(getenv("DEV_JWT_ISSUER")),
		audience: strings.TrimSpace(getenv("DEV_JWT_AUDIENCE")),
		secret:   getenv("DEV_JWT_HMAC_SECRET"),
	}

	selected := env.authMode == developmentMode || env.authMode == devJWTMode || strings.EqualFold(env.enabled, "true")
	if !selected {
		return env, false, nil
	}

	if env.appEnv == "staging" || env.appEnv == "production" {
		return env, true, fmt.Errorf("%w: dev-jwt cannot run when APP_ENV=%s", ErrDevAuthForbidden, env.appEnv)
	}
	if env.appEnv != "development" || (env.authMode != developmentMode && env.authMode != devJWTMode) || env.enabled != "true" {
		return env, true, fmt.Errorf("%w: APP_ENV, AUTH_MODE, and ENABLE_DEV_AUTH gates are not all satisfied", ErrDevAuthConfiguration)
	}
	if env.issuer == "" || env.audience == "" || len(env.secret) < minimumDevJWTSecretLen {
		return env, true, fmt.Errorf("%w: issuer, audience, or signing secret is missing or invalid", ErrDevAuthConfiguration)
	}

	return env, true, nil
}

func newDevJWTAuthenticator(config devJWTConfig, lookup StaffByExternalSubjectLookup) (*DevJWTAuthenticator, error) {
	if lookup == nil || config.issuer == "" || config.audience == "" || len(config.secret) < minimumDevJWTSecretLen {
		return nil, ErrDevAuthConfiguration
	}
	if config.now == nil {
		config.now = time.Now
	}
	return &DevJWTAuthenticator{lookup: lookup, config: config}, nil
}

type devJWTClaims struct {
	Issuer    string
	Audience  json.RawMessage
	Expires   int64
	Subject   string
	NotBefore *int64
	IssuedAt  *int64
}

type devJWTTokenClaims struct {
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

var integerNumericDate = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

// IssueDevJWTForSubjectFromEnv signs a short-lived development JWT using the
// same fail-closed development-only configuration as the authenticator.
func IssueDevJWTForSubjectFromEnv(subject string, lifetime time.Duration) (string, error) {
	env, selected, err := loadDevJWTEnvironment(os.Getenv)
	if !selected || err != nil {
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("%w: dev-jwt mode is not selected", ErrDevAuthConfiguration)
	}
	return issueDevJWT(devJWTConfig{
		issuer:   env.issuer,
		audience: env.audience,
		secret:   []byte(env.secret),
		now:      time.Now,
	}, subject, lifetime)
}

func issueDevJWT(config devJWTConfig, subject string, lifetime time.Duration) (string, error) {
	subject = strings.TrimSpace(subject)
	if config.issuer == "" || config.audience == "" || len(config.secret) < minimumDevJWTSecretLen || subject == "" {
		return "", ErrDevAuthConfiguration
	}
	if config.now == nil {
		config.now = time.Now
	}
	if lifetime <= 0 || lifetime > 24*time.Hour {
		return "", ErrDevAuthConfiguration
	}
	now := config.now().UTC()
	header, err := encodeJWTSegment(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := encodeJWTSegment(devJWTTokenClaims{
		Issuer:    config.issuer,
		Audience:  config.audience,
		Subject:   subject,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(lifetime).Unix(),
	})
	if err != nil {
		return "", err
	}
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, config.secret)
	_, _ = mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func encodeJWTSegment(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode development token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func (a *DevJWTAuthenticator) Authenticate(ctx context.Context, credential string) (*Principal, error) {
	token := strings.TrimSpace(credential)
	if token == "" {
		return nil, ErrUnauthenticated
	}
	if !devJWTCredentialWithinLimit(token) {
		return nil, ErrInvalidToken
	}

	claims, err := a.verify(token)
	if err != nil {
		return nil, err
	}

	staff, err := a.lookup.GetStaffUserByExternalAuthSubject(ctx, claims.Subject)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUnknownStaff
		}
		return nil, fmt.Errorf("%w: %v", ErrAuthenticationUnavailable, err)
	}
	if staff == nil {
		return nil, ErrUnknownStaff
	}
	if staff.Status != "active" {
		return nil, ErrInactiveStaff
	}
	if staff.ExternalAuthSubject == nil || *staff.ExternalAuthSubject != claims.Subject {
		return nil, ErrUnknownStaff
	}

	principal, ok := newPrincipal(staff.ID, claims.Subject, staff.Email, staff.DisplayName, devJWTMode)
	if !ok {
		return nil, ErrUnknownStaff
	}
	return &principal, nil
}

func devJWTCredentialWithinLimit(token string) bool {
	return len(token) <= MaxDevJWTCredentialLen
}

func (a *DevJWTAuthenticator) verify(token string) (*devJWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	header, err := decodeJSONObject(headerBytes, map[string]struct{}{"alg": {}})
	if err != nil || decodeJSONString(header["alg"]) != "HS256" {
		return nil, ErrInvalidToken
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, a.config.secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	values, err := decodeJSONObject(payloadBytes, map[string]struct{}{
		"iss": {}, "aud": {}, "sub": {}, "exp": {}, "nbf": {}, "iat": {},
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims := devJWTClaims{
		Issuer:   decodeJSONString(values["iss"]),
		Audience: values["aud"],
		Subject:  decodeJSONString(values["sub"]),
	}
	claims.Expires, err = decodeNumericDate(values["exp"])
	if err != nil {
		return nil, ErrInvalidToken
	}
	if raw, present := values["nbf"]; present {
		value, numericErr := decodeNumericDate(raw)
		if numericErr != nil {
			return nil, ErrInvalidToken
		}
		claims.NotBefore = &value
	}
	if raw, present := values["iat"]; present {
		value, numericErr := decodeNumericDate(raw)
		if numericErr != nil {
			return nil, ErrInvalidToken
		}
		claims.IssuedAt = &value
	}

	if strings.TrimSpace(claims.Subject) == "" || claims.Issuer != a.config.issuer {
		return nil, ErrInvalidToken
	}
	if !audienceContains(claims.Audience, a.config.audience) {
		return nil, ErrInvalidToken
	}
	now := a.config.now().Unix()
	if claims.Expires <= now || (claims.NotBefore != nil && *claims.NotBefore > now) || (claims.IssuedAt != nil && *claims.IssuedAt > now) {
		return nil, ErrInvalidToken
	}

	return &claims, nil
}

func decodeJSONObject(data []byte, rejectDuplicate map[string]struct{}) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, ErrInvalidToken
	}
	values := make(map[string]json.RawMessage)
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, ErrInvalidToken
		}
		key, ok := token.(string)
		if !ok {
			return nil, ErrInvalidToken
		}
		if _, registered := rejectDuplicate[key]; registered {
			if _, duplicate := seen[key]; duplicate {
				return nil, ErrInvalidToken
			}
			seen[key] = struct{}{}
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, ErrInvalidToken
		}
		values[key] = append(json.RawMessage(nil), raw...)
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return nil, ErrInvalidToken
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, ErrInvalidToken
	}
	return values, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrInvalidToken
	}
	return nil
}

func decodeJSONString(raw json.RawMessage) string {
	var value string
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return value
}

func decodeNumericDate(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || !integerNumericDate.Match(raw) {
		return 0, ErrInvalidToken
	}
	value, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		return 0, ErrInvalidToken
	}
	return value, nil
}

func audienceContains(raw json.RawMessage, required string) bool {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return single == required
	}

	var multiple []string
	if err := json.Unmarshal(raw, &multiple); err != nil {
		return false
	}
	for _, audience := range multiple {
		if audience == required {
			return true
		}
	}
	return false
}
