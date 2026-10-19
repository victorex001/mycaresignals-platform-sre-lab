-- 015_child_signal_responses.up.sql

CREATE TABLE child_signal_responses (
    id                BIGSERIAL PRIMARY KEY,
    child_id          BIGINT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
    signal_kind       TEXT   NOT NULL, -- e.g. 'risk', 'emotional_behaviour', 'overall'

    did_checkin       BOOLEAN NOT NULL DEFAULT FALSE,
    adjusted_support  BOOLEAN NOT NULL DEFAULT FALSE,
    informed_manager  BOOLEAN NOT NULL DEFAULT FALSE,
    informed_network  BOOLEAN NOT NULL DEFAULT FALSE,
    monitor_only      BOOLEAN NOT NULL DEFAULT FALSE,

    actions_summary   TEXT,
    future_notes      TEXT,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_child_signal_responses_child_created
    ON child_signal_responses (child_id, created_at DESC);