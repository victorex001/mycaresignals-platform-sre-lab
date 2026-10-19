package models

import "testing"

func TestValidMonthlyVoiceRatingsRequiresAllCanonicalDomainsAndPreservesZero(t *testing.T) {
	ratings := make(map[string]int, len(CanonicalMonthlyVoiceDomains))
	for index, domain := range CanonicalMonthlyVoiceDomains {
		ratings[domain] = index % 5
	}
	if !ValidMonthlyVoiceRatings(ratings) {
		t.Fatal("complete canonical ratings rejected")
	}
	delete(ratings, CanonicalMonthlyVoiceDomains[0])
	if ValidMonthlyVoiceRatings(ratings) {
		t.Fatal("incomplete ratings accepted")
	}
}
