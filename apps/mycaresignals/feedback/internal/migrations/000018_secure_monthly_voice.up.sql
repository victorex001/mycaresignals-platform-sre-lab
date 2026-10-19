-- Secure monthly young-person voice contexts and structured ratings.

ALTER TABLE intake_contexts
  DROP CONSTRAINT intake_contexts_purpose_check,
  ADD CONSTRAINT intake_contexts_purpose_check CHECK (purpose IN (
    'visitor_feedback',
    'young_person_voice',
    'young_person_monthly_voice'
  )),
  ADD COLUMN period_month DATE,
  ADD COLUMN used_at TIMESTAMPTZ,
  ADD CONSTRAINT chk_monthly_voice_context_scope CHECK (
    purpose <> 'young_person_monthly_voice'
    OR (
      child_external_id IS NOT NULL
      AND period_month IS NOT NULL
      AND period_month = date_trunc('month', period_month)::date
      AND expires_at IS NOT NULL
    )
  );

CREATE UNIQUE INDEX uq_monthly_voice_context_child_period
  ON intake_contexts (child_external_id, period_month)
  WHERE purpose = 'young_person_monthly_voice' AND revoked_at IS NULL;

ALTER TABLE young_voice_feedback
  ADD COLUMN domain_ratings JSONB NOT NULL DEFAULT '{}'::jsonb
    CHECK (jsonb_typeof(domain_ratings) = 'object'),
  ADD COLUMN follow_up_requested BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN follow_up_preference TEXT;

CREATE UNIQUE INDEX uq_young_voice_monthly_context
  ON young_voice_feedback (intake_context_id)
  WHERE intake_context_id IS NOT NULL;
