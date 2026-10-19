// internal/handlers/admin.go
package handlers

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"qr-feedback-api/internal/models"
	"qr-feedback-api/internal/storage"

	"github.com/gin-gonic/gin"
)

// AdminHandler wraps the Repo for admin-only endpoints.
type AdminHandler struct {
	repo *storage.Repo
}

type monthlyVoiceContextRequest struct {
	HomeID      int64  `json:"home_id"`
	ChildID     int64  `json:"child_id"`
	PeriodMonth string `json:"period_month"`
	ExpiresAt   string `json:"expires_at"`
}

func (h *AdminHandler) CreateMonthlyVoiceContext(c *gin.Context) {
	var request monthlyVoiceContextRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.HomeID <= 0 || request.ChildID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid monthly voice context"})
		return
	}
	period, err := time.Parse("2006-01", strings.TrimSpace(request.PeriodMonth))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period_month must be YYYY-MM"})
		return
	}
	expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(request.ExpiresAt))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expires_at must be RFC3339"})
		return
	}
	created, err := h.repo.CreateMonthlyVoiceContext(c.Request.Context(), models.MonthlyVoiceContextInput{
		HomeExternalID: request.HomeID, ChildExternalID: request.ChildID,
		PeriodMonth: period, ExpiresAt: expiresAt,
	})
	if err != nil {
		if errors.Is(err, storage.ErrMonthlyContextExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "monthly voice context already exists"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not create monthly voice context"})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func NewAdminHandler(repo *storage.Repo) *AdminHandler {
	return &AdminHandler{repo: repo}
}

// helper: parse "YYYY-MM-DD" into *time.Time
func parseQueryDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListFeedback – admin JSON list, supports optional ?from=YYYY-MM-DD&to=YYYY-MM-DD
// now uses the richer ListFeedbackAdmin (includes feedback_type, answers, risk_level)
func (h *AdminHandler) ListFeedback(c *gin.Context) {
	filters, err := feedbackFilters(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rows, err := h.repo.ListFeedbackAdmin(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list feedback"})
		return
	}

	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, feedbackAdminView(row, false))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items), "summary": inboxSummary(rows)})
}

// ExportFeedbackCSV – full CSV export for a date range (or all), for Excel / offline analysis
func (h *AdminHandler) ExportFeedbackCSV(c *gin.Context) {
	filters, err := feedbackFilters(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rows, err := h.repo.ListFeedbackAdmin(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list feedback"})
		return
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=feedback.csv")

	w := csv.NewWriter(c.Writer)
	defer w.Flush()
	_ = w.Write([]string{"ReceivedAtUTC", "SourceType", "Rating", "Recommended", "ReviewStatus", "FollowUpStatus", "ReviewedAtUTC", "ReviewPrompt"})

	for _, r := range rows {
		rating := ""
		if r.Rating != nil {
			rating = strconv.Itoa(*r.Rating)
		}

		reviewedAt := ""
		if r.ReviewedAt != nil {
			reviewedAt = r.ReviewedAt.Format(time.RFC3339)
		}
		_ = w.Write([]string{r.SubmittedAt.Format(time.RFC3339), spreadsheetSafe(r.Who), rating,
			strconv.FormatBool(r.Recommend), spreadsheetSafe(r.CurrentStatus), spreadsheetSafe(r.FollowUpStatus),
			reviewedAt, spreadsheetSafe(reviewPromptLabel(r))})
	}
}

// ExportPositiveCSV – only “good” feedback (rating >= 4, no safeguarding flag)
func (h *AdminHandler) ExportPositiveCSV(c *gin.Context) {
	filters, err := feedbackFilters(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rows, err := h.repo.ListFeedbackAdmin(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list feedback"})
		return
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=positive_feedback.csv")

	w := csv.NewWriter(c.Writer)
	defer w.Flush()
	_ = w.Write([]string{"FeedbackID", "SubmittedAtUTC", "From", "Source", "Rating"})

	for _, r := range rows {
		if r.Rating == nil || *r.Rating < 4 || r.Safeguarding || r.RequiresHumanReview {
			continue
		}
		rating := strconv.Itoa(*r.Rating)

		_ = w.Write([]string{r.ID, r.SubmittedAt.Format(time.RFC3339), spreadsheetSafe(r.Who), spreadsheetSafe(r.Source), rating})
	}
}

func (h *AdminHandler) GetFeedback(c *gin.Context) {
	row, err := h.repo.GetFeedbackAdmin(c.Request.Context(), c.Param("feedbackID"))
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "feedback not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load feedback"})
		return
	}
	c.JSON(http.StatusOK, feedbackAdminView(*row, true))
}

func (h *AdminHandler) AddFeedbackReview(c *gin.Context) {
	feedbackID := c.Param("feedbackID")
	if _, err := h.repo.GetFeedbackAdmin(c.Request.Context(), feedbackID); errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "feedback not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load feedback"})
		return
	}
	var input models.FeedbackReviewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid review"})
		return
	}
	input.Outcome = strings.TrimSpace(input.Outcome)
	input.ManagerNote = strings.TrimSpace(input.ManagerNote)
	if !models.ValidFeedbackOutcome(input.Outcome) || len(input.ManagerNote) < 3 || len(input.ManagerNote) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "select an outcome and add a manager note"})
		return
	}
	review, err := h.repo.AddFeedbackReview(c.Request.Context(), feedbackID, "Authenticated admin token", input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not record review"})
		return
	}
	c.JSON(http.StatusCreated, review)
}

func (h *AdminHandler) MonthlyFeedbackReport(c *gin.Context) {
	month := strings.TrimSpace(c.Query("month"))
	from, err := time.Parse("2006-01", month)
	if err != nil || from.Format("2006-01") != month {
		c.JSON(http.StatusBadRequest, gin.H{"error": "month must be YYYY-MM"})
		return
	}
	to := from.AddDate(0, 1, -1)
	rows, err := h.repo.ListFeedbackAdmin(c.Request.Context(), models.FeedbackFilters{From: &from, To: &to})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not build monthly report"})
		return
	}
	c.JSON(http.StatusOK, monthlyReport(month, rows))
}

func feedbackFilters(c *gin.Context) (models.FeedbackFilters, error) {
	filters := models.FeedbackFilters{Source: strings.TrimSpace(c.Query("source")), Status: strings.TrimSpace(c.Query("status"))}
	validSources := map[string]bool{"Parent": true, "SocialWorker": true, "Professional": true, "Ofsted": true, "Other": true}
	if filters.Source != "" && !validSources[filters.Source] {
		return filters, errors.New("invalid source type")
	}
	validStatuses := map[string]bool{models.FeedbackStatusNew: true, models.FeedbackOutcomeAcknowledged: true, models.FeedbackOutcomeNoAction: true, models.FeedbackOutcomeFollowUp: true, models.FeedbackOutcomeClosed: true}
	if filters.Status != "" && !validStatuses[filters.Status] {
		return filters, errors.New("invalid review status")
	}
	if month := strings.TrimSpace(c.Query("month")); month != "" {
		from, err := time.Parse("2006-01", month)
		if err != nil || from.Format("2006-01") != month {
			return filters, errors.New("month must be YYYY-MM")
		}
		to := from.AddDate(0, 1, -1)
		filters.From, filters.To = &from, &to
	} else {
		var err error
		if filters.From, err = parseQueryDate(c.Query("from")); err != nil {
			return filters, errors.New("invalid from date, use YYYY-MM-DD")
		}
		if filters.To, err = parseQueryDate(c.Query("to")); err != nil {
			return filters, errors.New("invalid to date, use YYYY-MM-DD")
		}
	}
	if value := strings.TrimSpace(c.Query("requires_review")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return filters, errors.New("requires_review must be true or false")
		}
		filters.RequiresReview = &parsed
	}
	return filters, nil
}

func feedbackAdminView(row models.FeedbackRow, detail bool) gin.H {
	view := gin.H{"id": row.ID, "received_at": row.SubmittedAt, "source_type": row.Who,
		"rating": row.Rating, "recommend": row.Recommend, "current_status": row.CurrentStatus,
		"follow_up_status": row.FollowUpStatus, "reviewed_at": row.ReviewedAt,
		"requires_human_review": row.RequiresHumanReview, "review_priority": row.ReviewPriority}
	if detail {
		view["reason"] = row.Reason
		view["what_went_well"] = row.WhatWentWell
		view["what_to_improve"] = row.WhatToImprove
		view["answers"] = row.Answers
		view["matched_rule_ids"] = row.MatchedRuleIDs
		view["rule_version"] = row.RuleVersion
		view["reviews"] = row.Reviews
	}
	return view
}

func inboxSummary(rows []models.FeedbackRow) gin.H {
	summary := gin.H{"responses": len(rows), "needs_review": 0, "follow_up": 0, "reviewed": 0}
	for _, row := range rows {
		if row.CurrentStatus == models.FeedbackStatusNew {
			summary["needs_review"] = summary["needs_review"].(int) + 1
		}
		if row.CurrentStatus == models.FeedbackOutcomeFollowUp {
			summary["follow_up"] = summary["follow_up"].(int) + 1
		}
		if row.CurrentStatus != models.FeedbackStatusNew && row.CurrentStatus != models.FeedbackOutcomeFollowUp {
			summary["reviewed"] = summary["reviewed"].(int) + 1
		}
	}
	return summary
}

func reviewPromptLabel(row models.FeedbackRow) string {
	if !row.RequiresHumanReview {
		return ""
	}
	if row.ReviewPriority == "urgent" {
		return "Urgent review"
	}
	if row.ReviewPriority == "priority" {
		return "Priority review"
	}
	return "Review suggested"
}

func monthlyReport(month string, rows []models.FeedbackRow) gin.H {
	bySource := map[string]int{}
	promptStatus := map[string]int{}
	well := make([]gin.H, 0)
	improve := make([]gin.H, 0)
	ratingTotal, ratingCount, recommendations, prompts := 0, 0, 0, 0
	followUpRequired, openFollowUp, closedFollowUp := 0, 0, 0
	for _, row := range rows {
		bySource[row.Who]++
		if row.Rating != nil {
			ratingTotal += *row.Rating
			ratingCount++
		}
		if row.Recommend {
			recommendations++
		}
		if row.RequiresHumanReview {
			prompts++
			promptStatus[row.CurrentStatus]++
		}
		if row.HadFollowUp {
			followUpRequired++
		}
		if row.HadFollowUp && row.FollowUpStatus == "open" {
			openFollowUp++
		}
		if row.HadFollowUp && row.FollowUpStatus == "closed" {
			closedFollowUp++
		}
		if row.CurrentStatus != models.FeedbackStatusNew && strings.TrimSpace(row.WhatWentWell) != "" && (row.Rating == nil || *row.Rating >= 4) {
			well = append(well, gin.H{"received_at": row.SubmittedAt, "source_type": row.Who, "rating": row.Rating, "text": row.WhatWentWell})
		}
		if strings.TrimSpace(pointerString(row.WhatToImprove)) != "" {
			improve = append(improve, gin.H{"received_at": row.SubmittedAt, "source_type": row.Who, "text": pointerString(row.WhatToImprove)})
		}
	}
	var average *float64
	if ratingCount > 0 {
		value := float64(ratingTotal) / float64(ratingCount)
		average = &value
	}
	return gin.H{"month": month, "total_responses": len(rows), "by_source": bySource, "average_rating": average,
		"recommendation_count": recommendations, "went_well": well, "areas_for_improvement": improve,
		"follow_up":      gin.H{"required": followUpRequired, "open": openFollowUp, "closed": closedFollowUp},
		"review_prompts": gin.H{"total": prompts, "by_status": promptStatus}}
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// Summary – lightweight stats for Reg 44 / Ofsted dashboard
// GET /api/v1/admin/feedback/summary?from=YYYY-MM-DD&to=YYYY-MM-DD&home=Millbrow%20House
func (h *AdminHandler) Summary(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	home := c.Query("home") // full home name, e.g. "Millbrow House"

	from, err := parseQueryDate(fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from date, use YYYY-MM-DD"})
		return
	}
	to, err := parseQueryDate(toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to date, use YYYY-MM-DD"})
		return
	}

	summary, err := h.repo.FeedbackSummary(
		c.Request.Context(),
		home,
		from,
		to,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not build summary"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":      summary.Total,
		"avg_rating": summary.AvgRating,
		"by_who":     summary.ByWho,
		"review_prompts": gin.H{
			"total":    summary.ReviewPrompts.Total,
			"urgent":   summary.ReviewPrompts.Urgent,
			"priority": summary.ReviewPrompts.Priority,
			"routine":  summary.ReviewPrompts.Routine,
		},
	})
}

// ListYoungVoice returns young people's monthly voice entries for a home/month.
func (h *AdminHandler) ListYoungVoice(c *gin.Context) {
	home := c.Query("home")                // "Millbrow House"
	periodMonth := c.Query("period_month") // "2025-11" (optional)

	rows, err := h.repo.ListYoungVoiceAdmin(
		c.Request.Context(),
		home,
		periodMonth,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not load young people feedback",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": rows,
	})
}

func (h *AdminHandler) ExportYoungVoiceCSV(c *gin.Context) {
	home := c.Query("home")
	periodMonth := c.Query("period_month")

	rows, err := h.repo.ListYoungVoiceAdmin(
		c.Request.Context(),
		home,
		periodMonth,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not export young people feedback",
		})
		return
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=young_voice_feedback.csv")

	w := csv.NewWriter(c.Writer)
	defer w.Flush()
	_ = w.Write([]string{"ID", "Home", "Period", "SubmittedAt", "Alias", "Overall", "Safe", "Unsafe", "Peers", "Unkind", "Worries", "WantsFollowUp", "FollowUpPreference", "RequiresHumanReview", "ReviewPriority", "RuleVersion"})

	for _, r := range rows {
		_ = w.Write([]string{
			r.ID, spreadsheetSafe(r.Home), r.PeriodMonth, r.SubmittedAt.Format(time.RFC3339),
			spreadsheetSafe(r.YoungPersonAlias), spreadsheetSafe(r.Q1Overall), spreadsheetSafe(r.Q2Safe),
			spreadsheetSafe(r.Q2Unsafe), spreadsheetSafe(r.Q4Peers), spreadsheetSafe(r.Q4Unkind),
			spreadsheetSafe(r.Q9Worries), strconv.FormatBool(r.Q9WantTalk), spreadsheetSafe(r.Q9FollowUpPreference),
			strconv.FormatBool(r.RequiresHumanReview), r.ReviewPriority, r.RuleVersion,
		})
	}
}

func spreadsheetSafe(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}
