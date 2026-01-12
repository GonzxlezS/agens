DROP TRIGGER IF EXISTS enforce_message_limit ON history;
DROP FUNCTION IF EXISTS limit_messages_per_conversation();

DROP TABLE IF EXISTS history_agent_limits;

DROP INDEX IF EXISTS idx_history_created_at;
DROP INDEX IF EXISTS idx_history_agent_context;

DROP TABLE IF EXISTS history;