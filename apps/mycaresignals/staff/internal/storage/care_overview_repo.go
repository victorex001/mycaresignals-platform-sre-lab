package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"therapeutic-engine/internal/models"
)

const careOverviewRecentWindow = 30 * 24 * time.Hour

type careOverviewSource struct {
	child            models.CareOverviewChild
	baseline         *models.ChildOnboardingBaseline
	priorities       []models.ChildPriorityDomain
	goals            []models.ChildGoal
	targets          []models.ChildTarget
	risks            []models.ChildRisk
	riskReviews      []models.ChildRiskReview
	plans            []models.ChildCarePlan
	planReviews      []models.ChildCarePlanReview
	sessions         []*models.KeyworkSession
	externalEvidence []careOverviewExternalEvidence
	monthlyVoice     []careOverviewMonthlyVoice
	dailyProgress    []models.ChildDailyProgress
	monthlyProgress  []models.MonthlyPerspectiveComparison
}

type careOverviewExternalEvidence struct {
	ReviewedAt time.Time
	SourceType string
}

type careOverviewMonthlyVoice struct {
	ReceiptID           int64
	ReceivedAt          time.Time
	FollowUpRequested   bool
	RequiresHumanReview bool
	MatchedRuleIDs      []string
	TriageStatus        models.FeedbackTriageStatus
}

// GetChildCareOverview builds a read-only view from the child's authoritative
// records. It does not persist or infer any new care information.
func (r *Repo) GetChildCareOverview(ctx context.Context, childID int64, now time.Time) (*models.ChildCareOverview, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("care overview repository not initialised")
	}

	source := careOverviewSource{}
	child, err := r.getCareOverviewChild(ctx, childID)
	if err != nil {
		return nil, err
	}
	source.child = child

	source.baseline, source.priorities, err = r.getCareOverviewBaseline(ctx, childID)
	if err != nil {
		return nil, err
	}
	if source.goals, err = r.ListChildGoals(ctx, GoalFilters{ChildID: childID}); err != nil {
		return nil, err
	}
	if source.targets, err = r.ListChildTargets(ctx, TargetFilters{ChildID: childID}); err != nil {
		return nil, err
	}
	if source.risks, err = r.ListChildRisks(ctx, childID); err != nil {
		return nil, err
	}
	if source.riskReviews, err = r.listCareOverviewRiskReviews(ctx, childID); err != nil {
		return nil, err
	}
	if source.plans, err = r.ListChildCarePlans(ctx, childID); err != nil {
		return nil, err
	}
	if source.planReviews, err = r.listCareOverviewPlanReviews(ctx, childID); err != nil {
		return nil, err
	}
	if source.sessions, err = r.ListKeyworkSessionsByChild(ctx, childID); err != nil {
		return nil, err
	}
	if source.externalEvidence, err = r.listCareOverviewExternalEvidence(ctx, childID); err != nil {
		return nil, err
	}
	if source.monthlyVoice, err = r.listCareOverviewMonthlyVoice(ctx, childID); err != nil {
		return nil, err
	}
	if source.dailyProgress, err = r.ListDailyProgress(ctx, childID, 3); err != nil {
		return nil, err
	}
	month := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	if source.monthlyProgress, err = r.ListDailyProgressMonthly(ctx, childID, month.AddDate(0, -11, 0), month.AddDate(0, 1, 0)); err != nil {
		return nil, err
	}

	overview := buildChildCareOverview(now, source)
	return &overview, nil
}

func (r *Repo) listCareOverviewMonthlyVoice(ctx context.Context, childID int64) ([]careOverviewMonthlyVoice, error) {
	const query = `
SELECT id, received_at, follow_up_requested, requires_human_review,
       matched_rule_ids, triage_status
FROM external_feedback_receipts
WHERE child_id = $1
  AND intake_purpose = 'young_person_monthly_voice'
ORDER BY received_at DESC, id DESC`
	rows, err := r.db.QueryContext(ctx, query, childID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]careOverviewMonthlyVoice, 0)
	for rows.Next() {
		var item careOverviewMonthlyVoice
		var rules []byte
		if err := rows.Scan(
			&item.ReceiptID, &item.ReceivedAt, &item.FollowUpRequested,
			&item.RequiresHumanReview, &rules, &item.TriageStatus,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(rules, &item.MatchedRuleIDs); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repo) listCareOverviewExternalEvidence(ctx context.Context, childID int64) ([]careOverviewExternalEvidence, error) {
	const query = `
SELECT fr.reviewed_at, receipt.source_type
FROM external_feedback_reviews fr
JOIN external_feedback_receipts receipt ON receipt.id = fr.receipt_id
WHERE fr.child_id = $1
  AND fr.outcome = 'link_as_evidence'
  AND receipt.intake_type = 'young_person_voice'
  AND receipt.intake_purpose = 'young_person_monthly_voice'
ORDER BY fr.reviewed_at DESC, fr.id DESC`
	rows, err := r.db.QueryContext(ctx, query, childID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]careOverviewExternalEvidence, 0)
	for rows.Next() {
		var item careOverviewExternalEvidence
		if err := rows.Scan(&item.ReviewedAt, &item.SourceType); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repo) getCareOverviewChild(ctx context.Context, childID int64) (models.CareOverviewChild, error) {
	const query = `
SELECT c.initials, c.first_name, c.last_name, h.name, c.admission_date
FROM children c
JOIN homes h ON h.id = c.home_id
WHERE c.id = $1`
	var child models.CareOverviewChild
	var firstName, lastName sql.NullString
	var admissionDate sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, childID).Scan(
		&child.Initials, &firstName, &lastName, &child.HomeName, &admissionDate,
	); err != nil {
		return child, fmt.Errorf("get care overview child: %w", err)
	}
	child.DisplayName = strings.TrimSpace(strings.Join([]string{firstName.String, lastName.String}, " "))
	if child.DisplayName == "" {
		child.DisplayName = child.Initials
	}
	child.AdmissionDate = timeFromNull(admissionDate)
	return child, nil
}

func (r *Repo) getCareOverviewBaseline(ctx context.Context, childID int64) (*models.ChildOnboardingBaseline, []models.ChildPriorityDomain, error) {
	const baselineQuery = `
SELECT id, child_id, status, approved_at, created_at, updated_at
FROM child_onboarding_baselines
WHERE child_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1`
	var baseline models.ChildOnboardingBaseline
	var approvedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, baselineQuery, childID).Scan(
		&baseline.ID, &baseline.ChildID, &baseline.Status, &approvedAt,
		&baseline.CreatedAt, &baseline.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, []models.ChildPriorityDomain{}, nil
		}
		return nil, nil, fmt.Errorf("get care overview baseline: %w", err)
	}
	baseline.ApprovedAt = timeFromNull(approvedAt)

	const prioritiesQuery = `
SELECT id, baseline_id, domain_key, COALESCE(focus_summary, ''),
       COALESCE(hoped_for_change, ''), sort_order
FROM child_priority_domains
WHERE baseline_id = $1
ORDER BY sort_order, id`
	rows, err := r.db.QueryContext(ctx, prioritiesQuery, baseline.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("list care overview priorities: %w", err)
	}
	defer rows.Close()
	priorities := make([]models.ChildPriorityDomain, 0)
	for rows.Next() {
		var priority models.ChildPriorityDomain
		if err := rows.Scan(
			&priority.ID, &priority.BaselineID, &priority.DomainKey,
			&priority.FocusSummary, &priority.HopedForChange, &priority.SortOrder,
		); err != nil {
			return nil, nil, err
		}
		priorities = append(priorities, priority)
	}
	return &baseline, priorities, rows.Err()
}

func (r *Repo) listCareOverviewRiskReviews(ctx context.Context, childID int64) ([]models.ChildRiskReview, error) {
	const query = `
SELECT id, risk_id, child_id, reviewed_at, reviewed_residual_likelihood,
       reviewed_residual_severity, outcome, next_review_date, created_at
FROM child_risk_reviews
WHERE child_id = $1
ORDER BY reviewed_at DESC, id DESC`
	rows, err := r.db.QueryContext(ctx, query, childID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reviews := make([]models.ChildRiskReview, 0)
	for rows.Next() {
		var review models.ChildRiskReview
		var nextReview sql.NullTime
		if err := rows.Scan(
			&review.ID, &review.RiskID, &review.ChildID, &review.ReviewedAt,
			&review.ReviewedResidualLikelihood, &review.ReviewedResidualSeverity,
			&review.Outcome, &nextReview, &review.CreatedAt,
		); err != nil {
			return nil, err
		}
		review.NextReviewDate = timeFromNull(nextReview)
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (r *Repo) listCareOverviewPlanReviews(ctx context.Context, childID int64) ([]models.ChildCarePlanReview, error) {
	const query = `
SELECT id, care_plan_id, child_id, reviewed_at, outcome, next_review_date,
       successor_care_plan_id, created_at
FROM child_care_plan_reviews
WHERE child_id = $1
ORDER BY reviewed_at DESC, id DESC`
	rows, err := r.db.QueryContext(ctx, query, childID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reviews := make([]models.ChildCarePlanReview, 0)
	for rows.Next() {
		var review models.ChildCarePlanReview
		var nextReview sql.NullTime
		var successor sql.NullInt64
		if err := rows.Scan(
			&review.ID, &review.CarePlanID, &review.ChildID, &review.ReviewedAt,
			&review.Outcome, &nextReview, &successor, &review.CreatedAt,
		); err != nil {
			return nil, err
		}
		review.NextReviewDate = timeFromNull(nextReview)
		review.SuccessorCarePlanID = int64FromNull(successor)
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func buildChildCareOverview(now time.Time, source careOverviewSource) models.ChildCareOverview {
	now = now.UTC()
	today := dayStart(now)
	recentSince := now.Add(-careOverviewRecentWindow)
	overview := models.ChildCareOverview{
		Child:               source.child,
		CurrentPriorities:   []models.CareOverviewDomainPriority{},
		RecentKeyworking:    []models.CareOverviewKeyworking{},
		RecentDailyProgress: []models.CareOverviewDailyProgress{},
		NeedsAttention:      []models.CareOverviewAttentionItem{},
		Timeline:            []models.CareOverviewTimelineEvent{},
	}
	if source.baseline == nil {
		overview.Baseline.Status = "not_started"
	} else {
		createdAt := source.baseline.CreatedAt
		overview.Baseline = models.CareOverviewBaseline{
			Status: source.baseline.Status, CreatedAt: &createdAt,
			ApprovedAt: source.baseline.ApprovedAt,
		}
	}

	goalsByID := make(map[int64]models.ChildGoal, len(source.goals))
	for _, goal := range source.goals {
		goalsByID[goal.ID] = goal
	}
	targetsByID := make(map[int64]models.ChildTarget, len(source.targets))
	for _, target := range source.targets {
		targetsByID[target.ID] = target
	}

	buildGoalsTargetsOverview(&overview, source.goals, source.targets, goalsByID, today, recentSince)
	buildRiskOverview(&overview, source.risks, source.riskReviews, today, recentSince)
	currentPlanID := buildCarePlanOverview(&overview, source.plans, source.planReviews)
	buildKeyworkingOverview(&overview, source.sessions, targetsByID, goalsByID)
	buildDailyProgressOverview(&overview, source.dailyProgress, source.monthlyProgress)
	buildVerifiedMonthlyVoiceOverview(&overview, source.monthlyVoice, recentSince)
	buildPrioritiesOverview(&overview, source, today, recentSince)
	buildAttentionOverview(&overview, today, source.monthlyVoice)
	buildTimelineOverview(&overview, source, goalsByID, targetsByID, currentPlanID)
	return overview
}

func buildVerifiedMonthlyVoiceOverview(overview *models.ChildCareOverview, receipts []careOverviewMonthlyVoice, recentSince time.Time) {
	for _, receipt := range receipts {
		if !receipt.ReceivedAt.Before(recentSince) {
			overview.ChildVoice.RecentEntryCount++
		}
		if overview.ChildVoice.LatestDate == nil || receipt.ReceivedAt.After(*overview.ChildVoice.LatestDate) {
			value := receipt.ReceivedAt
			overview.ChildVoice.LatestDate = &value
		}
	}
}

func buildDailyProgressOverview(
	overview *models.ChildCareOverview,
	entries []models.ChildDailyProgress,
	monthly []models.MonthlyPerspectiveComparison,
) {
	for _, entry := range entries {
		targetLabel := ""
		if entry.TargetLabel != nil {
			targetLabel = *entry.TargetLabel
		}
		note := ""
		if entry.ObservationNote != nil {
			note = *entry.ObservationNote
		}
		overview.RecentDailyProgress = append(overview.RecentDailyProgress, models.CareOverviewDailyProgress{
			ObservedOn: entry.ObservedOn, Domain: entry.Domain, TargetLabel: targetLabel,
			WellbeingScore: entry.WellbeingScore, ProgressScore: entry.ProgressScore,
			ObservationNote: note, RecordedByStaffName: entry.RecordedByStaffName,
		})
	}
	for _, comparison := range monthly {
		if comparison.PersistentReviewSuggested {
			overview.DailyProgressReviewSuggested = true
			break
		}
	}
}

func buildGoalsTargetsOverview(
	overview *models.ChildCareOverview,
	goals []models.ChildGoal,
	targets []models.ChildTarget,
	goalsByID map[int64]models.ChildGoal,
	today time.Time,
	recentSince time.Time,
) {
	activeGoals := make([]models.ChildGoal, 0)
	for _, goal := range goals {
		if goal.Status == models.GoalStatusActive {
			activeGoals = append(activeGoals, goal)
		}
	}
	sort.SliceStable(activeGoals, func(i, j int) bool {
		if activeGoals[i].TargetDate != nil && activeGoals[j].TargetDate != nil {
			return activeGoals[i].TargetDate.Before(*activeGoals[j].TargetDate)
		}
		if activeGoals[i].TargetDate != nil {
			return true
		}
		if activeGoals[j].TargetDate != nil {
			return false
		}
		return activeGoals[i].UpdatedAt.After(activeGoals[j].UpdatedAt)
	})
	overview.GoalsTargets.ActiveGoals = len(activeGoals)
	overview.GoalsTargets.Goals = make([]models.CareOverviewGoalItem, 0, min(3, len(activeGoals)))
	for _, goal := range activeGoals[:min(3, len(activeGoals))] {
		overview.GoalsTargets.Goals = append(overview.GoalsTargets.Goals, models.CareOverviewGoalItem{
			Title: goal.Title, Domain: pointerString(goal.Domain), Status: string(goal.Status),
		})
	}

	activeTargets := make([]models.ChildTarget, 0)
	for _, target := range targets {
		if target.Status == models.GoalStatusAchieved && !target.UpdatedAt.Before(recentSince) {
			overview.GoalsTargets.AchievedRecently++
		}
		if target.Status != models.GoalStatusActive {
			continue
		}
		activeTargets = append(activeTargets, target)
		if target.ReviewDate != nil && !dayStart(*target.ReviewDate).After(today) {
			overview.GoalsTargets.TargetsDue++
			if dayStart(*target.ReviewDate).Before(today) {
				overview.GoalsTargets.TargetsOverdue++
			}
		}
	}
	sort.SliceStable(activeTargets, func(i, j int) bool {
		if activeTargets[i].ReviewDate != nil && activeTargets[j].ReviewDate != nil {
			return activeTargets[i].ReviewDate.Before(*activeTargets[j].ReviewDate)
		}
		if activeTargets[i].ReviewDate != nil {
			return true
		}
		if activeTargets[j].ReviewDate != nil {
			return false
		}
		return activeTargets[i].UpdatedAt.After(activeTargets[j].UpdatedAt)
	})
	overview.GoalsTargets.ActiveTargets = len(activeTargets)
	overview.GoalsTargets.Targets = make([]models.CareOverviewTargetItem, 0, min(4, len(activeTargets)))
	for _, target := range activeTargets[:min(4, len(activeTargets))] {
		goalTitle := ""
		if target.GoalID != nil {
			goalTitle = goalsByID[*target.GoalID].Title
		}
		overview.GoalsTargets.Targets = append(overview.GoalsTargets.Targets, models.CareOverviewTargetItem{
			Label: target.ShortLabel, GoalTitle: goalTitle, Domain: target.Domain,
			ReviewDate: target.ReviewDate, Status: string(target.Status),
		})
	}
}

func buildRiskOverview(
	overview *models.ChildCareOverview,
	risks []models.ChildRisk,
	reviews []models.ChildRiskReview,
	today time.Time,
	recentSince time.Time,
) {
	active := make([]models.ChildRisk, 0)
	for _, review := range reviews {
		if !review.ReviewedAt.Before(recentSince) {
			overview.RiskSafety.RecentCompletedReviews++
		}
	}
	for _, risk := range risks {
		if risk.Status != models.RiskStatusActive {
			continue
		}
		active = append(active, risk)
		if risk.NextReviewDate != nil && !dayStart(*risk.NextReviewDate).After(today) {
			overview.RiskSafety.ReviewsDue++
		}
		if risk.ManagementApproach == models.RiskControlsAndDevelopmentalWork && len(risk.GoalIDs) == 0 && len(risk.TargetIDs) == 0 {
			overview.RiskSafety.DevelopmentalWorkMissingLinks++
		}
		rating := recordedRiskRating(risk)
		if rating == nil {
			overview.RiskSafety.RecordedNotRated++
			continue
		}
		switch recordedRiskBand(*rating) {
		case "Low":
			overview.RiskSafety.RecordedLow++
		case "Medium":
			overview.RiskSafety.RecordedMedium++
		case "High":
			overview.RiskSafety.RecordedHigh++
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		if active[i].NextReviewDate != nil && active[j].NextReviewDate != nil {
			return active[i].NextReviewDate.Before(*active[j].NextReviewDate)
		}
		if active[i].NextReviewDate != nil {
			return true
		}
		if active[j].NextReviewDate != nil {
			return false
		}
		return active[i].UpdatedAt.After(active[j].UpdatedAt)
	})
	overview.RiskSafety.ActiveRisks = len(active)
	overview.RiskSafety.Items = make([]models.CareOverviewRiskItem, 0, min(3, len(active)))
	for _, risk := range active[:min(3, len(active))] {
		rating := recordedRiskRating(risk)
		band := ""
		if rating != nil {
			band = recordedRiskBand(*rating)
		}
		overview.RiskSafety.Items = append(overview.RiskSafety.Items, models.CareOverviewRiskItem{
			Title: risk.Title, Domain: pointerString(risk.Domain), RecordedRating: rating,
			RecordedBand: band, NextReviewDate: risk.NextReviewDate,
		})
	}
}

func buildCarePlanOverview(
	overview *models.ChildCareOverview,
	plans []models.ChildCarePlan,
	reviews []models.ChildCarePlanReview,
) int64 {
	currentIndex := -1
	for i := range plans {
		if plans[i].Status == models.CarePlanStatusActive {
			currentIndex = i
		}
	}
	if currentIndex == -1 {
		for i := len(plans) - 1; i >= 0; i-- {
			if plans[i].Status == models.CarePlanStatusDraft {
				currentIndex = i
				break
			}
		}
	}
	if currentIndex == -1 {
		return 0
	}
	plan := plans[currentIndex]
	effective := plan.EffectiveFrom
	overview.CarePlan = models.CareOverviewCarePlan{
		Exists: true, Status: string(plan.Status), Version: currentIndex + 1,
		EffectiveFrom: &effective, ReviewDate: plan.ReviewDate,
	}
	for _, section := range plan.Sections {
		switch section.InformationStatus {
		case models.CarePlanInformationNotKnownYet:
			overview.CarePlan.SectionsNotKnown++
		case models.CarePlanInformationAwaitingConfirmation:
			overview.CarePlan.SectionsAwaitingConfirmation++
		}
	}
	for _, review := range reviews {
		if review.CarePlanID == plan.ID {
			date := review.ReviewedAt
			overview.CarePlan.LatestReviewOutcome = string(review.Outcome)
			overview.CarePlan.LatestReviewedAt = &date
			break
		}
	}
	for index, candidate := range plans {
		if candidate.Status == models.CarePlanStatusDraft && candidate.PredecessorCarePlanID != nil && *candidate.PredecessorCarePlanID == plan.ID {
			overview.CarePlan.SuccessorDraftExists = true
			overview.CarePlan.SuccessorDraftVersion = index + 1
			break
		}
	}
	return plan.ID
}

func buildKeyworkingOverview(
	overview *models.ChildCareOverview,
	sessions []*models.KeyworkSession,
	targetsByID map[int64]models.ChildTarget,
	goalsByID map[int64]models.ChildGoal,
) {
	sorted := append([]*models.KeyworkSession(nil), sessions...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].SessionAt.Equal(sorted[j].SessionAt) {
			return sorted[i].ID > sorted[j].ID
		}
		return sorted[i].SessionAt.After(sorted[j].SessionAt)
	})
	for _, session := range sorted[:min(3, len(sorted))] {
		item := models.CareOverviewKeyworking{
			SessionAt: session.SessionAt, Domain: pointerString(session.PrimaryDomain),
			SignedByKeyWorker: session.SignedByKeyWorker,
		}
		if session.TargetID != nil {
			if target, ok := targetsByID[*session.TargetID]; ok {
				item.TargetLabel = target.ShortLabel
				if item.Domain == "" {
					item.Domain = target.Domain
				}
				if target.GoalID != nil {
					item.GoalTitle = goalsByID[*target.GoalID].Title
				}
			}
		}
		overview.RecentKeyworking = append(overview.RecentKeyworking, item)
	}
}

func buildPrioritiesOverview(overview *models.ChildCareOverview, source careOverviewSource, today, recentSince time.Time) {
	byKey := make(map[string]*models.CareOverviewDomainPriority)
	order := make([]string, 0)
	ensure := func(raw string) *models.CareOverviewDomainPriority {
		key, label := canonicalCareDomain(raw)
		if existing, ok := byKey[key]; ok {
			return existing
		}
		priority := &models.CareOverviewDomainPriority{DomainKey: key, Label: label}
		byKey[key] = priority
		order = append(order, key)
		return priority
	}
	for _, baselinePriority := range source.priorities {
		priority := ensure(baselinePriority.DomainKey)
		priority.FocusSummary = baselinePriority.FocusSummary
		priority.HopedForChange = baselinePriority.HopedForChange
	}
	for _, goal := range source.goals {
		if goal.Status == models.GoalStatusActive && goal.Domain != nil {
			ensure(*goal.Domain).ActiveGoals++
		}
	}
	for _, target := range source.targets {
		if target.Status != models.GoalStatusActive {
			continue
		}
		priority := ensure(target.Domain)
		priority.ActiveTargets++
		if target.ReviewDate != nil && !dayStart(*target.ReviewDate).After(today) {
			priority.TargetReviewsDue++
		}
	}
	for _, risk := range source.risks {
		if risk.Status == models.RiskStatusActive {
			domain := risk.Category
			if risk.Domain != nil {
				domain = *risk.Domain
			}
			ensure(domain).ActiveRisks++
		}
	}
	for _, session := range source.sessions {
		if session.SessionAt.Before(recentSince) {
			continue
		}
		domain := pointerString(session.PrimaryDomain)
		if domain == "" && session.TargetID != nil {
			for _, target := range source.targets {
				if target.ID == *session.TargetID {
					domain = target.Domain
					break
				}
			}
		}
		if domain != "" {
			ensure(domain).RecentKeyworking++
		}
	}
	for _, key := range order {
		priority := byKey[key]
		if priority.FocusSummary != "" || priority.HopedForChange != "" || priority.ActiveGoals > 0 || priority.ActiveTargets > 0 || priority.ActiveRisks > 0 || priority.RecentKeyworking > 0 {
			overview.CurrentPriorities = append(overview.CurrentPriorities, *priority)
		}
	}
}

func buildAttentionOverview(overview *models.ChildCareOverview, today time.Time, monthlyVoice []careOverviewMonthlyVoice) {
	items := make([]models.CareOverviewAttentionItem, 0)
	monthlyFollowUps := 0
	monthlyHumanReviews := 0
	for _, receipt := range monthlyVoice {
		if receipt.TriageStatus != models.FeedbackStatusNew && receipt.TriageStatus != models.FeedbackStatusFollowUp {
			continue
		}
		if receipt.FollowUpRequested {
			monthlyFollowUps++
		}
		if receipt.RequiresHumanReview && hasIndependentReviewRule(receipt.MatchedRuleIDs) {
			monthlyHumanReviews++
		}
	}
	if monthlyFollowUps > 0 {
		label := "Follow-up requested"
		if monthlyFollowUps > 1 {
			label = fmt.Sprintf("%d monthly voice follow-ups requested", monthlyFollowUps)
		}
		items = append(items, models.CareOverviewAttentionItem{
			Kind: "monthly_voice_follow_up", Label: label, Priority: 10, Source: "child-voice",
		})
	}
	if monthlyHumanReviews > 0 {
		label := "Young Person Monthly Voice requires human review"
		if monthlyHumanReviews > 1 {
			label = fmt.Sprintf("%d Young Person Monthly Voice submissions require human review", monthlyHumanReviews)
		}
		items = append(items, models.CareOverviewAttentionItem{
			Kind: "monthly_voice_human_review", Label: label, Priority: 5, Source: "child-voice",
		})
	}
	if overview.DailyProgressReviewSuggested {
		items = append(items, models.CareOverviewAttentionItem{
			Kind: "perspective_difference", Label: "Persistent difference in perspectives — review suggested.",
			Priority: 20, Source: "daily-progress",
		})
	}
	if overview.CarePlan.Exists && overview.CarePlan.Status == string(models.CarePlanStatusActive) && overview.CarePlan.ReviewDate != nil && !dayStart(*overview.CarePlan.ReviewDate).After(today) {
		label := "Care Plan review due"
		priority := 20
		if dayStart(*overview.CarePlan.ReviewDate).Before(today) {
			label = "Care Plan review overdue"
			priority = 10
		}
		items = append(items, models.CareOverviewAttentionItem{Kind: "care_plan_review", Label: label, DueDate: overview.CarePlan.ReviewDate, Priority: priority, Source: "care-plan"})
	}
	if overview.GoalsTargets.TargetsDue > 0 {
		label := fmt.Sprintf("%d target review%s due", overview.GoalsTargets.TargetsDue, pluralSuffix(overview.GoalsTargets.TargetsDue))
		priority := 20
		if overview.GoalsTargets.TargetsOverdue > 0 {
			label = fmt.Sprintf("%d target review%s due (%d overdue)", overview.GoalsTargets.TargetsDue, pluralSuffix(overview.GoalsTargets.TargetsDue), overview.GoalsTargets.TargetsOverdue)
			priority = 10
		}
		items = append(items, models.CareOverviewAttentionItem{Kind: "target_review", Label: label, Priority: priority, Source: "goals"})
	}
	if overview.RiskSafety.ReviewsDue > 0 {
		items = append(items, models.CareOverviewAttentionItem{
			Kind: "risk_review", Label: fmt.Sprintf("%d risk review%s due — professional review required", overview.RiskSafety.ReviewsDue, pluralSuffix(overview.RiskSafety.ReviewsDue)),
			Priority: 10, Source: "risks",
		})
	}
	if overview.CarePlan.SectionsAwaitingConfirmation > 0 {
		items = append(items, models.CareOverviewAttentionItem{
			Kind: "care_plan_awaiting", Label: fmt.Sprintf("%d Care Plan section%s awaiting confirmation", overview.CarePlan.SectionsAwaitingConfirmation, pluralSuffix(overview.CarePlan.SectionsAwaitingConfirmation)),
			Priority: 30, Source: "care-plan",
		})
	}
	if overview.CarePlan.SectionsNotKnown > 0 {
		items = append(items, models.CareOverviewAttentionItem{
			Kind: "care_plan_not_known", Label: fmt.Sprintf("%d Care Plan section%s recorded as not known yet", overview.CarePlan.SectionsNotKnown, pluralSuffix(overview.CarePlan.SectionsNotKnown)),
			Priority: 40, Source: "care-plan",
		})
	}
	if overview.RiskSafety.DevelopmentalWorkMissingLinks > 0 {
		items = append(items, models.CareOverviewAttentionItem{
			Kind: "risk_planning_link", Label: fmt.Sprintf("%d developmental-work risk%s without a linked goal or target", overview.RiskSafety.DevelopmentalWorkMissingLinks, pluralSuffix(overview.RiskSafety.DevelopmentalWorkMissingLinks)),
			Priority: 30, Source: "risks",
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Priority < items[j].Priority })
	if len(items) > 5 {
		items = items[:5]
	}
	overview.NeedsAttention = items
}

func hasIndependentReviewRule(ruleIDs []string) bool {
	for _, ruleID := range ruleIDs {
		if ruleID != "follow_up_requested" {
			return true
		}
	}
	return false
}

func buildTimelineOverview(
	overview *models.ChildCareOverview,
	source careOverviewSource,
	goalsByID map[int64]models.ChildGoal,
	targetsByID map[int64]models.ChildTarget,
	currentPlanID int64,
) {
	events := make([]models.CareOverviewTimelineEvent, 0)
	if source.child.AdmissionDate != nil {
		events = append(events, timelineEvent(*source.child.AdmissionDate, "placement", "Placement started", source.child.HomeName, "dashboard"))
	}
	if source.baseline != nil {
		events = append(events, timelineEvent(source.baseline.CreatedAt, "baseline", "Onboarding baseline recorded", "", "baseline-report"))
		if source.baseline.ApprovedAt != nil {
			events = append(events, timelineEvent(*source.baseline.ApprovedAt, "baseline_approved", "Onboarding baseline approved", "", "baseline-report"))
		}
	}
	for _, goal := range source.goals {
		events = append(events, timelineEvent(goal.CreatedAt, "goal", "Goal recorded", goal.Title, "goals"))
	}
	for _, target := range source.targets {
		detail := target.ShortLabel
		if target.GoalID != nil && goalsByID[*target.GoalID].Title != "" {
			detail += " · Goal: " + goalsByID[*target.GoalID].Title
		}
		events = append(events, timelineEvent(target.CreatedAt, "target", "Target recorded", detail, "goals"))
	}
	for _, risk := range source.risks {
		events = append(events, timelineEvent(risk.CreatedAt, "risk", "Risk information recorded", risk.Title, "risks"))
	}
	for _, review := range source.riskReviews {
		rating := review.ReviewedResidualLikelihood * review.ReviewedResidualSeverity
		events = append(events, timelineEvent(review.ReviewedAt, "risk_review", "Risk reviewed", fmt.Sprintf("Current recorded residual rating: %d / %s", rating, recordedRiskBand(rating)), "risks"))
	}
	for _, plan := range source.plans {
		if plan.ActivatedAt == nil {
			continue
		}
		title := "Care Plan activated"
		if plan.PredecessorCarePlanID != nil {
			title = "Successor Care Plan activated"
		}
		events = append(events, timelineEvent(*plan.ActivatedAt, "care_plan_activated", title, "", "care-plan"))
	}
	for _, review := range source.planReviews {
		detail := "Outcome: " + titleCase(string(review.Outcome))
		if review.CarePlanID == currentPlanID {
			detail += " · Current plan"
		}
		events = append(events, timelineEvent(review.ReviewedAt, "care_plan_review", "Care Plan reviewed", detail, "care-plan"))
	}
	for _, session := range source.sessions {
		detail := ""
		if session.TargetID != nil {
			detail = targetsByID[*session.TargetID].ShortLabel
			if detail != "" {
				detail = "Target: " + detail
			}
		}
		events = append(events, timelineEvent(session.SessionAt, "keyworking", "Key Working", detail, "keywork-history"))
	}
	for _, evidence := range source.externalEvidence {
		title := "External feedback linked as evidence"
		if evidence.SourceType == "YoungPersonVoice" {
			title = "Young person's direct voice linked as evidence"
		}
		events = append(events, timelineEvent(evidence.ReviewedAt, "external_feedback", title, externalFeedbackSourceLabel(evidence.SourceType), "child-voice"))
	}
	for _, receipt := range source.monthlyVoice {
		events = append(events, timelineEvent(
			receipt.ReceivedAt, "young_person_monthly_voice",
			"Young Person Monthly Voice received", "Verified monthly voice", "child-voice",
		))
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].OccurredAt.Equal(events[j].OccurredAt) {
			return events[i].Type < events[j].Type
		}
		return events[i].OccurredAt.After(events[j].OccurredAt)
	})
	if len(events) > 16 {
		events = events[:16]
	}
	overview.Timeline = events
}

func externalFeedbackSourceLabel(sourceType string) string {
	switch sourceType {
	case "YoungPersonVoice":
		return "Young Person Voice"
	case "SocialWorker":
		return "Social Worker feedback"
	case "Ofsted":
		return "Ofsted / Reg 44 feedback"
	case "Parent":
		return "Parent / Carer feedback"
	case "Professional":
		return "Professional feedback"
	default:
		return "External feedback"
	}
}

func canonicalCareDomain(raw string) (string, string) {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.NewReplacer("-", "_", " ", "_", "/", "_").Replace(value)
	switch {
	case strings.Contains(value, "safety"), strings.Contains(value, "risk"), strings.Contains(value, "stability"), strings.Contains(value, "safeguard"):
		return "safety_stability", "Safety & Stability"
	case strings.Contains(value, "emotion"), strings.Contains(value, "behaviour"), strings.Contains(value, "coping"), strings.Contains(value, "psych"):
		return "emotional_wellbeing", "Emotional Wellbeing"
	case strings.Contains(value, "health"), strings.Contains(value, "physical"):
		return "health", "Health"
	case strings.Contains(value, "education"), strings.Contains(value, "learning"), strings.Contains(value, "future"):
		return "education_future", "Education & Future"
	case strings.Contains(value, "relationship"), strings.Contains(value, "belong"):
		return "relationships_belonging", "Relationships & Belonging"
	case strings.Contains(value, "independence"), strings.Contains(value, "daily_living"), strings.Contains(value, "self_care"):
		return "independence_daily_living", "Independence & Daily Living"
	case strings.Contains(value, "identity"), strings.Contains(value, "culture"):
		return "identity_culture", "Identity & Culture"
	default:
		if value == "" {
			return "other", "Other current work"
		}
		return value, titleCase(strings.ReplaceAll(value, "_", " "))
	}
}

func recordedRiskRating(risk models.ChildRisk) *int {
	if risk.ResidualLikelihood == nil || risk.ResidualSeverity == nil {
		return nil
	}
	rating := *risk.ResidualLikelihood * *risk.ResidualSeverity
	return &rating
}

func recordedRiskBand(rating int) string {
	switch {
	case rating <= 4:
		return "Low"
	case rating <= 9:
		return "Medium"
	default:
		return "High"
	}
}

func dayStart(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func titleCase(value string) string {
	words := strings.Fields(value)
	for i := range words {
		if words[i] != "" {
			words[i] = strings.ToUpper(words[i][:1]) + words[i][1:]
		}
	}
	return strings.Join(words, " ")
}

func timelineEvent(occurredAt time.Time, eventType, title, detail, source string) models.CareOverviewTimelineEvent {
	return models.CareOverviewTimelineEvent{
		OccurredAt: occurredAt, Type: eventType, Title: title, Detail: detail, Source: source,
	}
}
