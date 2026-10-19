package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
)

type testStaffLookup struct {
	staff *models.StaffUser
}

func (lookup testStaffLookup) GetStaffUserByExternalAuthSubject(context.Context, string) (*models.StaffUser, error) {
	return lookup.staff, nil
}

func TestGenerateTokenMatchesDevJWTContract(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	token, err := generateToken(tokenConfig{
		Issuer:   "https://issuer.example.test",
		Audience: "mycaresignals-dev",
		Subject:  "existing-staff-subject",
		Secret:   secret,
		Lifetime: time.Hour,
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d segments, want 3", len(parts))
	}

	var header map[string]string
	decodeSegment(t, parts[0], &header)
	if header["alg"] != "HS256" || header["typ"] != "JWT" {
		t.Fatalf("header = %#v", header)
	}

	var claims tokenClaims
	decodeSegment(t, parts[1], &claims)
	if claims.Issuer != "https://issuer.example.test" || claims.Audience != "mycaresignals-dev" || claims.Subject != "existing-staff-subject" {
		t.Fatalf("claims identity = %#v", claims)
	}
	if claims.IssuedAt != now.Unix() || claims.ExpiresAt != now.Add(time.Hour).Unix() {
		t.Fatalf("claims times = %#v", claims)
	}

	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	wantSignature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(wantSignature)) {
		t.Fatal("token signature is not HS256 with the configured secret")
	}
}

func TestGenerateTokenRejectsUnsafeConfiguration(t *testing.T) {
	valid := tokenConfig{
		Issuer:   "issuer",
		Audience: "audience",
		Subject:  "subject",
		Secret:   []byte("01234567890123456789012345678901"),
		Lifetime: time.Hour,
	}
	tests := []struct {
		name   string
		mutate func(*tokenConfig)
	}{
		{name: "missing issuer", mutate: func(c *tokenConfig) { c.Issuer = "" }},
		{name: "missing audience", mutate: func(c *tokenConfig) { c.Audience = "" }},
		{name: "missing subject", mutate: func(c *tokenConfig) { c.Subject = "" }},
		{name: "weak secret", mutate: func(c *tokenConfig) { c.Secret = []byte("too-short") }},
		{name: "non-positive ttl", mutate: func(c *tokenConfig) { c.Lifetime = 0 }},
		{name: "long ttl", mutate: func(c *tokenConfig) { c.Lifetime = 25 * time.Hour }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := valid
			test.mutate(&config)
			if _, err := generateToken(config, time.Now()); err == nil {
				t.Fatal("generateToken accepted unsafe configuration")
			}
		})
	}
}

func TestGeneratedTokenIsAcceptedByBackendDevAuthenticator(t *testing.T) {
	const (
		issuer   = "https://issuer.example.test"
		audience = "mycaresignals-dev"
		subject  = "existing-staff-subject"
		secret   = "01234567890123456789012345678901"
	)
	t.Setenv("APP_ENV", "development")
	t.Setenv("AUTH_MODE", "dev-jwt")
	t.Setenv("ENABLE_DEV_AUTH", "true")
	t.Setenv("DEV_JWT_ISSUER", issuer)
	t.Setenv("DEV_JWT_AUDIENCE", audience)
	t.Setenv("DEV_JWT_HMAC_SECRET", secret)

	token, err := generateToken(tokenConfig{
		Issuer: issuer, Audience: audience, Subject: subject,
		Secret: []byte(secret), Lifetime: time.Hour,
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	authenticator, err := security.NewDevJWTAuthenticatorFromEnv(testStaffLookup{staff: &models.StaffUser{
		ID: 7, Email: "staff@example.test", DisplayName: "Dev Staff",
		ExternalAuthSubject: stringPointer(subject), Status: "active",
	}})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := authenticator.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if principal.StaffUserID() != 7 || principal.ExternalSubject() != subject {
		t.Fatalf("principal = %#v", principal)
	}
}

func decodeSegment(t *testing.T, segment string, target any) {
	t.Helper()
	decoded, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(decoded, target); err != nil {
		t.Fatal(err)
	}
}

func stringPointer(value string) *string {
	return &value
}
