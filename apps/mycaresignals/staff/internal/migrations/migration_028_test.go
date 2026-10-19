package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigration028CreatesSeparateAppendOnlyEvidenceSources(t *testing.T) {
	contents, err := os.ReadFile("028_daily_progress_monthly_voice.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(contents)
	for _, required := range []string{
		"CREATE TABLE child_daily_progress",
		"recorded_by_staff_id",
		"wellbeing_score SMALLINT",
		"progress_score SMALLINT",
		"fk_child_daily_progress_target_child",
		"CREATE TABLE child_monthly_voice_ratings",
		"uq_child_monthly_voice_child_domain_month",
		"young_person_monthly_voice",
		"review.outcome = 'link_as_evidence'",
		"daily progress evidence is append-only",
		"monthly young-person voice evidence is immutable",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration 028 missing %q", required)
		}
	}
	for _, forbidden := range []string{"INSERT INTO child_risks", "UPDATE child_risks", "UPDATE child_care_plans", "UPDATE child_targets"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration 028 must not contain %q", forbidden)
		}
	}
}
