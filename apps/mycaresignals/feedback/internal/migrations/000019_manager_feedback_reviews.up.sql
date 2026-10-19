CREATE TABLE feedback_management_reviews (
  id BIGSERIAL PRIMARY KEY,
  feedback_id UUID NOT NULL REFERENCES feedback(id) ON DELETE RESTRICT,
  reviewer_identity TEXT NOT NULL,
  reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  outcome TEXT NOT NULL CHECK (outcome IN (
    'acknowledged', 'no_action_required', 'follow_up_required', 'closed'
  )),
  manager_note TEXT NOT NULL CHECK (
    char_length(btrim(manager_note)) BETWEEN 3 AND 2000
  ),
  follow_up_required BOOLEAN NOT NULL,
  follow_up_status TEXT NOT NULL CHECK (follow_up_status IN ('none', 'open', 'closed'))
);

CREATE INDEX idx_feedback_management_reviews_feedback
  ON feedback_management_reviews (feedback_id, reviewed_at DESC, id DESC);

CREATE OR REPLACE FUNCTION prevent_feedback_review_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'feedback management review history is immutable';
END;
$$;

CREATE TRIGGER trg_feedback_management_reviews_immutable
  BEFORE UPDATE OR DELETE ON feedback_management_reviews
  FOR EACH ROW EXECUTE FUNCTION prevent_feedback_review_mutation();
