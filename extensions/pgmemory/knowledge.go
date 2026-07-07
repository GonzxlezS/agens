package pgmemory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gonzxlezs/agens"
	"github.com/lib/pq"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	pgv "github.com/pgvector/pgvector-go"
)

const Provider = "pgmemory"

var ErrEmbedderRequired = errors.New("pgmemory: embedder is required")

const (
	CreateKnowledgeEmbeddingsQueryFormat = `CREATE EXTENSION IF NOT EXISTS vector;
	CREATE TABLE IF NOT EXISTS %s (
		id SERIAL PRIMARY KEY,
        agent_id TEXT NOT NULL,
        embedder_name TEXT NOT NULL,
		label TEXT NOT NULL,
        content TEXT NOT NULL,
        content_hash TEXT NOT NULL,
        embedding vector(%d) NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    )`

	CreateKnowledgeLabelIndexQueryFormat = `CREATE INDEX IF NOT EXISTS idx_%s_label ON %s (agent_id, embedder_name, label)`

	CreateKnowledgeHashIndexQueryFormat = `CREATE INDEX IF NOT EXISTS idx_%s_hash ON %s (agent_id, embedder_name, content_hash)`

	CreateKnowledgeHNSWIndexQueryFormat = `CREATE INDEX IF NOT EXISTS idx_%s_hnsw ON %s USING hnsw (embedding vector_cosine_ops)`
)

const (
	CheckKnowledgeSchemaQueryFormat = `DO $$
	DECLARE
		mismatch RECORD;
	BEGIN
		FOR mismatch IN
			WITH expected_layout AS (
				SELECT * FROM (VALUES
					('id', 'int4'),
					('agent_id', 'text'),
					('embedder_name', 'text'),
					('label', 'text'),
					('content', 'text'),
					('content_hash', 'text'),
					('embedding', 'vector'),
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
	DeleteByLabelQueryFormat = `DELETE FROM %s WHERE agent_id = $1 AND embedder_name = $2 AND label = $3`

	IndexKnowledgeQueryFormat = `INSERT INTO %s (
		agent_id,
		embedder_name,
		label,
		content,
		content_hash,
		embedding
	) VALUES ($1, $2, $3, $4, $5, $6)`

	CheckHashesQueryFormat = `SELECT content_hash FROM %s
		WHERE agent_id = $1
			AND embedder_name = $2
			AND label = $3
			AND content_hash = ANY($4)`

	RetrieveKnowledgeQueryFormat = `SELECT label, content
		FROM %s
		WHERE agent_id = $1 AND embedder_name = $2
		ORDER BY embedding <=> $3 LIMIT $4`

	ReassignKnowledgeQueryFormat = `UPDATE %s
		SET agent_id = $1
		WHERE agent_id = $2 AND label = $3`
)

var _ agens.KnowledgeMemory = &KnowledgeMemory{}

type KnowledgeMemoryConfig struct {
	Name        string
	Description string

	Embedder         ai.EmbedderArg
	Dimensions       int
	RetrieverOptions *ai.RetrieverOptions

	DB      *sql.DB
	Timeout time.Duration
	OwnsDB  bool
}

type KnowledgeMemory struct {
	cfg       *KnowledgeMemoryConfig
	retriever ai.Retriever
	reg       api.Registry
	tool      *ai.ToolDef[KnowledgeQuery, KnowledgeResponse]

	stmtDelete    *sql.Stmt
	stmtIndex     *sql.Stmt
	stmtCheckHash *sql.Stmt
	stmtRetrieve  *sql.Stmt
	stmtReassign  *sql.Stmt
}

func NewKnowledgeMemory(cfg KnowledgeMemoryConfig) (*KnowledgeMemory, error) {
	if cfg.Embedder == nil {
		return nil, ErrEmbedderRequired
	}

	// table name
	tableName, err := knowledgeTableName(cfg.Dimensions)
	if err != nil {
		return nil, err
	}

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
	query := fmt.Sprintf(CreateKnowledgeEmbeddingsQueryFormat, tableName, cfg.Dimensions)
	if _, err := tx.Exec(query); err != nil {
		return nil, fmt.Errorf("error creating table: %w", err)
	}

	// check
	query = fmt.Sprintf(CheckKnowledgeSchemaQueryFormat, tableName)
	if _, err := tx.Exec(query); err != nil {
		return nil, err
	}

	// index
	query = fmt.Sprintf(CreateKnowledgeLabelIndexQueryFormat, tableName, tableName)
	if _, err := tx.Exec(query); err != nil {
		return nil, fmt.Errorf("error creating index: %w", err)
	}

	query = fmt.Sprintf(CreateKnowledgeHashIndexQueryFormat, tableName, tableName)
	if _, err := tx.Exec(query); err != nil {
		return nil, fmt.Errorf("error creating index: %w", err)
	}

	query = fmt.Sprintf(CreateKnowledgeHNSWIndexQueryFormat, tableName, tableName)
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

	// retriever
	memory := &KnowledgeMemory{cfg: &cfg}

	if err := memory.prepareStmts(tableName); err != nil {
		return nil, err
	}

	memory.retriever = ai.NewRetriever(
		api.NewName(Provider, cfg.Name),
		cfg.RetrieverOptions,
		memory.retrieverFn,
	)

	return memory, nil
}

// Close releases the resources of all prepared statements
// associated with KnowledgeMemory to prevent memory leaks and open database connections.
func (m *KnowledgeMemory) Close() error {
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

func (m *KnowledgeMemory) DeleteKnowledge(ctx context.Context, agentID string, label string) error {
	if (m.cfg == nil) || (m.cfg.DB == nil) {
		return ErrDBNotInitialized
	}

	// Timeout
	dbCtx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()

	_, err := m.stmtDelete.ExecContext(dbCtx, agentID, m.cfg.Embedder.Name(), label)
	if err != nil {
		return fmt.Errorf("error deleting knowledge: %w", err)
	}
	return nil
}

func (m *KnowledgeMemory) IndexKnowledge(ctx context.Context, agentID string, label string, docs []*ai.Document) error {
	if (m.cfg == nil) || (m.cfg.DB == nil) {
		return ErrDBNotInitialized
	}

	// hash
	var (
		hashes         []string
		hashToDocIndex = make(map[string]int)
		hashToDocText  = make(map[string]string)
	)

	for i, doc := range docs {
		content := documentToText(doc)
		if content == "" {
			continue
		}

		cHash := calculateHash(content)

		hashToDocIndex[cHash] = i
		hashToDocText[cHash] = content
		hashes = append(hashes, cHash)
	}

	if len(hashes) == 0 {
		return nil
	}

	// timeout
	dbCtx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()

	existsMap, err := m.checkHashes(dbCtx, agentID, label, hashes)
	if err != nil {
		return err
	}

	// filter
	var (
		hashesToEmbed []string
		docsToEmbed   []*ai.Document
	)

	for _, hash := range hashes {
		if !existsMap[hash] {
			hashesToEmbed = append(hashesToEmbed, hash)

			docIndex := hashToDocIndex[hash]
			docsToEmbed = append(docsToEmbed, docs[docIndex])
		}
	}

	if len(hashesToEmbed) == 0 {
		return nil
	}

	// embed
	res, err := m.Embed(dbCtx, ai.WithDocs(docsToEmbed...))
	if err != nil {
		return err
	}

	var embedderName = m.cfg.Embedder.Name()

	for i, emb := range res.Embeddings {
		var (
			currentHash = hashesToEmbed[i]
			content     = hashToDocText[currentHash]
			embedding   = pgv.NewVector(emb.Embedding)
		)

		_, err := m.stmtIndex.ExecContext(dbCtx, agentID, embedderName, label, content, currentHash, embedding)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *KnowledgeMemory) checkHashes(ctx context.Context, agentID string, label string, hashes []string) (map[string]bool, error) {
	rows, err := m.stmtCheckHash.QueryContext(ctx, agentID, m.cfg.Embedder.Name(), label, pq.Array(hashes))
	if err != nil {
		return nil, fmt.Errorf("error checking batch existence: %w", err)
	}
	defer rows.Close()

	existsMap := make(map[string]bool)
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, err
		}
		existsMap[hash] = true
	}

	return existsMap, rows.Err()
}

func (m *KnowledgeMemory) ReassignKnowledge(ctx context.Context, fromAgentID string, label string, toAgentID string) error {
	if (m.cfg == nil) || (m.cfg.DB == nil) {
		return ErrDBNotInitialized
	}

	// Timeout
	dbCtx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()

	_, err := m.stmtReassign.ExecContext(dbCtx, toAgentID, fromAgentID, label)
	if err != nil {
		return fmt.Errorf("failed to reassign knowledge: %w", err)
	}
	return nil
}

func (m *KnowledgeMemory) Register(r api.Registry) {
	m.reg = r
	m.retriever.Register(r)
}

func (m *KnowledgeMemory) closeStmts() error {
	var errs []error

	if m.stmtDelete != nil {
		err := m.stmtDelete.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("error closing stmtDelete: %w", err))
		}
	}
	if m.stmtIndex != nil {
		err := m.stmtIndex.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("error closing stmtIndex: %w", err))
		}
	}
	if m.stmtCheckHash != nil {
		err := m.stmtCheckHash.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("error closing stmtCheckHash: %w", err))
		}
	}
	if m.stmtRetrieve != nil {
		err := m.stmtRetrieve.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("error closing stmtRetrieve: %w", err))
		}
	}
	if m.stmtReassign != nil {
		err := m.stmtReassign.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("error closing stmtReassign: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (m *KnowledgeMemory) prepareStmts(tableName string) error {
	var err error

	// stmtDelete
	stmtDelete, err := m.cfg.DB.Prepare(fmt.Sprintf(DeleteByLabelQueryFormat, tableName))
	if err != nil {
		m.closeStmts()
		return fmt.Errorf("error preparing stmtDelete: %w", err)
	}
	m.stmtDelete = stmtDelete

	// stmtIndex
	stmtIndex, err := m.cfg.DB.Prepare(fmt.Sprintf(IndexKnowledgeQueryFormat, tableName))
	if err != nil {
		m.closeStmts()
		return fmt.Errorf("error preparing stmtIndex: %w", err)
	}
	m.stmtIndex = stmtIndex

	// stmtCheckHash
	stmtCheckHash, err := m.cfg.DB.Prepare(fmt.Sprintf(CheckHashesQueryFormat, tableName))
	if err != nil {
		m.closeStmts()
		return fmt.Errorf("error preparing stmtCheckHash: %w", err)
	}
	m.stmtCheckHash = stmtCheckHash

	// stmtRetrieve
	stmtRetrieve, err := m.cfg.DB.Prepare(fmt.Sprintf(RetrieveKnowledgeQueryFormat, tableName))
	if err != nil {
		m.closeStmts()
		return fmt.Errorf("error preparing stmtRetrieve: %w", err)
	}
	m.stmtRetrieve = stmtRetrieve

	// stmtReassign
	stmtReassign, err := m.cfg.DB.Prepare(fmt.Sprintf(ReassignKnowledgeQueryFormat, tableName))
	if err != nil {
		m.closeStmts()
		return fmt.Errorf("error preparing stmtReassign: %w", err)
	}
	m.stmtReassign = stmtReassign

	return nil
}
