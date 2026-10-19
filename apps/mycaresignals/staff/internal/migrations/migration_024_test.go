package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestGuidedChildCarePlansMigration(t *testing.T) {
	content, err := os.ReadFile("024_guided_child_care_plans.up.sql")
	if err != nil {
		t.Fatalf("read migration 024 up: %v", err)
	}
	up := string(content)
	for _, required := range []string{
		"CREATE TABLE child_care_plans",
		"CREATE TABLE child_care_plan_sections",
		"chk_child_care_plans_status",
		"WHERE status = 'active'",
		"uq_child_care_plans_one_active_per_child",
		"uq_child_care_plan_sections_plan_key",
		"chk_child_care_plan_sections_key",
		"chk_child_care_plan_sections_information_status",
		"'not_known_yet'",
		"'awaiting_confirmation'",
		"created_by_staff_id BIGINT NOT NULL REFERENCES staff_users(id) ON DELETE RESTRICT",
	} {
		if !strings.Contains(up, required) {
			t.Errorf("migration 024 missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"CREATE TABLE care_plan_goals", "CREATE TABLE care_plan_targets",
		"CREATE TABLE care_plan_risks", "CREATE TABLE care_plan_sessions",
		"DROP TABLE", "TRUNCATE", "DELETE FROM",
	} {
		if strings.Contains(strings.ToUpper(up), strings.ToUpper(forbidden)) {
			t.Errorf("migration 024 contains forbidden operation %q", forbidden)
		}
	}
}

func TestGuidedChildCarePlansDownIsScoped(t *testing.T) {
	content, err := os.ReadFile("024_guided_child_care_plans.down.sql")
	if err != nil {
		t.Fatalf("read migration 024 down: %v", err)
	}
	down := string(content)
	for _, required := range []string{
		"DROP TABLE IF EXISTS child_care_plan_sections",
		"DROP TABLE IF EXISTS child_care_plans",
	} {
		if !strings.Contains(down, required) {
			t.Errorf("migration 024 down missing %q", required)
		}
	}
	for _, forbidden := range []string{"child_goals", "child_targets", "child_risks", "keyworking_sessions", "domain_checkins", "child_onboarding_baselines"} {
		if strings.Contains(down, forbidden) {
			t.Errorf("migration 024 down references existing first-class data %q", forbidden)
		}
	}
}
