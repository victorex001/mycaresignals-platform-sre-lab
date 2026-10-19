-- 005_add_psych_fields.sql

-- Goals: who set it + child voice + formulation + strengths
ALTER TABLE child_goals
  ADD COLUMN goal_type TEXT NOT NULL DEFAULT 'staff',  -- 'staff', 'young_person', 'joint'
  ADD COLUMN child_voice TEXT,                        -- their words / perspective
  ADD COLUMN formulation_summary TEXT,                -- short case formulation
  ADD COLUMN protective_factors TEXT;                 -- strengths / what's going well

-- Targets: barriers, supports and what staff will actually do
ALTER TABLE child_targets
  ADD COLUMN barriers TEXT,
  ADD COLUMN supports_needed TEXT,
  ADD COLUMN intervention_strategy TEXT;