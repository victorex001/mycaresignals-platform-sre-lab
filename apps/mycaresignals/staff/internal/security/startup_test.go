package security

import (
	"errors"
	"testing"
)

func TestAuthenticationModeValidation(t *testing.T) {
	validDev := map[string]string{
		"APP_ENV": "development", "AUTH_MODE": string(AuthModeDevelopment), "ENABLE_DEV_AUTH": "true",
		"DEV_JWT_ISSUER": "issuer", "DEV_JWT_AUDIENCE": "audience", "DEV_JWT_HMAC_SECRET": "01234567890123456789012345678901",
	}
	getenv := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}

	if err := validateAuthenticationConfiguration(getenv(map[string]string{"AUTH_MODE": "unknown"}), false); !errors.Is(err, ErrUnsupportedAuthMode) {
		t.Fatalf("unknown mode error = %v", err)
	}
	for _, appEnv := range []string{"staging", "production"} {
		values := cloneStringMap(validDev)
		values["APP_ENV"] = appEnv
		if err := validateAuthenticationConfiguration(getenv(values), false); !errors.Is(err, ErrDevAuthForbidden) {
			t.Fatalf("%s dev-jwt error = %v", appEnv, err)
		}
	}
	incomplete := cloneStringMap(validDev)
	delete(incomplete, "DEV_JWT_AUDIENCE")
	if err := validateAuthenticationConfiguration(getenv(incomplete), false); !errors.Is(err, ErrDevAuthConfiguration) {
		t.Fatalf("incomplete dev-jwt error = %v", err)
	}
	for _, values := range []map[string]string{{}, {"AUTH_MODE": string(AuthModeNone)}, {"APP_ENV": "development", "AUTH_MODE": string(AuthModeNone)}} {
		if err := validateAuthenticationConfiguration(getenv(values), false); err != nil {
			t.Fatalf("dormant none rejected: %v", err)
		}
		if err := validateAuthenticationConfiguration(getenv(values), true); !errors.Is(err, ErrProtectedAuthRequired) {
			t.Fatalf("protected none error = %v", err)
		}
	}
	if err := validateAuthenticationConfiguration(getenv(validDev), true); err != nil {
		t.Fatalf("protected valid dev-jwt rejected: %v", err)
	}
	validCognito := map[string]string{
		"APP_ENV": "production", "AUTH_MODE": string(AuthModeCognito), "AWS_REGION": "eu-west-2",
		"COGNITO_USER_POOL_ID": "eu-west-2_example", "COGNITO_CLIENT_ID": "client",
	}
	if err := validateAuthenticationConfiguration(getenv(validCognito), true); err != nil {
		t.Fatalf("protected valid cognito rejected: %v", err)
	}
	devCognito := cloneStringMap(validCognito)
	devCognito["APP_ENV"] = "development"
	if err := validateAuthenticationConfiguration(getenv(devCognito), true); !errors.Is(err, ErrDevAuthForbidden) {
		t.Fatalf("development cognito gate error = %v", err)
	}
	missingCognito := cloneStringMap(validCognito)
	delete(missingCognito, "COGNITO_CLIENT_ID")
	if err := validateAuthenticationConfiguration(getenv(missingCognito), true); !errors.Is(err, ErrDevAuthConfiguration) {
		t.Fatalf("missing cognito config error = %v", err)
	}
}

func TestDormantRoutePolicy(t *testing.T) {
	getenv := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}
	valid := map[string]string{"APP_ENV": "development", "ALLOW_DEMO_SEED": "true", "DEMO_SEED_KEY": "configured"}
	if err := validateDormantRoutePolicy(getenv(valid)); err != nil {
		t.Fatal(err)
	}
	if !DemoSeedRouteMayBeRegistered(getenv(valid)) {
		t.Fatal("valid development demo route policy rejected")
	}
	for _, appEnv := range []string{"staging", "production"} {
		values := cloneStringMap(valid)
		values["APP_ENV"] = appEnv
		if err := validateDormantRoutePolicy(getenv(values)); !errors.Is(err, ErrDemoSeedConfiguration) {
			t.Fatalf("%s demo policy error = %v", appEnv, err)
		}
		if DemoSeedRouteMayBeRegistered(getenv(values)) {
			t.Fatalf("%s demo route allowed", appEnv)
		}
	}
	missingKey := cloneStringMap(valid)
	delete(missingKey, "DEMO_SEED_KEY")
	if err := validateDormantRoutePolicy(getenv(missingKey)); !errors.Is(err, ErrDemoSeedConfiguration) {
		t.Fatalf("missing key error = %v", err)
	}
	if !CheckInRoutePolicyResolved {
		t.Fatal("check-in route policy must remain resolved")
	}
}

func TestAuthenticatorFactoryDoesNotFallBackFromCognitoToDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("AUTH_MODE", string(AuthModeCognito))
	t.Setenv("AWS_REGION", "eu-west-2")
	t.Setenv("COGNITO_USER_POOL_ID", "eu-west-2_example")
	t.Setenv("COGNITO_CLIENT_ID", "client")
	t.Setenv("DEV_JWT_ISSUER", "issuer")
	t.Setenv("DEV_JWT_AUDIENCE", "audience")
	t.Setenv("DEV_JWT_HMAC_SECRET", "01234567890123456789012345678901")
	t.Setenv("ENABLE_DEV_AUTH", "true")

	authenticator, err := NewAuthenticatorFromEnv(fakeStaffLookup{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := authenticator.(*CognitoAuthenticator); !ok {
		t.Fatalf("factory returned %T, want *CognitoAuthenticator", authenticator)
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
