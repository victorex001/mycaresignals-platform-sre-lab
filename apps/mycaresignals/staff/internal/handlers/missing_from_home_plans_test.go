package handlers

import (
	"strings"
	"testing"

	"therapeutic-engine/internal/models"
)

func validMFHSubmission() models.MissingFromHomePlanContent {
	return models.MissingFromHomePlanContent{
		CurrentSituation: models.MFHCurrentSituation{
			SupervisionArrangement: "Synthetic supervision arrangement.",
		},
		MissingHistory: models.MFHMissingHistory{
			HistoryType:       "awaiting_information",
			InformationStatus: models.MFHInformationAwaitingConfirmation,
		},
		PeoplePlaces: []models.MFHPersonPlace{{
			Kind:              "known_location",
			NameOrLabel:       "Synthetic location A",
			Source:            "Synthetic development fixture",
			InformationStatus: models.MFHInformationPartlyKnown,
		}},
		Triggers: []models.MFHTrigger{{
			Description:       "Synthetic change in routine.",
			Source:            "Synthetic development fixture",
			InformationStatus: models.MFHInformationKnownConfirmed,
		}},
		Operational: models.MFHOperationalResponse{
			ImmediateChecks:         "Follow the home checklist.",
			ContactAttempts:         "Use the agreed contact plan.",
			LocalSearchExpectations: "Follow the home search procedure.",
			PolicePolicyWording:     "Follow the current provider policy and professional judgement.",
			InformationForPolice:    "Provide the approved child-specific plan details.",
		},
		Notifications: []models.MFHNotification{{
			Role:         "Social Worker",
			WhenToNotify: "According to the agreed provider procedure.",
			Source:       "Synthetic development fixture",
		}},
		ReturnFollowUp: models.MFHReturnFollowUp{
			ImmediateWelfareResponse:    "Offer a calm welfare check.",
			CalmWelcomeRecoveryApproach: "Use the agreed calm welcome approach.",
			ProfessionalNotifications:   "Notify agreed professionals according to procedure.",
		},
	}
}

func TestValidateMFHContentAllowsIncompleteDraftAndCompleteSubmission(t *testing.T) {
	if err := validateMFHContent(models.MissingFromHomePlanContent{}, false); err != nil {
		t.Fatalf("incomplete drafts must be saveable: %v", err)
	}
	if err := validateMFHContent(models.MissingFromHomePlanContent{}, true); err == nil {
		t.Fatal("incomplete plan must not be submittable")
	}
	if err := validateMFHContent(validMFHSubmission(), true); err != nil {
		t.Fatalf("complete synthetic plan should be submittable: %v", err)
	}
}

func TestValidateMFHContentRejectsInvalidStructuredValues(t *testing.T) {
	content := validMFHSubmission()
	content.PeoplePlaces[0].InformationStatus = "system_inferred"
	if err := validateMFHContent(content, false); err == nil {
		t.Fatal("invalid information status must be rejected")
	}

	content = validMFHSubmission()
	content.MissingHistory.HistoryType = "automatically_high_risk"
	if err := validateMFHContent(content, true); err == nil {
		t.Fatal("invalid history choice must be rejected")
	}
}

func TestValidateMFHContentBoundsSensitiveRepeatedData(t *testing.T) {
	content := validMFHSubmission()
	content.PeoplePlaces = make([]models.MFHPersonPlace, 51)
	if err := validateMFHContent(content, false); err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatalf("expected repeated item bound, got %v", err)
	}
}
