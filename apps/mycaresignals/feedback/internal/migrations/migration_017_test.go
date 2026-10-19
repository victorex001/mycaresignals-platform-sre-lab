package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestSecureIntakeMigrationProvidesImmutableReceiptsAndContexts(t *testing.T) {
	content, err := os.ReadFile("000017_secure_intake_foundation.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, required := range []string{
		"CREATE TABLE intake_contexts", "token_hash TEXT NOT NULL UNIQUE",
		"requires_human_review BOOLEAN", "review_priority TEXT",
		"matched_rule_ids JSONB", "rule_version TEXT",
		"CREATE TABLE intake_delivery_outbox", "prevent_raw_intake_mutation",
		"BEFORE UPDATE OR DELETE ON feedback", "BEFORE UPDATE OR DELETE ON young_voice_feedback",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("migration missing %q", required)
		}
	}
}
