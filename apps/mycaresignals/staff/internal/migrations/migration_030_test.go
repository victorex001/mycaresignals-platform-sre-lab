package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigration030AddsVersionedSixDomainFrameworkWithoutRewritingLegacy(t *testing.T) {
	contents, err := os.ReadFile("030_six_domain_onboarding_framework.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(contents)
	for _, required := range []string{
		"framework_model TEXT NOT NULL DEFAULT 'legacy_eight_domain'",
		"framework_version INT NOT NULL DEFAULT 1",
		"six_domain_v1",
		"new_placement", "existing_young_person",
		"known_confirmed", "partly_known", "awaiting_confirmation", "not_known_yet",
		"current_understanding", "strengths_protective_factors", "support_needs",
		"what_helps", "current_support", "young_person_view",
		"child_onboarding_framework_sources", "source_date DATE",
		"uq_child_onboarding_six_domain_version",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration 030 missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"UPDATE child_onboarding_baselines SET framework_model",
		"DELETE FROM child_onboarding_baselines",
		"INSERT INTO child_risks", "UPDATE child_risks",
		"INSERT INTO child_care_plans", "UPDATE child_care_plans",
		"INSERT INTO child_goals", "INSERT INTO child_targets",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration 030 must not contain %q", forbidden)
		}
	}
}

func TestMigration030IsReversible(t *testing.T) {
	contents, err := os.ReadFile("030_six_domain_onboarding_framework.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(contents)
	for _, required := range []string{
		"DROP TABLE IF EXISTS child_onboarding_framework_sources",
		"DROP COLUMN IF EXISTS current_understanding",
		"DROP COLUMN IF EXISTS framework_model",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration 030 down missing %q", required)
		}
	}
}
