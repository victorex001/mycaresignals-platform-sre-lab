DROP INDEX IF EXISTS idx_psychological_reviews_child_created_at;

ALTER TABLE child_onboarding_baselines
  DROP CONSTRAINT IF EXISTS chk_child_onboarding_baselines_status;

ALTER TABLE keyworking_sessions
  DROP CONSTRAINT IF EXISTS chk_keyworking_sessions_session_type;

ALTER TABLE child_targets
  DROP CONSTRAINT IF EXISTS chk_child_targets_status;

ALTER TABLE child_goals
  DROP CONSTRAINT IF EXISTS chk_child_goals_status;

ALTER TABLE session_target_scores
  DROP CONSTRAINT IF EXISTS chk_session_target_scores_progress_0_4,
  DROP CONSTRAINT IF EXISTS chk_session_target_scores_wellbeing_0_4;
