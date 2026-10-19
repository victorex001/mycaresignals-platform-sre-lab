-- +goose Up
CREATE TABLE therapeutic_profiles (
    id              BIGSERIAL PRIMARY KEY,
    child_id        BIGINT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
    version         INT NOT NULL DEFAULT 1,
    source          TEXT NOT NULL,   -- 'existing_child' | 'new_referral'
    status          TEXT NOT NULL,   -- 'draft' | 'approved'
    profile_json    JSONB NOT NULL,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      TEXT,
    approved_at     TIMESTAMPTZ,
    approved_by     TEXT
);

CREATE INDEX idx_therapeutic_profiles_child_id
    ON therapeutic_profiles (child_id);

CREATE INDEX idx_therapeutic_profiles_child_status
    ON therapeutic_profiles (child_id, status);

-- +goose Down
DROP TABLE IF EXISTS therapeutic_profiles;
