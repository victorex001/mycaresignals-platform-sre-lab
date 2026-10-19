package security

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"therapeutic-engine/internal/models"
)

func TestCognitoAuthenticatorValidatesJWTAndMapsDatabaseStaff(t *testing.T) {
	key := mustTestRSAKey(t)
	now := time.Unix(1700000000, 0)
	subject := "cognito-subject"
	staff := &models.StaffUser{ID: 7, Email: "manager@example.invalid", DisplayName: "Manager", ExternalAuthSubject: &subject, Status: "active"}
	client := testJWKSClient(t, key.Public().(*rsa.PublicKey), "kid-1")

	authenticator, err := newCognitoAuthenticator(cognitoConfig{
		issuer:   "https://cognito-idp.eu-west-2.amazonaws.com/eu-west-2_pool",
		clientID: "client-id",
		jwksURL:  "https://example.invalid/jwks.json",
		now:      func() time.Time { return now },
		client:   client,
	}, fakeStaffLookup{staffBySubject: map[string]*models.StaffUser{subject: staff}})
	if err != nil {
		t.Fatal(err)
	}

	token := signCognitoTestJWT(t, key, "kid-1", map[string]any{
		"iss": "https://cognito-idp.eu-west-2.amazonaws.com/eu-west-2_pool", "sub": subject,
		"client_id": "client-id", "token_use": "access", "exp": now.Add(time.Hour).Unix(), "iat": now.Unix(),
	})
	principal, err := authenticator.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if principal.Source() != cognitoSource || principal.StaffUserID() != staff.ID || principal.ExternalSubject() != subject {
		t.Fatalf("principal not database-bound Cognito identity: %#v", principal)
	}
}

func TestCognitoAuthenticatorRejectsInvalidTokens(t *testing.T) {
	key := mustTestRSAKey(t)
	otherKey := mustTestRSAKey(t)
	now := time.Unix(1700000000, 0)
	subject := "cognito-subject"
	staff := &models.StaffUser{ID: 7, Email: "manager@example.invalid", DisplayName: "Manager", ExternalAuthSubject: &subject, Status: "active"}
	client := testJWKSClient(t, key.Public().(*rsa.PublicKey), "kid-1")
	authenticator, err := newCognitoAuthenticator(cognitoConfig{
		issuer: "issuer", clientID: "client-id", jwksURL: "https://example.invalid/jwks.json", now: func() time.Time { return now }, client: client,
	}, fakeStaffLookup{staffBySubject: map[string]*models.StaffUser{subject: staff}})
	if err != nil {
		t.Fatal(err)
	}
	valid := map[string]any{"iss": "issuer", "sub": subject, "client_id": "client-id", "token_use": "access", "exp": now.Add(time.Hour).Unix()}
	tests := map[string]string{
		"malformed":       "not-a-jwt",
		"expired":         signCognitoTestJWT(t, key, "kid-1", withCognitoClaim(valid, "exp", now.Add(-time.Minute).Unix())),
		"wrong issuer":    signCognitoTestJWT(t, key, "kid-1", withCognitoClaim(valid, "iss", "other")),
		"wrong client":    signCognitoTestJWT(t, key, "kid-1", withCognitoClaim(valid, "client_id", "other")),
		"wrong token use": signCognitoTestJWT(t, key, "kid-1", withCognitoClaim(valid, "token_use", "refresh")),
		"wrong signature": signCognitoTestJWT(t, otherKey, "kid-1", valid),
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := authenticator.Authenticate(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("Authenticate() error = %v, want %v", err, ErrInvalidToken)
			}
		})
	}
}

func TestCognitoModeDoesNotAcceptDevelopmentJWT(t *testing.T) {
	key := mustTestRSAKey(t)
	now := time.Unix(1700000000, 0)
	subject := "cognito-subject"
	staff := &models.StaffUser{ID: 7, Email: "manager@example.invalid", DisplayName: "Manager", ExternalAuthSubject: &subject, Status: "active"}
	client := testJWKSClient(t, key.Public().(*rsa.PublicKey), "kid-1")
	authenticator, err := newCognitoAuthenticator(cognitoConfig{
		issuer: "issuer", clientID: "client-id", jwksURL: "https://example.invalid/jwks.json", now: func() time.Time { return now }, client: client,
	}, fakeStaffLookup{staffBySubject: map[string]*models.StaffUser{subject: staff}})
	if err != nil {
		t.Fatal(err)
	}
	devToken, err := issueDevJWT(devJWTConfig{issuer: "issuer", audience: "client-id", secret: []byte("01234567890123456789012345678901"), now: func() time.Time { return now }}, subject, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authenticator.Authenticate(context.Background(), devToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("dev token in cognito mode error = %v, want %v", err, ErrInvalidToken)
	}
}

func mustTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func testJWKS(key *rsa.PublicKey, kid string) map[string]any {
	return map[string]any{"keys": []map[string]any{{
		"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256",
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}}
}

func signCognitoTestJWT(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	header := encodeTestSegment(t, map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	payload := encodeTestSegment(t, claims)
	signingInput := header + "." + payload
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func encodeTestSegment(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func withCognitoClaim(source map[string]any, key string, value any) map[string]any {
	clone := make(map[string]any, len(source))
	for k, v := range source {
		clone[k] = v
	}
	clone[key] = value
	return clone
}

func TestCognitoAuthenticatorRejectsUnmappedStaff(t *testing.T) {
	key := mustTestRSAKey(t)
	now := time.Unix(1700000000, 0)
	client := testJWKSClient(t, key.Public().(*rsa.PublicKey), "kid-1")
	authenticator, err := newCognitoAuthenticator(cognitoConfig{
		issuer: "issuer", clientID: "client-id", jwksURL: "https://example.invalid/jwks.json", now: func() time.Time { return now }, client: client,
	}, fakeStaffLookup{})
	if err != nil {
		t.Fatal(err)
	}
	token := signCognitoTestJWT(t, key, "kid-1", map[string]any{
		"iss": "issuer", "sub": "unmapped", "client_id": "client-id", "token_use": "access", "exp": now.Add(time.Hour).Unix(),
	})
	if _, err := authenticator.Authenticate(context.Background(), token); !errors.Is(err, ErrUnknownStaff) {
		t.Fatalf("unmapped staff error = %v, want %v", err, ErrUnknownStaff)
	}
}

func TestCognitoIDTokenAudienceAccepted(t *testing.T) {
	key := mustTestRSAKey(t)
	now := time.Unix(1700000000, 0)
	subject := "subject"
	staff := &models.StaffUser{ID: 9, Email: "staff@example.invalid", DisplayName: "Staff", ExternalAuthSubject: &subject, Status: "active"}
	client := testJWKSClient(t, key.Public().(*rsa.PublicKey), "kid-1")
	authenticator, err := newCognitoAuthenticator(cognitoConfig{
		issuer: "issuer", clientID: "client-id", jwksURL: "https://example.invalid/jwks.json", now: func() time.Time { return now }, client: client,
	}, fakeStaffLookup{staffBySubject: map[string]*models.StaffUser{subject: staff}})
	if err != nil {
		t.Fatal(err)
	}
	token := signCognitoTestJWT(t, key, "kid-1", map[string]any{
		"iss": "issuer", "sub": subject, "aud": "client-id", "token_use": "id", "exp": now.Add(time.Hour).Unix(),
	})
	if _, err := authenticator.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("id token rejected: %v", err)
	}
}

func testJWKSClient(t *testing.T, key *rsa.PublicKey, kid string) *http.Client {
	t.Helper()
	body, err := json.Marshal(testJWKS(key, kid))
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Header:     make(http.Header),
		}, nil
	})}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
