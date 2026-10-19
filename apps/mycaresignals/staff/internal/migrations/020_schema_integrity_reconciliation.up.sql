ALTER TABLE session_target_scores
  ADD CONSTRAINT chk_session_target_scores_wellbeing_0_4
  CHECK (wellbeing_score IS NULL OR wellbeing_score BETWEEN 0 AND 4) NOT VALID,
  ADD CONSTRAINT chk_session_target_scores_progress_0_4
  CHECK (progress_score IS NULL OR progress_score BETWEEN 0 AND 4) NOT VALID;

ALTER TABLE child_goals
  ADD CONSTRAINT chk_child_goals_status
  CHECK (status IN ('active', 'paused', 'achieved', 'closed')) NOT VALID;

ALTER TABLE child_targets
  ADD CONSTRAINT chk_child_targets_status
  CHECK (status IN ('active', 'paused', 'achieved', 'closed')) NOT VALID;

ALTER TABLE keyworking_sessions
  ADD CONSTRAINT chk_keyworking_sessions_session_type
  CHECK (session_type IN ('keywork')) NOT VALID;

ALTER TABLE child_onboarding_baselines
  ADD CONSTRAINT chk_child_onboarding_baselines_status
  CHECK (status IN ('draft', 'submitted', 'approved')) NOT VALID;

CREATE INDEX IF NOT EXISTS idx_psychological_reviews_child_created_at
  ON psychological_reviews (child_id, created_at DESC);
