\set ON_ERROR_STOP on

\if :{?confirmation}
\else
  \echo 'confirmation is required'
  \quit 3
\endif

SELECT :'confirmation' = 'SRE_LAB_SYNTHETIC_ONLY' AS confirmation_valid \gset
\if :confirmation_valid
\else
  \echo 'Refusing to seed without the SRE_LAB_SYNTHETIC_ONLY confirmation.'
  \quit 3
\endif

\if :{?user_count}
\else
  \set user_count 1000000
\endif
\if :{?home_count}
\else
  \set home_count 10000
\endif
\if :{?active_children_per_home}
\else
  \set active_children_per_home 4
\endif
\if :{?historical_children_per_home}
\else
  \set historical_children_per_home 0
\endif
\if :{?audit_event_count}
\else
  \set audit_event_count 100000
\endif

SELECT
  (:user_count)::bigint BETWEEN 10000 AND 1000000
  AND (:home_count)::bigint BETWEEN 110 AND 10000
  AND (:active_children_per_home)::bigint BETWEEN 1 AND 20
  AND (:historical_children_per_home)::bigint BETWEEN 0 AND 50
  AND (:audit_event_count)::bigint BETWEEN 0 AND 100000000 AS inputs_valid
\gset

\if :inputs_valid
\else
  \echo 'Seed counts are outside the reviewed SRE-lab limits.'
  \quit 3
\endif

BEGIN;
SET LOCAL synchronous_commit = off;
SET LOCAL statement_timeout = 0;
SET LOCAL lock_timeout = '30s';

CREATE TABLE IF NOT EXISTS sre_lab_dataset_manifest (
  dataset_name TEXT PRIMARY KEY,
  user_count BIGINT NOT NULL,
  home_count BIGINT NOT NULL,
  active_child_count BIGINT NOT NULL,
  historical_child_count BIGINT NOT NULL,
  audit_event_count BIGINT NOT NULL,
  synthetic_only BOOLEAN NOT NULL CHECK (synthetic_only),
  seeded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO homes (id, name, external_ref)
SELECT
  1000000 + n,
  format('Synthetic Home %s', n),
  format('SRE-LAB-HOME-%s', lpad(n::text, 5, '0'))
FROM generate_series(1, (:home_count)::bigint) AS n
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name,
    external_ref = EXCLUDED.external_ref;

WITH counts AS (
  SELECT
    (:home_count)::bigint AS homes,
    (:active_children_per_home)::bigint AS active_per_home,
    (:historical_children_per_home)::bigint AS historical_per_home
), generated AS (
  SELECT
    n,
    counts.homes,
    counts.active_per_home * counts.homes AS active_total
  FROM counts,
       generate_series(
         1,
         (counts.active_per_home + counts.historical_per_home) * counts.homes
       ) AS n
)
INSERT INTO children (
  id, home_id, external_ref, initials, first_name, last_name,
  date_of_birth, legal_status, admission_date, gender, active
)
SELECT
  2000000 + n,
  1000000 + (((n - 1) % homes) + 1),
  format('SRE-LAB-CHILD-%s', n),
  upper(substr(md5(n::text), 1, 2)),
  format('Synthetic%s', n),
  'Record',
  DATE '2010-01-01' + ((n % 1460)::integer),
  'SYNTHETIC',
  DATE '2025-01-01' + ((n % 365)::integer),
  CASE WHEN n % 3 = 0 THEN 'non-binary' WHEN n % 2 = 0 THEN 'female' ELSE 'male' END,
  n <= active_total
FROM generated
ON CONFLICT (id) DO UPDATE
SET home_id = EXCLUDED.home_id,
    external_ref = EXCLUDED.external_ref,
    initials = EXCLUDED.initials,
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    date_of_birth = EXCLUDED.date_of_birth,
    legal_status = EXCLUDED.legal_status,
    admission_date = EXCLUDED.admission_date,
    gender = EXCLUDED.gender,
    active = EXCLUDED.active,
    updated_at = now();

INSERT INTO staff_users (
  id, email, display_name, external_auth_subject, status
)
SELECT
  3000000 + n,
  format('sre-user-%s@example.invalid', n),
  format('Synthetic User %s', n),
  format('sre-user-%s', n),
  'active'
FROM generate_series(1, (:user_count)::bigint) AS n
ON CONFLICT (id) DO UPDATE
SET email = EXCLUDED.email,
    display_name = EXCLUDED.display_name,
    external_auth_subject = EXCLUDED.external_auth_subject,
    status = EXCLUDED.status,
    updated_at = now();

-- The first home_count users are one manager per home. Remaining identities are
-- read-oriented key workers distributed evenly across every synthetic tenant.
INSERT INTO staff_home_roles (
  id, staff_user_id, home_id, role, active
)
SELECT
  4000000 + n,
  3000000 + n,
  1000000 + CASE
    WHEN n <= (:home_count)::bigint THEN n
    ELSE ((n - 1) % (:home_count)::bigint) + 1
  END,
  CASE WHEN n <= (:home_count)::bigint THEN 'manager' ELSE 'key_worker' END,
  TRUE
FROM generate_series(1, (:user_count)::bigint) AS n
ON CONFLICT (id) DO UPDATE
SET staff_user_id = EXCLUDED.staff_user_id,
    home_id = EXCLUDED.home_id,
    role = EXCLUDED.role,
    active = EXCLUDED.active;

INSERT INTO audit_events (
  id, home_id, child_id, staff_user_id, actor_email, actor_name,
  event_type, entity_type, entity_id, request_method, request_path,
  metadata, created_at
)
SELECT
  5000000 + n,
  1000000 + (((n - 1) % (:home_count)::bigint) + 1),
  2000000 + (((n - 1) % ((:home_count)::bigint * (:active_children_per_home)::bigint)) + 1),
  3000000 + (((n - 1) % (:user_count)::bigint) + 1),
  format('sre-user-%s@example.invalid', ((n - 1) % (:user_count)::bigint) + 1),
  'Synthetic Seed',
  'sre.synthetic.seed',
  'synthetic_record',
  n,
  'SEED',
  '/sre-lab/synthetic-seed',
  jsonb_build_object('synthetic', true, 'sequence', n),
  now() - make_interval(secs => (n % 2592000)::integer)
FROM generate_series(1, (:audit_event_count)::bigint) AS n
ON CONFLICT (id) DO NOTHING;

INSERT INTO sre_lab_dataset_manifest (
  dataset_name, user_count, home_count, active_child_count,
  historical_child_count, audit_event_count, synthetic_only, seeded_at
)
VALUES (
  'mycaresignals-1m-v1',
  (:user_count)::bigint,
  (:home_count)::bigint,
  (:home_count)::bigint * (:active_children_per_home)::bigint,
  (:home_count)::bigint * (:historical_children_per_home)::bigint,
  (:audit_event_count)::bigint,
  TRUE,
  now()
)
ON CONFLICT (dataset_name) DO UPDATE
SET user_count = EXCLUDED.user_count,
    home_count = EXCLUDED.home_count,
    active_child_count = EXCLUDED.active_child_count,
    historical_child_count = EXCLUDED.historical_child_count,
    audit_event_count = EXCLUDED.audit_event_count,
    synthetic_only = TRUE,
    seeded_at = now();

SELECT setval(pg_get_serial_sequence('homes', 'id'), GREATEST((SELECT max(id) FROM homes), 1), true);
SELECT setval(pg_get_serial_sequence('children', 'id'), GREATEST((SELECT max(id) FROM children), 1), true);
SELECT setval(pg_get_serial_sequence('staff_users', 'id'), GREATEST((SELECT max(id) FROM staff_users), 1), true);
SELECT setval(pg_get_serial_sequence('staff_home_roles', 'id'), GREATEST((SELECT max(id) FROM staff_home_roles), 1), true);
SELECT setval(pg_get_serial_sequence('audit_events', 'id'), GREATEST((SELECT max(id) FROM audit_events), 1), true);

COMMIT;

ANALYZE homes;
ANALYZE children;
ANALYZE staff_users;
ANALYZE staff_home_roles;
ANALYZE audit_events;

SELECT * FROM sre_lab_dataset_manifest WHERE dataset_name = 'mycaresignals-1m-v1';
