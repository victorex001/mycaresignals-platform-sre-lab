DROP INDEX IF EXISTS uq_young_voice_monthly_context;

ALTER TABLE young_voice_feedback
  DROP COLUMN IF EXISTS follow_up_preference,
  DROP COLUMN IF EXISTS follow_up_requested,
  DROP COLUMN IF EXISTS domain_ratings;

DROP INDEX IF EXISTS uq_monthly_voice_context_child_period;

ALTER TABLE intake_contexts
  DROP CONSTRAINT IF EXISTS chk_monthly_voice_context_scope,
  DROP COLUMN IF EXISTS used_at,
  DROP COLUMN IF EXISTS period_month,
  DROP CONSTRAINT IF EXISTS intake_contexts_purpose_check,
  ADD CONSTRAINT intake_contexts_purpose_check
    CHECK (purpose IN ('visitor_feedback', 'young_person_voice'));
