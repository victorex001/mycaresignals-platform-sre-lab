-- Six-domain, human-led onboarding framework. Existing baseline rows remain
-- legacy_eight_domain records and are not rewritten or mapped.

ALTER TABLE child_onboarding_baselines
  ADD COLUMN framework_model TEXT NOT NULL DEFAULT 'legacy_eight_domain',
  ADD COLUMN framework_version INT NOT NULL DEFAULT 1,
  ADD COLUMN onboarding_pathway TEXT,
  ADD COLUMN immediate_information TEXT,
  ADD COLUMN immediate_information_status TEXT,
  ADD COLUMN background_information_status TEXT,
  ADD COLUMN known_safety_information TEXT,
  ADD COLUMN safety_information_status TEXT,
  ADD COLUMN created_by_staff_id BIGINT REFERENCES staff_users(id) ON DELETE SET NULL;

ALTER TABLE child_onboarding_baselines
  ADD CONSTRAINT chk_child_onboarding_framework_model
    CHECK (framework_model IN ('legacy_eight_domain', 'six_domain_v1')),
  ADD CONSTRAINT chk_child_onboarding_pathway
    CHECK (onboarding_pathway IS NULL OR onboarding_pathway IN ('new_placement', 'existing_young_person')),
  ADD CONSTRAINT chk_child_onboarding_framework_version
    CHECK (framework_version > 0),
  ADD CONSTRAINT chk_child_onboarding_immediate_status
    CHECK (immediate_information_status IS NULL OR immediate_information_status IN ('known_confirmed', 'partly_known', 'awaiting_confirmation', 'not_known_yet')),
  ADD CONSTRAINT chk_child_onboarding_background_status
    CHECK (background_information_status IS NULL OR background_information_status IN ('known_confirmed', 'partly_known', 'awaiting_confirmation', 'not_known_yet')),
  ADD CONSTRAINT chk_child_onboarding_safety_status
    CHECK (safety_information_status IS NULL OR safety_information_status IN ('known_confirmed', 'partly_known', 'awaiting_confirmation', 'not_known_yet'));

CREATE INDEX idx_child_onboarding_framework_latest
  ON child_onboarding_baselines (child_id, framework_model, framework_version DESC, id DESC);

CREATE UNIQUE INDEX uq_child_onboarding_six_domain_version
  ON child_onboarding_baselines (child_id, framework_version)
  WHERE framework_model = 'six_domain_v1';

ALTER TABLE child_onboarding_domain_states
  ADD COLUMN current_understanding TEXT,
  ADD COLUMN strengths_protective_factors TEXT,
  ADD COLUMN support_needs TEXT,
  ADD COLUMN what_helps TEXT,
  ADD COLUMN current_support TEXT,
  ADD COLUMN young_person_view TEXT,
  ADD COLUMN information_status TEXT,
  ADD COLUMN information_source TEXT,
  ADD COLUMN source_details TEXT,
  ADD COLUMN source_date DATE,
  ADD COLUMN priority_reason TEXT;

ALTER TABLE child_onboarding_domain_states
  ADD CONSTRAINT chk_child_onboarding_domain_information_status
    CHECK (information_status IS NULL OR information_status IN ('known_confirmed', 'partly_known', 'awaiting_confirmation', 'not_known_yet'));

CREATE TABLE child_onboarding_framework_sources (
  id BIGSERIAL PRIMARY KEY,
  baseline_id BIGINT NOT NULL REFERENCES child_onboarding_baselines(id) ON DELETE CASCADE,
  source_type TEXT NOT NULL CHECK (source_type IN (
    'local_authority_information',
    'existing_care_plan',
    'placement_plan',
    'existing_risk_assessment',
    'missing_from_home_plan',
    'health_plan',
    'education_pep',
    'behaviour_support_plan',
    'contact_arrangements',
    'berri_assessment',
    'social_worker',
    'young_person',
    'parent_family',
    'school_education',
    'health_professional',
    'staff_observation',
    'other_professional_report',
    'other'
  )),
  source_name TEXT,
  source_date DATE,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_child_onboarding_framework_sources_baseline
  ON child_onboarding_framework_sources (baseline_id, id);
