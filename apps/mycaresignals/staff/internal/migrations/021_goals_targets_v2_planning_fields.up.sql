ALTER TABLE child_goals
  ADD COLUMN IF NOT EXISTS domain TEXT;

ALTER TABLE child_targets
  ADD COLUMN IF NOT EXISTS review_date DATE;
