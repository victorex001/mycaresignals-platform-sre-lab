DROP TRIGGER IF EXISTS trg_child_care_plan_review_sections_immutable ON child_care_plan_review_sections;
DROP TRIGGER IF EXISTS trg_child_care_plan_reviews_immutable ON child_care_plan_reviews;
DROP TABLE IF EXISTS child_care_plan_review_sections;
DROP TABLE IF EXISTS child_care_plan_reviews;
DROP FUNCTION IF EXISTS prevent_child_care_plan_review_mutation();

DROP INDEX IF EXISTS idx_child_care_plans_predecessor;
ALTER TABLE child_care_plans
  DROP CONSTRAINT IF EXISTS chk_child_care_plans_predecessor_not_self,
  DROP CONSTRAINT IF EXISTS fk_child_care_plans_predecessor_child,
  DROP COLUMN IF EXISTS predecessor_care_plan_id;
