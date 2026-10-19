-- 017_align_goal_target_psych_fields.up.sql
-- Makes frontend goal/target fields real in the database.
-- Safe to run even if older manual migration already added them.

ALTER TABLE child_goals
  ADD COLUMN IF NOT EXISTS child_voice TEXT,
  ADD COLUMN IF NOT EXISTS formulation_summary TEXT,
  ADD COLUMN IF NOT EXISTS protective_factors TEXT;

ALTER TABLE child_targets
  ADD COLUMN IF NOT EXISTS barriers TEXT,
  ADD COLUMN IF NOT EXISTS supports_needed TEXT,
  ADD COLUMN IF NOT EXISTS intervention_strategy TEXT;
