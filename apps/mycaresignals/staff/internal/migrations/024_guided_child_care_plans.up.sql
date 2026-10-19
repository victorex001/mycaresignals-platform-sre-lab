CREATE TABLE child_care_plans (
  id BIGSERIAL PRIMARY KEY,
  child_id BIGINT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'draft'
    CONSTRAINT chk_child_care_plans_status CHECK (status IN ('draft', 'active', 'archived')),
  effective_from DATE NOT NULL DEFAULT CURRENT_DATE,
  review_date DATE,
  created_by_staff_id BIGINT NOT NULL REFERENCES staff_users(id) ON DELETE RESTRICT,
  activated_by_staff_id BIGINT REFERENCES staff_users(id) ON DELETE RESTRICT,
  activated_at TIMESTAMPTZ,
  archived_by_staff_id BIGINT REFERENCES staff_users(id) ON DELETE RESTRICT,
  archived_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_child_care_plans_activation_pair
    CHECK ((activated_by_staff_id IS NULL) = (activated_at IS NULL)),
  CONSTRAINT chk_child_care_plans_archive_pair
    CHECK ((archived_by_staff_id IS NULL) = (archived_at IS NULL)),
  CONSTRAINT uq_child_care_plans_id_child_id UNIQUE (id, child_id)
);

CREATE UNIQUE INDEX uq_child_care_plans_one_active_per_child
  ON child_care_plans (child_id)
  WHERE status = 'active';

CREATE INDEX idx_child_care_plans_child_status
  ON child_care_plans (child_id, status, updated_at DESC);

CREATE TABLE child_care_plan_sections (
  id BIGSERIAL PRIMARY KEY,
  care_plan_id BIGINT NOT NULL REFERENCES child_care_plans(id) ON DELETE CASCADE,
  section_key TEXT NOT NULL
    CONSTRAINT chk_child_care_plan_sections_key CHECK (section_key IN (
      'about_me',
      'safety_stability',
      'emotional_wellbeing',
      'health',
      'education_future',
      'relationships_belonging',
      'independence_daily_living',
      'identity_culture'
    )),
  information_status TEXT NOT NULL DEFAULT 'not_known_yet'
    CONSTRAINT chk_child_care_plan_sections_information_status CHECK (information_status IN (
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
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_child_care_plan_sections_plan_key UNIQUE (care_plan_id, section_key)
);

CREATE INDEX idx_child_care_plan_sections_plan
  ON child_care_plan_sections (care_plan_id, section_key);
