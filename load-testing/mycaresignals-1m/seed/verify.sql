\set ON_ERROR_STOP on

SELECT
  dataset_name,
  user_count,
  home_count,
  active_child_count,
  historical_child_count,
  audit_event_count,
  synthetic_only,
  seeded_at
FROM sre_lab_dataset_manifest
WHERE dataset_name = 'mycaresignals-1m-v1';

SELECT
  (SELECT count(*) FROM staff_users WHERE external_auth_subject LIKE 'sre-user-%') AS synthetic_users,
  (SELECT count(*) FROM homes WHERE external_ref LIKE 'SRE-LAB-HOME-%') AS synthetic_homes,
  (SELECT count(*) FROM children WHERE external_ref LIKE 'SRE-LAB-CHILD-%' AND active) AS active_children,
  (SELECT count(*) FROM children WHERE external_ref LIKE 'SRE-LAB-CHILD-%' AND NOT active) AS historical_children,
  (SELECT count(*) FROM audit_events WHERE event_type = 'sre.synthetic.seed') AS seeded_audit_events;

SELECT count(*) AS cross_tenant_role_errors
FROM staff_home_roles role
JOIN staff_users staff ON staff.id = role.staff_user_id
WHERE staff.external_auth_subject LIKE 'sre-user-%'
  AND (
    role.home_id < 1000001 OR role.home_id > 1010000 OR
    role.active IS NOT TRUE
  );

DO $$
DECLARE
  manifest sre_lab_dataset_manifest%ROWTYPE;
  actual_users BIGINT;
  actual_homes BIGINT;
  actual_active_children BIGINT;
  actual_historical_children BIGINT;
  actual_seed_audits BIGINT;
  actual_roles BIGINT;
  invalid_roles BIGINT;
BEGIN
  SELECT * INTO manifest
  FROM sre_lab_dataset_manifest
  WHERE dataset_name = 'mycaresignals-1m-v1';

  IF NOT FOUND OR manifest.synthetic_only IS NOT TRUE THEN
    RAISE EXCEPTION 'Synthetic dataset manifest is absent or invalid';
  END IF;

  SELECT count(*) INTO actual_users
  FROM staff_users WHERE external_auth_subject LIKE 'sre-user-%';
  SELECT count(*) INTO actual_homes
  FROM homes WHERE external_ref LIKE 'SRE-LAB-HOME-%';
  SELECT count(*) FILTER (WHERE active), count(*) FILTER (WHERE NOT active)
    INTO actual_active_children, actual_historical_children
  FROM children WHERE external_ref LIKE 'SRE-LAB-CHILD-%';
  SELECT count(*) INTO actual_seed_audits
  FROM audit_events WHERE event_type = 'sre.synthetic.seed';
  SELECT count(*) INTO actual_roles
  FROM staff_home_roles role
  JOIN staff_users staff ON staff.id = role.staff_user_id
  WHERE staff.external_auth_subject LIKE 'sre-user-%';

  SELECT count(*) INTO invalid_roles
  FROM staff_home_roles role
  JOIN staff_users staff ON staff.id = role.staff_user_id
  WHERE staff.external_auth_subject LIKE 'sre-user-%'
    AND (
      role.active IS NOT TRUE OR
      role.home_id <> 1000000 + CASE
        WHEN staff.id - 3000000 <= manifest.home_count
          THEN staff.id - 3000000
        ELSE ((staff.id - 3000001) % manifest.home_count) + 1
      END OR
      role.role <> CASE
        WHEN staff.id - 3000000 <= manifest.home_count THEN 'manager'
        ELSE 'key_worker'
      END
    );

  IF actual_users < manifest.user_count OR
     actual_homes < manifest.home_count OR
     actual_active_children < manifest.active_child_count OR
     actual_historical_children < manifest.historical_child_count OR
     actual_seed_audits < manifest.audit_event_count OR
     actual_roles < manifest.user_count OR
     invalid_roles <> 0 THEN
    RAISE EXCEPTION USING MESSAGE = format(
      'Synthetic verification failed users=%s homes=%s active=%s historical=%s audits=%s roles=%s invalid_roles=%s',
      actual_users, actual_homes, actual_active_children,
      actual_historical_children, actual_seed_audits, actual_roles, invalid_roles
    );
  END IF;
END
$$;

SELECT 'synthetic dataset verification passed' AS result;
