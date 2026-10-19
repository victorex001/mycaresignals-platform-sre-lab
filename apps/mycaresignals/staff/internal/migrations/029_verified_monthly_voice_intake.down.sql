DROP TRIGGER IF EXISTS trg_child_monthly_voice_source ON child_monthly_voice_ratings;
DROP FUNCTION IF EXISTS validate_child_monthly_voice_source();

ALTER TABLE child_monthly_voice_ratings
  DROP CONSTRAINT IF EXISTS uq_child_monthly_voice_source_domain,
  DROP CONSTRAINT IF EXISTS fk_child_monthly_voice_context_receipt_child,
  DROP CONSTRAINT IF EXISTS chk_child_monthly_voice_one_verification_source,
  DROP COLUMN IF EXISTS source_context_id;

ALTER TABLE child_monthly_voice_ratings
  ALTER COLUMN source_review_id SET NOT NULL;

CREATE OR REPLACE FUNCTION validate_child_monthly_voice_source()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  source_received_at TIMESTAMPTZ;
BEGIN
  SELECT receipt.received_at
    INTO source_received_at
  FROM external_feedback_receipts receipt
  JOIN external_feedback_reviews review
    ON review.id = NEW.source_review_id
   AND review.receipt_id = receipt.id
   AND review.child_id = NEW.child_id
  WHERE receipt.id = NEW.source_receipt_id
    AND receipt.child_id = NEW.child_id
    AND receipt.intake_type = 'young_person_voice'
    AND receipt.source_type = 'YoungPersonVoice'
    AND review.outcome = 'link_as_evidence';

  IF source_received_at IS NULL THEN
    RAISE EXCEPTION 'monthly voice rating requires verified linked young-person voice evidence';
  END IF;

  NEW.submitted_at := source_received_at;
  NEW.source := 'young_person_monthly_voice';
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_child_monthly_voice_source
  BEFORE INSERT ON child_monthly_voice_ratings
  FOR EACH ROW EXECUTE FUNCTION validate_child_monthly_voice_source();

ALTER TABLE external_feedback_receipts
  DROP CONSTRAINT IF EXISTS uq_external_feedback_context_receipt_child;

DROP INDEX IF EXISTS uq_external_feedback_monthly_voice_context;

ALTER TABLE external_feedback_receipts
  DROP CONSTRAINT IF EXISTS chk_external_feedback_follow_up_preference,
  DROP CONSTRAINT IF EXISTS chk_external_feedback_monthly_voice_envelope,
  DROP COLUMN IF EXISTS follow_up_preference,
  DROP COLUMN IF EXISTS follow_up_requested,
  DROP COLUMN IF EXISTS domain_ratings,
  DROP COLUMN IF EXISTS period_month,
  DROP COLUMN IF EXISTS intake_purpose,
  DROP COLUMN IF EXISTS intake_context_id;

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
