-- 018_keyworking_journey_fields.up.sql
-- Phase 1A: structured key-working journey fields.
-- Extends the existing keyworking_sessions table. No duplicate sessions table.

ALTER TABLE keyworking_sessions
  ADD COLUMN IF NOT EXISTS session_type TEXT NOT NULL DEFAULT 'keywork',
  ADD COLUMN IF NOT EXISTS primary_domain TEXT,
  ADD COLUMN IF NOT EXISTS target_id BIGINT REFERENCES child_targets(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS young_person_presentation TEXT,
  ADD COLUMN IF NOT EXISTS young_person_voice_note TEXT,
  ADD COLUMN IF NOT EXISTS key_worker_reflection TEXT,
  ADD COLUMN IF NOT EXISTS signed_by_key_worker BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS key_worker_signed_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS signed_by_manager BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS manager_signed_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS manager_name TEXT;

CREATE INDEX IF NOT EXISTS idx_keyworking_sessions_child_target
  ON keyworking_sessions (child_id, target_id);

CREATE INDEX IF NOT EXISTS idx_keyworking_sessions_child_domain
  ON keyworking_sessions (child_id, primary_domain);

CREATE INDEX IF NOT EXISTS idx_keyworking_sessions_manager_signoff
  ON keyworking_sessions (child_id, signed_by_manager);
