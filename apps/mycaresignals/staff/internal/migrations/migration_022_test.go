package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestRiskSafetyV1Migration(t *testing.T) {
	content, err := os.ReadFile("022_risk_safety_v1.up.sql")
	if err != nil {
		t.Fatalf("read migration 022 up: %v", err)
	}
	up := string(content)

	for _, table := range []string{"child_risks", "child_risk_goals", "child_risk_targets"} {
		if !strings.Contains(up, "CREATE TABLE "+table) {
			t.Errorf("migration 022 does not create %s", table)
		}
	}
	for _, constraint := range []string{
		"chk_child_risks_status",
		"chk_child_risks_information_status",
		"chk_child_risks_initial_likelihood",
		"chk_child_risks_initial_severity",
		"chk_child_risks_residual_likelihood",
		"chk_child_risks_residual_severity",
		"chk_child_risks_initial_rating_pair",
		"chk_child_risks_residual_rating_pair",
		"chk_child_risks_management_approach",
		"fk_child_risk_goals_risk_child",
		"fk_child_risk_goals_goal_child",
		"fk_child_risk_targets_risk_child",
		"fk_child_risk_targets_target_child",
	} {
		if !strings.Contains(up, constraint) {
			t.Errorf("migration 022 does not define %s", constraint)
		}
	}
	for _, relationship := range []string{
		"FOREIGN KEY (risk_id, child_id) REFERENCES child_risks(id, child_id)",
		"FOREIGN KEY (goal_id, child_id) REFERENCES child_goals(id, child_id)",
		"FOREIGN KEY (target_id, child_id) REFERENCES child_targets(id, child_id)",
	} {
		if !strings.Contains(up, relationship) {
			t.Errorf("migration 022 is missing cross-child protection %q", relationship)
		}
	}
	if strings.Contains(strings.ToUpper(up), "DROP TABLE") || strings.Contains(strings.ToUpper(up), "DELETE FROM") {
		t.Error("migration 022 up contains destructive data operations")
	}
}

func TestRiskSafetyV1DownRemovesOnlyVersion022Objects(t *testing.T) {
	content, err := os.ReadFile("022_risk_safety_v1.down.sql")
	if err != nil {
		t.Fatalf("read migration 022 down: %v", err)
	}
	down := string(content)
	for _, table := range []string{"child_risk_targets", "child_risk_goals", "child_risks"} {
		if !strings.Contains(down, "DROP TABLE IF EXISTS "+table) {
			t.Errorf("migration 022 down does not drop %s", table)
		}
	}
	for _, forbidden := range []string{"DROP TABLE child_goals", "DROP TABLE child_targets", "DELETE FROM", "TRUNCATE"} {
		if strings.Contains(strings.ToUpper(down), strings.ToUpper(forbidden)) {
			t.Errorf("migration 022 down contains forbidden operation %q", forbidden)
		}
	}
}
