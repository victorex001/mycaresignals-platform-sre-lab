package notify

import (
	"strings"
	"testing"
	"time"

	"qr-feedback-api/internal/config"
)

func TestHomeFeedbackNotificationContainsOperationalFieldsOnly(t *testing.T) {
	received := time.Date(2026, time.August, 24, 12, 30, 0, 0, time.UTC)
	subject, body := homeFeedbackNotification(config.Config{
		EmailSubjectPrefix: "[Synthetic]",
		ManagerReviewURL:   "https://care.example.test/visitor-feedback",
	}, "receipt-reference", "Synthetic Home", "Professional", "priority", received)

	for _, expected := range []string{
		"New visitor feedback received", "Synthetic Home", "Professional",
		"2026-08-24T12:30:00Z", "receipt-reference", "Review priority: priority", "https://care.example.test/visitor-feedback",
	} {
		if !strings.Contains(subject+body, expected) {
			t.Fatalf("notification missing %q", expected)
		}
	}
	for _, forbidden := range []string{"what_went_well", "open_comments", "contact_email"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("notification includes narrative/contact field %q", forbidden)
		}
	}
}
