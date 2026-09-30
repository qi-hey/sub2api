-- Preserve per-account overrides and unrestricted empty mappings.
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{model_mapping}',
    jsonb_build_object('gpt-6.1-sol', 'gpt-6.1-sol') ||
        (credentials->'model_mapping'),
    true
)
WHERE platform = 'openai'
  AND type IN ('oauth', 'setup-token')
  AND deleted_at IS NULL
  AND jsonb_typeof(credentials->'model_mapping') = 'object'
  AND credentials->'model_mapping' <> '{}'::jsonb
  AND NOT (credentials->'model_mapping' ? 'gpt-6.1-sol');
