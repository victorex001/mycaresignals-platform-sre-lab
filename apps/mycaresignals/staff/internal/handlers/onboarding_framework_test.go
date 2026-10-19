package handlers

import (
	"testing"

	"therapeutic-engine/internal/models"
)

func validFrameworkPayload(pathway string) onboardingFrameworkPayload {
	domains := make([]frameworkDomainPayload, 0, len(models.CanonicalProgressDomains))
	for _, domain := range models.CanonicalProgressDomains {
		domains = append(domains, frameworkDomainPayload{
			DomainKey: domain, InformationStatus: "not_known_yet",
		})
	}
	return onboardingFrameworkPayload{
		OnboardingPathway:           pathway,
		ImmediateInformationStatus:  "partly_known",
		BackgroundInformationStatus: "awaiting_confirmation",
		SafetyInformationStatus:     "not_known_yet",
		Domains:                     domains,
		Status:                      "draft",
	}
}

func TestValidateFrameworkPayloadAcceptsBothHumanLedEntryPaths(t *testing.T) {
	for _, pathway := range []string{"new_placement", "existing_young_person"} {
		t.Run(pathway, func(t *testing.T) {
			payload := validFrameworkPayload(pathway)
			payload.Sources = []frameworkSourcePayload{{SourceType: "staff_observation"}}
			if err := validateFrameworkPayload(payload); err != nil {
				t.Fatalf("valid payload rejected: %v", err)
			}
		})
	}
}

func TestValidateFrameworkPayloadRequiresExactCanonicalSix(t *testing.T) {
	payload := validFrameworkPayload("new_placement")
	payload.Domains = payload.Domains[:5]
	if err := validateFrameworkPayload(payload); err == nil {
		t.Fatal("five domains accepted")
	}

	payload = validFrameworkPayload("new_placement")
	payload.Domains[0].DomainKey = "Physical health"
	if err := validateFrameworkPayload(payload); err == nil {
		t.Fatal("legacy domain accepted as canonical")
	}
}

func TestValidateFrameworkPayloadSupportsUncertaintyAndLimitsPriorities(t *testing.T) {
	submitted := validFrameworkPayload("existing_young_person")
	submitted.Status = "submitted"
	if err := validateFrameworkPayload(submitted); err != nil {
		t.Fatalf("submitted framework with explicit unresolved information rejected: %v", err)
	}

	payload := validFrameworkPayload("existing_young_person")
	for index := 0; index < 4; index++ {
		payload.Domains[index].Priority = true
	}
	if err := validateFrameworkPayload(payload); err == nil {
		t.Fatal("four priority domains accepted")
	}

	payload = validFrameworkPayload("existing_young_person")
	payload.Domains[0].InformationStatus = "certain_enough"
	if err := validateFrameworkPayload(payload); err == nil {
		t.Fatal("invented information status accepted")
	}
}

func TestValidateFrameworkPayloadRejectsUnknownProvenanceAndInvalidDates(t *testing.T) {
	payload := validFrameworkPayload("existing_young_person")
	payload.Sources = []frameworkSourcePayload{{SourceType: "ai_generated_conclusion"}}
	if err := validateFrameworkPayload(payload); err == nil {
		t.Fatal("unknown source accepted")
	}

	payload = validFrameworkPayload("existing_young_person")
	badDate := "not-a-date"
	payload.Domains[0].SourceDate = &badDate
	if err := validateFrameworkPayload(payload); err == nil {
		t.Fatal("invalid source date accepted")
	}
}

func TestUnresolvedInformationRemainsVisible(t *testing.T) {
	payload := validFrameworkPayload("new_placement")
	baselineStatus := models.ChildOnboardingBaseline{
		ImmediateInformationStatus:  payload.ImmediateInformationStatus,
		BackgroundInformationStatus: payload.BackgroundInformationStatus,
		SafetyInformationStatus:     payload.SafetyInformationStatus,
	}
	domains := make([]models.ChildOnboardingDomainState, 0, len(payload.Domains))
	for _, item := range payload.Domains {
		domains = append(domains, models.ChildOnboardingDomainState{InformationStatus: item.InformationStatus})
	}
	if got := unresolvedInformationCount(&baselineStatus, domains); got != 8 {
		t.Fatalf("unresolved count=%d want=8", got)
	}
}
