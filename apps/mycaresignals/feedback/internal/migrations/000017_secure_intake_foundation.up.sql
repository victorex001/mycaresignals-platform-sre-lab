-- Secure public intake foundation.
-- Existing LOW/MEDIUM/HIGH values remain untouched as legacy provenance.

CREATE TABLE intake_contexts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash TEXT NOT NULL UNIQUE,
  home_external_id INTEGER NOT NULL CHECK (home_external_id > 0),
  child_external_id INTEGER CHECK (child_external_id > 0),
  home_label TEXT NOT NULL CHECK (NULLIF(btrim(home_label), '') IS NOT NULL),
  purpose TEXT NOT NULL CHECK (purpose IN ('visitor_feedback', 'young_person_voice')),
  expires_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE feedback
  ALTER COLUMN risk_level DROP NOT NULL,
  ALTER COLUMN risk_level DROP DEFAULT,
  ADD COLUMN intake_context_id UUID REFERENCES intake_contexts(id) ON DELETE RESTRICT,
  ADD COLUMN raw_submission JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN requires_human_review BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN review_priority TEXT NOT NULL DEFAULT 'routine'
    CHECK (review_priority IN ('routine', 'priority', 'urgent')),
  ADD COLUMN matched_rule_ids JSONB NOT NULL DEFAULT '[]'::jsonb
    CHECK (jsonb_typeof(matched_rule_ids) = 'array'),
  ADD COLUMN rule_version TEXT NOT NULL DEFAULT 'legacy-v0';

ALTER TABLE young_voice_feedback
  ALTER COLUMN risk_level DROP NOT NULL,
  ALTER COLUMN risk_level DROP DEFAULT,
  ADD COLUMN intake_context_id UUID REFERENCES intake_contexts(id) ON DELETE RESTRICT,
  ADD COLUMN raw_submission JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN requires_human_review BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN review_priority TEXT NOT NULL DEFAULT 'routine'
    CHECK (review_priority IN ('routine', 'priority', 'urgent')),
  ADD COLUMN matched_rule_ids JSONB NOT NULL DEFAULT '[]'::jsonb
    CHECK (jsonb_typeof(matched_rule_ids) = 'array'),
  ADD COLUMN rule_version TEXT NOT NULL DEFAULT 'legacy-v0';

CREATE TABLE intake_delivery_outbox (
  id BIGSERIAL PRIMARY KEY,
  receipt_id UUID NOT NULL,
  intake_type TEXT NOT NULL CHECK (intake_type IN ('visitor_feedback', 'young_person_voice')),
  payload JSONB NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  delivered_at TIMESTAMPTZ,
  last_error_code TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (receipt_id, intake_type)
);

CREATE INDEX idx_intake_delivery_outbox_pending
  ON intake_delivery_outbox (next_attempt_at, id)
  WHERE delivered_at IS NULL;

CREATE OR REPLACE FUNCTION prevent_raw_intake_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'raw intake receipts are immutable';
END;
$$;

CREATE TRIGGER trg_feedback_raw_immutable
  BEFORE UPDATE OR DELETE ON feedback
  FOR EACH ROW EXECUTE FUNCTION prevent_raw_intake_mutation();

CREATE TRIGGER trg_young_voice_raw_immutable
  BEFORE UPDATE OR DELETE ON young_voice_feedback
  FOR EACH ROW EXECUTE FUNCTION prevent_raw_intake_mutation();
