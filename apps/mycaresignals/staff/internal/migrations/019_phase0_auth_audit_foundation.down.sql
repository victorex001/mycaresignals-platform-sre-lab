-- 019_phase0_auth_audit_foundation.down.sql

DROP INDEX IF EXISTS idx_audit_events_type_created;
DROP INDEX IF EXISTS idx_audit_events_entity;
DROP INDEX IF EXISTS idx_audit_events_staff_created;
DROP INDEX IF EXISTS idx_audit_events_child_created;
DROP INDEX IF EXISTS idx_audit_events_home_created;
DROP INDEX IF EXISTS idx_staff_home_roles_home_role;
DROP INDEX IF EXISTS idx_staff_home_roles_staff;

DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS staff_home_roles;
DROP TABLE IF EXISTS staff_users;
