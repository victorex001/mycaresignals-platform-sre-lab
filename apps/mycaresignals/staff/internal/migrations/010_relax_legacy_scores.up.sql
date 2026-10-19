-- 010_relax_legacy_scores.sql
-- Make legacy baseline/session score columns optional so we can
-- use wellbeing_score and progress_score going forward.

ALTER TABLE session_target_scores
  ALTER COLUMN baseline_score DROP NOT NULL,
  ALTER COLUMN session_score DROP NOT NULL;

ALTER TABLE session_target_scores
  DROP CONSTRAINT IF EXISTS session_target_scores_baseline_score_check,
  DROP CONSTRAINT IF EXISTS session_target_scores_session_score_check;