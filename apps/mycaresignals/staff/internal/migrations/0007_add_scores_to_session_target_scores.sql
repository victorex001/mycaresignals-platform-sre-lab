-- +goose Up
ALTER TABLE session_target_scores
    ADD COLUMN wellbeing_score integer NOT NULL DEFAULT 0,
    ADD COLUMN progress_score  integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE session_target_scores
    DROP COLUMN IF EXISTS wellbeing_score,
    DROP COLUMN IF EXISTS progress_score;