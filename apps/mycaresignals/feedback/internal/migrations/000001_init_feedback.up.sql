-- migrations/001_init.sql

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS feedback (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  submitted_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  source             TEXT NOT NULL,  -- e.g. 'qr', 'tablet', 'kiosk'
  who                TEXT NOT NULL,  -- e.g. 'visitor', 'professional', 'young_person'
  reason             TEXT,           -- free text: why they’re giving feedback
  rating             SMALLINT CHECK (rating BETWEEN 1 AND 5),
  what_went_well     TEXT NOT NULL,
  what_to_improve    TEXT,
  recommend          BOOLEAN NOT NULL DEFAULT FALSE,
  contact_name       TEXT,
  contact_email      TEXT,
  ip_hash            TEXT,
  ua                 TEXT,           -- user agent
  safeguarding_flag  BOOLEAN NOT NULL DEFAULT FALSE,
  triage_status      TEXT NOT NULL DEFAULT 'New'
                      CHECK (triage_status IN ('New','InReview','Closed')),

  -- myCareSignals alignment
  feedback_type      TEXT NOT NULL DEFAULT 'visitor',       -- 'visitor' | 'young_checkin' | 'monthly_child' etc.
  answers            JSONB NOT NULL DEFAULT '{}'::jsonb,    -- structured emoji / domain answers
  risk_level         TEXT NOT NULL DEFAULT 'LOW',           -- 'LOW' | 'MEDIUM' | 'HIGH'

  -- link back to the therapeutic engine
  home_id            INTEGER,
  home_name          TEXT,
  child_id           INTEGER,
  child_initials     TEXT
);