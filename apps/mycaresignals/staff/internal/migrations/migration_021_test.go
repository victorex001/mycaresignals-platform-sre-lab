package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestGoalsTargetsV2PlanningFieldsMigration(t *testing.T) {
	upBytes, err := os.ReadFile("021_goals_targets_v2_planning_fields.up.sql")
	if err != nil {
		t.Fatalf("read migration 021 up: %v", err)
	}
	downBytes, err := os.ReadFile("021_goals_targets_v2_planning_fields.down.sql")
	if err != nil {
		t.Fatalf("read migration 021 down: %v", err)
	}

	up := string(upBytes)
	down := string(downBytes)
	for _, field := range []string{"ADD COLUMN IF NOT EXISTS domain TEXT", "ADD COLUMN IF NOT EXISTS review_date DATE"} {
		if !strings.Contains(up, field) {
			t.Errorf("migration 021 up is missing %q", field)
		}
	}
	for _, field := range []string{"DROP COLUMN IF EXISTS review_date", "DROP COLUMN IF EXISTS domain"} {
		if !strings.Contains(down, field) {
			t.Errorf("migration 021 down is missing %q", field)
		}
	}
	for _, forbidden := range []string{"CREATE TABLE", "DELETE FROM", "TRUNCATE", "NOT NULL"} {
		if strings.Contains(strings.ToUpper(up), forbidden) {
			t.Errorf("migration 021 up contains forbidden operation %q", forbidden)
		}
	}
}
