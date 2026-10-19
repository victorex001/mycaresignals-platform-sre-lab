package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestImmutableRiskReviewHistoryMigration(t *testing.T) {
	content, err := os.ReadFile("023_immutable_risk_review_history.up.sql")
	if err != nil {
		t.Fatalf("read migration 023 up: %v", err)
	}
	up := string(content)
	for _, required := range []string{
		"CREATE TABLE child_risk_reviews",
		"reviewer_staff_id BIGINT NOT NULL REFERENCES staff_users(id) ON DELETE RESTRICT",
		"FOREIGN KEY (risk_id, child_id) REFERENCES child_risks(id, child_id) ON DELETE RESTRICT",
		"chk_child_risk_reviews_previous_rating_pair",
		"chk_child_risk_reviews_reviewed_likelihood",
		"chk_child_risk_reviews_reviewed_severity",
		"chk_child_risk_reviews_note",
		"chk_child_risk_reviews_outcome",
		"CREATE TRIGGER trg_child_risk_reviews_immutable",
		"BEFORE UPDATE OR DELETE ON child_risk_reviews",
		"ON child_risk_reviews (risk_id, reviewed_at DESC, id DESC)",
	} {
		if !strings.Contains(up, required) {
			t.Errorf("migration 023 missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(up), "DROP TABLE") || strings.Contains(strings.ToUpper(up), "TRUNCATE") {
		t.Error("migration 023 up contains destructive table operations")
	}
}

func TestImmutableRiskReviewHistoryDownIsScoped(t *testing.T) {
	content, err := os.ReadFile("023_immutable_risk_review_history.down.sql")
	if err != nil {
		t.Fatalf("read migration 023 down: %v", err)
	}
	down := string(content)
	for _, required := range []string{
		"DROP TRIGGER IF EXISTS trg_child_risk_reviews_immutable ON child_risk_reviews",
		"DROP TABLE IF EXISTS child_risk_reviews",
		"DROP FUNCTION IF EXISTS prevent_child_risk_review_mutation",
	} {
		if !strings.Contains(down, required) {
			t.Errorf("migration 023 down missing %q", required)
		}
	}
	for _, forbidden := range []string{"DROP TABLE child_risks", "DROP TABLE child_goals", "DROP TABLE child_targets", "TRUNCATE"} {
		if strings.Contains(strings.ToUpper(down), strings.ToUpper(forbidden)) {
			t.Errorf("migration 023 down contains forbidden operation %q", forbidden)
		}
	}
}
