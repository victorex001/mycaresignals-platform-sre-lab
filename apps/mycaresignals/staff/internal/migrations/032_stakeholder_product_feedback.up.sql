-- Stakeholder product feedback for the isolated demo environment.
-- This is platform feedback only. It is intentionally not linked to children,
-- therapeutic evidence, Daily Progress, Key Working, Young Person Voice or QR
-- visitor feedback.

CREATE TABLE stakeholder_product_feedback (
  id BIGSERIAL PRIMARY KEY,
  staff_user_id BIGINT NOT NULL REFERENCES staff_users(id) ON DELETE RESTRICT,
  reviewer_name TEXT NOT NULL,
  reviewer_email TEXT NOT NULL,
  organisation TEXT,
  environment TEXT NOT NULL CHECK (environment IN ('demo')),
  page_path TEXT NOT NULL,
  overall_rating SMALLINT NOT NULL CHECK (overall_rating BETWEEN 1 AND 5),
  most_useful TEXT,
  unclear_or_difficult TEXT,
  missing TEXT,
  anything_else TEXT,
  submitted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_stakeholder_product_feedback_submitted
  ON stakeholder_product_feedback (submitted_at DESC, id DESC);

CREATE INDEX idx_stakeholder_product_feedback_staff
  ON stakeholder_product_feedback (staff_user_id, submitted_at DESC);
