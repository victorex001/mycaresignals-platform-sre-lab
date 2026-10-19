package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
)

type feedbackHandlerStore struct {
	ingested *models.ExternalFeedbackIngest
}

func (s *feedbackHandlerStore) IngestExternalFeedback(_ context.Context, input models.ExternalFeedbackIngest) (*models.ExternalFeedbackReceipt, error) {
	s.ingested = &input
	return &models.ExternalFeedbackReceipt{ID: 1, IntakeReceiptID: input.ReceiptID, HomeID: input.HomeID}, nil
}
func (s *feedbackHandlerStore) ListChildMonthlyVoiceReceipts(context.Context, int64, string) ([]models.ExternalFeedbackReceipt, error) {
	return []models.ExternalFeedbackReceipt{}, nil
}
func (s *feedbackHandlerStore) GetChildMonthlyVoiceReceipt(context.Context, int64, int64) (*models.ExternalFeedbackReceipt, error) {
	return &models.ExternalFeedbackReceipt{}, nil
}
func (s *feedbackHandlerStore) ReviewExternalFeedback(context.Context, int64, int64, security.Principal, models.ExternalFeedbackReviewInput, string) (*models.ExternalFeedbackReview, error) {
	return &models.ExternalFeedbackReview{}, nil
}

func TestFeedbackIngestRejectsVisitorFeedback(t *testing.T) {
	store := &feedbackHandlerStore{}
	handler := NewExternalFeedbackHandler(store)
	payload := map[string]any{
		"receipt_id":  "10000000-0000-4000-8000-000000000001",
		"intake_type": "visitor_feedback", "source_type": "Parent", "home_id": 1,
		"received_at": time.Now().UTC(), "requires_human_review": true,
		"review_priority": "priority", "matched_rule_ids": []string{"safety_language_review"},
		"rule_version": "intake-review-v1", "raw_payload": map[string]any{"what_went_well": "synthetic"},
	}
	body, _ := json.Marshal(payload)
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/feedback-intake/receipts", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handler.IngestReceipt(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if store.ingested != nil {
		t.Fatalf("visitor feedback reached Therapeutic Engine storage: %#v", store.ingested)
	}
	if bytes.Contains(body, []byte(`"risk_level"`)) {
		t.Fatal("test payload unexpectedly contains a risk label")
	}
}

func TestFeedbackIngestRejectsVisitorFeedbackWithChildLinkage(t *testing.T) {
	childID := int64(1)
	input := models.ExternalFeedbackIngest{
		ReceiptID: "10000000-0000-4000-8000-000000000009", IntakeType: "visitor_feedback",
		SourceType: "Parent", HomeID: 1, ChildID: &childID, ReceivedAt: time.Now().UTC(),
		ReviewPriority: "routine", RuleVersion: "intake-review-v1", RawPayload: json.RawMessage(`{"what_went_well":"synthetic"}`),
	}
	if validFeedbackIngest(input) {
		t.Fatal("home visitor feedback with child linkage was accepted")
	}
}

func TestFeedbackIngestAcceptsVerifiedMonthlyVoiceEnvelope(t *testing.T) {
	store := &feedbackHandlerStore{}
	handler := NewExternalFeedbackHandler(store)
	ratings := syntheticMonthlyVoiceRatings(1)
	payload := map[string]any{
		"receipt_id":  "10000000-0000-4000-8000-000000000002",
		"intake_type": "young_person_voice", "source_type": "YoungPersonVoice",
		"home_id": 1, "child_id": 1, "received_at": time.Now().UTC(),
		"requires_human_review": true, "review_priority": "routine",
		"matched_rule_ids": []string{"follow_up_requested"}, "rule_version": "intake-review-v1",
		"intake_context_id": "20000000-0000-4000-8000-000000000001",
		"intake_purpose":    models.YoungPersonMonthlyVoiceSource,
		"period_month":      time.Now().UTC().Format("2006-01"), "domain_ratings": ratings,
		"follow_up_requested": true, "follow_up_preference": "Synthetic trusted adult",
		"raw_payload": map[string]any{
			"q1_overall": "Synthetic month", "domain_ratings": ratings,
			"q9_want_talk": true, "q9_follow_up_preference": "Synthetic trusted adult",
		},
	}
	body, _ := json.Marshal(payload)
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/feedback-intake/receipts", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handler.IngestReceipt(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if store.ingested == nil || store.ingested.ChildID == nil || *store.ingested.ChildID != 1 ||
		store.ingested.IntakePurpose != models.YoungPersonMonthlyVoiceSource {
		t.Fatalf("ingest=%#v", store.ingested)
	}
}

func TestFeedbackIngestRejectsUnverifiedOrForgedMonthlyVoice(t *testing.T) {
	base := models.ExternalFeedbackIngest{
		ReceiptID:  "10000000-0000-4000-8000-000000000003",
		IntakeType: "young_person_voice", SourceType: "YoungPersonVoice",
		HomeID: 1, ChildID: testInt64Pointer(1), ReceivedAt: time.Now().UTC(),
		ReviewPriority: "routine", RuleVersion: "intake-review-v1",
		IntakeContextID: "20000000-0000-4000-8000-000000000002",
		IntakePurpose:   models.YoungPersonMonthlyVoiceSource,
		PeriodMonth:     time.Now().UTC().Format("2006-01"), DomainRatings: syntheticMonthlyVoiceRatings(2),
	}
	base.RawPayload, _ = json.Marshal(map[string]any{
		"domain_ratings": base.DomainRatings, "q9_want_talk": false, "q9_follow_up_preference": "",
	})
	if !validFeedbackIngest(base) {
		t.Fatal("valid verified monthly envelope rejected")
	}
	checks := []struct {
		name string
		edit func(*models.ExternalFeedbackIngest)
	}{
		{"missing context", func(input *models.ExternalFeedbackIngest) { input.IntakeContextID = "" }},
		{"wrong purpose", func(input *models.ExternalFeedbackIngest) { input.IntakePurpose = "visitor_feedback" }},
		{"wrong child", func(input *models.ExternalFeedbackIngest) { input.ChildID = nil }},
		{"invalid rating", func(input *models.ExternalFeedbackIngest) {
			input.DomainRatings[models.CanonicalProgressDomains[0]] = 5
		}},
		{"raw mismatch", func(input *models.ExternalFeedbackIngest) {
			input.DomainRatings[models.CanonicalProgressDomains[0]] = 3
		}},
		{"context token leaked", func(input *models.ExternalFeedbackIngest) {
			input.RawPayload, _ = json.Marshal(map[string]any{"domain_ratings": input.DomainRatings, "q9_want_talk": false, "q9_follow_up_preference": "", "context_token": "secret"})
		}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			candidate := base
			candidate.DomainRatings = syntheticMonthlyVoiceRatings(2)
			check.edit(&candidate)
			if validFeedbackIngest(candidate) {
				t.Fatalf("forged envelope accepted: %#v", candidate)
			}
		})
	}
}

func syntheticMonthlyVoiceRatings(value int) map[string]int {
	ratings := make(map[string]int, len(models.CanonicalProgressDomains))
	for _, domain := range models.CanonicalProgressDomains {
		ratings[domain] = value
	}
	return ratings
}

func testInt64Pointer(value int64) *int64 { return &value }

func TestFeedbackReviewValidationRejectsManualCareLinkage(t *testing.T) {
	childID := int64(1)
	for _, input := range []models.ExternalFeedbackReviewInput{
		{Outcome: models.FeedbackOutcomeAcknowledge, Rationale: ""},
		{Outcome: models.FeedbackOutcomeLinkAsEvidence, Rationale: "Reviewed", ChildID: nil},
		{Outcome: models.FeedbackOutcomeAcknowledge, Rationale: "Reviewed", ChildID: &childID},
		{Outcome: models.FeedbackOutcomeAcknowledge, Rationale: "Reviewed", Domain: func() *string { value := "Safety"; return &value }()},
	} {
		if validFeedbackReview(input) {
			t.Fatalf("invalid review accepted: %#v", input)
		}
	}
	if !validFeedbackReview(models.ExternalFeedbackReviewInput{Outcome: models.FeedbackOutcomeAcknowledge, Rationale: "Synthetic review"}) {
		t.Fatal("valid human review was rejected")
	}
}
