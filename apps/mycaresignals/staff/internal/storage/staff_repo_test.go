package storage

import (
	"encoding/json"
	"testing"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
)

func TestAuditInsertArgsHandlesNullableFields(t *testing.T) {
	event := &models.AuditEvent{
		EventType:  "child.goal.created",
		EntityType: "child_goal",
		Metadata:   json.RawMessage(`{}`),
	}
	args := auditInsertArgs(event)
	if len(args) != 11 {
		t.Fatalf("auditInsertArgs() len = %d, want 11", len(args))
	}
	for _, index := range []int{0, 1, 2, 3, 4, 7, 8, 9} {
		if args[index] != nil {
			t.Fatalf("auditInsertArgs()[%d] = %#v, want nil", index, args[index])
		}
	}
	if string(args[10].(json.RawMessage)) != "{}" {
		t.Fatalf("auditInsertArgs()[10] = %s, want {}", args[10])
	}
}

func TestDailyProgressAuditAllowsAuthoritativeTargetRelationship(t *testing.T) {
	targetID := int64(4)
	bound, err := newBoundAuditEvent(resourceAuditScope{
		ChildAuditScope: ChildAuditScope{homeID: 1, childID: 1, staffUserID: 7},
		entityType:      "daily_progress",
		entityID:        20,
	}, AuditEventDailyProgressCreate, security.AuditMetadataFields{TargetID: &targetID}, AuditRouteDailyProgress)
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(bound.metadata.JSON(), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["resource_id"] != float64(20) || metadata["target_id"] != float64(4) {
		t.Fatalf("daily progress audit metadata = %#v", metadata)
	}
}
