DROP TABLE IF EXISTS child_risk_targets;
DROP TABLE IF EXISTS child_risk_goals;
DROP TABLE IF EXISTS child_risks;

ALTER TABLE child_targets
  DROP CONSTRAINT IF EXISTS uq_child_targets_id_child_id;

ALTER TABLE child_goals
  DROP CONSTRAINT IF EXISTS uq_child_goals_id_child_id;
