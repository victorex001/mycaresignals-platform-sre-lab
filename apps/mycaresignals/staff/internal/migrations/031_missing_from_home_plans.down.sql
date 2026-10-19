DROP TRIGGER IF EXISTS trg_mfh_approved_immutable ON child_missing_from_home_plans;
DROP FUNCTION IF EXISTS prevent_approved_mfh_plan_rewrite();
DROP TABLE IF EXISTS child_missing_from_home_plans;
