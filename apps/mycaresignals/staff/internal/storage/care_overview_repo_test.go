package storage

import (
	"testing"
	"time"

	"therapeutic-engine/internal/models"
)

func TestBuildChildCareOverviewUsesActiveRecordsAndDeterministicDueRules(t *testing.T) {
	now := careOverviewTestTime(2026, time.August, 19)
	dueYesterday := careOverviewTestTime(2026, time.August, 18)
	dueToday := careOverviewTestTime(2026, time.August, 19)
	recent := careOverviewTestTime(2026, time.August, 10)
	old := careOverviewTestTime(2026, time.June, 1)
	domain := "risk"
	goalID := int64(10)
	linkedGoalID := int64(10)
	lowLikelihood, lowSeverity := 1, 2
	mediumLikelihood, mediumSeverity := 3, 3
	highLikelihood, highSeverity := 4, 3

	activePlan := models.ChildCarePlan{
		ID: 22, Status: models.CarePlanStatusActive, EffectiveFrom: old,
		ReviewDate: &dueYesterday, ActivatedAt: &old, CreatedAt: old,
		Sections: []models.ChildCarePlanSection{
			{InformationStatus: models.CarePlanInformationAwaitingConfirmation},
			{InformationStatus: models.CarePlanInformationNotKnownYet},
		},
	}
	source := careOverviewSource{
		child:      models.CareOverviewChild{DisplayName: "Synthetic Child", Initials: "SC", HomeName: "Test Home"},
		baseline:   &models.ChildOnboardingBaseline{Status: "approved", CreatedAt: old, ApprovedAt: &old},
		priorities: []models.ChildPriorityDomain{{DomainKey: "risk", FocusSummary: "Maintain predictable routines", SortOrder: 1}},
		goals: []models.ChildGoal{
			{ID: goalID, Title: "Feel settled", Domain: &domain, Status: models.GoalStatusActive, CreatedAt: old, UpdatedAt: recent},
			{ID: 11, Title: "Closed goal", Domain: &domain, Status: models.GoalStatusClosed, CreatedAt: old, UpdatedAt: recent},
		},
		targets: []models.ChildTarget{
			{ID: 30, GoalID: &goalID, ShortLabel: "Use agreed check-in", Domain: "risk", Status: models.GoalStatusActive, ReviewDate: &dueYesterday, CreatedAt: old, UpdatedAt: recent},
			{ID: 31, GoalID: &goalID, ShortLabel: "Ask for support", Domain: "risk", Status: models.GoalStatusActive, ReviewDate: &dueToday, CreatedAt: old, UpdatedAt: recent},
			{ID: 32, ShortLabel: "Achieved recently", Domain: "risk", Status: models.GoalStatusAchieved, CreatedAt: old, UpdatedAt: recent},
			{ID: 33, ShortLabel: "Closed target", Domain: "risk", Status: models.GoalStatusClosed, ReviewDate: &dueYesterday, CreatedAt: old, UpdatedAt: recent},
		},
		risks: []models.ChildRisk{
			{ID: 40, Title: "Controls only", Domain: &domain, Status: models.RiskStatusActive, ManagementApproach: models.RiskControlsOnly, ResidualLikelihood: &lowLikelihood, ResidualSeverity: &lowSeverity, NextReviewDate: &dueToday, UpdatedAt: recent},
			{ID: 41, Title: "Linked developmental work", Domain: &domain, Status: models.RiskStatusActive, ManagementApproach: models.RiskControlsAndDevelopmentalWork, GoalIDs: []int64{linkedGoalID}, ResidualLikelihood: &mediumLikelihood, ResidualSeverity: &mediumSeverity, UpdatedAt: recent},
			{ID: 42, Title: "Unlinked developmental work", Domain: &domain, Status: models.RiskStatusActive, ManagementApproach: models.RiskControlsAndDevelopmentalWork, ResidualLikelihood: &highLikelihood, ResidualSeverity: &highSeverity, UpdatedAt: recent},
			{ID: 43, Title: "Closed risk", Domain: &domain, Status: models.RiskStatusClosed, ManagementApproach: models.RiskControlsAndDevelopmentalWork, NextReviewDate: &dueYesterday, UpdatedAt: recent},
		},
		riskReviews: []models.ChildRiskReview{{RiskID: 40, ReviewedAt: recent, ReviewedResidualLikelihood: 1, ReviewedResidualSeverity: 2}},
		plans: []models.ChildCarePlan{
			{ID: 21, Status: models.CarePlanStatusArchived, CreatedAt: old},
			activePlan,
			{ID: 23, Status: models.CarePlanStatusDraft, PredecessorCarePlanID: int64Pointer(22), CreatedAt: recent},
		},
		planReviews: []models.ChildCarePlanReview{{CarePlanID: 22, Outcome: models.CarePlanReviewRevise, ReviewedAt: recent}},
		sessions: []*models.KeyworkSession{
			{ID: 50, SessionAt: dueYesterday, TargetID: int64Pointer(30), SignedByKeyWorker: true},
			{ID: 51, SessionAt: dueToday, TargetID: int64Pointer(31)},
		},
		monthlyVoice: []careOverviewMonthlyVoice{{ReceiptID: 90, ReceivedAt: recent, TriageStatus: models.FeedbackStatusReviewed}},
	}

	overview := buildChildCareOverview(now, source)

	if overview.GoalsTargets.ActiveGoals != 1 || overview.GoalsTargets.ActiveTargets != 2 {
		t.Fatalf("active goals/targets = %d/%d, want 1/2", overview.GoalsTargets.ActiveGoals, overview.GoalsTargets.ActiveTargets)
	}
	if overview.GoalsTargets.TargetsDue != 2 || overview.GoalsTargets.TargetsOverdue != 1 || overview.GoalsTargets.AchievedRecently != 1 {
		t.Fatalf("target due/overdue/achieved = %d/%d/%d, want 2/1/1", overview.GoalsTargets.TargetsDue, overview.GoalsTargets.TargetsOverdue, overview.GoalsTargets.AchievedRecently)
	}
	if overview.RiskSafety.ActiveRisks != 3 || overview.RiskSafety.ReviewsDue != 1 {
		t.Fatalf("active risks/reviews due = %d/%d, want 3/1", overview.RiskSafety.ActiveRisks, overview.RiskSafety.ReviewsDue)
	}
	if overview.RiskSafety.RecordedLow != 1 || overview.RiskSafety.RecordedMedium != 1 || overview.RiskSafety.RecordedHigh != 1 {
		t.Fatalf("recorded bands = low %d medium %d high %d, want 1 each", overview.RiskSafety.RecordedLow, overview.RiskSafety.RecordedMedium, overview.RiskSafety.RecordedHigh)
	}
	if overview.RiskSafety.DevelopmentalWorkMissingLinks != 1 {
		t.Fatalf("developmental missing links = %d, want 1", overview.RiskSafety.DevelopmentalWorkMissingLinks)
	}
	if !overview.CarePlan.SuccessorDraftExists || overview.CarePlan.SuccessorDraftVersion != 3 {
		t.Fatalf("successor = %t version %d, want true/3", overview.CarePlan.SuccessorDraftExists, overview.CarePlan.SuccessorDraftVersion)
	}
	if len(overview.RecentKeyworking) != 2 || overview.RecentKeyworking[0].TargetLabel != "Ask for support" {
		t.Fatalf("recent keyworking not newest-first: %#v", overview.RecentKeyworking)
	}
	if len(overview.CurrentPriorities) != 1 || overview.CurrentPriorities[0].ActiveRisks != 3 || overview.CurrentPriorities[0].RecentKeyworking != 2 {
		t.Fatalf("risk priority counts = %#v", overview.CurrentPriorities)
	}
	if overview.ChildVoice.RecentEntryCount != 1 || overview.ChildVoice.LatestDate == nil {
		t.Fatalf("child voice = %#v, want one entry", overview.ChildVoice)
	}
	if len(overview.NeedsAttention) != 5 || overview.NeedsAttention[0].Priority > overview.NeedsAttention[1].Priority {
		t.Fatalf("attention items = %#v, want five priority-ordered actions", overview.NeedsAttention)
	}
	for _, item := range overview.NeedsAttention {
		if item.Kind == "risk_planning_link" && overview.RiskSafety.DevelopmentalWorkMissingLinks != 1 {
			t.Fatal("unexpected developmental planning warning")
		}
	}
	assertTimelineNewestFirst(t, overview.Timeline)
}

func TestBuildChildCareOverviewEmptyPlacementIsCalmAndUseful(t *testing.T) {
	admission := careOverviewTestTime(2026, time.August, 18)
	overview := buildChildCareOverview(careOverviewTestTime(2026, time.August, 19), careOverviewSource{
		child: models.CareOverviewChild{DisplayName: "New Placement", Initials: "NP", HomeName: "Test Home", AdmissionDate: &admission},
	})

	if overview.Baseline.Status != "not_started" {
		t.Fatalf("baseline status = %q, want not_started", overview.Baseline.Status)
	}
	if overview.CarePlan.Exists || overview.GoalsTargets.ActiveGoals != 0 || overview.RiskSafety.ActiveRisks != 0 {
		t.Fatalf("empty overview has unexpected current records: %#v", overview)
	}
	if len(overview.NeedsAttention) != 0 {
		t.Fatalf("empty placement attention = %#v, want none", overview.NeedsAttention)
	}
	if len(overview.Timeline) != 1 || overview.Timeline[0].Type != "placement" {
		t.Fatalf("empty placement timeline = %#v, want placement event", overview.Timeline)
	}
	if overview.CurrentPriorities == nil || overview.RecentKeyworking == nil || overview.RecentDailyProgress == nil || overview.Timeline == nil {
		t.Fatal("empty collections must encode as arrays")
	}
}

func TestBuildChildCareOverviewSurfacesDailyProgressWithoutCombiningPerspectives(t *testing.T) {
	staffScore := 4
	staffMedian := 4.0
	voiceScore := 1
	observed := careOverviewTestTime(2026, time.August, 18)
	note := "Synthetic concise observation"
	overview := buildChildCareOverview(careOverviewTestTime(2026, time.August, 19), careOverviewSource{
		dailyProgress: []models.ChildDailyProgress{{
			ObservedOn: observed, Domain: "Relationships & Connection",
			ProgressScore: &staffScore, ObservationNote: &note, RecordedByStaffName: "Test Worker",
		}},
		monthlyProgress: []models.MonthlyPerspectiveComparison{{
			PeriodMonth: "2026-08", Domain: "Relationships & Connection",
			StaffDescriptiveValue: &staffMedian, YoungPersonRating: &voiceScore,
			PersistentReviewSuggested: true,
		}},
	})

	if len(overview.RecentDailyProgress) != 1 || overview.RecentDailyProgress[0].ProgressScore == nil || *overview.RecentDailyProgress[0].ProgressScore != 4 {
		t.Fatalf("recent daily progress = %#v", overview.RecentDailyProgress)
	}
	if !overview.DailyProgressReviewSuggested {
		t.Fatal("persistent perspective difference should be visible as review suggested")
	}
	if len(overview.NeedsAttention) != 1 || overview.NeedsAttention[0].Kind != "perspective_difference" {
		t.Fatalf("attention = %#v, want one perspective review suggestion", overview.NeedsAttention)
	}
}

func TestBuildChildCareOverviewAddsOneMonthlyVoiceEventAndSeparateAttention(t *testing.T) {
	received := careOverviewTestTime(2026, time.August, 18)
	overview := buildChildCareOverview(careOverviewTestTime(2026, time.August, 19), careOverviewSource{
		monthlyVoice: []careOverviewMonthlyVoice{
			{
				ReceiptID: 10, ReceivedAt: received, FollowUpRequested: true,
				RequiresHumanReview: true,
				MatchedRuleIDs:      []string{"follow_up_requested", "safety_language_review"},
				TriageStatus:        models.FeedbackStatusNew,
			},
		},
	})
	monthlyEvents := 0
	for _, event := range overview.Timeline {
		if event.Type == "young_person_monthly_voice" {
			monthlyEvents++
		}
	}
	if monthlyEvents != 1 {
		t.Fatalf("monthly voice events = %d, want one: %#v", monthlyEvents, overview.Timeline)
	}
	for _, event := range overview.Timeline {
		if event.Type == "young_person_monthly_voice" && event.Source != "child-voice" {
			t.Fatalf("monthly voice source = %q, want child-voice", event.Source)
		}
	}
	if len(overview.NeedsAttention) != 2 || overview.NeedsAttention[0].Kind != "monthly_voice_human_review" || overview.NeedsAttention[1].Kind != "monthly_voice_follow_up" {
		t.Fatalf("monthly voice attention = %#v", overview.NeedsAttention)
	}
	for _, item := range overview.NeedsAttention {
		if item.Source != "child-voice" {
			t.Fatalf("monthly voice attention source = %q, want child-voice", item.Source)
		}
	}
	if overview.ChildVoice.RecentEntryCount != 1 || overview.ChildVoice.LatestDate == nil {
		t.Fatalf("child voice summary = %#v", overview.ChildVoice)
	}
}

func TestBuildChildCareOverviewTimelineIsLimitedAndNewestFirst(t *testing.T) {
	sessions := make([]*models.KeyworkSession, 20)
	for index := range sessions {
		sessions[index] = &models.KeyworkSession{ID: int64(index + 1), SessionAt: careOverviewTestTime(2026, time.August, index+1)}
	}
	overview := buildChildCareOverview(careOverviewTestTime(2026, time.August, 21), careOverviewSource{sessions: sessions})
	if len(overview.Timeline) != 16 {
		t.Fatalf("timeline length = %d, want 16", len(overview.Timeline))
	}
	assertTimelineNewestFirst(t, overview.Timeline)
}

func assertTimelineNewestFirst(t *testing.T, events []models.CareOverviewTimelineEvent) {
	t.Helper()
	for index := 1; index < len(events); index++ {
		if events[index].OccurredAt.After(events[index-1].OccurredAt) {
			t.Fatalf("timeline event %d is newer than preceding event", index)
		}
	}
}

func careOverviewTestTime(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
}

func int64Pointer(value int64) *int64 {
	return &value
}
