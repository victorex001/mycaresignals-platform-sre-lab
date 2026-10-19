-- Rollback for 001_init.sql

-- Drop tables created in 001
DROP TABLE IF EXISTS feedback;

-- Optional: remove extension (only if you want to fully undo it)
DROP EXTENSION IF EXISTS pgcrypto;