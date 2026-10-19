package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"qr-feedback-api/internal/models"
	"qr-feedback-api/internal/reviewrules"
	"qr-feedback-api/internal/storage"

	"github.com/gin-gonic/gin"
)

type feedbackStore interface {
	ResolveIntakeContext(context.Context, string, string, time.Time) (*models.IntakeContext, error)
	HashIP(string) string
	InsertFeedback(context.Context, models.FeedbackIn, *models.IntakeContext, models.ReviewMetadata, string, string, []byte, []byte) (string, time.Time, error)
}

type homeFeedbackNotifier interface {
	SendHomeFeedbackReceived(receiptID, home, submitterType, reviewPriority string, receivedAt time.Time) error
}

type reviewNotifier interface {
	SendReviewRequired(receiptID, intakeType, reviewPriority string) error
}

type feedbackSubmission struct {
	models.FeedbackIn
	ContextToken string `json:"context_token"`
}

func buildAnswersJSON(in models.FeedbackIn) ([]byte, error) {
	answers := map[string]any{
		"house_rating": in.HouseRating, "staff_rating": in.StaffRating,
		"felt_safe": in.FeltSafe, "open_comments": in.OpenComments,
		"recommend": in.Recommend, "contact_wanted": in.ContactWanted,
		"who": in.Who,
	}
	if in.Rating != nil {
		answers["overall_rating"] = *in.Rating
	}
	return json.Marshal(answers)
}

func validateFeedback(in models.FeedbackIn) bool {
	validWho := map[string]bool{
		"SocialWorker": true, "Ofsted": true, "Parent": true,
		"Professional": true, "Other": true,
	}
	if !validWho[in.Who] || len(in.Reason) > 200 || len(in.WhatWentWell) < 3 || len(in.WhatWentWell) > 2000 || len(in.WhatToImprove) > 2000 || len(in.OpenComments) > 2000 {
		return false
	}
	if len(in.ContactName) > 100 || len(in.ContactEmail) > 200 {
		return false
	}
	if in.ContactEmail != "" {
		address, err := mail.ParseAddress(in.ContactEmail)
		if err != nil || address.Address != in.ContactEmail {
			return false
		}
	}
	for _, rating := range []*int{in.Rating, in.HouseRating, in.StaffRating} {
		if rating != nil && (*rating < 1 || *rating > 5) {
			return false
		}
	}
	return in.FeltSafe == "" || in.FeltSafe == "yes" || in.FeltSafe == "no" || in.FeltSafe == "unsure"
}

func SubmitFeedback(repo feedbackStore, notifier homeFeedbackNotifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		var submission feedbackSubmission
		rawSubmission, err := decodeIntakeBody(c.Request, &submission)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		in := submission.FeedbackIn
		in.Who = strings.TrimSpace(in.Who)
		in.Reason = strings.TrimSpace(in.Reason)
		in.WhatWentWell = strings.TrimSpace(in.WhatWentWell)
		in.WhatToImprove = strings.TrimSpace(in.WhatToImprove)
		in.FeltSafe = strings.ToLower(strings.TrimSpace(in.FeltSafe))
		in.OpenComments = strings.TrimSpace(in.OpenComments)
		in.ContactName = strings.TrimSpace(in.ContactName)
		in.ContactEmail = strings.TrimSpace(in.ContactEmail)
		if !validateFeedback(in) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}

		resolved, err := repo.ResolveIntakeContext(c.Request.Context(), submission.ContextToken, models.IntakeTypeVisitorFeedback, time.Now().UTC())
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, storage.ErrInvalidIntakeContext) {
				status = http.StatusNotFound
			}
			c.JSON(status, gin.H{"error": "invalid or expired feedback link"})
			return
		}
		if resolved.ChildExternalID != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "invalid or expired feedback link"})
			return
		}

		answersJSON, err := buildAnswersJSON(in)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		text := strings.Join([]string{in.Reason, in.WhatWentWell, in.WhatToImprove, in.OpenComments}, "\n")
		metadata := reviewrules.Evaluate(text, in.FeltSafe, in.ContactWanted)
		id, receivedAt, err := repo.InsertFeedback(
			c.Request.Context(), in, resolved, metadata,
			repo.HashIP(c.ClientIP()), c.Request.UserAgent(), answersJSON, rawSubmission,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save feedback"})
			return
		}
		if notifier != nil {
			priority := ""
			if metadata.RequiresHumanReview {
				priority = metadata.ReviewPriority
			}
			go func() { _ = notifier.SendHomeFeedbackReceived(id, resolved.HomeLabel, in.Who, priority, receivedAt) }()
		}
		c.JSON(http.StatusAccepted, gin.H{"receipt_id": id})
	}
}
