package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestKeyworkingManagerReviewMigration(t *testing.T) {
	content, err := os.ReadFile("026_keyworking_manager_review.up.sql")
	if err != nil {
		t.Fatalf("read migration 026 up: %v", err)
	}
	up := string(content)
	for _, required := range []string{
		"ADD COLUMN manager_review_decision TEXT",
		"ADD COLUMN manager_review_comment TEXT",
		"manager_reviewed_by_staff_id BIGINT REFERENCES staff_users(id) ON DELETE RESTRICT",
		"manager_review_decision IN ('approved', 'returned_for_review')",
		"chk_keyworking_manager_return_comment",
		"signed_by_manager = (manager_review_decision = 'approved')",
		"idx_keyworking_sessions_manager_review_pending",
	} {
		if !strings.Contains(up, required) {
			t.Errorf("migration 026 missing %q", required)
		}
	}
	for _, forbidden := range []string{"CREATE TABLE", "DELETE FROM", "TRUNCATE", "DROP TABLE", "manager_reviewed_at"} {
		if strings.Contains(strings.ToUpper(up), strings.ToUpper(forbidden)) {
			t.Errorf("migration 026 contains forbidden or duplicate operation %q", forbidden)
		}
	}
}

func TestKeyworkingManagerReviewDownIsScoped(t *testing.T) {
	content, err := os.ReadFile("026_keyworking_manager_review.down.sql")
	if err != nil {
		t.Fatalf("read migration 026 down: %v", err)
	}
	down := string(content)
	for _, required := range []string{
		"DROP INDEX IF EXISTS idx_keyworking_sessions_manager_review_pending",
		"DROP COLUMN IF EXISTS manager_reviewed_by_staff_id",
		"DROP COLUMN IF EXISTS manager_review_comment",
		"DROP COLUMN IF EXISTS manager_review_decision",
	} {
		if !strings.Contains(down, required) {
			t.Errorf("migration 026 down missing %q", required)
		}
	}
	for _, migration018Field := range []string{"signed_by_manager", "manager_signed_at", "manager_name"} {
		if strings.Contains(down, "DROP COLUMN IF EXISTS "+migration018Field) {
			t.Errorf("migration 026 down drops migration 018 field %q", migration018Field)
		}
	}
}
