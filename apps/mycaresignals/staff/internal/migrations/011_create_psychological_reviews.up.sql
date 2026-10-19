-- 011_create_psychological_reviews.up.sql

CREATE TABLE IF NOT EXISTS psychological_reviews (
    id BIGSERIAL PRIMARY KEY,
    child_id BIGINT NOT NULL REFERENCES children(id) ON DELETE CASCADE,

    -- how many days the review window covers (e.g. 42)
    window_days INTEGER NOT NULL DEFAULT 28,

    -- review period
    review_from DATE NOT NULL,
    review_to   DATE NOT NULL,

    -- psychologist's narrative
    formulation         TEXT NOT NULL,
    protective_factors  TEXT,
    recommendations     TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by TEXT NOT NULL
);