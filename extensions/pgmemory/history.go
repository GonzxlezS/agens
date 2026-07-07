package pgmemory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gonzxlezs/agens"

	"github.com/firebase/genkit/go/ai"
	"github.com/lib/pq"
)

const (
	CreateHistoryTableQueryFormat = `CREATE TABLE IF NOT EXISTS %s (
		id SERIAL PRIMARY KEY,
		agent_id TEXT NOT NULL,
		gateway_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		message JSONB NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`

	CreateHistoryAgentGatewaySessionIndexQueryFormat = `CREATE INDEX IF NOT EXISTS idx_%s_agent_gateway_session_order ON %s (agent_id, gateway_id, session_id, created_at ASC)`
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
					('gateway_id', 'text'),
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
	DeleteAgentHistoriesQueryFormat = `DELETE FROM %s WHERE agent_id = $1`

	DeleteGatewayHistoriesQueryFormat = `DELETE FROM %s
	WHERE agent_id = $1 AND gateway_id = $2`

	DeleteSessionHistoryQueryFormat = `DELETE FROM %s
	WHERE agent_id = $1 AND gateway_id = $2 AND session_id = $3`

	RetrieveHistoryQueryFormat = `SELECT id, message FROM %s
    WHERE agent_id = $1
    	AND gateway_id = $2
		AND session_id = $3
    ORDER BY created_at ASC`

	StoreHistoryQueryFormat = `INSERT INTO %s (agent_id, gateway_id, session_id, message) VALUES %s`

	DeleteOldMessagesQueryFormat = `DELETE FROM %s
	WHERE agent_id = $1
		AND gateway_id = $2
		AND session_id = $3
		AND id != ALL($4)`

	CopyInQueryFormat = `COPY %s (agent_id, gateway_id, session_id, message) FROM STDIN`

	LockQueryFormat = `SELECT pg_advisory_xact_lock(hashtext($1))`
)

var _ agens.HistoryMemory = &HistoryMemory{}

type HistoryMemoryConfig struct {
	TableName string
	DB        *sql.DB
	Timeout   time.Duration
	OwnsDB    bool
}

type HistoryMemory struct {
	cfg *HistoryMemoryConfig

	stmtDeleteHistories   *sql.Stmt
	stmtDeleteGateway     *sql.Stmt
	stmtDeleteSession     *sql.Stmt
	stmtRetrieve          *sql.Stmt
	stmtDeleteOldMessages *sql.Stmt
}

func NewHistoryMemory(cfg HistoryMemoryConfig) (*HistoryMemory, error) {
	tableName, err := historyTableName(cfg.TableName)
	if err != nil {
		return nil, err
	}
	cfg.TableName = tableName

	// ping
	if err := cfg.DB.Ping(); err != nil {
		return nil, err
	}

	// transaction
	tx, err := cfg.DB.Begin()
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
	query = fmt.Sprintf(CreateHistoryAgentGatewaySessionIndexQueryFormat, tableName, tableName)
	if _, err := tx.Exec(query); err != nil {
		return nil, fmt.Errorf("error creating index: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// timeout
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}

	// memory
	memory := &HistoryMemory{cfg: &cfg}

	if err := memory.prepareStmts(); err != nil {
		return nil, err
	}
	return memory, nil
}

// Close releases the resources of all prepared statements
// associated with HistoryMemory to prevent memory leaks and open database connections.
func (m *HistoryMemory) Close() error {
	if (m.cfg == nil) || (m.cfg.DB == nil) {
		return ErrDBNotInitialized
	}

	err := m.closeStmts()

	if m.cfg.OwnsDB {
		errDB := m.cfg.DB.Close()

		if errDB != nil {
			err = errors.Join(err, errDB)
		}
	}

	return err
}

func (m *HistoryMemory) DeleteAgentHistories(ctx context.Context, agentID string) error {
	if (m.cfg == nil) || (m.cfg.DB == nil) {
		return ErrDBNotInitialized
	}

	// Timeout
	dbCtx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()

	_, err := m.stmtDeleteHistories.ExecContext(dbCtx, agentID)
	if err != nil {
		return fmt.Errorf("error deleting history: %w", err)
	}
	return nil
}

func (m *HistoryMemory) DeleteGatewayHistories(ctx context.Context, agentID string, gatewayID string) error {
	if (m.cfg == nil) || (m.cfg.DB == nil) {
		return ErrDBNotInitialized
	}

	// Timeout
	dbCtx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()

	_, err := m.stmtDeleteGateway.ExecContext(dbCtx, agentID, gatewayID)
	if err != nil {
		return fmt.Errorf("error deleting history: %w", err)
	}
	return nil
}

func (m *HistoryMemory) DeleteHistory(ctx context.Context, agentID string, gatewayID string, sessionID string) error {
	if (m.cfg == nil) || (m.cfg.DB == nil) {
		return ErrDBNotInitialized
	}

	// Timeout
	dbCtx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()

	_, err := m.stmtDeleteSession.ExecContext(dbCtx, agentID, gatewayID, sessionID)
	if err != nil {
		return fmt.Errorf("error deleting history: %w", err)
	}
	return nil
}

func (m *HistoryMemory) RetrieveHistory(ctx context.Context, agentID string, gatewayID string, sessionID string) ([]*ai.Message, error) {
	if (m.cfg == nil) || (m.cfg.DB == nil) {
		return nil, ErrDBNotInitialized
	}

	// Timeout
	dbCtx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()

	tx, err := m.cfg.DB.BeginTx(dbCtx, nil)
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	// lock
	_, err = tx.ExecContext(dbCtx, LockQueryFormat, generateLockKey(agentID, gatewayID, sessionID))
	if err != nil {
		return nil, fmt.Errorf("error acquiring session advisory lock: %w", err)
	}

	// retrieve
	rows, err := tx.StmtContext(dbCtx, m.stmtRetrieve).Query(agentID, gatewayID, sessionID)
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

		messages = append(messages, SetStoredID(&msg, storedID))
	}

	return messages, rows.Err()
}

func (m *HistoryMemory) StoreHistory(ctx context.Context, agentID string, gatewayID string, sessionID string, history []*ai.Message) error {
	if (m.cfg == nil) || (m.cfg.DB == nil) {
		return ErrDBNotInitialized
	}

	// filter
	var (
		newMessages []*ai.Message
		keep        []int64
	)

	for _, msg := range history {
		// Skip system messages and those that have already been stored.
		if msg.Role != ai.RoleSystem {
			storedID, err := GetStoredID(msg)
			if err != nil {
				return err
			} else if storedID != 0 {
				keep = append(keep, storedID)
				continue
			}

			newMessages = append(newMessages, msg)
		}
	}

	// Timeout
	dbCtx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()

	tx, err := m.cfg.DB.BeginTx(dbCtx, nil)
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	// lock
	_, err = tx.ExecContext(dbCtx, LockQueryFormat, generateLockKey(agentID, gatewayID, sessionID))
	if err != nil {
		return fmt.Errorf("error acquiring session advisory lock: %w", err)
	}

	// delete old messages
	if len(keep) > 0 {
		_, err = tx.StmtContext(dbCtx, m.stmtDeleteOldMessages).Exec(agentID, gatewayID, sessionID, pq.Array(keep))
	} else {
		_, err = tx.StmtContext(dbCtx, m.stmtDeleteSession).Exec(agentID, gatewayID, sessionID)
	}

	if err != nil {
		return fmt.Errorf("error deleting old messages: %w", err)
	}

	// new messages
	if len(newMessages) == 0 {
		return tx.Commit()
	}

	stmt, err := tx.PrepareContext(dbCtx, fmt.Sprintf(CopyInQueryFormat, m.cfg.TableName))
	if err != nil {
		return fmt.Errorf("error preparing non-deprecated bulk copy: %w", err)
	}
	defer stmt.Close()

	for _, msg := range newMessages {
		msgJSON, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("error serializing message: %w", err)
		}

		_, err = stmt.ExecContext(dbCtx, agentID, gatewayID, sessionID, string(msgJSON))
		if err != nil {
			return fmt.Errorf("error streaming bulk data: %w", err)
		}
	}

	if _, err := stmt.ExecContext(dbCtx); err != nil {
		return fmt.Errorf("error flushing bulk data: %w", err)
	}
	return tx.Commit()
}

func (m *HistoryMemory) closeStmts() error {
	var errs []error

	if m.stmtDeleteHistories != nil {
		err := m.stmtDeleteHistories.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("error closing stmtDeleteHistories: %w", err))
		}
	}

	if m.stmtDeleteGateway != nil {
		err := m.stmtDeleteGateway.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("error closing stmtDeleteGateway: %w", err))
		}
	}

	if m.stmtDeleteSession != nil {
		err := m.stmtDeleteSession.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("error closing stmtDeleteSession: %w", err))
		}
	}

	if m.stmtRetrieve != nil {
		err := m.stmtRetrieve.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("error closing stmtRetrieve: %w", err))
		}
	}

	if m.stmtDeleteOldMessages != nil {
		err := m.stmtDeleteOldMessages.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("error closing stmtDeleteOldMessages: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (m *HistoryMemory) prepareStmts() error {
	var err error

	// stmtDeleteHistories
	stmtDeleteHistories, err := m.cfg.DB.Prepare(
		fmt.Sprintf(DeleteAgentHistoriesQueryFormat, m.cfg.TableName),
	)
	if err != nil {
		m.closeStmts()
		return fmt.Errorf("error preparing stmtDeleteHistories: %w", err)
	}
	m.stmtDeleteHistories = stmtDeleteHistories

	// stmtDeleteGateway
	stmtDeleteGateway, err := m.cfg.DB.Prepare(
		fmt.Sprintf(DeleteGatewayHistoriesQueryFormat, m.cfg.TableName),
	)
	if err != nil {
		m.closeStmts()
		return fmt.Errorf("error preparing stmtDeleteGateway: %w", err)
	}
	m.stmtDeleteGateway = stmtDeleteGateway

	// stmtDeleteSession
	stmtDeleteSession, err := m.cfg.DB.Prepare(
		fmt.Sprintf(DeleteSessionHistoryQueryFormat, m.cfg.TableName),
	)
	if err != nil {
		m.closeStmts()
		return fmt.Errorf("error preparing stmtDeleteSession: %w", err)
	}
	m.stmtDeleteSession = stmtDeleteSession

	// stmtRetrieve
	stmtRetrieve, err := m.cfg.DB.Prepare(
		fmt.Sprintf(RetrieveHistoryQueryFormat, m.cfg.TableName),
	)
	if err != nil {
		m.closeStmts()
		return fmt.Errorf("error preparing stmtRetrieve: %w", err)
	}
	m.stmtRetrieve = stmtRetrieve

	// stmtDeleteOldMessages
	stmtDeleteOldMessages, err := m.cfg.DB.Prepare(
		fmt.Sprintf(DeleteOldMessagesQueryFormat, m.cfg.TableName),
	)
	if err != nil {
		m.closeStmts()
		return fmt.Errorf("error preparing stmtDeleteOldMessages: %w", err)
	}
	m.stmtDeleteOldMessages = stmtDeleteOldMessages

	return nil
}
