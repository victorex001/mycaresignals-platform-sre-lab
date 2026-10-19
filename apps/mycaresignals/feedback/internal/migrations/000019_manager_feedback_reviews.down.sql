DROP TRIGGER IF EXISTS trg_feedback_management_reviews_immutable ON feedback_management_reviews;
DROP FUNCTION IF EXISTS prevent_feedback_review_mutation();
DROP INDEX IF EXISTS idx_feedback_management_reviews_feedback;
DROP TABLE IF EXISTS feedback_management_reviews;
