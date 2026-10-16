-- Run only after a complete backup and restored-database rehearsal.
-- Existing aliases remain valid; only 4.5 targets and the 4.7 identity change.
BEGIN;
-- Older downstream deployments installed this trigger outside migrations.
-- Fill missing defaults without overwriting intentional account mappings.
CREATE OR REPLACE FUNCTION public.ensure_grok_default_model_mapping()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    desired jsonb := '{"gpt-5.6-terra":"grok-4.7","gpt-5.6-luna":"grok-4.7",
        "gpt-5.6-sol":"grok-4.7","gpt-5.4":"grok-4.7","gpt-5.4-mini":"grok-4.7",
        "gpt-5.5":"grok-4.7","gpt-5.2":"grok-4.7","claude-opus-4-8":"grok-4.7",
        "grok-4.5":"grok-4.7","grok-4.7":"grok-4.7"}'::jsonb;
    current_mapping jsonb;
BEGIN
    IF NEW.platform IS DISTINCT FROM 'grok' OR NEW.deleted_at IS NOT NULL THEN
        RETURN NEW;
    END IF;
    NEW.credentials := COALESCE(NEW.credentials,'{}'::jsonb);
    current_mapping := CASE WHEN jsonb_typeof(NEW.credentials->'model_mapping') = 'object'
        THEN NEW.credentials->'model_mapping' ELSE '{}'::jsonb END;
    NEW.credentials := jsonb_set(NEW.credentials,'{model_mapping}',desired || current_mapping,true);
    RETURN NEW;
END;
$$;
WITH planned AS (
    SELECT id,
           COALESCE((SELECT jsonb_object_agg(key,
               CASE WHEN value IN ('"grok-4.5"'::jsonb, '"gork-4.5"'::jsonb)
                    THEN '"grok-4.7"'::jsonb ELSE value END)
             FROM jsonb_each(CASE WHEN jsonb_typeof(credentials->'model_mapping') = 'object'
                                 THEN credentials->'model_mapping' ELSE '{}'::jsonb END)),
                    '{}'::jsonb) || '{"grok-4.7":"grok-4.7"}'::jsonb AS mapping
    FROM accounts WHERE platform = 'grok' AND deleted_at IS NULL
), updated AS (
    UPDATE accounts a
       SET credentials = jsonb_set(COALESCE(a.credentials,'{}'::jsonb),
                                   '{model_mapping}', p.mapping),
           updated_at = NOW()
      FROM planned p
     WHERE a.id = p.id AND a.credentials->'model_mapping' IS DISTINCT FROM p.mapping
    RETURNING a.id
)
INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload)
SELECT 'account_changed',id,NULL,NULL FROM updated;
COMMIT;
