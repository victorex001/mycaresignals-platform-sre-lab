package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestSchemaIntegrityReconciliationMigration(t *testing.T) {
	content, err := os.ReadFile("020_schema_integrity_reconciliation.up.sql")
	if err != nil {
		t.Fatalf("read migration 020 up: %v", err)
	}
	up := string(content)

	if got := strings.Count(up, "ADD CONSTRAINT"); got != 6 {
		t.Fatalf("ADD CONSTRAINT count = %d, want 6", got)
	}
	if got := strings.Count(up, "NOT VALID"); got != 6 {
		t.Fatalf("NOT VALID count = %d, want 6", got)
	}

	constraints := []string{
		"chk_session_target_scores_wellbeing_0_4",
		"chk_session_target_scores_progress_0_4",
		"chk_child_goals_status",
		"chk_child_targets_status",
		"chk_keyworking_sessions_session_type",
		"chk_child_onboarding_baselines_status",
	}
	for _, name := range constraints {
		if !strings.Contains(up, "ADD CONSTRAINT "+name) {
			t.Errorf("migration 020 does not add %s", name)
		}
	}

	if !strings.Contains(up, "CREATE INDEX IF NOT EXISTS idx_psychological_reviews_child_created_at") ||
		!strings.Contains(up, "ON psychological_reviews (child_id, created_at DESC)") {
		t.Error("migration 020 does not create the psychological-review child/created_at index")
	}

	for _, forbidden := range []string{"CREATE TABLE therapeutic_profiles", "CREATE TABLE care_plan_goals", "CREATE TABLE keywork_targets"} {
		if strings.Contains(strings.ToUpper(up), strings.ToUpper(forbidden)) {
			t.Errorf("migration 020 contains forbidden schema creation %q", forbidden)
		}
	}
}

func TestSchemaIntegrityReconciliationDownRemovesOnlyNewObjects(t *testing.T) {
	content, err := os.ReadFile("020_schema_integrity_reconciliation.down.sql")
	if err != nil {
		t.Fatalf("read migration 020 down: %v", err)
	}
	down := string(content)

	if got := strings.Count(down, "DROP CONSTRAINT IF EXISTS"); got != 6 {
		t.Fatalf("DROP CONSTRAINT count = %d, want 6", got)
	}
	if got := strings.Count(down, "DROP INDEX IF EXISTS"); got != 1 {
		t.Fatalf("DROP INDEX count = %d, want 1", got)
	}
	for _, forbidden := range []string{"DROP TABLE", "DELETE FROM", "TRUNCATE"} {
		if strings.Contains(strings.ToUpper(down), forbidden) {
			t.Errorf("migration 020 down contains forbidden operation %q", forbidden)
		}
	}
}
