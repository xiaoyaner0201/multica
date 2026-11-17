ALTER TABLE agent
    DROP COLUMN IF EXISTS provider_retry_interval_seconds,
    DROP COLUMN IF EXISTS provider_retry_max;
