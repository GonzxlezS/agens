package pgmemory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gonzxlezs/agens"

	"github.com/firebase/genkit/go/ai"
)

const (
	HistoryTableQuery = `CREATE TABLE IF NOT EXISTS history (
    id SERIAL PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    message JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    )`

	HistorySourceIndexQuery = `CREATE INDEX IF NOT EXISTS idx_history_conversation_id ON history(conversation_id)`

	HistoryTimeIndexQuery = `CREATE INDEX IF NOT EXISTS idx_history_created_at ON history(created_at ASC)`

	RetrieveHistory = `SELECT id, message
    FROM history
    WHERE conversation_id = $1
    ORDER BY created_at ASC`

	DeleteHistory = `DELETE FROM history WHERE conversation_id = $1`

	HistoryTrigger = `DROP TRIGGER IF EXISTS enforce_message_limit ON history;
CREATE TRIGGER enforce_message_limit
AFTER INSERT ON history
FOR EACH ROW
EXECUTE FUNCTION limit_messages_per_conversation();`

	HistoryTriggerFunctionFormat = `CREATE OR REPLACE FUNCTION limit_messages_per_conversation()
RETURNS TRIGGER AS $$
DECLARE
    message_count INTEGER;
    message_limit INTEGER := %d;
BEGIN
    -- Get the current number of messages for the conversation
    SELECT COUNT(*) INTO message_count
    FROM history
    WHERE conversation_id = NEW.conversation_id;

    -- If the count exceeds the limit
    IF message_count > message_limit THEN
        -- Delete the oldest messages
        DELETE FROM history
        WHERE conversation_id = NEW.conversation_id
        AND id IN (
            SELECT id
            FROM history
            WHERE conversation_id = NEW.conversation_id
            ORDER BY created_at ASC -- Order by oldest messages first
            LIMIT (message_count - message_limit) -- Limit to the exact number of excess messages
        );
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;`
)

var ErrDBNotInitialized = errors.New("database connection not initialized")

var _ agens.HistoryMemory = &HistoryMemory{}

type HistoryMemory struct {
	db *sql.DB
}

func NewHistoryMemory(ctx context.Context, db *sql.DB) (*HistoryMemory, error) {
	memory := &HistoryMemory{db: db}
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return memory, memory.Init(ctx)
}

func (m *HistoryMemory) Init(ctx context.Context) error {
	if m.db == nil {
		return ErrDBNotInitialized
	}

	if _, err := m.db.ExecContext(ctx, HistoryTableQuery); err != nil {
		return fmt.Errorf("error creating history table: %w", err)
	}

	if _, err := m.db.ExecContext(ctx, HistorySourceIndexQuery); err != nil {
		return fmt.Errorf("error creating history conversation index: %w", err)
	}

	if _, err := m.db.ExecContext(ctx, HistoryTimeIndexQuery); err != nil {
		return fmt.Errorf("error creating history time index: %w", err)
	}

	return nil
}

func (m *HistoryMemory) RetrieveHistory(ctx context.Context, conversationID string) ([]*ai.Message, error) {
	rows, err := m.db.QueryContext(ctx, RetrieveHistory, conversationID)
	if err != nil {
		return nil, fmt.Errorf("error querying history: %w", err)
	}
	defer rows.Close()

	var messages []*ai.Message
	for rows.Next() {
		var (
			storedID int64
			msgJSON  []byte
		)

		if err := rows.Scan(&storedID, &msgJSON); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		var msg ai.Message
		if err := json.Unmarshal(msgJSON, &msg); err != nil {
			return nil, fmt.Errorf("error unmarshaling message: %w", err)
		}

		msg = *agens.SetStoredID(
			&msg,
			strconv.FormatInt(storedID, 10),
		)

		messages = append(messages, &msg)
	}

	return messages, nil
}

func (m *HistoryMemory) StoreHistory(ctx context.Context, conversationID string, history []*ai.Message) error {
	var filtered []*ai.Message
	for _, msg := range history {
		// Skip system messages and those that have already been stored.
		if msg.Role != ai.RoleSystem {
			storedID, err := agens.GetStoredID(msg)
			if err != nil {
				return err
			} else if storedID != "" {
				continue
			}

			filtered = append(filtered, msg)
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	var (
		vStrings []string
		vArgs    []any
	)
	for i, msg := range filtered {
		msgJSON, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("error serializing message: %w", err)
		}

		vStrings = append(vStrings, fmt.Sprintf("($%d, $%d)", (i*2)+1, (i*2)+2))
		vArgs = append(vArgs, conversationID, msgJSON)
	}

	stmt := fmt.Sprintf(
		"INSERT INTO history (conversation_id, message) VALUES %s",
		strings.Join(vStrings, ", "),
	)

	if _, err := tx.ExecContext(ctx, stmt, vArgs...); err != nil {
		return fmt.Errorf("error inserting history: %w", err)
	}
	return tx.Commit()
}

func (m *HistoryMemory) DeleteHistory(ctx context.Context, conversationID string) error {
	_, err := m.db.ExecContext(ctx, DeleteHistory, conversationID)
	if err != nil {
		return fmt.Errorf("error deleting history: %w", err)
	}
	return nil
}

func (m *HistoryMemory) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

func (m *HistoryMemory) SetMaxMessagesPerConversation(ctx context.Context, max int) error {
	if m.db == nil {
		return ErrDBNotInitialized
	}

	historyTriggerFunction := fmt.Sprintf(HistoryTriggerFunctionFormat, max)
	if _, err := m.db.ExecContext(ctx, historyTriggerFunction); err != nil {
		return fmt.Errorf("error creating function: %w", err)
	}

	if _, err := m.db.ExecContext(ctx, HistoryTrigger); err != nil {
		return fmt.Errorf("error creating trigger: %w", err)
	}

	return nil
}
