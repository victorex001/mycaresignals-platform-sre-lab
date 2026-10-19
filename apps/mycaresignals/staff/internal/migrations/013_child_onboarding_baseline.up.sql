-- 013_child_onboarding_baseline.up.sql

CREATE TABLE child_onboarding_baselines (
    id                       BIGSERIAL PRIMARY KEY,
    child_id                 BIGINT NOT NULL REFERENCES children(id) ON DELETE CASCADE,

    -- narrative baseline
    summary_of_history       TEXT,
    recent_placement_pattern TEXT,
    current_concerns         TEXT,
    strengths                TEXT,
    existing_plans           TEXT,
    childs_voice             TEXT,

    -- relapse / regression
    relapse_risks            TEXT,
    relapse_early_signals    TEXT,
    relapse_support_plan     TEXT,

    -- review & coordination
    review_frequency         TEXT,
    next_review_date         DATE,
    lead_reviewer            TEXT,
    other_professionals      TEXT,
    early_warning_signals    TEXT,
    notes_for_team           TEXT,

    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE child_onboarding_domain_states (
    id          BIGSERIAL PRIMARY KEY,
    baseline_id BIGINT NOT NULL REFERENCES child_onboarding_baselines(id) ON DELETE CASCADE,
    domain_key  TEXT NOT NULL,   -- e.g. 'emotional_wellbeing', 'behaviour'
    active      BOOLEAN NOT NULL DEFAULT FALSE,
    reason      TEXT
);

CREATE TABLE child_priority_domains (
    id               BIGSERIAL PRIMARY KEY,
    baseline_id      BIGINT NOT NULL REFERENCES child_onboarding_baselines(id) ON DELETE CASCADE,
    domain_key       TEXT NOT NULL,
    focus_summary    TEXT,
    hoped_for_change TEXT,
    sort_order       INT NOT NULL
);