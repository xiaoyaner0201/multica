-- Per-agent automatic retry for model-provider failures (GitHub #10815).
--
-- provider_retry_max is how many times a run that failed with
-- agent_error.provider_capacity_or_rate_limit or
-- agent_error.provider_server_error may be retried in a row before the failure
-- becomes final. 0 (the default) disables the feature, so every existing agent
-- keeps failing fast exactly as before. The count restarts with every new
-- run that is not itself an automatic retry (a new comment, assignment, or
-- manual rerun), so a success always resets it.
--
-- provider_retry_interval_seconds is the fixed wait before each such retry.
-- It only matters while provider_retry_max > 0.
--
-- Ranges (max 0..20, interval 30..86400) are validated in the handler, as for
-- max_concurrent_tasks; there is no CHECK constraint so the ALTER stays a
-- catalog-only change with no table scan. Read by primary key only, so no
-- index.
ALTER TABLE agent
    ADD COLUMN IF NOT EXISTS provider_retry_max INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS provider_retry_interval_seconds INTEGER NOT NULL DEFAULT 1800;
