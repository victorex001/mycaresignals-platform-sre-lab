-- Key Working manager review v1.
-- Reuses migration 018's signed_by_manager and manager_signed_at fields while
-- replacing free-text attribution with an authoritative staff relationship.

ALTER TABLE keyworking_sessions
  ADD COLUMN manager_review_decision TEXT,
  ADD COLUMN manager_review_comment TEXT,
  ADD COLUMN manager_reviewed_by_staff_id BIGINT REFERENCES staff_users(id) ON DELETE RESTRICT;

ALTER TABLE keyworking_sessions
  ADD CONSTRAINT chk_keyworking_manager_review_decision
    CHECK (manager_review_decision IS NULL OR manager_review_decision IN ('approved', 'returned_for_review')),
  ADD CONSTRAINT chk_keyworking_manager_return_comment
    CHECK (
      manager_review_decision <> 'returned_for_review'
      OR NULLIF(btrim(manager_review_comment), '') IS NOT NULL
    ),
  ADD CONSTRAINT chk_keyworking_manager_review_state
    CHECK (
      manager_review_decision IS NULL
      OR (
        manager_signed_at IS NOT NULL
        AND manager_reviewed_by_staff_id IS NOT NULL
        AND signed_by_manager = (manager_review_decision = 'approved')
      )
    );

CREATE INDEX idx_keyworking_sessions_manager_review_pending
  ON keyworking_sessions (child_id, session_date DESC, id DESC)
  WHERE signed_by_key_worker = TRUE AND manager_review_decision IS NULL;
