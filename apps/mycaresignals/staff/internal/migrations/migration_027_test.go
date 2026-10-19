package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestSecureFeedbackTriageMigration(t *testing.T) {
	content, err := os.ReadFile("027_secure_feedback_human_triage.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := string(content)
	for _, required := range []string{
		"CREATE TABLE external_feedback_receipts", "CREATE TABLE external_feedback_reviews",
		"requires_human_review BOOLEAN", "review_priority TEXT", "matched_rule_ids JSONB",
		"escalate_for_risk_review", "fk_external_feedback_receipt_child_home",
		"prevent_external_feedback_source_mutation", "prevent_external_feedback_review_mutation",
	} {
		if !strings.Contains(up, required) {
			t.Errorf("migration 027 missing %q", required)
		}
	}
	for _, forbidden := range []string{"INSERT INTO child_risks", "UPDATE child_risks", "DELETE FROM child_risks"} {
		if strings.Contains(up, forbidden) {
			t.Errorf("migration 027 must not mutate Risk rows: %q", forbidden)
		}
	}
}
