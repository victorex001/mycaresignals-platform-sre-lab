package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"qr-feedback-api/internal/models"
	"qr-feedback-api/internal/reviewrules"
	"qr-feedback-api/internal/storage"

	"github.com/gin-gonic/gin"
)

type youngVoiceStore interface {
	ResolveIntakeContext(context.Context, string, string, time.Time) (*models.IntakeContext, error)
	InsertYoungVoice(context.Context, *models.YoungVoice) error
}

type youngVoiceInput struct {
	ContextToken      string         `json:"context_token"`
	Home              string         `json:"home"`
	YoungPersonAlias  string         `json:"young_person_alias"`
	PeriodMonth       string         `json:"period_month"`
	DomainRatings     map[string]int `json:"domain_ratings"`
	Q1Overall         string         `json:"q1_overall"`
	Q1Best            string         `json:"q1_best"`
	Q1Hardest         string         `json:"q1_hardest"`
	Q2Safe            string         `json:"q2_safe"`
	Q2Unsafe          string         `json:"q2_unsafe"`
	Q2SupportWho      string         `json:"q2_support_who"`
	Q3Listened        string         `json:"q3_listened"`
	Q3TrustedAdult    string         `json:"q3_trusted_adult"`
	Q3ChangeSupport   string         `json:"q3_change_support"`
	Q4Peers           string         `json:"q4_peers"`
	Q4Unkind          string         `json:"q4_unkind"`
	Q4HelpWithPeers   string         `json:"q4_help_with_peers"`
	Q5Enjoy           string         `json:"q5_enjoy"`
	Q5WishMore        string         `json:"q5_wish_more"`
	Q5HouseChange     string         `json:"q5_house_change"`
	Q6Education       string         `json:"q6_education"`
	Q6EducationHelp   string         `json:"q6_education_help"`
	Q7ContactFeelings string         `json:"q7_contact_feelings"`
	Q7ContactChange   string         `json:"q7_contact_change"`
	Q8SayInDecisions  string         `json:"q8_say_in_decisions"`
	Q8ChangeExample   string         `json:"q8_change_example"`
	Q9Worries         string         `json:"q9_worries"`
	Q9OneChange       string         `json:"q9_one_change"`
	Q9WantTalk        bool           `json:"q9_want_talk"`
	Q9FollowUpPref    string         `json:"q9_follow_up_preference"`
	Q10WhatTellOfsted string         `json:"q10_what_tell_ofsted"`
	Q10NeverForget    string         `json:"q10_never_forget"`
}

func SubmitYoungVoice(repo youngVoiceStore, notifier reviewNotifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in youngVoiceInput
		rawSubmission, err := decodeIntakeBody(c.Request, &in)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		if !models.ValidMonthlyVoiceRatings(in.DomainRatings) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "all monthly domain ratings must be between 0 and 4"})
			return
		}
		resolved, err := repo.ResolveIntakeContext(c.Request.Context(), in.ContextToken, models.IntakePurposeMonthlyVoice, time.Now().UTC())
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, storage.ErrInvalidIntakeContext) {
				status = http.StatusNotFound
			}
			c.JSON(status, gin.H{"error": "invalid or expired feedback link"})
			return
		}
		if resolved.ChildExternalID == nil || resolved.PeriodMonth == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "invalid or expired feedback link"})
			return
		}
		allText := strings.Join([]string{
			in.Q1Overall, in.Q1Best, in.Q1Hardest, in.Q2Safe, in.Q2Unsafe, in.Q2SupportWho,
			in.Q3Listened, in.Q3TrustedAdult, in.Q3ChangeSupport, in.Q4Peers, in.Q4Unkind,
			in.Q4HelpWithPeers, in.Q5Enjoy, in.Q5WishMore, in.Q5HouseChange, in.Q6Education,
			in.Q6EducationHelp, in.Q7ContactFeelings, in.Q7ContactChange, in.Q8SayInDecisions,
			in.Q8ChangeExample, in.Q9Worries, in.Q9OneChange, in.Q10WhatTellOfsted, in.Q10NeverForget,
		}, "\n")
		metadata := reviewrules.Evaluate(allText, "", in.Q9WantTalk)
		record := &models.YoungVoice{
			Home: resolved.HomeLabel, HomeExternalID: resolved.HomeExternalID,
			ChildExternalID: resolved.ChildExternalID, IntakeContextID: resolved.ID,
			PeriodMonth: *resolved.PeriodMonth, DomainRatings: copyRatings(in.DomainRatings),
			Q1Overall: strings.TrimSpace(in.Q1Overall), Q1Best: strings.TrimSpace(in.Q1Best), Q1Hardest: strings.TrimSpace(in.Q1Hardest),
			Q2Safe: strings.TrimSpace(in.Q2Safe), Q2Unsafe: strings.TrimSpace(in.Q2Unsafe), Q2SupportWho: strings.TrimSpace(in.Q2SupportWho),
			Q3Listened: strings.TrimSpace(in.Q3Listened), Q3TrustedAdult: strings.TrimSpace(in.Q3TrustedAdult), Q3ChangeSupport: strings.TrimSpace(in.Q3ChangeSupport),
			Q4Peers: strings.TrimSpace(in.Q4Peers), Q4Unkind: strings.TrimSpace(in.Q4Unkind), Q4HelpWithPeers: strings.TrimSpace(in.Q4HelpWithPeers),
			Q5Enjoy: strings.TrimSpace(in.Q5Enjoy), Q5WishMore: strings.TrimSpace(in.Q5WishMore), Q5HouseChange: strings.TrimSpace(in.Q5HouseChange),
			Q6Education: strings.TrimSpace(in.Q6Education), Q6EducationHelp: strings.TrimSpace(in.Q6EducationHelp),
			Q7ContactFeelings: strings.TrimSpace(in.Q7ContactFeelings), Q7ContactChange: strings.TrimSpace(in.Q7ContactChange),
			Q8SayInDecisions: strings.TrimSpace(in.Q8SayInDecisions), Q8ChangeExample: strings.TrimSpace(in.Q8ChangeExample),
			Q9Worries: strings.TrimSpace(in.Q9Worries), Q9OneChange: strings.TrimSpace(in.Q9OneChange), Q9WantTalk: in.Q9WantTalk, Q9FollowUpPref: strings.TrimSpace(in.Q9FollowUpPref),
			FollowUpRequested: in.Q9WantTalk, FollowUpPreference: strings.TrimSpace(in.Q9FollowUpPref),
			Q10WhatTellOfsted: strings.TrimSpace(in.Q10WhatTellOfsted), Q10NeverForget: strings.TrimSpace(in.Q10NeverForget),
			RawSubmission: rawSubmission, ReviewMetadata: metadata, TriageStatus: "New",
		}
		if err := repo.InsertYoungVoice(c.Request.Context(), record); err != nil {
			if errors.Is(err, storage.ErrMonthlyVoiceConflict) {
				c.JSON(http.StatusConflict, gin.H{"error": "this monthly feedback link has already been used"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save young person feedback"})
			return
		}
		if metadata.RequiresHumanReview && notifier != nil {
			go func() {
				_ = notifier.SendReviewRequired(record.ID, models.IntakeTypeYoungPersonVoice, metadata.ReviewPriority)
			}()
		}
		c.JSON(http.StatusAccepted, gin.H{"receipt_id": record.ID})
	}
}

func copyRatings(input map[string]int) map[string]int {
	result := make(map[string]int, len(input))
	for domain, rating := range input {
		result[domain] = rating
	}
	return result
}
