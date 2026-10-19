-- 017_align_goal_target_psych_fields.down.sql

ALTER TABLE child_targets
  DROP COLUMN IF EXISTS intervention_strategy,
  DROP COLUMN IF EXISTS supports_needed,
  DROP COLUMN IF EXISTS barriers;

ALTER TABLE child_goals
  DROP COLUMN IF EXISTS protective_factors,
  DROP COLUMN IF EXISTS formulation_summary,
  DROP COLUMN IF EXISTS child_voice;
