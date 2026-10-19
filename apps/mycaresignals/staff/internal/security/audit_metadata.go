package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

var ErrAuditMetadataValue = errors.New("audit metadata field is invalid")

var auditRequestIDSuffixPattern = regexp.MustCompile(`^[A-Za-z0-9]{10}-[0-9]{6,20}$`)

var serverRequestIDCounter atomic.Uint64

type SessionType string

const SessionTypeKeywork SessionType = "keywork"

type ManagerReviewDecision string

type FeedbackTriageOutcome string

const (
	ManagerReviewApproved          ManagerReviewDecision = "approved"
	ManagerReviewReturnedForReview ManagerReviewDecision = "returned_for_review"
)

const (
	FeedbackTriageAcknowledge           FeedbackTriageOutcome = "acknowledge"
	FeedbackTriageNoFurtherAction       FeedbackTriageOutcome = "no_further_action"
	FeedbackTriageFollowUpRequired      FeedbackTriageOutcome = "follow_up_required"
	FeedbackTriageLinkAsEvidence        FeedbackTriageOutcome = "link_as_evidence"
	FeedbackTriageEscalateForRiskReview FeedbackTriageOutcome = "escalate_for_risk_review"
)

type AuditStatus string

const (
	AuditStatusActive    AuditStatus = "active"
	AuditStatusInactive  AuditStatus = "inactive"
	AuditStatusPaused    AuditStatus = "paused"
	AuditStatusAchieved  AuditStatus = "achieved"
	AuditStatusClosed    AuditStatus = "closed"
	AuditStatusDraft     AuditStatus = "draft"
	AuditStatusSubmitted AuditStatus = "submitted"
	AuditStatusApproved  AuditStatus = "approved"
	AuditStatusArchived  AuditStatus = "archived"
)

// AuditMetadataFields is a closed, strongly typed set of operational audit
// attributes. Narrative text and arbitrary container values are deliberately
// not representable.
type AuditMetadataFields struct {
	ResourceID            *int64
	ParentID              *int64
	GoalID                *int64
	TargetID              *int64
	RiskID                *int64
	RiskReviewID          *int64
	CarePlanID            *int64
	CarePlanSectionID     *int64
	CarePlanReviewID      *int64
	SessionID             *int64
	BaselineID            *int64
	ReviewID              *int64
	SignalResponseID      *int64
	FeedbackReceiptID     *int64
	FeedbackReviewID      *int64
	SessionType           SessionType
	ManagerReviewDecision ManagerReviewDecision
	FeedbackTriageOutcome FeedbackTriageOutcome
	ScoreCount            *int
	DomainCount           *int
	PriorityCount         *int
	PreviousStatus        AuditStatus
	NewStatus             AuditStatus
	Approved              *bool
	Signed                *bool
	Role                  Role
	RequestID             string
}

// AuditMetadata owns an immutable JSON encoding of validated fields.
type AuditMetadata struct {
	jsonBytes []byte
}

func NewAuditMetadata(fields AuditMetadataFields) (AuditMetadata, error) {
	if err := validateAuditMetadataFields(fields); err != nil {
		return AuditMetadata{}, err
	}

	values := make(map[string]any)
	addAuditInt64(values, "resource_id", fields.ResourceID)
	addAuditInt64(values, "parent_id", fields.ParentID)
	addAuditInt64(values, "goal_id", fields.GoalID)
	addAuditInt64(values, "target_id", fields.TargetID)
	addAuditInt64(values, "risk_id", fields.RiskID)
	addAuditInt64(values, "risk_review_id", fields.RiskReviewID)
	addAuditInt64(values, "care_plan_id", fields.CarePlanID)
	addAuditInt64(values, "care_plan_section_id", fields.CarePlanSectionID)
	addAuditInt64(values, "care_plan_review_id", fields.CarePlanReviewID)
	addAuditInt64(values, "session_id", fields.SessionID)
	addAuditInt64(values, "baseline_id", fields.BaselineID)
	addAuditInt64(values, "review_id", fields.ReviewID)
	addAuditInt64(values, "signal_response_id", fields.SignalResponseID)
	addAuditInt64(values, "feedback_receipt_id", fields.FeedbackReceiptID)
	addAuditInt64(values, "feedback_review_id", fields.FeedbackReviewID)
	addAuditString(values, "session_type", string(fields.SessionType))
	addAuditString(values, "manager_review_decision", string(fields.ManagerReviewDecision))
	addAuditString(values, "feedback_triage_outcome", string(fields.FeedbackTriageOutcome))
	if fields.ScoreCount != nil {
		values["score_count"] = *fields.ScoreCount
	}
	if fields.DomainCount != nil {
		values["domain_count"] = *fields.DomainCount
	}
	if fields.PriorityCount != nil {
		values["priority_count"] = *fields.PriorityCount
	}
	addAuditString(values, "previous_status", string(fields.PreviousStatus))
	addAuditString(values, "new_status", string(fields.NewStatus))
	if fields.Approved != nil {
		values["approved"] = *fields.Approved
	}
	if fields.Signed != nil {
		values["signed"] = *fields.Signed
	}
	addAuditString(values, "role", string(fields.Role))
	addAuditString(values, "request_id", fields.RequestID)

	encoded, err := json.Marshal(values)
	if err != nil {
		return AuditMetadata{}, fmt.Errorf("encode audit metadata: %w", err)
	}
	return AuditMetadata{jsonBytes: append([]byte(nil), encoded...)}, nil
}

func EmptyAuditMetadata() AuditMetadata {
	return AuditMetadata{jsonBytes: []byte(`{}`)}
}

// JSON returns a defensive copy so callers cannot mutate stored metadata.
func (m AuditMetadata) JSON() json.RawMessage {
	if len(m.jsonBytes) == 0 {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage(nil), m.jsonBytes...)
}

func validateAuditMetadataFields(fields AuditMetadataFields) error {
	ids := []*int64{
		fields.ResourceID, fields.ParentID, fields.GoalID, fields.TargetID, fields.RiskID, fields.RiskReviewID,
		fields.CarePlanID, fields.CarePlanSectionID, fields.CarePlanReviewID,
		fields.SessionID, fields.BaselineID, fields.ReviewID, fields.SignalResponseID,
		fields.FeedbackReceiptID, fields.FeedbackReviewID,
	}
	for _, id := range ids {
		if id != nil && *id <= 0 {
			return fmt.Errorf("%w: identifiers must be positive", ErrAuditMetadataValue)
		}
	}
	for name, count := range map[string]*int{
		"score_count": fields.ScoreCount, "domain_count": fields.DomainCount, "priority_count": fields.PriorityCount,
	} {
		if count != nil && *count < 0 {
			return fmt.Errorf("%w: %s must not be negative", ErrAuditMetadataValue, name)
		}
	}
	if fields.SessionType != "" && fields.SessionType != SessionTypeKeywork {
		return fmt.Errorf("%w: session_type", ErrAuditMetadataValue)
	}
	if fields.ManagerReviewDecision != "" &&
		fields.ManagerReviewDecision != ManagerReviewApproved &&
		fields.ManagerReviewDecision != ManagerReviewReturnedForReview {
		return fmt.Errorf("%w: manager_review_decision", ErrAuditMetadataValue)
	}
	if fields.FeedbackTriageOutcome != "" {
		switch fields.FeedbackTriageOutcome {
		case FeedbackTriageAcknowledge, FeedbackTriageNoFurtherAction,
			FeedbackTriageFollowUpRequired, FeedbackTriageLinkAsEvidence,
			FeedbackTriageEscalateForRiskReview:
		default:
			return fmt.Errorf("%w: feedback_triage_outcome", ErrAuditMetadataValue)
		}
	}
	for name, status := range map[string]AuditStatus{"previous_status": fields.PreviousStatus, "new_status": fields.NewStatus} {
		if status != "" && !status.Valid() {
			return fmt.Errorf("%w: %s", ErrAuditMetadataValue, name)
		}
	}
	if fields.Role != "" && !fields.Role.Valid() {
		return fmt.Errorf("%w: role", ErrAuditMetadataValue)
	}
	if fields.RequestID != "" && !validChiGeneratedRequestID(fields.RequestID) {
		return fmt.Errorf("%w: request_id", ErrAuditMetadataValue)
	}
	return nil
}

func validChiGeneratedRequestID(requestID string) bool {
	hostname, err := os.Hostname()
	if hostname == "" || err != nil {
		hostname = "localhost"
	}
	prefix := hostname + "/"
	return strings.HasPrefix(requestID, prefix) && auditRequestIDSuffixPattern.MatchString(strings.TrimPrefix(requestID, prefix))
}

// NewServerRequestID creates a server-owned identifier compatible with the
// current Chi request-ID shape. Protected middleware uses it instead of any
// caller-supplied X-Request-Id value when establishing audit provenance.
func NewServerRequestID() string {
	hostname, err := os.Hostname()
	if hostname == "" || err != nil {
		hostname = "localhost"
	}
	counter := serverRequestIDCounter.Add(1)
	randomBytes := make([]byte, 5)
	if _, err := rand.Read(randomBytes); err != nil {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", hostname, counter, time.Now().UnixNano())))
		copy(randomBytes, sum[:len(randomBytes)])
	}
	return fmt.Sprintf("%s/%s-%06d", hostname, hex.EncodeToString(randomBytes), counter)
}

func (s AuditStatus) Valid() bool {
	switch s {
	case AuditStatusActive, AuditStatusInactive, AuditStatusPaused, AuditStatusAchieved,
		AuditStatusClosed, AuditStatusDraft, AuditStatusSubmitted, AuditStatusApproved,
		AuditStatusArchived:
		return true
	default:
		return false
	}
}

func addAuditInt64(values map[string]any, key string, value *int64) {
	if value != nil {
		values[key] = *value
	}
}

func addAuditString(values map[string]any, key, value string) {
	if value != "" {
		values[key] = value
	}
}
