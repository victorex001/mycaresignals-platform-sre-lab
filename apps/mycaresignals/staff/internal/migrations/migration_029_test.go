package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigration029LinksVerifiedMonthlyVoiceWithoutCareMutations(t *testing.T) {
	contents, err := os.ReadFile("029_verified_monthly_voice_intake.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(contents)
	for _, required := range []string{
		"intake_context_id UUID",
		"intake_purpose TEXT",
		"period_month DATE",
		"domain_ratings JSONB",
		"follow_up_requested BOOLEAN",
		"uq_external_feedback_monthly_voice_context",
		"source_context_id UUID",
		"uq_child_monthly_voice_source_domain",
		"monthly voice rating requires a matching verified intake context",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration 029 missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"INSERT INTO child_risks", "UPDATE child_risks",
		"INSERT INTO child_care_plans", "UPDATE child_care_plans",
		"INSERT INTO child_targets", "UPDATE child_targets",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration 029 must not contain %q", forbidden)
		}
	}
}
