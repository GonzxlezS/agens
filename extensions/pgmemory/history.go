package pgmemory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gonzxlezs/agens"

	"github.com/firebase/genkit/go/ai"
)

const (
	CreateHistoryTableQueryFormat = `CREATE TABLE IF NOT EXISTS %s (
		id SERIAL PRIMARY KEY,
  		agent_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		message JSONB NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`

	CreateHistoryAgentSessionIndexQueryFormat = `CREATE INDEX IF NOT EXISTS idx_history_agent_session_created ON %s (agent_id, session_id, created_at)`
)

const (
	CheckHistorySchemaQueryFormat = `DO $$
	DECLARE
		mismatch RECORD;
	BEGIN
		FOR mismatch IN
			WITH expected_layout AS (
				SELECT * FROM (VALUES
					('id', 'int4'),
					('agent_id', 'text'),
					('session_id', 'text'),
					('message', 'jsonb'),
					('created_at', 'timestamptz')
				) AS t(col_name, col_type)
			),
			current_layout AS (
				SELECT column_name, udt_name
				FROM information_schema.columns
				WHERE table_name = '%s' AND table_schema = 'public'
			)
			SELECT
				e.col_name,
				CASE
					WHEN c.column_name IS NULL
						THEN 'MISSING COLUMN'
					WHEN e.col_type != c.udt_name
						THEN 'INVALID TYPE (expected ' || e.col_type || ', found ' || c.udt_name || ')'
					ELSE NULL
				END as error_msg
			FROM expected_layout e
			LEFT JOIN current_layout c ON e.col_name = c.column_name
			WHERE c.column_name IS NULL OR e.col_type != c.udt_name
		LOOP
			RAISE EXCEPTION '%%: %%', mismatch.col_name, mismatch.error_msg;
		END LOOP;
	END $$;`
)

const (
	DeleteHistoryQueryFormat = `DELETE FROM %s WHERE agent_id = $1 AND session_id = $2`

	RetrieveHistoryQueryFormat = `SELECT id, message
    FROM %s
    WHERE agent_id = $1
		AND session_id = $2
    ORDER BY created_at ASC`

	StoreHistoryQueryFormat = `INSERT INTO %s (agent_id, session_id, message) VALUES %s`

	DeleteOldMessagesQueryFormat = `DELETE FROM %s
		WHERE id IN (
			SELECT id FROM %s
			WHERE agent_id = $1 AND session_id = $2
			ORDER BY created_at DESC
			OFFSET $3
		)`
)

const StoredIDKey = "stored_id"

var (
	ErrDBNotInitialized = errors.New("pgmemory: database connection not initialized")

	ErrStoredIDNotAnInt64 = errors.New("pgmemory: stored ID is not of type int64")

	ErrEmptyTableName = errors.New("pgmemory: table name cannot be empty")
)

var _ agens.HistoryMemory = &HistoryMemory{}

type HistoryMemory struct {
	tableName             string
	db                    *sql.DB
	stmtDelete            *sql.Stmt
	stmtRetrieve          *sql.Stmt
	stmtDeleteOldMessages *sql.Stmt
}

func NewHistoryMemory(tableName string, db *sql.DB) (*HistoryMemory, error) {
	tableName = strings.TrimSpace(tableName)
	if tableName == "" {
		return nil, ErrEmptyTableName
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// transaction
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	// table
	query := fmt.Sprintf(CreateHistoryTableQueryFormat, tableName)
	if _, err := tx.Exec(query); err != nil {
		return nil, fmt.Errorf("error creating table: %w", err)
	}

	// check
	query = fmt.Sprintf(CheckHistorySchemaQueryFormat, tableName)
	if _, err := tx.Exec(query); err != nil {
		return nil, err
	}

	// idx
	query = fmt.Sprintf(CreateHistoryAgentSessionIndexQueryFormat, tableName)
	if _, err := tx.Exec(query); err != nil {
		return nil, fmt.Errorf("error creating index: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// statements
	stmtDelete, err := db.Prepare(fmt.Sprintf(DeleteHistoryQueryFormat, tableName))
	if err != nil {
		return nil, err
	}

	stmtRetrieve, err := db.Prepare(fmt.Sprintf(RetrieveHistoryQueryFormat, tableName))
	if err != nil {
		return nil, err
	}

	stmtDeleteOldMessages, err := db.Prepare(fmt.Sprintf(DeleteOldMessagesQueryFormat, tableName, tableName))
	if err != nil {
		return nil, err
	}

	return &HistoryMemory{
		tableName:             tableName,
		db:                    db,
		stmtDelete:            stmtDelete,
		stmtRetrieve:          stmtRetrieve,
		stmtDeleteOldMessages: stmtDeleteOldMessages,
	}, nil
}

func (m *HistoryMemory) DeleteHistory(ctx context.Context, agentID string, sessionID string) error {
	if m.db == nil {
		return ErrDBNotInitialized
	}

	_, err := m.stmtDelete.ExecContext(ctx, agentID, sessionID)
	if err != nil {
		return fmt.Errorf("error deleting history: %w", err)
	}
	return nil
}

func (m *HistoryMemory) RetrieveHistory(ctx context.Context, agentID string, sessionID string) ([]*ai.Message, error) {
	if m.db == nil {
		return nil, ErrDBNotInitialized
	}

	rows, err := m.stmtRetrieve.QueryContext(ctx, agentID, sessionID)
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

		msg = *SetStoredID(&msg, storedID)

		messages = append(messages, &msg)
	}

	return messages, rows.Err()
}

func (m *HistoryMemory) StoreHistory(ctx context.Context, agentID string, sessionID string, history []*ai.Message, maxMessages int) error {
	if m.db == nil {
		return ErrDBNotInitialized
	}

	// filter
	var filtered []*ai.Message
	for _, msg := range history {
		// Skip system messages and those that have already been stored.
		if msg.Role != ai.RoleSystem {
			storedID, err := GetStoredID(msg)
			if err != nil {
				return err
			} else if storedID != 0 {
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

	// messages
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
		vArgs = append(vArgs, agentID, sessionID, msgJSON)
	}

	query := fmt.Sprintf(StoreHistoryQueryFormat, m.tableName, strings.Join(vStrings, ", "))
	if _, err := tx.ExecContext(ctx, query, vArgs...); err != nil {
		return fmt.Errorf("error inserting history: %w", err)
	}

	// maxMessages
	_, err = tx.StmtContext(ctx, m.stmtDeleteOldMessages).Exec(agentID, sessionID, maxMessages)
	if err != nil {
		return fmt.Errorf("error deleting old messages: %w", err)
	}
	return tx.Commit()
}

// GetStoredID retrieves the unique stored identifier from a message's metadata.
func GetStoredID(msg *ai.Message) (int64, error) {
	if (msg == nil) || (msg.Metadata == nil) {
		return 0, nil
	}

	v := msg.Metadata[StoredIDKey]
	if id, ok := v.(int64); ok {
		return id, nil
	}
	return 0, ErrStoredIDNotAnInt64
}

// SetStoredID sets the unique stored identifier in a message's metadata.
func SetStoredID(msg *ai.Message, id int64) *ai.Message {
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]any)
	}

	msg.Metadata[StoredIDKey] = id
	return msg
}
