package progress

import (
	"math"
	"sort"
	"time"

	"therapeutic-engine/internal/models"
)

const (
	PerspectiveDifferenceThreshold = 2.0
	PerspectiveConsecutiveMonths   = 2
	PerspectiveRuleVersion         = "perspective-difference-v1"
	PersistentDifferenceWording    = "Persistent difference in perspectives — review suggested."
	StaffValueBasis                = "median daily progress score"
)

type monthlyKey struct {
	month  time.Time
	domain string
}

type scorePoint struct {
	value int
	date  time.Time
}

type monthlyGroup struct {
	observations []models.ChildDailyProgress
	voices       []models.ChildMonthlyVoiceRating
	keyworking   int
}

func BuildMonthlyComparisons(
	observations []models.ChildDailyProgress,
	voices []models.ChildMonthlyVoiceRating,
	keyworking []models.MonthlyKeyworkingCount,
) []models.MonthlyPerspectiveComparison {
	groups := make(map[monthlyKey]*monthlyGroup)
	ensure := func(month time.Time, domain string) *monthlyGroup {
		key := monthlyKey{month: monthStart(month), domain: domain}
		group := groups[key]
		if group == nil {
			group = &monthlyGroup{}
			groups[key] = group
		}
		return group
	}

	for _, observation := range observations {
		group := ensure(observation.ObservedOn, observation.Domain)
		group.observations = append(group.observations, observation)
	}
	for _, voice := range voices {
		group := ensure(voice.PeriodMonth, voice.Domain)
		group.voices = append(group.voices, voice)
	}
	for _, count := range keyworking {
		ensure(count.PeriodMonth, count.Domain).keyworking += count.Count
	}

	keys := make([]monthlyKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].domain == keys[j].domain {
			return keys[i].month.Before(keys[j].month)
		}
		return keys[i].domain < keys[j].domain
	})

	comparisons := make([]models.MonthlyPerspectiveComparison, 0, len(keys))
	for _, key := range keys {
		group := groups[key]
		wellbeingPoints := make([]scorePoint, 0, len(group.observations))
		progressPoints := make([]scorePoint, 0, len(group.observations))
		for _, observation := range group.observations {
			if observation.WellbeingScore != nil {
				wellbeingPoints = append(wellbeingPoints, scorePoint{value: *observation.WellbeingScore, date: observation.ObservedOn})
			}
			if observation.ProgressScore != nil {
				progressPoints = append(progressPoints, scorePoint{value: *observation.ProgressScore, date: observation.ObservedOn})
			}
		}
		wellbeing := statistics(wellbeingPoints)
		progressStats := statistics(progressPoints)
		comparison := models.MonthlyPerspectiveComparison{
			PeriodMonth: key.month.Format("2006-01"), Domain: key.domain,
			ObservationCount: len(group.observations), Wellbeing: wellbeing, Progress: progressStats,
			StaffValueBasis: StaffValueBasis, KeyworkingSessionCount: group.keyworking,
			RuleVersion: PerspectiveRuleVersion, ComparisonStatus: "not_enough_comparable_data",
		}
		if progressStats.Median != nil {
			value := *progressStats.Median
			comparison.StaffDescriptiveValue = &value
		}
		if len(group.voices) == 1 {
			rating := group.voices[0].YoungPersonRating
			source := group.voices[0].Source
			receiptID := group.voices[0].SourceReceiptID
			comparison.YoungPersonRating = &rating
			comparison.YoungPersonSource = &source
			comparison.YoungPersonSourceReceipt = &receiptID
			submittedAt := group.voices[0].SubmittedAt
			comparison.YoungPersonSubmittedAt = &submittedAt
		} else if len(group.voices) > 1 {
			comparison.ComparisonStatus = "conflicting_monthly_voice_records"
		}
		if comparison.StaffDescriptiveValue != nil && comparison.YoungPersonRating != nil {
			difference := *comparison.StaffDescriptiveValue - float64(*comparison.YoungPersonRating)
			comparison.PerspectiveDifference = &difference
			comparison.ComparisonStatus = "comparable"
		}
		comparisons = append(comparisons, comparison)
	}

	applyPersistentDifferenceRule(comparisons)
	sort.SliceStable(comparisons, func(i, j int) bool {
		if comparisons[i].PeriodMonth == comparisons[j].PeriodMonth {
			return comparisons[i].Domain < comparisons[j].Domain
		}
		return comparisons[i].PeriodMonth > comparisons[j].PeriodMonth
	})
	return comparisons
}

func statistics(points []scorePoint) models.MonthlyScoreStatistics {
	distribution := map[string]int{"0": 0, "1": 0, "2": 0, "3": 0, "4": 0}
	stats := models.MonthlyScoreStatistics{Count: len(points), Distribution: distribution}
	if len(points) == 0 {
		return stats
	}
	values := make([]int, 0, len(points))
	sum := 0
	latest := points[0]
	for _, point := range points {
		values = append(values, point.value)
		sum += point.value
		distribution[string(rune('0'+point.value))]++
		if point.date.After(latest.date) {
			latest = point
		}
	}
	sort.Ints(values)
	mean := float64(sum) / float64(len(values))
	median := float64(values[len(values)/2])
	if len(values)%2 == 0 {
		median = float64(values[len(values)/2-1]+values[len(values)/2]) / 2
	}
	minimum, maximum, latestValue := values[0], values[len(values)-1], latest.value
	stats.Mean, stats.Median = &mean, &median
	stats.Min, stats.Max, stats.Latest = &minimum, &maximum, &latestValue
	return stats
}

func applyPersistentDifferenceRule(comparisons []models.MonthlyPerspectiveComparison) {
	byDomain := make(map[string][]int)
	for index := range comparisons {
		byDomain[comparisons[index].Domain] = append(byDomain[comparisons[index].Domain], index)
	}
	for _, indices := range byDomain {
		sort.Slice(indices, func(i, j int) bool {
			return comparisons[indices[i]].PeriodMonth < comparisons[indices[j]].PeriodMonth
		})
		streak := 0
		var previousMonth time.Time
		for _, index := range indices {
			comparison := &comparisons[index]
			month, _ := time.Parse("2006-01", comparison.PeriodMonth)
			divergent := comparison.PerspectiveDifference != nil && math.Abs(*comparison.PerspectiveDifference) >= PerspectiveDifferenceThreshold
			adjacent := !previousMonth.IsZero() && previousMonth.AddDate(0, 1, 0).Equal(month)
			if divergent {
				if adjacent {
					streak++
				} else {
					streak = 1
				}
			} else {
				streak = 0
			}
			if streak >= PerspectiveConsecutiveMonths {
				wording := PersistentDifferenceWording
				comparison.PersistentReviewSuggested = true
				comparison.ReviewSuggestion = &wording
			}
			previousMonth = month
		}
	}
}

func monthStart(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.UTC)
}
