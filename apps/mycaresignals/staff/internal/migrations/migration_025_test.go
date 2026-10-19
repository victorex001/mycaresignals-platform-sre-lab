package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestCarePlanReviewCycleMigration(t *testing.T) {
	content, err := os.ReadFile("025_care_plan_review_cycle.up.sql")
	if err != nil {
		t.Fatalf("read migration 025 up: %v", err)
	}
	up := string(content)
	for _, required := range []string{
		"ADD COLUMN predecessor_care_plan_id BIGINT",
		"fk_child_care_plans_predecessor_child",
		"CREATE TABLE child_care_plan_reviews",
		"CREATE TABLE child_care_plan_review_sections",
		"outcome IN ('continue', 'revise', 'close')",
		"successor_care_plan_id BIGINT",
		"prevent_child_care_plan_review_mutation",
		"trg_child_care_plan_reviews_immutable",
		"trg_child_care_plan_review_sections_immutable",
		"evidence_since TIMESTAMPTZ NOT NULL",
		"keyworking_session_count INTEGER NOT NULL",
	} {
		if !strings.Contains(up, required) {
			t.Errorf("migration 025 missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"CREATE TABLE care_plan_goals", "CREATE TABLE care_plan_targets",
		"CREATE TABLE care_plan_risks", "CREATE TABLE care_plan_sessions",
		"TRUNCATE", "DELETE FROM", "DROP TABLE",
	} {
		if strings.Contains(strings.ToUpper(up), strings.ToUpper(forbidden)) {
			t.Errorf("migration 025 contains forbidden operation %q", forbidden)
		}
	}
}

func TestCarePlanReviewCycleDownIsScoped(t *testing.T) {
	content, err := os.ReadFile("025_care_plan_review_cycle.down.sql")
	if err != nil {
		t.Fatalf("read migration 025 down: %v", err)
	}
	down := string(content)
	for _, required := range []string{
		"DROP TABLE IF EXISTS child_care_plan_review_sections",
		"DROP TABLE IF EXISTS child_care_plan_reviews",
		"DROP COLUMN IF EXISTS predecessor_care_plan_id",
	} {
		if !strings.Contains(down, required) {
			t.Errorf("migration 025 down missing %q", required)
		}
	}
	for _, existing := range []string{"child_goals", "child_targets", "child_risks", "keyworking_sessions", "domain_checkins"} {
		if strings.Contains(down, existing) {
			t.Errorf("migration 025 down references first-class linked data %q", existing)
		}
	}
}
