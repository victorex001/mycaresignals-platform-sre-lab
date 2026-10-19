package security

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	cognitoSource      = "cognito"
	maxCognitoTokenLen = 16 * 1024
)

type cognitoConfig struct {
	issuer   string
	clientID string
	jwksURL  string
	now      func() time.Time
	client   *http.Client
}

type CognitoAuthenticator struct {
	lookup StaffByExternalSubjectLookup
	config cognitoConfig
	cache  jwksCache
}

type jwksCache struct {
	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	expires time.Time
}

type cognitoClaims struct {
	Issuer    string          `json:"iss"`
	Subject   string          `json:"sub"`
	Audience  json.RawMessage `json:"aud"`
	ClientID  string          `json:"client_id"`
	TokenUse  string          `json:"token_use"`
	ExpiresAt int64           `json:"exp"`
	NotBefore *int64          `json:"nbf"`
	IssuedAt  *int64          `json:"iat"`
}

func NewCognitoAuthenticatorFromEnv(lookup StaffByExternalSubjectLookup) (*CognitoAuthenticator, error) {
	env := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }
	issuer := strings.TrimRight(env("COGNITO_ISSUER"), "/")
	if issuer == "" {
		region, poolID := env("AWS_REGION"), env("COGNITO_USER_POOL_ID")
		if region != "" && poolID != "" {
			issuer = fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, poolID)
		}
	}
	config := cognitoConfig{
		issuer:   issuer,
		clientID: env("COGNITO_CLIENT_ID"),
		jwksURL:  env("COGNITO_JWKS_URL"),
		now:      time.Now,
		client:   http.DefaultClient,
	}
	if config.jwksURL == "" && config.issuer != "" {
		config.jwksURL = config.issuer + "/.well-known/jwks.json"
	}
	return newCognitoAuthenticator(config, lookup)
}

func validateCognitoEnvironment(getenv func(string) string) error {
	appEnv := strings.TrimSpace(getenv("APP_ENV"))
	if appEnv == "development" && !strings.EqualFold(strings.TrimSpace(getenv("ALLOW_COGNITO_IN_DEVELOPMENT")), "true") {
		return fmt.Errorf("%w: cognito mode is not a development shortcut", ErrDevAuthForbidden)
	}
	issuer := strings.TrimRight(strings.TrimSpace(getenv("COGNITO_ISSUER")), "/")
	if issuer == "" {
		region, poolID := strings.TrimSpace(getenv("AWS_REGION")), strings.TrimSpace(getenv("COGNITO_USER_POOL_ID"))
		if region != "" && poolID != "" {
			issuer = fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, poolID)
		}
	}
	if issuer == "" || strings.TrimSpace(getenv("COGNITO_CLIENT_ID")) == "" {
		return fmt.Errorf("%w: Cognito issuer/user pool or client ID is missing", ErrDevAuthConfiguration)
	}
	return nil
}

func newCognitoAuthenticator(config cognitoConfig, lookup StaffByExternalSubjectLookup) (*CognitoAuthenticator, error) {
	if lookup == nil || strings.TrimSpace(config.issuer) == "" || strings.TrimSpace(config.clientID) == "" {
		return nil, ErrDevAuthConfiguration
	}
	config.issuer = strings.TrimRight(strings.TrimSpace(config.issuer), "/")
	config.clientID = strings.TrimSpace(config.clientID)
	if strings.TrimSpace(config.jwksURL) == "" {
		config.jwksURL = config.issuer + "/.well-known/jwks.json"
	}
	if config.now == nil {
		config.now = time.Now
	}
	if config.client == nil {
		config.client = http.DefaultClient
	}
	return &CognitoAuthenticator{lookup: lookup, config: config}, nil
}

func (a *CognitoAuthenticator) Authenticate(ctx context.Context, credential string) (*Principal, error) {
	token := strings.TrimSpace(credential)
	if token == "" {
		return nil, ErrUnauthenticated
	}
	if len(token) > maxCognitoTokenLen {
		return nil, ErrInvalidToken
	}
	claims, err := a.verify(ctx, token)
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
	if staff == nil || staff.Status != "active" || staff.ExternalAuthSubject == nil || *staff.ExternalAuthSubject != claims.Subject {
		return nil, ErrUnknownStaff
	}
	principal, ok := newPrincipal(staff.ID, claims.Subject, staff.Email, staff.DisplayName, cognitoSource)
	if !ok {
		return nil, ErrUnknownStaff
	}
	return &principal, nil
}

func (a *CognitoAuthenticator) verify(ctx context.Context, token string) (*cognitoClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	header, err := decodeJSONObject(headerBytes, map[string]struct{}{"alg": {}, "kid": {}})
	if err != nil || decodeJSONString(header["alg"]) != "RS256" {
		return nil, ErrInvalidToken
	}
	kid := decodeJSONString(header["kid"])
	if kid == "" {
		return nil, ErrInvalidToken
	}
	publicKey, err := a.publicKey(ctx, kid)
	if err != nil {
		return nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return nil, ErrInvalidToken
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims cognitoClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if err := a.validateClaims(claims); err != nil {
		return nil, err
	}
	return &claims, nil
}

func (a *CognitoAuthenticator) validateClaims(claims cognitoClaims) error {
	if strings.TrimSpace(claims.Subject) == "" || claims.Issuer != a.config.issuer {
		return ErrInvalidToken
	}
	switch claims.TokenUse {
	case "access":
		if claims.ClientID != a.config.clientID {
			return ErrInvalidToken
		}
	case "id":
		if !audienceContains(claims.Audience, a.config.clientID) {
			return ErrInvalidToken
		}
	default:
		return ErrInvalidToken
	}
	now := a.config.now().Unix()
	if claims.ExpiresAt <= now || (claims.NotBefore != nil && *claims.NotBefore > now) || (claims.IssuedAt != nil && *claims.IssuedAt > now) {
		return ErrInvalidToken
	}
	return nil
}

func (a *CognitoAuthenticator) publicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	now := a.config.now()
	a.cache.mu.Lock()
	if key := a.cache.keys[kid]; key != nil && now.Before(a.cache.expires) {
		a.cache.mu.Unlock()
		return key, nil
	}
	a.cache.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.config.jwksURL, nil)
	if err != nil {
		return nil, ErrInvalidToken
	}
	resp, err := a.config.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAuthenticationUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, ErrAuthenticationUnavailable
	}
	var jwks struct {
		Keys []struct {
			Kty string   `json:"kty"`
			Kid string   `json:"kid"`
			Use string   `json:"use"`
			Alg string   `json:"alg"`
			N   string   `json:"n"`
			E   string   `json:"e"`
			X5C []string `json:"x5c"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, ErrAuthenticationUnavailable
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, key := range jwks.Keys {
		if key.Kid == "" || key.Kty != "RSA" || key.Alg != "RS256" {
			continue
		}
		publicKey, err := rsaPublicKeyFromJWK(key.N, key.E, key.X5C)
		if err == nil {
			keys[key.Kid] = publicKey
		}
	}
	a.cache.mu.Lock()
	a.cache.keys = keys
	a.cache.expires = now.Add(time.Hour)
	publicKey := a.cache.keys[kid]
	a.cache.mu.Unlock()
	if publicKey == nil {
		return nil, ErrInvalidToken
	}
	return publicKey, nil
}

func rsaPublicKeyFromJWK(n, e string, x5c []string) (*rsa.PublicKey, error) {
	if len(x5c) > 0 {
		der, err := base64.StdEncoding.DecodeString(x5c[0])
		if err == nil {
			cert, certErr := x509.ParseCertificate(der)
			if certErr == nil {
				if key, ok := cert.PublicKey.(*rsa.PublicKey); ok {
					return key, nil
				}
			}
		}
	}
	modulusBytes, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, err
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, err
	}
	exponent := 0
	for _, b := range exponentBytes {
		exponent = exponent<<8 + int(b)
	}
	if exponent == 0 {
		return nil, ErrInvalidToken
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(modulusBytes), E: exponent}, nil
}

var _ Authenticator = (*CognitoAuthenticator)(nil)
