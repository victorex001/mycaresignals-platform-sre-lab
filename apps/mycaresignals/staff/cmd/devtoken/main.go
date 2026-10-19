package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

const maximumTokenLifetime = 24 * time.Hour

type tokenConfig struct {
	Issuer   string
	Audience string
	Subject  string
	Secret   []byte
	Lifetime time.Duration
}

type tokenClaims struct {
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func main() {
	subject := flag.String("subject", "", "existing staff external_auth_subject")
	lifetime := flag.Duration("ttl", time.Hour, "token lifetime")
	flag.Parse()

	token, err := generateToken(tokenConfig{
		Issuer:   os.Getenv("DEV_JWT_ISSUER"),
		Audience: os.Getenv("DEV_JWT_AUDIENCE"),
		Subject:  *subject,
		Secret:   []byte(os.Getenv("DEV_JWT_HMAC_SECRET")),
		Lifetime: *lifetime,
	}, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println(token)
}

func generateToken(config tokenConfig, now time.Time) (string, error) {
	config.Issuer = strings.TrimSpace(config.Issuer)
	config.Audience = strings.TrimSpace(config.Audience)
	config.Subject = strings.TrimSpace(config.Subject)
	if config.Issuer == "" || config.Audience == "" {
		return "", errors.New("DEV_JWT_ISSUER and DEV_JWT_AUDIENCE are required")
	}
	if config.Subject == "" {
		return "", errors.New("--subject is required")
	}
	if len(config.Secret) < 32 {
		return "", errors.New("DEV_JWT_HMAC_SECRET must contain at least 32 characters")
	}
	if config.Lifetime <= 0 || config.Lifetime > maximumTokenLifetime {
		return "", errors.New("--ttl must be greater than zero and no more than 24h")
	}

	header, err := encodeSegment(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := encodeSegment(tokenClaims{
		Issuer:    config.Issuer,
		Audience:  config.Audience,
		Subject:   config.Subject,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(config.Lifetime).Unix(),
	})
	if err != nil {
		return "", err
	}

	signingInput := header + "." + claims
	mac := hmac.New(sha256.New, config.Secret)
	_, _ = mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + signature, nil
}

func encodeSegment(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}
