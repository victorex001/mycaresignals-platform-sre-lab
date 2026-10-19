ALTER TABLE child_targets
  DROP COLUMN IF EXISTS review_date;

ALTER TABLE child_goals
  DROP COLUMN IF EXISTS domain;
