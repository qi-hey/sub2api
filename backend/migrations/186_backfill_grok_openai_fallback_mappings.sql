-- Add the OpenAI-compatible aliases used by the OpenAI-to-Grok fallback.
-- Existing account-specific mappings win over these defaults.
UPDATE accounts
SET credentials = jsonb_set(
    COALESCE(credentials, '{}'::jsonb),
    '{model_mapping}',
    '{
      "claude-opus-4-8": "grok-4.5",
      "gpt-5.2": "grok-4.5",
      "gpt-5.4": "grok-4.5",
      "gpt-5.4-mini": "grok-4.5",
      "gpt-5.5": "grok-4.5",
      "gpt-5.6-luna": "grok-4.5",
      "gpt-5.6-sol": "grok-4.5",
      "gpt-5.6-terra": "grok-4.5",
      "grok-4.5": "grok-4.5"
    }'::jsonb || COALESCE(credentials->'model_mapping', '{}'::jsonb),
    true
)
WHERE platform = 'grok'
  AND deleted_at IS NULL;
