-- internal/migrations/012_create_domain_checkins.up.sql

CREATE TABLE IF NOT EXISTS domain_checkins (
    id             BIGSERIAL PRIMARY KEY,
    home_name      TEXT NOT NULL,
    child_initials TEXT,
    date           TIMESTAMPTZ NOT NULL,
    context        TEXT,

    safety         INT,
    relationships  INT,
    health         INT,
    education      INT,
    identity       INT,
    coping         INT,

    notes          TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_domain_checkins_home_child_date
    ON domain_checkins (home_name, child_initials, date);