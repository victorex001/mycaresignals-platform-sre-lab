package handlers

import "testing"

func intPointer(value int) *int { return &value }

func TestRiskRatingBoundaries(t *testing.T) {
	tests := []struct {
		likelihood int
		severity   int
		score      int
		band       string
	}{
		{1, 1, 1, "low"}, {2, 2, 4, "low"}, {1, 5, 5, "medium"},
		{3, 3, 9, "medium"}, {2, 5, 10, "high"}, {5, 5, 25, "high"},
	}
	for _, test := range tests {
		score, band, ok := riskScore(intPointer(test.likelihood), intPointer(test.severity))
		if !ok || score != test.score || band != test.band {
			t.Errorf("%dx%d = (%d, %q, %v), want (%d, %q, true)", test.likelihood, test.severity, score, band, ok, test.score, test.band)
		}
	}
	if _, _, ok := riskScore(nil, intPointer(2)); ok {
		t.Error("incomplete rating was calculated")
	}
}

func TestRiskRatingValidationAllowsUnknownAndRequiresCompletePairs(t *testing.T) {
	if err := validateRiskRatingPair(nil, nil, "residual"); err != nil {
		t.Fatalf("unknown pair rejected: %v", err)
	}
	if err := validateRiskRatingPair(intPointer(2), nil, "residual"); err == nil {
		t.Error("incomplete pair accepted")
	}
	for _, invalid := range []int{0, 6} {
		if err := validateRiskRatingPair(intPointer(invalid), intPointer(2), "residual"); err == nil {
			t.Errorf("rating %d accepted", invalid)
		}
	}
}

func TestSameRiskRatingRequiresExactKnownPair(t *testing.T) {
	if !sameRiskRating(intPointer(2), intPointer(4), 2, 4) {
		t.Error("matching current rating was not preserved")
	}
	if sameRiskRating(intPointer(2), intPointer(4), 3, 4) {
		t.Error("changed rating treated as unchanged")
	}
	if sameRiskRating(nil, nil, 2, 4) {
		t.Error("unknown current rating treated as matching a reviewed rating")
	}
}
