package security

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

type auditTestStringer struct{ value string }

func (s auditTestStringer) String() string { return s.value }

func TestAuditMetadataTypedFields(t *testing.T) {
	resourceID, riskReviewID, carePlanID, sectionID, carePlanReviewID, scoreCount, signed := int64(12), int64(44), int64(55), int64(66), int64(77), 3, true
	metadata, err := NewAuditMetadata(AuditMetadataFields{
		ResourceID: &resourceID, RiskReviewID: &riskReviewID, CarePlanID: &carePlanID,
		CarePlanSectionID: &sectionID, CarePlanReviewID: &carePlanReviewID,
		SessionType: SessionTypeKeywork, ManagerReviewDecision: ManagerReviewApproved,
		ScoreCount: &scoreCount, Signed: &signed,
	})
	if err != nil {
		t.Fatalf("NewAuditMetadata() error = %v", err)
	}
	encoded := metadata.JSON()
	if !strings.Contains(string(encoded), `"session_type":"keywork"`) {
		t.Fatalf("AuditMetadata.JSON() = %s", encoded)
	}
	if !strings.Contains(string(encoded), `"risk_review_id":44`) {
		t.Fatalf("AuditMetadata.JSON() = %s", encoded)
	}
	if !strings.Contains(string(encoded), `"care_plan_id":55`) || !strings.Contains(string(encoded), `"care_plan_section_id":66`) {
		t.Fatalf("AuditMetadata.JSON() = %s", encoded)
	}
	if !strings.Contains(string(encoded), `"care_plan_review_id":77`) {
		t.Fatalf("AuditMetadata.JSON() = %s", encoded)
	}
	if !strings.Contains(string(encoded), `"manager_review_decision":"approved"`) {
		t.Fatalf("AuditMetadata.JSON() = %s", encoded)
	}

	encoded[0] = '['
	if got := string(metadata.JSON()); !strings.HasPrefix(got, "{") {
		t.Fatalf("AuditMetadata.JSON() was mutated: %s", got)
	}
	resourceID = 99
	if strings.Contains(string(metadata.JSON()), "99") {
		t.Fatal("metadata changed after input pointer mutation")
	}
}

func TestAuditMetadataRejectsInvalidTypedValues(t *testing.T) {
	badID, badCount := int64(0), -1
	for name, fields := range map[string]AuditMetadataFields{
		"non-positive id":                 {ResourceID: &badID},
		"negative count":                  {ScoreCount: &badCount},
		"unknown session type":            {SessionType: SessionType("direct_work")},
		"unknown manager review decision": {ManagerReviewDecision: ManagerReviewDecision("rewritten")},
		"unknown role":                    {Role: Role("owner")},
		"unknown status":                  {PreviousStatus: AuditStatus("the_child_reported_feeling_unsafe")},
		"arbitrary request id":            {RequestID: strings.Repeat("x", 128)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewAuditMetadata(fields); err == nil {
				t.Fatal("NewAuditMetadata() accepted invalid value")
			}
		})
	}
}

func TestAuditMetadataAcceptsOnlyChiGeneratedRequestIDShape(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "localhost"
	}
	valid := []string{hostname + "/A1b2C3d4E5-000001", hostname + "/0123456789-999999999999"}
	for _, requestID := range valid {
		if _, err := NewAuditMetadata(AuditMetadataFields{RequestID: requestID}); err != nil {
			t.Fatalf("request ID %q rejected: %v", requestID, err)
		}
	}
	invalid := []string{
		"", // empty means omitted and remains valid
		"client supplied narrative", hostname + "/random-000001?secret=value", hostname + "/short-000001",
		hostname + "/ABCDEFGHIJ-12345", hostname + "/ABCDEFGHIJ-123456789012345678901",
		"attacker-host/ABCDEFGHIJ-123456",
	}
	for _, requestID := range invalid[1:] {
		if _, err := NewAuditMetadata(AuditMetadataFields{RequestID: requestID}); err == nil {
			t.Fatalf("request ID %q accepted", requestID)
		}
	}
}

func TestAuditMetadataExactOperationalAllowlists(t *testing.T) {
	for _, role := range []Role{RoleKeyWorker, RoleSenior, RoleManager, RoleResponsibleIndividual, RoleAdmin} {
		if _, err := NewAuditMetadata(AuditMetadataFields{Role: role}); err != nil {
			t.Fatalf("role %q rejected: %v", role, err)
		}
	}
	for _, status := range []AuditStatus{AuditStatusActive, AuditStatusInactive, AuditStatusPaused, AuditStatusAchieved, AuditStatusClosed, AuditStatusDraft, AuditStatusSubmitted, AuditStatusApproved, AuditStatusArchived} {
		if _, err := NewAuditMetadata(AuditMetadataFields{PreviousStatus: status}); err != nil {
			t.Fatalf("status %q rejected: %v", status, err)
		}
	}
}

func TestAuditMetadataCannotRepresentArbitraryOrNarrativeValues(t *testing.T) {
	type namedBytes []byte
	forbiddenKinds := map[reflect.Kind]bool{
		reflect.Interface: true, reflect.Slice: true, reflect.Array: true,
		reflect.Map: true, reflect.Struct: true,
	}
	typ := reflect.TypeOf(AuditMetadataFields{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		kind := field.Type.Kind()
		if forbiddenKinds[kind] {
			t.Fatalf("AuditMetadataFields.%s exposes forbidden kind %s", field.Name, kind)
		}
		if kind == reflect.Pointer && forbiddenKinds[field.Type.Elem().Kind()] {
			t.Fatalf("AuditMetadataFields.%s points to forbidden kind %s", field.Name, field.Type.Elem().Kind())
		}
	}

	// These values have no assignable field in AuditMetadataFields. The type
	// assertions document the specifically prohibited shapes without creating a
	// deliberately uncompilable test.
	for _, value := range []any{
		json.RawMessage(`{"notes":"sensitive"}`), []byte("sensitive"), namedBytes("sensitive"),
		[]int{1, 2}, [2]int{1, 2}, map[string]any{"notes": "sensitive"}, struct{ Notes string }{"sensitive"},
		auditTestStringer{"sensitive"},
	} {
		valueType := reflect.TypeOf(value)
		if valueType.Kind() != reflect.Slice && valueType.Kind() != reflect.Array && valueType.Kind() != reflect.Map && valueType.Kind() != reflect.Struct {
			t.Fatalf("test value %T does not exercise a prohibited shape", value)
		}
	}

	for _, forbiddenName := range []string{"Notes", "Narrative", "ChildVoice", "Reflection", "RequestBody", "ResponseBody"} {
		if _, ok := typ.FieldByName(forbiddenName); ok {
			t.Fatalf("narrative field %s is representable", forbiddenName)
		}
	}
}
