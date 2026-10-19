package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigration018AddsAuthoritativeMonthlyVoiceContext(t *testing.T) {
	contents, err := os.ReadFile("000018_secure_monthly_voice.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(contents)
	for _, required := range []string{
		"young_person_monthly_voice",
		"period_month DATE",
		"used_at TIMESTAMPTZ",
		"uq_monthly_voice_context_child_period",
		"domain_ratings JSONB",
		"follow_up_requested BOOLEAN",
		"uq_young_voice_monthly_context",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration 018 missing %q", required)
		}
	}
}
