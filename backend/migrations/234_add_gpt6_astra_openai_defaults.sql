-- Add GPT-6 Astra to existing official OpenAI OAuth-like account mappings.
-- Existing account-specific Astra mappings win over this default.
UPDATE accounts
SET credentials = jsonb_set(
    COALESCE(credentials, '{}'::jsonb),
    '{model_mapping}',
    '{"gpt-6-astra": "gpt-6-astra"}'::jsonb ||
        CASE
            WHEN jsonb_typeof(credentials->'model_mapping') = 'object'
                THEN credentials->'model_mapping'
            ELSE '{}'::jsonb
        END,
    true
)
WHERE platform = 'openai'
  AND type IN ('oauth', 'setup-token')
  AND deleted_at IS NULL
  AND jsonb_typeof(credentials->'model_mapping') = 'object'
  AND credentials->'model_mapping' <> '{}'::jsonb
  AND COALESCE(credentials->'model_mapping'->>'gpt-6-astra', '') = '';
