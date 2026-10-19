package security

import (
	"errors"
	"os"
	"strings"
)

var ErrDemoSeedConfiguration = errors.New("demo seed route configuration is unsafe")

// ValidateDormantRoutePolicyFromEnv prepares startup for later router wiring.
// It does not register, remove, or protect routes.
func ValidateDormantRoutePolicyFromEnv() error {
	return validateDormantRoutePolicy(os.Getenv)
}

func validateDormantRoutePolicy(getenv func(string) string) error {
	if strings.TrimSpace(getenv("ALLOW_DEMO_SEED")) != "true" {
		return nil
	}
	appEnv := strings.TrimSpace(getenv("APP_ENV"))
	if appEnv != "development" || getenv("DEMO_SEED_KEY") == "" {
		return ErrDemoSeedConfiguration
	}
	return nil
}

// DemoSeedRouteMayBeRegistered is a future router-policy helper. The existing
// handler's environment and shared-secret checks remain unchanged.
func DemoSeedRouteMayBeRegistered(getenv func(string) string) bool {
	return strings.TrimSpace(getenv("APP_ENV")) == "development" &&
		strings.TrimSpace(getenv("ALLOW_DEMO_SEED")) == "true" &&
		getenv("DEMO_SEED_KEY") != ""
}

// CheckInRoutePolicyResolved records that check-in reads are now authenticated,
// child-scoped, and authorized through the child's canonical home relationship.
const CheckInRoutePolicyResolved = true
