package integration

import (
	"context"
	"encoding/json"
	"testing"

	"qr-feedback-api/internal/models"
)

type deliveryTestStore struct {
	items     []models.PendingDelivery
	completed []int64
	failed    []int64
}

func (s *deliveryTestStore) ListPendingDeliveries(context.Context, int) ([]models.PendingDelivery, error) {
	return s.items, nil
}

func (s *deliveryTestStore) MarkDeliveryComplete(_ context.Context, id int64) error {
	s.completed = append(s.completed, id)
	return nil
}

func (s *deliveryTestStore) MarkDeliveryFailed(_ context.Context, id int64, _ string) error {
	s.failed = append(s.failed, id)
	return nil
}

func TestDispatcherRetiresQueuedVisitorFeedbackWithoutSending(t *testing.T) {
	store := &deliveryTestStore{items: []models.PendingDelivery{{
		ID:      41,
		Payload: json.RawMessage(`{"intake_type":"visitor_feedback","raw_payload":{"comment":"synthetic"}}`),
	}}}
	dispatcher := NewDispatcher(store, "https://therapeutic.example.test", "test-token")

	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.completed) != 1 || store.completed[0] != 41 {
		t.Fatalf("completed deliveries = %v, want [41]", store.completed)
	}
	if len(store.failed) != 0 {
		t.Fatalf("failed deliveries = %v, want none", store.failed)
	}
}

func TestVisitorFeedbackDeliveryClassification(t *testing.T) {
	if !visitorFeedbackDelivery(json.RawMessage(`{"intake_type":"visitor_feedback"}`)) {
		t.Fatal("visitor feedback delivery was not classified")
	}
	if visitorFeedbackDelivery(json.RawMessage(`{"intake_type":"young_person_voice"}`)) {
		t.Fatal("young person monthly voice was classified as visitor feedback")
	}
}
