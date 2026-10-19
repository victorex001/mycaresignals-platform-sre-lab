-- 20251211_add_goals_targets_sessions.sql

-- Optional enum for goal_type
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'goal_type') THEN
    CREATE TYPE goal_type AS ENUM ('care_plan', 'placement', 'therapeutic');
  END IF;
END$$;

-- 1) Child goals (from care/placement plan etc.)
CREATE TABLE IF NOT EXISTS child_goals (
  id           BIGSERIAL PRIMARY KEY,
  child_id     BIGINT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
  goal_type    goal_type NOT NULL DEFAULT 'care_plan',
  title        TEXT NOT NULL,
  description  TEXT,
  start_date   DATE,
  target_date  DATE,
  status       TEXT NOT NULL DEFAULT 'active', -- active / paused / achieved / closed
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2) Targets (short, domain-based, linked to goals)
CREATE TABLE IF NOT EXISTS child_targets (
  id               BIGSERIAL PRIMARY KEY,
  child_id         BIGINT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
  goal_id          BIGINT REFERENCES child_goals(id) ON DELETE SET NULL,
  domain           TEXT NOT NULL, -- e.g. "emotional", "behaviour", "education"
  short_label      TEXT NOT NULL, -- e.g. "Calm down after conflict in <10 mins"
  description      TEXT,
  status           TEXT NOT NULL DEFAULT 'active', -- active / paused / achieved / closed
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 3) Key-working sessions
CREATE TABLE IF NOT EXISTS keyworking_sessions (
  id               BIGSERIAL PRIMARY KEY,
  child_id         BIGINT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
  session_date     DATE NOT NULL,
  staff_name       TEXT,          -- optional; can also store staff_id later
  external_ref     TEXT,          -- e.g. ClearCare note ID
  notes_summary    TEXT,          -- brief summary, NOT full narrative
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 4) Scores per target per session (BERRI-like, but yours)
CREATE TABLE IF NOT EXISTS session_target_scores (
  id               BIGSERIAL PRIMARY KEY,
  session_id       BIGINT NOT NULL REFERENCES keyworking_sessions(id) ON DELETE CASCADE,
  target_id        BIGINT NOT NULL REFERENCES child_targets(id) ON DELETE CASCADE,
  baseline_score   SMALLINT NOT NULL CHECK (baseline_score BETWEEN 1 AND 10),
  session_score    SMALLINT NOT NULL CHECK (session_score BETWEEN 1 AND 10),
  comment          TEXT,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Basic helpful indexes
CREATE INDEX IF NOT EXISTS idx_child_goals_child_id
  ON child_goals (child_id);

CREATE INDEX IF NOT EXISTS idx_child_targets_child_id
  ON child_targets (child_id);

CREATE INDEX IF NOT EXISTS idx_keyworking_sessions_child_id_date
  ON keyworking_sessions (child_id, session_date);

CREATE INDEX IF NOT EXISTS idx_session_scores_session_id
  ON session_target_scores (session_id);