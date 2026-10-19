CREATE TABLE psychological_reviews (
    id BIGSERIAL PRIMARY KEY,
    child_id BIGINT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
    window_days INT NOT NULL DEFAULT 28,
    review_from DATE NOT NULL,
    review_to DATE NOT NULL,
    formulation TEXT NOT NULL DEFAULT '',
    protective_factors TEXT NOT NULL DEFAULT '',
    recommendations TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT 'psychologist'
);

CREATE INDEX idx_psych_reviews_child_created_at
    ON psychological_reviews(child_id, created_at DESC);