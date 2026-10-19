-- 008_add_wellbeing_progress_to_session_target_scores.sql

ALTER TABLE session_target_scores
  ADD COLUMN wellbeing_score integer,
  ADD COLUMN progress_score integer;