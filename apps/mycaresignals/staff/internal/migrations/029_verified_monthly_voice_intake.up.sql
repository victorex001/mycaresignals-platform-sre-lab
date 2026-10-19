-- Verified monthly Young Person Voice ingestion from the separate QR service.
-- The authenticated receipt is immutable source evidence; rating rows are a
-- structured projection only and never create or alter care decisions.

ALTER TABLE external_feedback_receipts
  ADD COLUMN intake_context_id UUID,
  ADD COLUMN intake_purpose TEXT,
  ADD COLUMN period_month DATE,
  ADD COLUMN domain_ratings JSONB,
  ADD COLUMN follow_up_requested BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN follow_up_preference TEXT,
  ADD CONSTRAINT chk_external_feedback_monthly_voice_envelope CHECK (
    intake_purpose IS NULL OR (
      intake_type = 'young_person_voice'
      AND source_type = 'YoungPersonVoice'
      AND intake_purpose = 'young_person_monthly_voice'
      AND intake_context_id IS NOT NULL
      AND child_id IS NOT NULL
      AND period_month IS NOT NULL
      AND period_month = date_trunc('month', period_month)::date
      AND jsonb_typeof(domain_ratings) = 'object'
      AND domain_ratings ?& ARRAY[
        'Emotional & Psychological Wellbeing',
        'Behaviour & Self-Regulation',
        'Relationships & Connection',
        'Daily Living, Learning & Participation',
        'Safety, Risk & Coping with Stress',
        'Skills, Identity & Future'
      ]
      AND domain_ratings - ARRAY[
        'Emotional & Psychological Wellbeing',
        'Behaviour & Self-Regulation',
        'Relationships & Connection',
        'Daily Living, Learning & Participation',
        'Safety, Risk & Coping with Stress',
        'Skills, Identity & Future'
      ] = '{}'::jsonb
    )
  ),
  ADD CONSTRAINT chk_external_feedback_follow_up_preference CHECK (
    follow_up_requested OR follow_up_preference IS NULL
  );

CREATE UNIQUE INDEX uq_external_feedback_monthly_voice_context
  ON external_feedback_receipts (intake_context_id)
  WHERE intake_purpose = 'young_person_monthly_voice';

ALTER TABLE external_feedback_receipts
  ADD CONSTRAINT uq_external_feedback_context_receipt_child
  UNIQUE (intake_context_id, id, child_id);

CREATE OR REPLACE FUNCTION prevent_external_feedback_source_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.intake_receipt_id IS DISTINCT FROM OLD.intake_receipt_id
     OR NEW.intake_type IS DISTINCT FROM OLD.intake_type
     OR NEW.source_type IS DISTINCT FROM OLD.source_type
     OR NEW.home_id IS DISTINCT FROM OLD.home_id
     OR (OLD.intake_context_id IS NOT NULL AND NEW.child_id IS DISTINCT FROM OLD.child_id)
     OR NEW.received_at IS DISTINCT FROM OLD.received_at
     OR NEW.requires_human_review IS DISTINCT FROM OLD.requires_human_review
     OR NEW.review_priority IS DISTINCT FROM OLD.review_priority
     OR NEW.matched_rule_ids IS DISTINCT FROM OLD.matched_rule_ids
     OR NEW.rule_version IS DISTINCT FROM OLD.rule_version
     OR NEW.raw_payload IS DISTINCT FROM OLD.raw_payload
     OR NEW.intake_context_id IS DISTINCT FROM OLD.intake_context_id
     OR NEW.intake_purpose IS DISTINCT FROM OLD.intake_purpose
     OR NEW.period_month IS DISTINCT FROM OLD.period_month
     OR NEW.domain_ratings IS DISTINCT FROM OLD.domain_ratings
     OR NEW.follow_up_requested IS DISTINCT FROM OLD.follow_up_requested
     OR NEW.follow_up_preference IS DISTINCT FROM OLD.follow_up_preference
     OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
    RAISE EXCEPTION 'external feedback source evidence is immutable';
  END IF;
  RETURN NEW;
END;
$$;

ALTER TABLE child_monthly_voice_ratings
  ALTER COLUMN source_review_id DROP NOT NULL,
  ADD COLUMN source_context_id UUID,
  ADD CONSTRAINT chk_child_monthly_voice_one_verification_source CHECK (
    num_nonnulls(source_review_id, source_context_id) = 1
  ),
  ADD CONSTRAINT fk_child_monthly_voice_context_receipt_child
    FOREIGN KEY (source_context_id, source_receipt_id, child_id)
    REFERENCES external_feedback_receipts(intake_context_id, id, child_id)
    ON DELETE RESTRICT,
  ADD CONSTRAINT uq_child_monthly_voice_source_domain
    UNIQUE (source_receipt_id, domain);

CREATE OR REPLACE FUNCTION validate_child_monthly_voice_source()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  source_received_at TIMESTAMPTZ;
  source_rating SMALLINT;
BEGIN
  IF NEW.source_context_id IS NOT NULL THEN
    SELECT receipt.received_at,
           (receipt.domain_ratings ->> NEW.domain)::smallint
      INTO source_received_at, source_rating
    FROM external_feedback_receipts receipt
    WHERE receipt.id = NEW.source_receipt_id
      AND receipt.child_id = NEW.child_id
      AND receipt.intake_context_id = NEW.source_context_id
      AND receipt.intake_type = 'young_person_voice'
      AND receipt.source_type = 'YoungPersonVoice'
      AND receipt.intake_purpose = 'young_person_monthly_voice'
      AND receipt.period_month = NEW.period_month
      AND receipt.domain_ratings ? NEW.domain;

    IF source_received_at IS NULL OR source_rating IS DISTINCT FROM NEW.young_person_rating THEN
      RAISE EXCEPTION 'monthly voice rating requires a matching verified intake context';
    END IF;
  ELSE
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
  END IF;

  NEW.submitted_at := source_received_at;
  NEW.source := 'young_person_monthly_voice';
  RETURN NEW;
END;
$$;
