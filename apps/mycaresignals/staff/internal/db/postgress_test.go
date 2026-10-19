package db

import (
	"strings"
	"testing"
	"time"
)

func TestLoadPoolConfigDefaults(t *testing.T) {
	config, err := loadPoolConfig(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if config.MaxOpenConns != 10 || config.MaxIdleConns != 5 ||
		config.ConnMaxLifetime != 30*time.Minute || config.ConnMaxIdleTime != 5*time.Minute {
		t.Fatalf("unexpected defaults: %#v", config)
	}
}

func TestLoadPoolConfigOverrides(t *testing.T) {
	values := map[string]string{
		"DB_MAX_OPEN_CONNS":     "20",
		"DB_MAX_IDLE_CONNS":     "8",
		"DB_CONN_MAX_LIFETIME":  "45m",
		"DB_CONN_MAX_IDLE_TIME": "2m",
	}
	config, err := loadPoolConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if config.MaxOpenConns != 20 || config.MaxIdleConns != 8 ||
		config.ConnMaxLifetime != 45*time.Minute || config.ConnMaxIdleTime != 2*time.Minute {
		t.Fatalf("unexpected overrides: %#v", config)
	}
}

func TestLoadPoolConfigRejectsUnsafeValues(t *testing.T) {
	for _, test := range []struct {
		name   string
		values map[string]string
		match  string
	}{
		{name: "non-positive open", values: map[string]string{"DB_MAX_OPEN_CONNS": "0"}, match: "DB_MAX_OPEN_CONNS"},
		{name: "invalid duration", values: map[string]string{"DB_CONN_MAX_LIFETIME": "eventually"}, match: "DB_CONN_MAX_LIFETIME"},
		{name: "idle exceeds open", values: map[string]string{"DB_MAX_OPEN_CONNS": "4", "DB_MAX_IDLE_CONNS": "5"}, match: "must not exceed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := loadPoolConfig(func(key string) string { return test.values[key] })
			if err == nil || !strings.Contains(err.Error(), test.match) {
				t.Fatalf("error = %v, want match %q", err, test.match)
			}
		})
	}
}
