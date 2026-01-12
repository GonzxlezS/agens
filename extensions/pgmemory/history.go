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
	RetrieveHistoryQuery = `SELECT id, message
    FROM history
    WHERE agent_name = $1
		AND conversation_id = $2
    ORDER BY created_at ASC`

	DeleteHistoryQuery = `DELETE FROM history WHERE agent_name = $1 AND conversation_id = $2`

	SetMaxMessagesPerConversationQuery = `INSERT INTO history_agent_limits (
	agent_name, 
	max_msgs_conversation
	) VALUES ($1, $2)
		ON CONFLICT (agent_name)
		DO UPDATE SET 
    		max_msgs_conversation = EXCLUDED.max_msgs_conversation;`
)

var ErrDBNotInitialized = errors.New("pgmemory: database connection not initialized")

var _ agens.HistoryProvider = &HistoryProvider{}
var _ agens.HistoryMemory = &historyMemory{}

type HistoryProvider struct {
	db *sql.DB
}

func NewHistoryProvider(db *sql.DB) (*HistoryProvider, error) {
	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := runModuleMigration(db, "history", "migrations_history"); err != nil {
		return nil, fmt.Errorf("history migrations failed: %w", err)
	}

	return &HistoryProvider{db: db}, nil
}

func (p *HistoryProvider) ForAgent(agentName string, maxMessages int) (agens.HistoryMemory, error) {
	err := p.setMaxMessagesPerConversation(agentName, maxMessages)
	if err != nil {
		return nil, err
	}
	return &historyMemory{provider: p, agentName: agentName}, nil
}

func (p *HistoryProvider) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func (p *HistoryProvider) setMaxMessagesPerConversation(agentName string, max int) error {
	if p.db == nil {
		return ErrDBNotInitialized
	}

	if _, err := p.db.Exec(SetMaxMessagesPerConversationQuery, agentName, max); err != nil {
		return fmt.Errorf("error set max messages: %w", err)
	}

	return nil
}

func (p *HistoryProvider) deleteHistory(ctx context.Context, agentName string, conversationID string) error {
	if p.db == nil {
		return ErrDBNotInitialized
	}

	_, err := p.db.ExecContext(ctx, DeleteHistoryQuery, agentName, conversationID)
	if err != nil {
		return fmt.Errorf("error deleting history: %w", err)
	}
	return nil
}

func (p *HistoryProvider) retrieveHistory(ctx context.Context, agentName string, conversationID string) ([]*ai.Message, error) {
	if p.db == nil {
		return nil, ErrDBNotInitialized
	}

	rows, err := p.db.QueryContext(ctx, RetrieveHistoryQuery, agentName, conversationID)
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

	return messages, rows.Err()
}

func (p *HistoryProvider) storeHistory(ctx context.Context, agentName string, conversationID string, history []*ai.Message) error {
	if p.db == nil {
		return ErrDBNotInitialized
	}

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

	tx, err := p.db.BeginTx(ctx, nil)
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

		vStrings = append(vStrings, fmt.Sprintf("($%d, $%d, $%d)", (i*3)+1, (i*3)+2, (i*3)+3))
		vArgs = append(vArgs, agentName, conversationID, msgJSON)
	}

	stmt := fmt.Sprintf(
		"INSERT INTO history (agent_name, conversation_id, message) VALUES %s",
		strings.Join(vStrings, ", "),
	)

	if _, err := tx.ExecContext(ctx, stmt, vArgs...); err != nil {
		return fmt.Errorf("error inserting history: %w", err)
	}
	return tx.Commit()
}

type historyMemory struct {
	provider  *HistoryProvider
	agentName string
}

func (m *historyMemory) DeleteHistory(ctx context.Context, conversationID string) error {
	return m.provider.deleteHistory(ctx, m.agentName, conversationID)
}

func (m *historyMemory) RetrieveHistory(ctx context.Context, conversationID string) ([]*ai.Message, error) {
	return m.provider.retrieveHistory(ctx, m.agentName, conversationID)
}

func (m *historyMemory) StoreHistory(ctx context.Context, conversationID string, history []*ai.Message) error {
	return m.provider.storeHistory(ctx, m.agentName, conversationID, history)
}

func (_ *historyMemory) Close() error {
	return nil
}
