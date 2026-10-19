CREATE TABLE child_risk_reviews (
  id BIGSERIAL PRIMARY KEY,
  risk_id BIGINT NOT NULL,
  child_id BIGINT NOT NULL,
  reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  reviewer_staff_id BIGINT NOT NULL REFERENCES staff_users(id) ON DELETE RESTRICT,
  previous_residual_likelihood SMALLINT
    CONSTRAINT chk_child_risk_reviews_previous_likelihood CHECK (previous_residual_likelihood BETWEEN 1 AND 5),
  previous_residual_severity SMALLINT
    CONSTRAINT chk_child_risk_reviews_previous_severity CHECK (previous_residual_severity BETWEEN 1 AND 5),
  reviewed_residual_likelihood SMALLINT NOT NULL
    CONSTRAINT chk_child_risk_reviews_reviewed_likelihood CHECK (reviewed_residual_likelihood BETWEEN 1 AND 5),
  reviewed_residual_severity SMALLINT NOT NULL
    CONSTRAINT chk_child_risk_reviews_reviewed_severity CHECK (reviewed_residual_severity BETWEEN 1 AND 5),
  review_note TEXT NOT NULL
    CONSTRAINT chk_child_risk_reviews_note CHECK (btrim(review_note) <> ''),
  outcome TEXT NOT NULL
    CONSTRAINT chk_child_risk_reviews_outcome CHECK (outcome IN ('continue', 'modify', 'close')),
  next_review_date DATE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_child_risk_reviews_previous_rating_pair
    CHECK ((previous_residual_likelihood IS NULL) = (previous_residual_severity IS NULL)),
  CONSTRAINT fk_child_risk_reviews_risk_child
    FOREIGN KEY (risk_id, child_id) REFERENCES child_risks(id, child_id) ON DELETE RESTRICT
);

CREATE INDEX idx_child_risk_reviews_risk_reviewed
  ON child_risk_reviews (risk_id, reviewed_at DESC, id DESC);

CREATE INDEX idx_child_risk_reviews_child_reviewed
  ON child_risk_reviews (child_id, reviewed_at DESC, id DESC);

CREATE FUNCTION prevent_child_risk_review_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'child risk reviews are immutable';
END;
$$;

CREATE TRIGGER trg_child_risk_reviews_immutable
BEFORE UPDATE OR DELETE ON child_risk_reviews
FOR EACH ROW EXECUTE FUNCTION prevent_child_risk_review_mutation();
