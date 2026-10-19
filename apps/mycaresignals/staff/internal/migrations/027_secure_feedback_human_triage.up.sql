-- Secure Feedback Intake + Human Triage Foundation.
-- Public raw intake remains in the separate QR service. This schema stores the
-- authenticated review copy and human decisions; it never creates a child Risk.

ALTER TABLE children
  ADD CONSTRAINT uq_children_id_home_id UNIQUE (id, home_id);

CREATE TABLE external_feedback_receipts (
  id BIGSERIAL PRIMARY KEY,
  intake_receipt_id UUID NOT NULL UNIQUE,
  intake_type TEXT NOT NULL
    CHECK (intake_type IN ('visitor_feedback', 'young_person_voice')),
  source_type TEXT NOT NULL
    CHECK (source_type IN ('YoungPersonVoice', 'Parent', 'SocialWorker', 'Professional', 'Ofsted', 'Other')),
  home_id BIGINT NOT NULL REFERENCES homes(id) ON DELETE RESTRICT,
  child_id BIGINT,
  received_at TIMESTAMPTZ NOT NULL,
  requires_human_review BOOLEAN NOT NULL DEFAULT FALSE,
  review_priority TEXT NOT NULL
    CHECK (review_priority IN ('routine', 'priority', 'urgent')),
  matched_rule_ids JSONB NOT NULL DEFAULT '[]'::jsonb
    CHECK (jsonb_typeof(matched_rule_ids) = 'array'),
  rule_version TEXT NOT NULL,
  raw_payload JSONB NOT NULL,
  triage_status TEXT NOT NULL DEFAULT 'new'
    CHECK (triage_status IN ('new', 'acknowledged', 'follow_up', 'reviewed')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT fk_external_feedback_receipt_child_home
    FOREIGN KEY (child_id, home_id) REFERENCES children(id, home_id) ON DELETE RESTRICT
);

CREATE TABLE external_feedback_reviews (
  id BIGSERIAL PRIMARY KEY,
  receipt_id BIGINT NOT NULL REFERENCES external_feedback_receipts(id) ON DELETE RESTRICT,
  home_id BIGINT NOT NULL REFERENCES homes(id) ON DELETE RESTRICT,
  reviewer_staff_id BIGINT NOT NULL REFERENCES staff_users(id) ON DELETE RESTRICT,
  reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  outcome TEXT NOT NULL CHECK (outcome IN (
    'acknowledge',
    'no_further_action',
    'follow_up_required',
    'link_as_evidence',
    'escalate_for_risk_review'
  )),
  rationale TEXT NOT NULL CHECK (NULLIF(btrim(rationale), '') IS NOT NULL),
  follow_up_required BOOLEAN NOT NULL DEFAULT FALSE,
  child_id BIGINT,
  domain TEXT,
  goal_id BIGINT,
  target_id BIGINT,
  risk_id BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_external_feedback_review_child_links CHECK (
    child_id IS NOT NULL OR (domain IS NULL AND goal_id IS NULL AND target_id IS NULL AND risk_id IS NULL)
  ),
  CONSTRAINT chk_external_feedback_evidence_requires_child CHECK (
    outcome <> 'link_as_evidence' OR child_id IS NOT NULL
  ),
  CONSTRAINT fk_external_feedback_review_child_home
    FOREIGN KEY (child_id, home_id) REFERENCES children(id, home_id) ON DELETE RESTRICT,
  CONSTRAINT fk_external_feedback_review_goal_child
    FOREIGN KEY (goal_id, child_id) REFERENCES child_goals(id, child_id) ON DELETE RESTRICT,
  CONSTRAINT fk_external_feedback_review_target_child
    FOREIGN KEY (target_id, child_id) REFERENCES child_targets(id, child_id) ON DELETE RESTRICT,
  CONSTRAINT fk_external_feedback_review_risk_child
    FOREIGN KEY (risk_id, child_id) REFERENCES child_risks(id, child_id) ON DELETE RESTRICT
);

CREATE INDEX idx_external_feedback_receipts_home_queue
  ON external_feedback_receipts (home_id, triage_status, requires_human_review DESC, received_at DESC);

CREATE INDEX idx_external_feedback_receipts_child
  ON external_feedback_receipts (child_id, received_at DESC)
  WHERE child_id IS NOT NULL;

CREATE INDEX idx_external_feedback_reviews_child_evidence
  ON external_feedback_reviews (child_id, reviewed_at DESC)
  WHERE child_id IS NOT NULL AND outcome = 'link_as_evidence';

CREATE INDEX idx_external_feedback_reviews_receipt
  ON external_feedback_reviews (receipt_id, reviewed_at DESC, id DESC);

CREATE OR REPLACE FUNCTION prevent_external_feedback_source_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.intake_receipt_id IS DISTINCT FROM OLD.intake_receipt_id
     OR NEW.intake_type IS DISTINCT FROM OLD.intake_type
     OR NEW.source_type IS DISTINCT FROM OLD.source_type
     OR NEW.home_id IS DISTINCT FROM OLD.home_id
     OR NEW.received_at IS DISTINCT FROM OLD.received_at
     OR NEW.requires_human_review IS DISTINCT FROM OLD.requires_human_review
     OR NEW.review_priority IS DISTINCT FROM OLD.review_priority
     OR NEW.matched_rule_ids IS DISTINCT FROM OLD.matched_rule_ids
     OR NEW.rule_version IS DISTINCT FROM OLD.rule_version
     OR NEW.raw_payload IS DISTINCT FROM OLD.raw_payload
     OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
    RAISE EXCEPTION 'external feedback source evidence is immutable';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_external_feedback_source_immutable
  BEFORE UPDATE ON external_feedback_receipts
  FOR EACH ROW EXECUTE FUNCTION prevent_external_feedback_source_mutation();

CREATE OR REPLACE FUNCTION prevent_external_feedback_review_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'external feedback review history is immutable';
END;
$$;

CREATE TRIGGER trg_external_feedback_review_immutable
  BEFORE UPDATE OR DELETE ON external_feedback_reviews
  FOR EACH ROW EXECUTE FUNCTION prevent_external_feedback_review_mutation();
