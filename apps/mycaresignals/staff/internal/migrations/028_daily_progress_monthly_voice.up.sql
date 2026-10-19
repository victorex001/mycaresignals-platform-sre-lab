-- Daily Progress v1 + verified monthly voice comparison foundation.
-- Staff observation, Key Working, and young-person voice remain separate evidence sources.

CREATE TABLE child_daily_progress (
  id BIGSERIAL PRIMARY KEY,
  child_id BIGINT NOT NULL REFERENCES children(id) ON DELETE RESTRICT,
  recorded_by_staff_id BIGINT NOT NULL REFERENCES staff_users(id) ON DELETE RESTRICT,
  observed_on DATE NOT NULL DEFAULT CURRENT_DATE,
  domain TEXT NOT NULL CHECK (domain IN (
    'Emotional & Psychological Wellbeing',
    'Behaviour & Self-Regulation',
    'Relationships & Connection',
    'Daily Living, Learning & Participation',
    'Safety, Risk & Coping with Stress',
    'Skills, Identity & Future'
  )),
  target_id BIGINT,
  wellbeing_score SMALLINT CHECK (wellbeing_score IS NULL OR wellbeing_score BETWEEN 0 AND 4),
  progress_score SMALLINT CHECK (progress_score IS NULL OR progress_score BETWEEN 0 AND 4),
  observation_note TEXT,
  young_person_comment TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_child_daily_progress_has_observation CHECK (
    wellbeing_score IS NOT NULL
    OR progress_score IS NOT NULL
    OR NULLIF(btrim(observation_note), '') IS NOT NULL
  ),
  CONSTRAINT fk_child_daily_progress_target_child
    FOREIGN KEY (target_id, child_id) REFERENCES child_targets(id, child_id) ON DELETE RESTRICT
);

CREATE INDEX idx_child_daily_progress_child_recent
  ON child_daily_progress (child_id, observed_on DESC, id DESC);

CREATE INDEX idx_child_daily_progress_child_domain_month
  ON child_daily_progress (child_id, domain, observed_on);

CREATE INDEX idx_child_daily_progress_target
  ON child_daily_progress (target_id, observed_on DESC)
  WHERE target_id IS NOT NULL;

CREATE OR REPLACE FUNCTION prevent_child_daily_progress_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'daily progress evidence is append-only';
END;
$$;

CREATE TRIGGER trg_child_daily_progress_immutable
  BEFORE UPDATE OR DELETE ON child_daily_progress
  FOR EACH ROW EXECUTE FUNCTION prevent_child_daily_progress_mutation();

ALTER TABLE external_feedback_receipts
  ADD CONSTRAINT uq_external_feedback_receipts_id_child_id UNIQUE (id, child_id);

ALTER TABLE external_feedback_reviews
  ADD CONSTRAINT uq_external_feedback_reviews_id_receipt_child UNIQUE (id, receipt_id, child_id);

CREATE TABLE child_monthly_voice_ratings (
  id BIGSERIAL PRIMARY KEY,
  child_id BIGINT NOT NULL REFERENCES children(id) ON DELETE RESTRICT,
  period_month DATE NOT NULL CHECK (period_month = date_trunc('month', period_month)::date),
  domain TEXT NOT NULL CHECK (domain IN (
    'Emotional & Psychological Wellbeing',
    'Behaviour & Self-Regulation',
    'Relationships & Connection',
    'Daily Living, Learning & Participation',
    'Safety, Risk & Coping with Stress',
    'Skills, Identity & Future'
  )),
  young_person_rating SMALLINT NOT NULL CHECK (young_person_rating BETWEEN 0 AND 4),
  source_receipt_id BIGINT NOT NULL,
  source_review_id BIGINT NOT NULL,
  source TEXT NOT NULL DEFAULT 'young_person_monthly_voice'
    CHECK (source = 'young_person_monthly_voice'),
  submitted_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_child_monthly_voice_child_domain_month UNIQUE (child_id, domain, period_month),
  CONSTRAINT fk_child_monthly_voice_receipt_child
    FOREIGN KEY (source_receipt_id, child_id)
    REFERENCES external_feedback_receipts(id, child_id) ON DELETE RESTRICT,
  CONSTRAINT fk_child_monthly_voice_review_receipt_child
    FOREIGN KEY (source_review_id, source_receipt_id, child_id)
    REFERENCES external_feedback_reviews(id, receipt_id, child_id) ON DELETE RESTRICT
);

CREATE INDEX idx_child_monthly_voice_child_month
  ON child_monthly_voice_ratings (child_id, period_month DESC, domain);

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

CREATE OR REPLACE FUNCTION prevent_child_monthly_voice_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'monthly young-person voice evidence is immutable';
END;
$$;

CREATE TRIGGER trg_child_monthly_voice_immutable
  BEFORE UPDATE OR DELETE ON child_monthly_voice_ratings
  FOR EACH ROW EXECUTE FUNCTION prevent_child_monthly_voice_mutation();
