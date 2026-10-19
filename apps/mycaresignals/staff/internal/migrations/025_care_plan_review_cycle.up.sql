ALTER TABLE child_care_plans
  ADD COLUMN predecessor_care_plan_id BIGINT;

ALTER TABLE child_care_plans
  ADD CONSTRAINT fk_child_care_plans_predecessor_child
    FOREIGN KEY (predecessor_care_plan_id, child_id)
    REFERENCES child_care_plans(id, child_id) ON DELETE RESTRICT,
  ADD CONSTRAINT chk_child_care_plans_predecessor_not_self
    CHECK (predecessor_care_plan_id IS NULL OR predecessor_care_plan_id <> id);

CREATE INDEX idx_child_care_plans_predecessor
  ON child_care_plans (predecessor_care_plan_id)
  WHERE predecessor_care_plan_id IS NOT NULL;

CREATE TABLE child_care_plan_reviews (
  id BIGSERIAL PRIMARY KEY,
  care_plan_id BIGINT NOT NULL,
  child_id BIGINT NOT NULL,
  reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  reviewer_staff_id BIGINT NOT NULL REFERENCES staff_users(id) ON DELETE RESTRICT,
  review_note TEXT NOT NULL
    CONSTRAINT chk_child_care_plan_reviews_note CHECK (btrim(review_note) <> ''),
  young_person_involvement_note TEXT,
  outcome TEXT NOT NULL
    CONSTRAINT chk_child_care_plan_reviews_outcome CHECK (outcome IN ('continue', 'revise', 'close')),
  next_review_date DATE,
  successor_care_plan_id BIGINT,
  snapshot_status TEXT NOT NULL
    CONSTRAINT chk_child_care_plan_reviews_snapshot_status CHECK (snapshot_status IN ('draft', 'active', 'archived')),
  snapshot_effective_from DATE NOT NULL,
  snapshot_review_date DATE,
  evidence_since TIMESTAMPTZ NOT NULL,
  active_goal_count INTEGER NOT NULL DEFAULT 0 CHECK (active_goal_count >= 0),
  active_target_count INTEGER NOT NULL DEFAULT 0 CHECK (active_target_count >= 0),
  achieved_target_count INTEGER NOT NULL DEFAULT 0 CHECK (achieved_target_count >= 0),
  paused_target_count INTEGER NOT NULL DEFAULT 0 CHECK (paused_target_count >= 0),
  targets_changed_count INTEGER NOT NULL DEFAULT 0 CHECK (targets_changed_count >= 0),
  targets_due_count INTEGER NOT NULL DEFAULT 0 CHECK (targets_due_count >= 0),
  active_risk_count INTEGER NOT NULL DEFAULT 0 CHECK (active_risk_count >= 0),
  risk_reviews_completed_count INTEGER NOT NULL DEFAULT 0 CHECK (risk_reviews_completed_count >= 0),
  keyworking_session_count INTEGER NOT NULL DEFAULT 0 CHECK (keyworking_session_count >= 0),
  sections_not_known_count INTEGER NOT NULL DEFAULT 0 CHECK (sections_not_known_count >= 0),
  sections_awaiting_confirmation_count INTEGER NOT NULL DEFAULT 0 CHECK (sections_awaiting_confirmation_count >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT fk_child_care_plan_reviews_plan_child
    FOREIGN KEY (care_plan_id, child_id)
    REFERENCES child_care_plans(id, child_id) ON DELETE RESTRICT,
  CONSTRAINT fk_child_care_plan_reviews_successor_child
    FOREIGN KEY (successor_care_plan_id, child_id)
    REFERENCES child_care_plans(id, child_id) ON DELETE RESTRICT,
  CONSTRAINT chk_child_care_plan_reviews_successor_outcome
    CHECK ((outcome = 'revise') = (successor_care_plan_id IS NOT NULL)),
  CONSTRAINT chk_child_care_plan_reviews_continue_next_review
    CHECK (outcome <> 'continue' OR next_review_date IS NOT NULL)
);

CREATE INDEX idx_child_care_plan_reviews_plan_reviewed
  ON child_care_plan_reviews (care_plan_id, reviewed_at DESC, id DESC);

CREATE INDEX idx_child_care_plan_reviews_child_reviewed
  ON child_care_plan_reviews (child_id, reviewed_at DESC, id DESC);

CREATE TABLE child_care_plan_review_sections (
  id BIGSERIAL PRIMARY KEY,
  review_id BIGINT NOT NULL REFERENCES child_care_plan_reviews(id) ON DELETE RESTRICT,
  section_key TEXT NOT NULL
    CONSTRAINT chk_child_care_plan_review_sections_key CHECK (section_key IN (
      'about_me',
      'safety_stability',
      'emotional_wellbeing',
      'health',
      'education_future',
      'relationships_belonging',
      'independence_daily_living',
      'identity_culture'
    )),
  information_status TEXT NOT NULL
    CONSTRAINT chk_child_care_plan_review_sections_information_status CHECK (information_status IN (
      'known',
      'partly_known',
      'not_known_yet',
      'awaiting_confirmation',
      'not_applicable'
    )),
  current_understanding TEXT,
  strengths TEXT,
  staff_support TEXT,
  young_person_involvement_note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_child_care_plan_review_sections_review_key UNIQUE (review_id, section_key)
);

CREATE INDEX idx_child_care_plan_review_sections_review
  ON child_care_plan_review_sections (review_id, section_key);

CREATE FUNCTION prevent_child_care_plan_review_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'child care plan reviews and snapshots are immutable';
END;
$$;

CREATE TRIGGER trg_child_care_plan_reviews_immutable
BEFORE UPDATE OR DELETE ON child_care_plan_reviews
FOR EACH ROW EXECUTE FUNCTION prevent_child_care_plan_review_mutation();

CREATE TRIGGER trg_child_care_plan_review_sections_immutable
BEFORE UPDATE OR DELETE ON child_care_plan_review_sections
FOR EACH ROW EXECUTE FUNCTION prevent_child_care_plan_review_mutation();
