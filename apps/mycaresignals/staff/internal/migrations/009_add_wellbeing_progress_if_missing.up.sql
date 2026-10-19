-- 009_add_wellbeing_progress_if_missing.up.sql

ALTER TABLE session_target_scores
  ADD COLUMN IF NOT EXISTS wellbeing_score integer,
  ADD COLUMN IF NOT EXISTS progress_score integer;