-- 001_init_children.sql
CREATE TABLE homes (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    external_ref TEXT,                -- e.g. your existing home id
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE children (
    id              BIGSERIAL PRIMARY KEY,
    home_id         BIGINT NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
    external_ref    TEXT,               -- ClearCare / LA ID
    initials        TEXT NOT NULL,
    first_name      TEXT,
    last_name       TEXT,
    date_of_birth   DATE NOT NULL,
    legal_status    TEXT,               -- CLA, ICO, etc.
    admission_date  DATE,
    gender          TEXT,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_children_home_id ON children(home_id);
