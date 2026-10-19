package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigration019AddsAppendOnlyManagerReviewHistory(t *testing.T) {
	contents, err := os.ReadFile("000019_manager_feedback_reviews.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(contents)
	for _, required := range []string{
		"CREATE TABLE feedback_management_reviews",
		"reviewer_identity TEXT NOT NULL",
		"reviewed_at TIMESTAMPTZ",
		"follow_up_required BOOLEAN",
		"follow_up_status TEXT",
		"BEFORE UPDATE OR DELETE",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration 019 missing %q", required)
		}
	}
}
