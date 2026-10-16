-- Run only after a complete backup and restored-database rehearsal.
-- Existing aliases remain valid; only 4.5 targets and the 4.7 identity change.
BEGIN;
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
