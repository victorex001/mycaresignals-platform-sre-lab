DROP TRIGGER IF EXISTS trg_external_feedback_review_immutable ON external_feedback_reviews;
DROP TRIGGER IF EXISTS trg_external_feedback_source_immutable ON external_feedback_receipts;
DROP FUNCTION IF EXISTS prevent_external_feedback_review_mutation();
DROP FUNCTION IF EXISTS prevent_external_feedback_source_mutation();

DROP INDEX IF EXISTS idx_external_feedback_reviews_receipt;
DROP INDEX IF EXISTS idx_external_feedback_reviews_child_evidence;
DROP INDEX IF EXISTS idx_external_feedback_receipts_child;
DROP INDEX IF EXISTS idx_external_feedback_receipts_home_queue;

DROP TABLE IF EXISTS external_feedback_reviews;
DROP TABLE IF EXISTS external_feedback_receipts;

ALTER TABLE children DROP CONSTRAINT IF EXISTS uq_children_id_home_id;
