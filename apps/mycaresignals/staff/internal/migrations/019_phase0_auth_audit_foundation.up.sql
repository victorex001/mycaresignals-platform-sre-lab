-- 019_phase0_auth_audit_foundation.up.sql
-- Phase 0A: staff identity, home roles and audit event foundation.

CREATE TABLE IF NOT EXISTS staff_users (
  id BIGSERIAL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  external_auth_subject TEXT UNIQUE,
  status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'inactive')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS staff_home_roles (
  id BIGSERIAL PRIMARY KEY,
  staff_user_id BIGINT NOT NULL REFERENCES staff_users(id) ON DELETE CASCADE,
  home_id BIGINT NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
  role TEXT NOT NULL
    CHECK (role IN (
      'key_worker',
      'senior',
      'manager',
      'responsible_individual',
      'admin'
    )),
  active BOOLEAN NOT NULL DEFAULT TRUE,
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (staff_user_id, home_id, role)
);

CREATE TABLE IF NOT EXISTS audit_events (
  id BIGSERIAL PRIMARY KEY,

  home_id BIGINT REFERENCES homes(id) ON DELETE SET NULL,
  child_id BIGINT REFERENCES children(id) ON DELETE SET NULL,
  staff_user_id BIGINT REFERENCES staff_users(id) ON DELETE SET NULL,

  actor_email TEXT,
  actor_name TEXT,

  event_type TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id BIGINT,

  request_method TEXT,
  request_path TEXT,

  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_staff_home_roles_staff
  ON staff_home_roles (staff_user_id);

CREATE INDEX IF NOT EXISTS idx_staff_home_roles_home_role
  ON staff_home_roles (home_id, role)
  WHERE active = TRUE;

CREATE INDEX IF NOT EXISTS idx_audit_events_home_created
  ON audit_events (home_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_events_child_created
  ON audit_events (child_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_events_staff_created
  ON audit_events (staff_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_events_entity
  ON audit_events (entity_type, entity_id);

CREATE INDEX IF NOT EXISTS idx_audit_events_type_created
  ON audit_events (event_type, created_at DESC);
