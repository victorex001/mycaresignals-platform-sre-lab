DROP TRIGGER IF EXISTS trg_child_monthly_voice_immutable ON child_monthly_voice_ratings;
DROP FUNCTION IF EXISTS prevent_child_monthly_voice_mutation();
DROP TRIGGER IF EXISTS trg_child_monthly_voice_source ON child_monthly_voice_ratings;
DROP FUNCTION IF EXISTS validate_child_monthly_voice_source();
DROP INDEX IF EXISTS idx_child_monthly_voice_child_month;
DROP TABLE IF EXISTS child_monthly_voice_ratings;

ALTER TABLE external_feedback_reviews
  DROP CONSTRAINT IF EXISTS uq_external_feedback_reviews_id_receipt_child;

ALTER TABLE external_feedback_receipts
  DROP CONSTRAINT IF EXISTS uq_external_feedback_receipts_id_child_id;

DROP TRIGGER IF EXISTS trg_child_daily_progress_immutable ON child_daily_progress;
DROP FUNCTION IF EXISTS prevent_child_daily_progress_mutation();
DROP INDEX IF EXISTS idx_child_daily_progress_target;
DROP INDEX IF EXISTS idx_child_daily_progress_child_domain_month;
DROP INDEX IF EXISTS idx_child_daily_progress_child_recent;
DROP TABLE IF EXISTS child_daily_progress;
