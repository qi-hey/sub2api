-- Grok accounts are provider-limited to two concurrent requests. Values above
-- two risk upstream account suspension, while non-positive values may be
-- interpreted as unlimited by older application paths.
UPDATE accounts
SET concurrency = 2,
    updated_at = NOW()
WHERE platform = 'grok'
  AND deleted_at IS NULL
  AND (concurrency <= 0 OR concurrency > 2);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'accounts_grok_concurrency_safe'
          AND conrelid = 'accounts'::regclass
    ) THEN
        ALTER TABLE accounts
            ADD CONSTRAINT accounts_grok_concurrency_safe
            CHECK (deleted_at IS NOT NULL OR platform <> 'grok' OR concurrency BETWEEN 1 AND 2)
            NOT VALID;
    END IF;
END
$$;

ALTER TABLE accounts
    VALIDATE CONSTRAINT accounts_grok_concurrency_safe;
