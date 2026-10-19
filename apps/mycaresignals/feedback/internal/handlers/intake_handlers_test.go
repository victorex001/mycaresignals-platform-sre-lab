package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"qr-feedback-api/internal/models"
	"qr-feedback-api/internal/storage"
)

type intakeTestStore struct {
	context        *models.IntakeContext
	resolveErr     error
	metadata       models.ReviewMetadata
	raw            []byte
	youngRaw       []byte
	youngRecord    *models.YoungVoice
	insertYoungErr error
}

func (s *intakeTestStore) ResolveIntakeContext(context.Context, string, string, time.Time) (*models.IntakeContext, error) {
	return s.context, s.resolveErr
}
func (s *intakeTestStore) HashIP(string) string { return "test-ip-hash" }
func (s *intakeTestStore) InsertFeedback(_ context.Context, _ models.FeedbackIn, _ *models.IntakeContext, metadata models.ReviewMetadata, _, _ string, _, raw []byte) (string, time.Time, error) {
	s.metadata = metadata
	s.raw = append([]byte(nil), raw...)
	return "10000000-0000-0000-0000-000000000001", time.Now().UTC(), nil
}
func (s *intakeTestStore) InsertYoungVoice(_ context.Context, record *models.YoungVoice) error {
	if s.insertYoungErr != nil {
		return s.insertYoungErr
	}
	record.ID = "10000000-0000-0000-0000-000000000002"
	record.SubmittedAt = time.Now().UTC()
	s.metadata = record.ReviewMetadata
	s.youngRaw = append([]byte(nil), record.RawSubmission...)
	copy := *record
	s.youngRecord = &copy
	return nil
}

func TestYoungVoiceChangedResubmissionReturnsConflict(t *testing.T) {
	period := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	childID := int64(1)
	store := &intakeTestStore{
		context: &models.IntakeContext{
			ID: "20000000-0000-0000-0000-000000000002", HomeExternalID: 1,
			ChildExternalID: &childID, Purpose: models.IntakePurposeMonthlyVoice, PeriodMonth: &period,
		},
		insertYoungErr: storage.ErrMonthlyVoiceConflict,
	}
	body := []byte(`{"context_token":"used-token","q1_overall":"Changed synthetic response","domain_ratings":{"Emotional & Psychological Wellbeing":1,"Behaviour & Self-Regulation":2,"Relationships & Connection":3,"Daily Living, Learning & Participation":4,"Safety, Risk & Coping with Stress":0,"Skills, Identity & Future":2}}`)
	response := httptest.NewRecorder()
	SubmitYoungVoice(store, nil)(newTestGinContext(response, httptest.NewRequest(http.MethodPost, "/api/v1/young-voice", bytes.NewReader(body))))
	if response.Code != http.StatusConflict {
		t.Fatalf("changed resubmission status = %d body=%s", response.Code, response.Body.String())
	}
}

type intakeTestNotifier struct{ calls int }

func (n *intakeTestNotifier) SendReviewRequired(string, string, string) error { n.calls++; return nil }

type homeIntakeTestNotifier struct{ called chan struct{} }

func (n *homeIntakeTestNotifier) SendHomeFeedbackReceived(string, string, string, string, time.Time) error {
	close(n.called)
	return nil
}

func TestPublicFeedbackReturnsMinimalReceiptAndStoresRuleMetadata(t *testing.T) {
	store := &intakeTestStore{context: &models.IntakeContext{
		ID: "20000000-0000-0000-0000-000000000001", HomeExternalID: 1,
		HomeLabel: "Synthetic Home", Purpose: models.IntakeTypeVisitorFeedback,
	}}
	body := []byte(`{"context_token":"opaque-test-token","source":"browser supplied","who":"Parent","what_went_well":"Synthetic feedback","what_to_improve":"I felt unsafe"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/feedback", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	SubmitFeedback(store, nil)(newTestGinContext(response, request))
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var receipt map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if len(receipt) != 1 || receipt["receipt_id"] == nil {
		t.Fatalf("response is not minimal: %#v", receipt)
	}
	if !store.metadata.RequiresHumanReview || store.metadata.ReviewPriority != "priority" {
		t.Fatalf("review metadata = %#v", store.metadata)
	}
	if bytes.Contains(store.raw, []byte("opaque-test-token")) {
		t.Fatal("context bearer token was retained in raw evidence")
	}
}

func TestPublicHomeFeedbackAlwaysNotifiesManagerPath(t *testing.T) {
	store := &intakeTestStore{context: &models.IntakeContext{
		ID: "20000000-0000-0000-0000-000000000011", HomeExternalID: 1,
		HomeLabel: "Synthetic Home", Purpose: models.IntakeTypeVisitorFeedback,
	}}
	notifier := &homeIntakeTestNotifier{called: make(chan struct{})}
	body := []byte(`{"context_token":"opaque-test-token","who":"Professional","what_went_well":"Synthetic routine feedback"}`)
	response := httptest.NewRecorder()
	SubmitFeedback(store, notifier)(newTestGinContext(response, httptest.NewRequest(http.MethodPost, "/api/v1/feedback", bytes.NewReader(body))))
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	select {
	case <-notifier.called:
	case <-time.After(time.Second):
		t.Fatal("home manager notification path was not called")
	}
}

func TestPublicHomeFeedbackRejectsChildScopedContext(t *testing.T) {
	childID := int64(1)
	store := &intakeTestStore{context: &models.IntakeContext{
		ID: "20000000-0000-0000-0000-000000000012", HomeExternalID: 1,
		HomeLabel: "Synthetic Home", ChildExternalID: &childID, Purpose: models.IntakeTypeVisitorFeedback,
	}}
	body := []byte(`{"context_token":"opaque-test-token","who":"Parent","what_went_well":"Synthetic feedback"}`)
	response := httptest.NewRecorder()
	SubmitFeedback(store, nil)(newTestGinContext(response, httptest.NewRequest(http.MethodPost, "/api/v1/feedback", bytes.NewReader(body))))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestYoungVoiceResponseDoesNotExposeRuleResult(t *testing.T) {
	period := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	childID := int64(1)
	store := &intakeTestStore{context: &models.IntakeContext{
		ID: "20000000-0000-0000-0000-000000000002", HomeExternalID: 1,
		HomeLabel: "Synthetic Home", ChildExternalID: &childID,
		Purpose: models.IntakePurposeMonthlyVoice, PeriodMonth: &period,
	}}
	body := []byte(`{"context_token":"opaque-test-token","home":"untrusted","period_month":"2099-12","q1_overall":"Synthetic monthly response","q9_want_talk":true,"q9_follow_up_preference":"Trusted adult","domain_ratings":{"Emotional & Psychological Wellbeing":1,"Behaviour & Self-Regulation":2,"Relationships & Connection":3,"Daily Living, Learning & Participation":4,"Safety, Risk & Coping with Stress":0,"Skills, Identity & Future":2}}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/young-voice", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	SubmitYoungVoice(store, nil)(newTestGinContext(response, request))
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var receipt map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if len(receipt) != 1 || receipt["receipt_id"] == nil {
		t.Fatalf("response exposes more than receipt ID: %#v", receipt)
	}
	if !store.metadata.RequiresHumanReview {
		t.Fatal("explicit follow-up did not create a human review prompt")
	}
	if store.youngRecord == nil || !store.youngRecord.PeriodMonth.Equal(period) || store.youngRecord.Home != "Synthetic Home" || store.youngRecord.YoungPersonAlias != "" {
		t.Fatalf("monthly context was not authoritative: %#v", store.youngRecord)
	}
	if store.youngRecord.DomainRatings["Safety, Risk & Coping with Stress"] != 0 || !store.youngRecord.FollowUpRequested {
		t.Fatalf("structured monthly voice not preserved: %#v", store.youngRecord)
	}
	if bytes.Contains(store.youngRaw, []byte("opaque-test-token")) {
		t.Fatal("context bearer token was retained in monthly raw evidence")
	}
}

func TestYoungVoiceAcceptsShortFormWithoutLegacyText(t *testing.T) {
	period := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	childID := int64(1)
	store := &intakeTestStore{context: &models.IntakeContext{
		ID: "20000000-0000-0000-0000-000000000022", HomeExternalID: 1,
		HomeLabel: "Synthetic Home", ChildExternalID: &childID,
		Purpose: models.IntakePurposeMonthlyVoice, PeriodMonth: &period,
	}}
	body := []byte(`{"context_token":"opaque-test-token","q9_want_talk":false,"domain_ratings":{"Emotional & Psychological Wellbeing":0,"Behaviour & Self-Regulation":1,"Relationships & Connection":2,"Daily Living, Learning & Participation":3,"Safety, Risk & Coping with Stress":4,"Skills, Identity & Future":2}}`)
	response := httptest.NewRecorder()
	SubmitYoungVoice(store, nil)(newTestGinContext(response, httptest.NewRequest(http.MethodPost, "/api/v1/young-voice", bytes.NewReader(body))))
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if store.youngRecord == nil || store.youngRecord.Q1Overall != "" {
		t.Fatalf("optional legacy text was not preserved as empty: %#v", store.youngRecord)
	}
	if store.youngRecord.DomainRatings["Emotional & Psychological Wellbeing"] != 0 {
		t.Fatalf("rating zero was not preserved: %#v", store.youngRecord.DomainRatings)
	}
}

func TestYoungVoiceRejectsInvalidRatingAndInvalidContext(t *testing.T) {
	period := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	childID := int64(1)
	store := &intakeTestStore{context: &models.IntakeContext{
		ID: "20000000-0000-0000-0000-000000000002", HomeExternalID: 1,
		ChildExternalID: &childID, Purpose: models.IntakePurposeMonthlyVoice, PeriodMonth: &period,
	}}
	invalidRating := []byte(`{"context_token":"opaque-test-token","q1_overall":"Synthetic","domain_ratings":{"Emotional & Psychological Wellbeing":5}}`)
	response := httptest.NewRecorder()
	SubmitYoungVoice(store, nil)(newTestGinContext(response, httptest.NewRequest(http.MethodPost, "/api/v1/young-voice", bytes.NewReader(invalidRating))))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid rating status = %d", response.Code)
	}

	store.resolveErr = storage.ErrInvalidIntakeContext
	validRatings := []byte(`{"context_token":"expired","q1_overall":"Synthetic","domain_ratings":{"Emotional & Psychological Wellbeing":1,"Behaviour & Self-Regulation":2,"Relationships & Connection":3,"Daily Living, Learning & Participation":4,"Safety, Risk & Coping with Stress":0,"Skills, Identity & Future":2}}`)
	response = httptest.NewRecorder()
	SubmitYoungVoice(store, nil)(newTestGinContext(response, httptest.NewRequest(http.MethodPost, "/api/v1/young-voice", bytes.NewReader(validRatings))))
	if response.Code != http.StatusNotFound {
		t.Fatalf("invalid context status = %d", response.Code)
	}
}
