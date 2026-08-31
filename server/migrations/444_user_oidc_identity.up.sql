-- Stable OIDC identities. Email is retained as the last observed claim for
-- operator diagnostics, but account lookup is always based on issuer + subject.
-- Relationship cleanup is owned by the application; this table intentionally
-- has no foreign key.
--
-- IF NOT EXISTS makes a retry safe when table creation committed but recording
-- this migration in schema_migrations did not. The catalog check fails closed
-- if an unrelated or partially-created relation already owns the name.
CREATE TABLE IF NOT EXISTS user_oidc_identity (
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL,
    user_id UUID NOT NULL,
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

DO $$
DECLARE
    compatible BOOLEAN;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_class AS relation
        JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
        WHERE namespace.nspname = current_schema()
          AND relation.relname = 'user_oidc_identity'
          AND relation.relkind IN ('r', 'p')
    ) THEN
        RAISE EXCEPTION USING
            ERRCODE = '55000',
            MESSAGE = 'existing user_oidc_identity relation is not a table';
    END IF;

    SELECT
        count(*) = 6
        AND count(*) FILTER (
            WHERE column_name = 'issuer'
              AND data_type = 'text'
              AND is_nullable = 'NO'
              AND column_default IS NULL
        ) = 1
        AND count(*) FILTER (
            WHERE column_name = 'subject'
              AND data_type = 'text'
              AND is_nullable = 'NO'
              AND column_default IS NULL
        ) = 1
        AND count(*) FILTER (
            WHERE column_name = 'user_id'
              AND data_type = 'uuid'
              AND is_nullable = 'NO'
              AND column_default IS NULL
        ) = 1
        AND count(*) FILTER (
            WHERE column_name = 'email'
              AND data_type = 'text'
              AND is_nullable = 'NO'
              AND column_default IS NULL
        ) = 1
        AND count(*) FILTER (
            WHERE column_name = 'created_at'
              AND data_type = 'timestamp with time zone'
              AND is_nullable = 'NO'
              AND column_default IS NOT NULL
        ) = 1
        AND count(*) FILTER (
            WHERE column_name = 'updated_at'
              AND data_type = 'timestamp with time zone'
              AND is_nullable = 'NO'
              AND column_default IS NOT NULL
        ) = 1
    INTO compatible
    FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'user_oidc_identity';

    IF NOT compatible THEN
        RAISE EXCEPTION USING
            ERRCODE = '55000',
            MESSAGE = 'existing user_oidc_identity table has an incompatible schema';
    END IF;
END
$$;
