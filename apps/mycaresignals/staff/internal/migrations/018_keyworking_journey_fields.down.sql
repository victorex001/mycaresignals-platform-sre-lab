-- 018_keyworking_journey_fields.down.sql

DROP INDEX IF EXISTS idx_keyworking_sessions_manager_signoff;
DROP INDEX IF EXISTS idx_keyworking_sessions_child_domain;
DROP INDEX IF EXISTS idx_keyworking_sessions_child_target;

ALTER TABLE keyworking_sessions
  DROP COLUMN IF EXISTS manager_name,
  DROP COLUMN IF EXISTS manager_signed_at,
  DROP COLUMN IF EXISTS signed_by_manager,
  DROP COLUMN IF EXISTS key_worker_signed_at,
  DROP COLUMN IF EXISTS signed_by_key_worker,
  DROP COLUMN IF EXISTS key_worker_reflection,
  DROP COLUMN IF EXISTS young_person_voice_note,
  DROP COLUMN IF EXISTS young_person_presentation,
  DROP COLUMN IF EXISTS target_id,
  DROP COLUMN IF EXISTS primary_domain,
  DROP COLUMN IF EXISTS session_type;
