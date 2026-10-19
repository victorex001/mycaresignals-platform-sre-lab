ALTER TABLE child_signal_responses
    ADD COLUMN did_referral BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN referral_destinations TEXT NOT NULL DEFAULT '',
    ADD COLUMN external_log_ref TEXT NOT NULL DEFAULT '';