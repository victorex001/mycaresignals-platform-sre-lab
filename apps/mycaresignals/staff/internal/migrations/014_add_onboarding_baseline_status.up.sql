-- 014_add_onboarding_baseline_status.up.sql

ALTER TABLE child_onboarding_baselines
    ADD COLUMN status TEXT NOT NULL DEFAULT 'submitted', -- draft | submitted | approved
    ADD COLUMN approved_by TEXT,
    ADD COLUMN approved_at TIMESTAMPTZ;