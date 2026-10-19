-- migrations/002_create_young_voice_feedback.sql

CREATE TABLE IF NOT EXISTS young_voice_feedback (
  id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  submitted_at            TIMESTAMPTZ NOT NULL DEFAULT now(),

  -- linkage
  home                    TEXT NOT NULL,
  home_id                 INTEGER,
  child_id                INTEGER,
  child_initials          TEXT,
  ip_hash                 TEXT,
  ua                      TEXT,
  period_month            DATE, -- first day of the month the feedback relates to

  -- Main questions (shortened DB field names)
  q1_overall              TEXT NOT NULL,
  q1_best                 TEXT,
  q1_hardest              TEXT,

  q2_safe                 TEXT,
  q2_unsafe               TEXT,
  q2_support_who          TEXT,

  q3_listened             TEXT,
  q3_trusted_adult        TEXT,
  q3_change_support       TEXT,

  q4_peers                TEXT,
  q4_unkind               TEXT,
  q4_help_with_peers      TEXT,

  q5_enjoy                TEXT,
  q5_wish_more            TEXT,
  q5_house_change         TEXT,

  q6_education            TEXT,
  q6_education_help       TEXT,

  q7_contact_feelings     TEXT,
  q7_contact_change       TEXT,

  q8_say_in_decisions     TEXT,
  q8_change_example       TEXT,

  q9_worries              TEXT,
  q9_one_change           TEXT,
  q9_want_talk            BOOLEAN NOT NULL DEFAULT false,
  q9_follow_up_preference TEXT,

  q10_what_tell_ofsted    TEXT,
  q10_never_forget        TEXT,

  safeguarding_flag       BOOLEAN NOT NULL DEFAULT false,
  risk_level              TEXT NOT NULL DEFAULT 'LOW',
  safeguarding_summary    TEXT,
  triage_status           TEXT NOT NULL DEFAULT 'New'
);