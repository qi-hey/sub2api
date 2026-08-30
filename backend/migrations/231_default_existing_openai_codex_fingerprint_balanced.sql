-- Move every existing OpenAI OAuth-like account to the account-balanced
-- fingerprint policy once. Administrators can still switch individual
-- accounts off after this migration; the migration is not rerun.
UPDATE accounts
SET extra = jsonb_set(
    COALESCE(extra, '{}'::jsonb),
    '{codex_fingerprint_mode}',
    '"session"'::jsonb,
    true
)
WHERE deleted_at IS NULL
  AND platform = 'openai'
  AND type IN ('oauth', 'setup-token')
  AND COALESCE(extra->>'codex_fingerprint_mode', '') <> 'session';

-- Every enabled account needs one stable system-managed seed. Preserve valid
-- existing seeds so the migration does not rotate an account's identity.
UPDATE accounts
SET extra = jsonb_set(
    COALESCE(extra, '{}'::jsonb),
    '{codex_fingerprint_seed}',
    to_jsonb(gen_random_uuid()::text),
    true
)
WHERE deleted_at IS NULL
  AND platform = 'openai'
  AND type IN ('oauth', 'setup-token')
  AND (
      extra->>'codex_fingerprint_seed' IS NULL
      OR btrim(extra->>'codex_fingerprint_seed') = ''
      OR NOT (
          extra->>'codex_fingerprint_seed' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
          AND extra->>'codex_fingerprint_seed' <> '00000000-0000-0000-0000-000000000000'
      )
  );
