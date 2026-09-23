-- Add GPT-6 Sol and Luna to existing official OpenAI OAuth-like account mappings.
-- Existing account-specific mappings win over these defaults.
UPDATE accounts
SET credentials = jsonb_set(
    COALESCE(credentials, '{}'::jsonb),
    '{model_mapping}',
    '{
        "gpt-6-sol": "gpt-6-sol",
        "gpt-6-luna": "gpt-6-luna"
    }'::jsonb ||
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
  AND (
      COALESCE(credentials->'model_mapping'->>'gpt-6-sol', '') = ''
      OR COALESCE(credentials->'model_mapping'->>'gpt-6-luna', '') = ''
  );
