package progress

import (
	"testing"
	"time"

	"therapeutic-engine/internal/models"
)

func TestBuildMonthlyComparisonsPreservesZeroAndDistribution(t *testing.T) {
	observations := []models.ChildDailyProgress{
		daily("2026-06-02", "Domain", intPointer(0), intPointer(0)),
		daily("2026-06-10", "Domain", intPointer(4), intPointer(2)),
	}
	result := BuildMonthlyComparisons(observations, nil, nil)
	if len(result) != 1 || result[0].ObservationCount != 2 {
		t.Fatalf("result = %#v", result)
	}
	if result[0].Wellbeing.Count != 2 || result[0].Wellbeing.Distribution["0"] != 1 || result[0].Wellbeing.Distribution["4"] != 1 {
		t.Fatalf("wellbeing stats = %#v", result[0].Wellbeing)
	}
	if result[0].Progress.Median == nil || *result[0].Progress.Median != 1 {
		t.Fatalf("progress stats = %#v", result[0].Progress)
	}
}

func TestPersistentDifferenceRequiresAdjacentDivergentMonths(t *testing.T) {
	domain := "Domain"
	observations := []models.ChildDailyProgress{
		daily("2026-05-02", domain, nil, intPointer(3)),
		daily("2026-06-02", domain, nil, intPointer(3)),
		daily("2026-07-02", domain, nil, intPointer(2)),
	}
	voices := []models.ChildMonthlyVoiceRating{
		voice("2026-05-01", domain, 1),
		voice("2026-06-01", domain, 1),
		voice("2026-07-01", domain, 2),
	}
	result := BuildMonthlyComparisons(observations, voices, nil)
	byMonth := monthlyByMonth(result)
	if byMonth["2026-05"].PersistentReviewSuggested {
		t.Fatal("one divergent month must not suggest persistent review")
	}
	if !byMonth["2026-06"].PersistentReviewSuggested || byMonth["2026-06"].ReviewSuggestion == nil || *byMonth["2026-06"].ReviewSuggestion != PersistentDifferenceWording {
		t.Fatalf("second divergent month = %#v", byMonth["2026-06"])
	}
	if byMonth["2026-07"].PersistentReviewSuggested {
		t.Fatal("aligned month must reset the difference streak")
	}
}

func TestMissingMonthBreaksPersistentDifferenceSequence(t *testing.T) {
	domain := "Domain"
	result := BuildMonthlyComparisons(
		[]models.ChildDailyProgress{daily("2026-05-02", domain, nil, intPointer(3)), daily("2026-07-02", domain, nil, intPointer(3))},
		[]models.ChildMonthlyVoiceRating{voice("2026-05-01", domain, 1), voice("2026-07-01", domain, 1)},
		nil,
	)
	for _, comparison := range result {
		if comparison.PersistentReviewSuggested {
			t.Fatalf("missing month must break sequence: %#v", result)
		}
	}
}

func TestMissingAndConflictingVoiceDoNotFabricateComparison(t *testing.T) {
	domain := "Domain"
	observation := daily("2026-06-02", domain, nil, intPointer(3))
	missing := BuildMonthlyComparisons([]models.ChildDailyProgress{observation}, nil, nil)
	if missing[0].PerspectiveDifference != nil || missing[0].ComparisonStatus != "not_enough_comparable_data" {
		t.Fatalf("missing voice comparison = %#v", missing[0])
	}
	conflicting := BuildMonthlyComparisons(
		[]models.ChildDailyProgress{observation},
		[]models.ChildMonthlyVoiceRating{voice("2026-06-01", domain, 1), voice("2026-06-01", domain, 4)},
		nil,
	)
	if conflicting[0].PerspectiveDifference != nil || conflicting[0].ComparisonStatus != "conflicting_monthly_voice_records" {
		t.Fatalf("conflicting voice comparison = %#v", conflicting[0])
	}
}

func daily(date, domain string, wellbeing, progress *int) models.ChildDailyProgress {
	observed, _ := time.Parse("2006-01-02", date)
	return models.ChildDailyProgress{ObservedOn: observed, Domain: domain, WellbeingScore: wellbeing, ProgressScore: progress}
}

func voice(month, domain string, rating int) models.ChildMonthlyVoiceRating {
	period, _ := time.Parse("2006-01-02", month)
	return models.ChildMonthlyVoiceRating{PeriodMonth: period, Domain: domain, YoungPersonRating: rating, Source: models.YoungPersonMonthlyVoiceSource, SourceReceiptID: 1}
}

func intPointer(value int) *int { return &value }

func monthlyByMonth(items []models.MonthlyPerspectiveComparison) map[string]models.MonthlyPerspectiveComparison {
	result := make(map[string]models.MonthlyPerspectiveComparison, len(items))
	for _, item := range items {
		result[item.PeriodMonth] = item
	}
	return result
}
