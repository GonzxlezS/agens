CREATE TABLE IF NOT EXISTS history (
  id SERIAL PRIMARY KEY,
  agent_name TEXT NOT NULL,
  conversation_id TEXT NOT NULL,
  message JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_history_agent_context ON history (agent_name, conversation_id);

CREATE INDEX IF NOT EXISTS idx_history_created_at ON history (created_at ASC);

CREATE TABLE IF NOT EXISTS history_agent_limits (
  agent_name TEXT PRIMARY KEY,
  max_msgs_conversation INTEGER NOT NULL
);

-- limit_messages_per_conversation
CREATE OR REPLACE FUNCTION limit_messages_per_conversation () RETURNS TRIGGER AS $$
DECLARE message_count INTEGER;
message_limit INTEGER;
BEGIN
SELECT COUNT(*) INTO message_count
FROM history
WHERE agent_name = NEW.agent_name
    AND conversation_id = NEW.conversation_id;
SELECT COALESCE(max_msgs_conversation, 10) INTO message_limit
FROM history_agent_limits
WHERE agent_name = NEW.agent_name;
-- If the count exceeds the limit
IF message_count > message_limit THEN -- Delete the oldest messages
DELETE FROM history
WHERE id IN (
        SELECT id
        FROM history
        WHERE agent_name = NEW.agent_name
            AND conversation_id = NEW.conversation_id
        ORDER BY created_at ASC -- Order by oldest messages first
        LIMIT (message_count - message_limit) -- Limit to the exact number of excess messages
    );
END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql;


-- enforce_message_limit
DROP TRIGGER IF EXISTS enforce_message_limit ON history;

CREATE TRIGGER enforce_message_limit
AFTER INSERT ON history FOR EACH ROW
EXECUTE FUNCTION limit_messages_per_conversation ();