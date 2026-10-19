package storage

import (
	"testing"
	"time"

	"therapeutic-engine/internal/models"
)

func TestMFHSourceSnapshotHashDetectsMaterialChange(t *testing.T) {
	updated := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	candidate := models.MFHReuseCandidate{
		CandidateKey: "framework:what_helps", SourceType: "initial_framework",
		Label: "What helps", Value: "Synthetic fixture", SourceLabel: "Initial Framework",
		InformationStatus: models.MFHInformationKnownConfirmed, SourceUpdatedAt: &updated,
	}
	original := candidateHash(candidate)
	if original == "" || original != candidateHash(candidate) {
		t.Fatal("source snapshot hashes must be deterministic")
	}
	candidate.Value = "Changed synthetic fixture"
	if original == candidateHash(candidate) {
		t.Fatal("changed source information must produce a different snapshot hash")
	}
}

func TestMFHInformationStatusNormalizesExistingSources(t *testing.T) {
	cases := map[string]models.MFHInformationStatus{
		"current_confirmed":     models.MFHInformationKnownConfirmed,
		"known_confirmed":       models.MFHInformationKnownConfirmed,
		"awaiting_confirmation": models.MFHInformationAwaitingConfirmation,
		"not_known_yet":         models.MFHInformationNotKnownYet,
		"historical":            models.MFHInformationPartlyKnown,
		"reported":              models.MFHInformationPartlyKnown,
	}
	for input, want := range cases {
		if got := mfhInformationStatus(input); got != want {
			t.Fatalf("mfhInformationStatus(%q)=%q, want %q", input, got, want)
		}
	}
}
