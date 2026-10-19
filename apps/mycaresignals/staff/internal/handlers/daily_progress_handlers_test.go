package handlers

import (
	"testing"
	"time"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
	"therapeutic-engine/internal/storage"
)

func TestValidateDailyProgressPreservesZeroAndOptionalTarget(t *testing.T) {
	zero := 0
	targetID := int64(10)
	input, err := validateDailyProgressRequest(dailyProgressRequest{
		ObservedOn: "2026-08-20", Domain: models.CanonicalProgressDomains[0], TargetID: &targetID,
		WellbeingScore: &zero, ProgressScore: &zero, ObservationNote: "  Synthetic observation  ",
	}, time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if input.WellbeingScore == nil || *input.WellbeingScore != 0 || input.ProgressScore == nil || *input.ProgressScore != 0 {
		t.Fatalf("zero scores were not preserved: %#v", input)
	}
	if input.TargetID == nil || *input.TargetID != targetID || input.ObservationNote == nil || *input.ObservationNote != "Synthetic observation" {
		t.Fatalf("validated input = %#v", input)
	}
}

func TestValidateDailyProgressAllowsDomainOnlyAndRejectsInvalidScores(t *testing.T) {
	valid, err := validateDailyProgressRequest(dailyProgressRequest{
		Domain: models.CanonicalProgressDomains[1], ObservationNote: "Synthetic domain-only observation",
	}, time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC))
	if err != nil || valid.TargetID != nil {
		t.Fatalf("domain-only input = %#v, err %v", valid, err)
	}
	invalid := 5
	if _, err := validateDailyProgressRequest(dailyProgressRequest{Domain: models.CanonicalProgressDomains[0], WellbeingScore: &invalid}, time.Now()); err == nil {
		t.Fatal("score above 4 accepted")
	}
}

func TestDailyProgressWritePolicyExcludesResponsibleIndividual(t *testing.T) {
	allowed := make(map[security.Role]bool)
	for _, role := range storage.DailyProgressWriteRoles {
		allowed[role] = true
	}
	for _, role := range []security.Role{security.RoleKeyWorker, security.RoleSenior, security.RoleManager} {
		if !allowed[role] {
			t.Fatalf("write role %q missing", role)
		}
	}
	if allowed[security.RoleResponsibleIndividual] {
		t.Fatal("responsible individual must remain read-only")
	}
}
