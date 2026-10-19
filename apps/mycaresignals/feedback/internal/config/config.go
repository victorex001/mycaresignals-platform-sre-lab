// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Config struct {
	Port           string
	AllowedOrigins []string
	DatabaseURL    string
	SecretKey      string

	SMTPHost           string
	SMTPPort           int
	SMTPUser           string
	SMTPPass           string
	NotifyTo           []string
	EmailSubjectPrefix string

	RateLimitRPS   float64
	RateLimitBurst int

	AdminToken             string
	ManagerReviewURL       string
	TherapeuticIntakeURL   string
	TherapeuticIntakeToken string

	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	DBConnMaxIdleTime time.Duration
}

// env returns the value of an env var or a default.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// csv splits a comma-separated env var into a trimmed slice.
func csv(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func positiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func positiveDuration(value string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func Load() (Config, error) {
	// Human-friendly console logging
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	})

	// Parse numeric envs with safe defaults
	smtpPort, _ := strconv.Atoi(env("SMTP_PORT", "587"))
	rps, _ := strconv.ParseFloat(env("RATE_LIMIT_RPS", "1"), 64)
	burst, _ := strconv.Atoi(env("RATE_LIMIT_BURST", "5"))

	cfg := Config{
		Port:           env("PORT", "8080"),
		AllowedOrigins: csv(env("ALLOWED_ORIGINS", "http://localhost:3000")),
		DatabaseURL:    env("DATABASE_URL", ""),
		SecretKey:      env("SECRET_KEY", ""),

		SMTPHost:           env("SMTP_HOST", ""),
		SMTPPort:           smtpPort,
		SMTPUser:           env("SMTP_USER", ""),
		SMTPPass:           env("SMTP_PASS", ""),
		NotifyTo:           csv(env("NOTIFY_TO", "")),
		EmailSubjectPrefix: env("EMAIL_SUBJECT_PREFIX", "[Feedback]"),

		RateLimitRPS:   rps,
		RateLimitBurst: burst,

		AdminToken:             env("ADMIN_TOKEN", ""),
		ManagerReviewURL:       strings.TrimSpace(env("MANAGER_REVIEW_URL", "")),
		TherapeuticIntakeURL:   strings.TrimRight(env("THERAPEUTIC_INTAKE_URL", ""), "/"),
		TherapeuticIntakeToken: env("THERAPEUTIC_INTAKE_TOKEN", ""),

		DBMaxOpenConns:    positiveInt(env("DB_MAX_OPEN_CONNS", "10"), 10),
		DBMaxIdleConns:    positiveInt(env("DB_MAX_IDLE_CONNS", "5"), 5),
		DBConnMaxLifetime: positiveDuration(env("DB_CONN_MAX_LIFETIME", "30m"), 30*time.Minute),
		DBConnMaxIdleTime: positiveDuration(env("DB_CONN_MAX_IDLE_TIME", "5m"), 5*time.Minute),
	}

	// Hard requirements for the service to start
	if cfg.DatabaseURL == "" || cfg.SecretKey == "" {
		return Config{}, fmt.Errorf("DATABASE_URL and SECRET_KEY must be set")
	}
	if cfg.AdminToken == "" {
		return Config{}, fmt.Errorf("ADMIN_TOKEN must be set for admin API")
	}
	if cfg.DBMaxIdleConns > cfg.DBMaxOpenConns {
		return Config{}, fmt.Errorf("DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}

	return cfg, nil
}
