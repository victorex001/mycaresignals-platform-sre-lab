package reviewrules

import (
	"regexp"
	"strings"

	"qr-feedback-api/internal/models"
)

const Version = "intake-review-v1"

const (
	RuleSafetyLanguageUrgent = "safety_language_urgent"
	RuleSafetyLanguageReview = "safety_language_review"
	RuleFeltUnsafe           = "felt_unsafe"
	RuleFollowUpRequested    = "follow_up_requested"
)

var (
	urgentLanguage = regexp.MustCompile(`(?i)\b(kill myself|suicide|sexual abuse|sexually abused|rape|raped|self[- ]harm|cut myself|cutting)\b`)
	reviewLanguage = regexp.MustCompile(`(?i)\b(abuse|abused|bully|bullied|bullying|threat|threatened|scared|unsafe|afraid|violence|violent|hurt|hitting|hit me)\b`)
)

func Evaluate(text, feltSafe string, followUpRequested bool) models.ReviewMetadata {
	matched := make([]string, 0, 3)
	priority := "routine"

	if urgentLanguage.MatchString(text) {
		matched = append(matched, RuleSafetyLanguageUrgent)
		priority = "urgent"
	}
	if reviewLanguage.MatchString(text) {
		matched = append(matched, RuleSafetyLanguageReview)
		if priority != "urgent" {
			priority = "priority"
		}
	}
	if strings.EqualFold(strings.TrimSpace(feltSafe), "no") {
		matched = append(matched, RuleFeltUnsafe)
		if priority != "urgent" {
			priority = "priority"
		}
	}
	if followUpRequested {
		matched = append(matched, RuleFollowUpRequested)
		if priority == "routine" {
			priority = "priority"
		}
	}

	return models.ReviewMetadata{
		RequiresHumanReview: len(matched) > 0,
		ReviewPriority:      priority,
		MatchedRuleIDs:      matched,
		RuleVersion:         Version,
	}
}
