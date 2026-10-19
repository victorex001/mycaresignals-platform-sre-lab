DROP INDEX IF EXISTS idx_keyworking_sessions_manager_review_pending;

ALTER TABLE keyworking_sessions
  DROP CONSTRAINT IF EXISTS chk_keyworking_manager_review_state,
  DROP CONSTRAINT IF EXISTS chk_keyworking_manager_return_comment,
  DROP CONSTRAINT IF EXISTS chk_keyworking_manager_review_decision,
  DROP COLUMN IF EXISTS manager_reviewed_by_staff_id,
  DROP COLUMN IF EXISTS manager_review_comment,
  DROP COLUMN IF EXISTS manager_review_decision;
