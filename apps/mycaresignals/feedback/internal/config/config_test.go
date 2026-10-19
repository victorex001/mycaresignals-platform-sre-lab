package config

import (
	"strings"
	"testing"
	"time"
)

func setRequiredConfig(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://example.invalid/test")
	t.Setenv("SECRET_KEY", "test-secret")
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
}

func TestPositivePoolParsersFallBackSafely(t *testing.T) {
	if got := positiveInt("not-a-number", 10); got != 10 {
		t.Fatalf("positiveInt fallback = %d", got)
	}
	if got := positiveInt("0", 10); got != 10 {
		t.Fatalf("positiveInt non-positive fallback = %d", got)
	}
	if got := positiveDuration("later", 5*time.Minute); got != 5*time.Minute {
		t.Fatalf("positiveDuration fallback = %s", got)
	}
	if got := positiveDuration("-1m", 5*time.Minute); got != 5*time.Minute {
		t.Fatalf("positiveDuration non-positive fallback = %s", got)
	}
}

func TestLoadDatabasePoolConfiguration(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("DB_MAX_OPEN_CONNS", "18")
	t.Setenv("DB_MAX_IDLE_CONNS", "7")
	t.Setenv("DB_CONN_MAX_LIFETIME", "40m")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "3m")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBMaxOpenConns != 18 || cfg.DBMaxIdleConns != 7 ||
		cfg.DBConnMaxLifetime != 40*time.Minute || cfg.DBConnMaxIdleTime != 3*time.Minute {
		t.Fatalf("unexpected database pool config: %#v", cfg)
	}
}

func TestLoadRejectsIdleConnectionsAboveOpenLimit(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("DB_MAX_OPEN_CONNS", "4")
	t.Setenv("DB_MAX_IDLE_CONNS", "5")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "must not exceed") {
		t.Fatalf("error = %v", err)
	}
}
