DROP INDEX IF EXISTS idx_child_onboarding_framework_sources_baseline;
DROP TABLE IF EXISTS child_onboarding_framework_sources;

ALTER TABLE child_onboarding_domain_states
  DROP CONSTRAINT IF EXISTS chk_child_onboarding_domain_information_status,
  DROP COLUMN IF EXISTS priority_reason,
  DROP COLUMN IF EXISTS source_date,
  DROP COLUMN IF EXISTS source_details,
  DROP COLUMN IF EXISTS information_source,
  DROP COLUMN IF EXISTS information_status,
  DROP COLUMN IF EXISTS young_person_view,
  DROP COLUMN IF EXISTS current_support,
  DROP COLUMN IF EXISTS what_helps,
  DROP COLUMN IF EXISTS support_needs,
  DROP COLUMN IF EXISTS strengths_protective_factors,
  DROP COLUMN IF EXISTS current_understanding;

DROP INDEX IF EXISTS idx_child_onboarding_framework_latest;
DROP INDEX IF EXISTS uq_child_onboarding_six_domain_version;

ALTER TABLE child_onboarding_baselines
  DROP CONSTRAINT IF EXISTS chk_child_onboarding_safety_status,
  DROP CONSTRAINT IF EXISTS chk_child_onboarding_background_status,
  DROP CONSTRAINT IF EXISTS chk_child_onboarding_immediate_status,
  DROP CONSTRAINT IF EXISTS chk_child_onboarding_framework_version,
  DROP CONSTRAINT IF EXISTS chk_child_onboarding_pathway,
  DROP CONSTRAINT IF EXISTS chk_child_onboarding_framework_model,
  DROP COLUMN IF EXISTS created_by_staff_id,
  DROP COLUMN IF EXISTS safety_information_status,
  DROP COLUMN IF EXISTS known_safety_information,
  DROP COLUMN IF EXISTS background_information_status,
  DROP COLUMN IF EXISTS immediate_information_status,
  DROP COLUMN IF EXISTS immediate_information,
  DROP COLUMN IF EXISTS onboarding_pathway,
  DROP COLUMN IF EXISTS framework_version,
  DROP COLUMN IF EXISTS framework_model;
