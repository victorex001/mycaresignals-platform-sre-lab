package security

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"therapeutic-engine/internal/models"
)

type fakeStaffLookup struct {
	staffBySubject map[string]*models.StaffUser
	err            error
}

func (f fakeStaffLookup) GetStaffUserByExternalAuthSubject(_ context.Context, subject string) (*models.StaffUser, error) {
	if f.err != nil {
		return nil, f.err
	}
	staff, ok := f.staffBySubject[subject]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return staff, nil
}

func TestDevJWTAuthenticator(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	secret := randomTestSecret(t)
	subject := "dev-subject"
	activeStaff := &models.StaffUser{
		ID:                  41,
		Email:               "dev@example.test",
		DisplayName:         "Development User",
		ExternalAuthSubject: &subject,
		Status:              "active",
	}
	lookup := fakeStaffLookup{staffBySubject: map[string]*models.StaffUser{subject: activeStaff}}
	authenticator, err := newDevJWTAuthenticator(devJWTConfig{
		issuer:   "https://dev-issuer.example.test",
		audience: "mycaresignals-dev",
		secret:   secret,
		now:      func() time.Time { return now },
	}, lookup)
	if err != nil {
		t.Fatalf("newDevJWTAuthenticator() error = %v", err)
	}

	validClaims := map[string]any{
		"iss": "https://dev-issuer.example.test",
		"aud": "mycaresignals-dev",
		"exp": now.Add(time.Hour).Unix(),
		"sub": subject,
	}

	t.Run("valid development token accepted", func(t *testing.T) {
		principal, err := authenticator.Authenticate(context.Background(), signTestJWT(t, secret, validClaims))
		if err != nil {
			t.Fatalf("Authenticate() error = %v", err)
		}
		if principal.StaffUserID() != activeStaff.ID || principal.ExternalSubject() != subject || principal.Source() != devJWTMode {
			t.Fatalf("Authenticate() principal = %#v", principal)
		}
	})

	tests := []struct {
		name       string
		claims     map[string]any
		signingKey []byte
		wantError  error
	}{
		{
			name:       "invalid signature rejected",
			claims:     cloneClaims(validClaims),
			signingKey: randomTestSecret(t),
			wantError:  ErrInvalidToken,
		},
		{
			name:      "expired token rejected",
			claims:    withClaim(validClaims, "exp", now.Add(-time.Second).Unix()),
			wantError: ErrInvalidToken,
		},
		{
			name:      "wrong issuer rejected",
			claims:    withClaim(validClaims, "iss", "https://wrong-issuer.example.test"),
			wantError: ErrInvalidToken,
		},
		{
			name:      "wrong audience rejected",
			claims:    withClaim(validClaims, "aud", "wrong-audience"),
			wantError: ErrInvalidToken,
		},
		{
			name:      "missing subject rejected",
			claims:    withClaim(validClaims, "sub", ""),
			wantError: ErrInvalidToken,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := test.signingKey
			if key == nil {
				key = secret
			}
			_, err := authenticator.Authenticate(context.Background(), signTestJWT(t, key, test.claims))
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Authenticate() error = %v, want %v", err, test.wantError)
			}
		})
	}

	t.Run("missing token rejected", func(t *testing.T) {
		_, err := authenticator.Authenticate(context.Background(), "")
		if !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("Authenticate() error = %v, want %v", err, ErrUnauthenticated)
		}
	})

	t.Run("unmapped staff rejected", func(t *testing.T) {
		unmappedAuthenticator, err := newDevJWTAuthenticator(authenticator.config, fakeStaffLookup{})
		if err != nil {
			t.Fatal(err)
		}
		_, err = unmappedAuthenticator.Authenticate(context.Background(), signTestJWT(t, secret, validClaims))
		if !errors.Is(err, ErrUnknownStaff) {
			t.Fatalf("Authenticate() error = %v, want %v", err, ErrUnknownStaff)
		}
	})

	t.Run("inactive staff rejected", func(t *testing.T) {
		inactive := *activeStaff
		inactive.Status = "inactive"
		inactiveAuthenticator, err := newDevJWTAuthenticator(
			authenticator.config,
			fakeStaffLookup{staffBySubject: map[string]*models.StaffUser{subject: &inactive}},
		)
		if err != nil {
			t.Fatal(err)
		}
		_, err = inactiveAuthenticator.Authenticate(context.Background(), signTestJWT(t, secret, validClaims))
		if !errors.Is(err, ErrInactiveStaff) {
			t.Fatalf("Authenticate() error = %v, want %v", err, ErrInactiveStaff)
		}
	})
}

func TestDevJWTStrictParsing(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	secret := randomTestSecret(t)
	subject := "strict-subject"
	databaseStaff := &models.StaffUser{ID: 51, Email: "database@example.test", DisplayName: "Database Name", ExternalAuthSubject: &subject, Status: "active"}
	authenticator, err := newDevJWTAuthenticator(devJWTConfig{
		issuer: "strict-issuer", audience: "strict-audience", secret: secret, now: func() time.Time { return now },
	}, fakeStaffLookup{staffBySubject: map[string]*models.StaffUser{subject: databaseStaff}})
	if err != nil {
		t.Fatal(err)
	}

	validPayload := fmt.Sprintf(`{"iss":"strict-issuer","aud":"strict-audience","sub":"%s","exp":%d}`, subject, now.Add(time.Hour).Unix())
	t.Run("audience string", func(t *testing.T) {
		requireJWTAccepted(t, authenticator, signRawTestJWT(t, secret, `{"alg":"HS256"}`, validPayload))
	})
	t.Run("audience array", func(t *testing.T) {
		payload := fmt.Sprintf(`{"iss":"strict-issuer","aud":["other","strict-audience"],"sub":"%s","exp":%d}`, subject, now.Add(time.Hour).Unix())
		requireJWTAccepted(t, authenticator, signRawTestJWT(t, secret, `{"alg":"HS256"}`, payload))
	})
	t.Run("database identity authoritative", func(t *testing.T) {
		payload := fmt.Sprintf(`{"iss":"strict-issuer","aud":"strict-audience","sub":"%s","exp":%d,"email":"token@example.test","name":"Token Name"}`, subject, now.Add(time.Hour).Unix())
		principal, err := authenticator.Authenticate(context.Background(), signRawTestJWT(t, secret, `{"alg":"HS256"}`, payload))
		if err != nil {
			t.Fatal(err)
		}
		if principal.Email() != databaseStaff.Email || principal.DisplayName() != databaseStaff.DisplayName {
			t.Fatalf("token identity overrode database: %#v", principal)
		}
	})

	invalid := map[string]string{
		"alg none":                       signRawTestJWT(t, secret, `{"alg":"none"}`, validPayload),
		"wrong algorithm":                signRawTestJWT(t, secret, `{"alg":"HS512"}`, validPayload),
		"wrong audience":                 signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.Replace(validPayload, "strict-audience", "wrong", 1)),
		"malformed audience object":      signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.Replace(validPayload, `"strict-audience"`, `{"bad":true}`, 1)),
		"malformed audience mixed array": signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.Replace(validPayload, `"strict-audience"`, `["strict-audience",7]`, 1)),
		"missing exp":                    signRawTestJWT(t, secret, `{"alg":"HS256"}`, fmt.Sprintf(`{"iss":"strict-issuer","aud":"strict-audience","sub":"%s"}`, subject)),
		"quoted exp":                     signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.Replace(validPayload, fmt.Sprintf(`%d`, now.Add(time.Hour).Unix()), fmt.Sprintf(`"%d"`, now.Add(time.Hour).Unix()), 1)),
		"fractional exp":                 signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.Replace(validPayload, fmt.Sprintf(`%d`, now.Add(time.Hour).Unix()), `1800003600.5`, 1)),
		"exponent exp":                   signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.Replace(validPayload, fmt.Sprintf(`%d`, now.Add(time.Hour).Unix()), `1.8000036e9`, 1)),
		"overflow exp":                   signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.Replace(validPayload, fmt.Sprintf(`%d`, now.Add(time.Hour).Unix()), `999999999999999999999999`, 1)),
		"trailing JSON":                  signRawTestJWT(t, secret, `{"alg":"HS256"}`, validPayload+` {}`),
		"duplicate issuer":               signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.TrimSuffix(validPayload, "}")+`,"iss":"strict-issuer"}`),
		"duplicate audience":             signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.TrimSuffix(validPayload, "}")+`,"aud":"strict-audience"}`),
		"duplicate subject":              signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.TrimSuffix(validPayload, "}")+fmt.Sprintf(`,"sub":"%s"}`, subject)),
		"duplicate exp":                  signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.TrimSuffix(validPayload, "}")+fmt.Sprintf(`,"exp":%d}`, now.Add(time.Hour).Unix())),
		"future nbf":                     signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.TrimSuffix(validPayload, "}")+fmt.Sprintf(`,"nbf":%d}`, now.Add(time.Minute).Unix())),
		"quoted nbf":                     signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.TrimSuffix(validPayload, "}")+fmt.Sprintf(`,"nbf":"%d"}`, now.Unix())),
		"duplicate nbf":                  signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.TrimSuffix(validPayload, "}")+fmt.Sprintf(`,"nbf":%d,"nbf":%d}`, now.Unix(), now.Unix())),
		"future iat":                     signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.TrimSuffix(validPayload, "}")+fmt.Sprintf(`,"iat":%d}`, now.Add(time.Minute).Unix())),
		"quoted iat":                     signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.TrimSuffix(validPayload, "}")+fmt.Sprintf(`,"iat":"%d"}`, now.Unix())),
		"duplicate iat":                  signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.TrimSuffix(validPayload, "}")+fmt.Sprintf(`,"iat":%d,"iat":%d}`, now.Unix(), now.Unix())),
		"subject wrong type":             signRawTestJWT(t, secret, `{"alg":"HS256"}`, strings.Replace(validPayload, fmt.Sprintf(`"%s"`, subject), `7`, 1)),
	}
	for name, token := range invalid {
		t.Run(name, func(t *testing.T) { requireJWTRejected(t, authenticator, token) })
	}

	t.Run("malformed segment counts", func(t *testing.T) {
		for _, token := range []string{"one", "one.two", "one.two.three.four"} {
			requireJWTRejected(t, authenticator, token)
		}
	})
	t.Run("malformed base64", func(t *testing.T) {
		for _, token := range []string{"!.e30.AA", "e30.!.AA", "e30.e30.!"} {
			requireJWTRejected(t, authenticator, token)
		}
	})
	t.Run("tampered header", func(t *testing.T) {
		token := signRawTestJWT(t, secret, `{"alg":"HS256","kid":"original"}`, validPayload)
		parts := strings.Split(token, ".")
		parts[0] = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","kid":"tampered"}`))
		requireJWTRejected(t, authenticator, strings.Join(parts, "."))
	})
	t.Run("tampered payload", func(t *testing.T) {
		token := signRawTestJWT(t, secret, `{"alg":"HS256"}`, validPayload)
		parts := strings.Split(token, ".")
		parts[1] = base64.RawURLEncoding.EncodeToString([]byte(strings.Replace(validPayload, subject, "attacker", 1)))
		requireJWTRejected(t, authenticator, strings.Join(parts, "."))
	})
}

func TestDevJWTRejectsEmptyAndWeakSecrets(t *testing.T) {
	for name, secret := range map[string][]byte{"empty": nil, "weak": []byte("too-short")} {
		t.Run(name, func(t *testing.T) {
			_, err := newDevJWTAuthenticator(devJWTConfig{issuer: "issuer", audience: "audience", secret: secret}, fakeStaffLookup{})
			if !errors.Is(err, ErrDevAuthConfiguration) {
				t.Fatalf("error = %v, want %v", err, ErrDevAuthConfiguration)
			}
		})
	}
}

func TestDevJWTClaimNameEncodingAndSizeLimits(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	secret := randomTestSecret(t)
	subject := "unicode-subject"
	staff := &models.StaffUser{ID: 61, Email: "db@example.test", DisplayName: "DB", ExternalAuthSubject: &subject, Status: "active"}
	authenticator, err := newDevJWTAuthenticator(devJWTConfig{issuer: "issuer", audience: "audience", secret: secret, now: func() time.Time { return now }}, fakeStaffLookup{staffBySubject: map[string]*models.StaffUser{subject: staff}})
	if err != nil {
		t.Fatal(err)
	}
	header := `{"alg":"HS256"}`
	base := fmt.Sprintf(`{"iss":"issuer","aud":"audience","sub":"%s","exp":%d`, subject, now.Add(time.Hour).Unix())

	t.Run("escaped duplicate registered claim rejected", func(t *testing.T) {
		requireJWTRejected(t, authenticator, signRawTestJWT(t, secret, header, base+`,"\u0069ss":"issuer"}`))
	})
	t.Run("unicode confusable does not replace ASCII claim", func(t *testing.T) {
		requireJWTAccepted(t, authenticator, signRawTestJWT(t, secret, header, base+`,"іss":"attacker"}`))
		requireJWTRejected(t, authenticator, signRawTestJWT(t, secret, header, fmt.Sprintf(`{"іss":"issuer","aud":"audience","sub":"%s","exp":%d}`, subject, now.Add(time.Hour).Unix())))
	})
	t.Run("credential length boundaries", func(t *testing.T) {
		if !devJWTCredentialWithinLimit(strings.Repeat("a", MaxDevJWTCredentialLen)) {
			t.Fatal("exact limit rejected by size gate")
		}
		if devJWTCredentialWithinLimit(strings.Repeat("a", MaxDevJWTCredentialLen+1)) {
			t.Fatal("one byte above limit accepted by size gate")
		}
		if devJWTCredentialWithinLimit(strings.Repeat("a", MaxDevJWTCredentialLen*100)) {
			t.Fatal("very large credential accepted by size gate")
		}
		for _, token := range []string{strings.Repeat("a", MaxDevJWTCredentialLen+1), strings.Repeat("a", MaxDevJWTCredentialLen*100)} {
			requireJWTRejected(t, authenticator, token)
		}
	})
}

func TestValidateAuthenticationModeFromEnvRejectsProductionDevJWT(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("AUTH_MODE", devJWTMode)
	t.Setenv("ENABLE_DEV_AUTH", "true")
	t.Setenv("DEV_JWT_ISSUER", "https://dev-issuer.example.test")
	t.Setenv("DEV_JWT_AUDIENCE", "mycaresignals-dev")
	t.Setenv("DEV_JWT_HMAC_SECRET", base64.RawURLEncoding.EncodeToString(randomTestSecret(t)))

	if err := ValidateAuthenticationModeFromEnv(); !errors.Is(err, ErrDevAuthForbidden) {
		t.Fatalf("ValidateAuthenticationModeFromEnv() error = %v, want %v", err, ErrDevAuthForbidden)
	}
}

func TestIssueDevJWTForSubjectFromEnvUsesDevelopmentGates(t *testing.T) {
	subject := "local-manager-subject"
	secret := base64.RawURLEncoding.EncodeToString(randomTestSecret(t))
	t.Setenv("APP_ENV", "development")
	t.Setenv("AUTH_MODE", devJWTMode)
	t.Setenv("ENABLE_DEV_AUTH", "true")
	t.Setenv("DEV_JWT_ISSUER", "https://dev-issuer.example.test")
	t.Setenv("DEV_JWT_AUDIENCE", "mycaresignals-dev")
	t.Setenv("DEV_JWT_HMAC_SECRET", secret)

	token, err := IssueDevJWTForSubjectFromEnv(subject, time.Hour)
	if err != nil {
		t.Fatalf("IssueDevJWTForSubjectFromEnv() error = %v", err)
	}
	if parts := strings.Split(token, "."); len(parts) != 3 {
		t.Fatalf("token segments = %d, want 3", len(parts))
	}
	staff := &models.StaffUser{
		ID:                  41,
		Email:               "dev@example.test",
		DisplayName:         "Development Manager",
		ExternalAuthSubject: &subject,
		Status:              "active",
	}
	authenticator, err := NewDevJWTAuthenticatorFromEnv(fakeStaffLookup{
		staffBySubject: map[string]*models.StaffUser{subject: staff},
	})
	if err != nil {
		t.Fatalf("NewDevJWTAuthenticatorFromEnv() error = %v", err)
	}
	if _, err := authenticator.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("issued token was rejected: %v", err)
	}

	t.Setenv("APP_ENV", "production")
	if _, err := IssueDevJWTForSubjectFromEnv(subject, time.Hour); !errors.Is(err, ErrDevAuthForbidden) {
		t.Fatalf("production issue error = %v, want %v", err, ErrDevAuthForbidden)
	}
}

func TestDevJWTModeRequiresEverySafetyGateAndSetting(t *testing.T) {
	complete := map[string]string{
		"APP_ENV":             "development",
		"AUTH_MODE":           devJWTMode,
		"ENABLE_DEV_AUTH":     "true",
		"DEV_JWT_ISSUER":      "https://dev-issuer.example.test",
		"DEV_JWT_AUDIENCE":    "mycaresignals-dev",
		"DEV_JWT_HMAC_SECRET": base64.RawURLEncoding.EncodeToString(randomTestSecret(t)),
	}

	for _, missing := range []string{
		"APP_ENV",
		"AUTH_MODE",
		"ENABLE_DEV_AUTH",
		"DEV_JWT_ISSUER",
		"DEV_JWT_AUDIENCE",
		"DEV_JWT_HMAC_SECRET",
	} {
		t.Run(missing, func(t *testing.T) {
			values := make(map[string]string, len(complete))
			for key, value := range complete {
				values[key] = value
			}
			delete(values, missing)
			_, selected, err := loadDevJWTEnvironment(func(key string) string { return values[key] })
			if missing == "AUTH_MODE" && !selected {
				t.Fatal("loadDevJWTEnvironment() did not detect ENABLE_DEV_AUTH=true")
			}
			if !errors.Is(err, ErrDevAuthConfiguration) {
				t.Fatalf("loadDevJWTEnvironment() error = %v, want %v", err, ErrDevAuthConfiguration)
			}
		})
	}
}

func signTestJWT(t *testing.T, secret []byte, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	unsigned := encodedHeader + "." + encodedPayload
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func signRawTestJWT(t *testing.T, secret []byte, header, payload string) string {
	t.Helper()
	encodedHeader := base64.RawURLEncoding.EncodeToString([]byte(header))
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	unsigned := encodedHeader + "." + encodedPayload
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func requireJWTAccepted(t *testing.T, authenticator *DevJWTAuthenticator, token string) {
	t.Helper()
	if _, err := authenticator.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
}

func requireJWTRejected(t *testing.T, authenticator *DevJWTAuthenticator, token string) {
	t.Helper()
	if _, err := authenticator.Authenticate(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Authenticate() error = %v, want %v", err, ErrInvalidToken)
	}
}

func randomTestSecret(t *testing.T) []byte {
	t.Helper()
	secret := make([]byte, minimumDevJWTSecretLen)
	if _, err := rand.Read(secret); err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}
	return secret
}

func cloneClaims(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func withClaim(source map[string]any, key string, value any) map[string]any {
	clone := cloneClaims(source)
	clone[key] = value
	return clone
}
