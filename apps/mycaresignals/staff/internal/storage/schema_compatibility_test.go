package storage

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	"therapeutic-engine/internal/models"
)

func TestApplyTimelineScoresRejectsIncompleteLegacyScores(t *testing.T) {
	tests := []struct {
		name      string
		wellbeing sql.NullInt64
		progress  sql.NullInt64
	}{
		{name: "both null"},
		{name: "wellbeing null", progress: sql.NullInt64{Int64: 2, Valid: true}},
		{name: "progress null", wellbeing: sql.NullInt64{Int64: 2, Valid: true}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := models.TimelineItem{WellbeingScore: 4, ProgressScore: 4}
			if applyTimelineScores(&item, test.wellbeing, test.progress) {
				t.Fatal("incomplete legacy score was treated as a current timeline score")
			}
		})
	}
}

func TestApplyTimelineScoresPreservesValidZeroScores(t *testing.T) {
	item := models.TimelineItem{WellbeingScore: 4, ProgressScore: 4}
	zero := sql.NullInt64{Int64: 0, Valid: true}

	if !applyTimelineScores(&item, zero, zero) {
		t.Fatal("valid zero scores were omitted")
	}
	if item.WellbeingScore != 0 {
		t.Fatalf("wellbeing score = %d, want 0", item.WellbeingScore)
	}
	if item.ProgressScore != 0 {
		t.Fatalf("progress score = %d, want 0", item.ProgressScore)
	}
}

func TestNullableStringReadsUseCoalesce(t *testing.T) {
	tests := []struct {
		file    string
		clauses []string
	}{
		{
			file: "therapeutic_repo.go",
			clauses: []string{
				"COALESCE(ks.staff_name, '') AS staff_name",
				"COALESCE(s.staff_name, '')    AS staff_name",
				"COALESCE(ks.notes_summary, '') AS notes",
				"COALESCE(sc.comment, '')      AS comment",
			},
		},
		{
			file: "repo_psych_reviews.go",
			clauses: []string{
				"COALESCE(protective_factors, '') AS protective_factors",
				"COALESCE(recommendations, '') AS recommendations",
			},
		},
		{
			file: "children_storage.go",
			clauses: []string{
				"COALESCE(actions_summary, '') AS actions_summary",
				"COALESCE(future_notes, '') AS future_notes",
			},
		},
	}

	for _, test := range tests {
		content, err := os.ReadFile(test.file)
		if err != nil {
			t.Fatalf("read %s: %v", test.file, err)
		}
		for _, clause := range test.clauses {
			if !strings.Contains(string(content), clause) {
				t.Errorf("%s does not contain NULL-safe clause %q", test.file, clause)
			}
		}
	}
}

func TestDormantLegacyStorageCodeIsAbsent(t *testing.T) {
	for _, removed := range []string{
		"therapeutic_profiles.go",
		"../models/therapeutic_profile.go",
		"../ai/profile_generator.go",
	} {
		if _, err := os.Stat(removed); !os.IsNotExist(err) {
			t.Errorf("removed storage file %s still exists", removed)
		}
	}

	content, err := os.ReadFile("therapeutic_repo.go")
	if err != nil {
		t.Fatalf("read therapeutic_repo.go: %v", err)
	}
	for _, legacy := range []string{"care_plan_goals", "keywork_targets", "CreateCarePlanGoal", "CreateKeyworkTarget"} {
		if strings.Contains(string(content), legacy) {
			t.Errorf("therapeutic_repo.go still references removed legacy code %q", legacy)
		}
	}
}
