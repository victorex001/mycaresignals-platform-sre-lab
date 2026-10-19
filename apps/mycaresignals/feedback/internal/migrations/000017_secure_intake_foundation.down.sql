DROP TRIGGER IF EXISTS trg_young_voice_raw_immutable ON young_voice_feedback;
DROP TRIGGER IF EXISTS trg_feedback_raw_immutable ON feedback;
DROP FUNCTION IF EXISTS prevent_raw_intake_mutation();

DROP INDEX IF EXISTS idx_intake_delivery_outbox_pending;
DROP TABLE IF EXISTS intake_delivery_outbox;

UPDATE young_voice_feedback
SET risk_level = 'LOW'
WHERE risk_level IS NULL;

ALTER TABLE young_voice_feedback
  DROP COLUMN IF EXISTS rule_version,
  DROP COLUMN IF EXISTS matched_rule_ids,
  DROP COLUMN IF EXISTS review_priority,
  DROP COLUMN IF EXISTS requires_human_review,
  DROP COLUMN IF EXISTS raw_submission,
  DROP COLUMN IF EXISTS intake_context_id,
  ALTER COLUMN risk_level SET DEFAULT 'LOW',
  ALTER COLUMN risk_level SET NOT NULL;

UPDATE feedback
SET risk_level = 'LOW'
WHERE risk_level IS NULL;

ALTER TABLE feedback
  DROP COLUMN IF EXISTS rule_version,
  DROP COLUMN IF EXISTS matched_rule_ids,
  DROP COLUMN IF EXISTS review_priority,
  DROP COLUMN IF EXISTS requires_human_review,
  DROP COLUMN IF EXISTS raw_submission,
  DROP COLUMN IF EXISTS intake_context_id,
  ALTER COLUMN risk_level SET DEFAULT 'LOW',
  ALTER COLUMN risk_level SET NOT NULL;

DROP TABLE IF EXISTS intake_contexts;
