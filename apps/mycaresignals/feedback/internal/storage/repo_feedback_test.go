package storage

import (
	"testing"

	"qr-feedback-api/internal/models"
)

func TestFeedbackReviewState(t *testing.T) {
	tests := []struct {
		outcome  string
		required bool
		status   string
	}{
		{models.FeedbackOutcomeAcknowledged, false, "none"},
		{models.FeedbackOutcomeNoAction, false, "none"},
		{models.FeedbackOutcomeFollowUp, true, "open"},
		{models.FeedbackOutcomeClosed, false, "closed"},
	}
	for _, test := range tests {
		required, status := feedbackReviewState(test.outcome)
		if required != test.required || status != test.status {
			t.Fatalf("%s state = (%v, %q), want (%v, %q)", test.outcome, required, status, test.required, test.status)
		}
	}
}
