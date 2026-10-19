package reviewrules

import (
	"reflect"
	"testing"
)

func TestEvaluateIsDeterministicAndNonClinical(t *testing.T) {
	first := Evaluate("I was bullied and want to talk", "no", true)
	second := Evaluate("I was bullied and want to talk", "no", true)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("rule output changed between identical evaluations: %#v %#v", first, second)
	}
	if first.ReviewPriority != "priority" || !first.RequiresHumanReview {
		t.Fatalf("unexpected review metadata: %#v", first)
	}
	for _, value := range []string{first.ReviewPriority, first.RuleVersion} {
		if value == "LOW" || value == "MEDIUM" || value == "HIGH" {
			t.Fatalf("clinical-looking legacy label returned: %q", value)
		}
	}
}

func TestEvaluateUsesWordBoundaries(t *testing.T) {
	result := Evaluate("The room was highlighted in white.", "yes", false)
	if result.RequiresHumanReview {
		t.Fatalf("embedded substring created a review prompt: %#v", result)
	}
}

func TestEvaluateUrgentLanguageCreatesReviewPromptOnly(t *testing.T) {
	result := Evaluate("synthetic disclosure: kill myself", "", false)
	if !result.RequiresHumanReview || result.ReviewPriority != "urgent" {
		t.Fatalf("unexpected urgent rule result: %#v", result)
	}
	if !reflect.DeepEqual(result.MatchedRuleIDs, []string{RuleSafetyLanguageUrgent}) {
		t.Fatalf("matched rules = %#v", result.MatchedRuleIDs)
	}
}
