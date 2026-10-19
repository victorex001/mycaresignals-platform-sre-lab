package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigration031AddsVersionedMissingFromHomePlansOnly(t *testing.T) {
	contents, err := os.ReadFile("031_missing_from_home_plans.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(contents)
	for _, required := range []string{
		"child_missing_from_home_plans", "source_snapshots JSONB", "predecessor_plan_id",
		"'draft', 'submitted', 'approved', 'superseded', 'archived'",
		"uq_child_mfh_open_plan", "uq_child_mfh_approved_plan",
		"prevent_approved_mfh_plan_rewrite", "approved Missing From Home plan versions are immutable",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration 031 missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"INSERT INTO child_risks", "UPDATE child_risks", "DELETE FROM child_risks",
		"INSERT INTO child_care_plans", "UPDATE child_care_plans", "DELETE FROM child_care_plans",
		"INSERT INTO child_goals", "INSERT INTO child_targets",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration 031 must not contain %q", forbidden)
		}
	}
}

func TestMigration031IsReversible(t *testing.T) {
	contents, err := os.ReadFile("031_missing_from_home_plans.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(contents)
	if !strings.Contains(sql, "DROP TABLE IF EXISTS child_missing_from_home_plans") ||
		!strings.Contains(sql, "DROP FUNCTION IF EXISTS prevent_approved_mfh_plan_rewrite") {
		t.Fatal("migration 031 down must remove the MFH plan schema")
	}
}
