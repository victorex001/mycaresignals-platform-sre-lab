package handlers

import (
	"net/http/httptest"
	"testing"
	"time"

	"qr-feedback-api/internal/models"

	"github.com/gin-gonic/gin"
)

func TestFeedbackFiltersAcceptMonthSourceStatusAndPrompt(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/v1/admin/feedback?month=2026-08&source=Ofsted&status=follow_up_required&requires_review=true", nil)
	context := newTestGinContext(httptest.NewRecorder(), request)
	filters, err := feedbackFilters(context)
	if err != nil {
		t.Fatal(err)
	}
	if filters.From == nil || filters.From.Format("2006-01-02") != "2026-08-01" || filters.To == nil || filters.To.Format("2006-01-02") != "2026-08-31" {
		t.Fatalf("month filter = %#v", filters)
	}
	if filters.Source != "Ofsted" || filters.Status != models.FeedbackOutcomeFollowUp || filters.RequiresReview == nil || !*filters.RequiresReview {
		t.Fatalf("filters = %#v", filters)
	}
}

func TestMonthlyReportUsesSyntheticSourceRecords(t *testing.T) {
	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	improvement := "More weekend activities"
	ratingFive, ratingThree := 5, 3
	rows := []models.FeedbackRow{
		{SubmittedAt: now, Who: "Parent", Rating: &ratingFive, Recommend: true, WhatWentWell: "Warm welcome", CurrentStatus: models.FeedbackOutcomeAcknowledged, RequiresHumanReview: false},
		{SubmittedAt: now, Who: "Professional", Rating: &ratingThree, WhatWentWell: "Clear handover", WhatToImprove: &improvement, CurrentStatus: models.FeedbackOutcomeFollowUp, FollowUpStatus: "open", HadFollowUp: true, RequiresHumanReview: true},
		{SubmittedAt: now, Who: "Ofsted", WhatWentWell: "Records available", CurrentStatus: models.FeedbackOutcomeClosed, FollowUpStatus: "closed", HadFollowUp: true, RequiresHumanReview: true},
	}
	report := monthlyReport("2026-08", rows)
	if report["total_responses"] != 3 || report["recommendation_count"] != 1 {
		t.Fatalf("report totals = %#v", report)
	}
	if report["by_source"].(map[string]int)["Ofsted"] != 1 {
		t.Fatalf("source totals = %#v", report["by_source"])
	}
	followUp := report["follow_up"].(gin.H)
	if followUp["required"] != 2 || followUp["open"] != 1 || followUp["closed"] != 1 {
		t.Fatalf("follow-up totals = %#v", followUp)
	}
}

func TestSpreadsheetSafeProtectsFormulaValues(t *testing.T) {
	for _, value := range []string{"=1+1", "+cmd", "-2", "@SUM(A1:A2)"} {
		if got := spreadsheetSafe(value); got != "'"+value {
			t.Fatalf("spreadsheetSafe(%q) = %q", value, got)
		}
	}
}
