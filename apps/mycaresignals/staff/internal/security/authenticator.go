package security

import (
	"context"
	"errors"
)

var (
	ErrUnauthenticated           = errors.New("authenticated principal is required")
	ErrInvalidToken              = errors.New("invalid authentication token")
	ErrUnknownStaff              = errors.New("authenticated subject is not mapped to staff")
	ErrInactiveStaff             = errors.New("staff user is inactive")
	ErrAuthenticationUnavailable = errors.New("authentication lookup unavailable")
	ErrDevAuthConfiguration      = errors.New("development authentication configuration is invalid")
	ErrDevAuthForbidden          = errors.New("development authentication is forbidden in this environment")
	ErrUnsupportedAuthMode       = errors.New("authentication mode is not supported")
	ErrProtectedAuthRequired     = errors.New("protected routing requires a non-none authentication mode")
)

type AuthMode string

const (
	AuthModeNone        AuthMode = "none"
	AuthModeDevelopment AuthMode = "development"
	AuthModeDevJWT      AuthMode = "dev-jwt"
	AuthModeCognito     AuthMode = "cognito"
)

// Authenticator converts a provider credential into a verified, database-
// mapped Principal. HTTP middleware can be added later without coupling routes
// to a particular identity provider.
type Authenticator interface {
	Authenticate(ctx context.Context, credential string) (*Principal, error)
}
