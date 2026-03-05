package pgmemory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
	"github.com/gonzxlezs/agens"
	pgv "github.com/pgvector/pgvector-go"
)

const Provider = "pgmemory"

const (
	TableNameFormat = "knowledge_embeddings_%d"

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

	CreateKnowledgeIvfflatIndexQueryFormat = `CREATE INDEX IF NOT EXISTS idx_%s_ivfflat ON %s USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)`
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

	IsIndexedQueryFormat = `SELECT EXISTS(
        SELECT 1 FROM %s 
            WHERE agent_id = $1 
            AND embedder_name = $2
            AND label = $3
            AND content_hash = $4
    )`

	RetrieveKnowledgeQueryFormat = `SELECT label, content 
    FROM %s 
    WHERE agent_id = $1 AND embedder_name = $2
    ORDER BY embedding <#> $3 LIMIT $4`
)

const labelKey = "label"

var ErrInvalidRetrieveOptions = errors.New("pgmemory: invalid or missing retrieval options")

var _ agens.KnowledgeMemory = &KnowledgeMemory{}

type RetrieveOptions struct {
	AgentID string
	Limit   int
}

type KnowledgeMemoryConfig struct {
	Name        string
	Description string

	Embedder   ai.Embedder
	Dimensions int

	RetrieverOptions *ai.RetrieverOptions
	EmbedderOptions  []ai.EmbedderOption
}

func (cfg *KnowledgeMemoryConfig) getEmbedderOptions(additionalOptions ...ai.EmbedderOption) []ai.EmbedderOption {
	embedderOpts := make([]ai.EmbedderOption, 0, len(cfg.EmbedderOptions)+len(additionalOptions)+1)

	embedderOpts = append(embedderOpts, cfg.EmbedderOptions...)
	embedderOpts = append(embedderOpts, additionalOptions...)

	if cfg.Embedder != nil {
		embedderOpts = append(embedderOpts, ai.WithEmbedder(cfg.Embedder))
	}
	return embedderOpts
}

type KnowledgeMemory struct {
	g   *genkit.Genkit
	db  *sql.DB
	cfg *KnowledgeMemoryConfig

	tableName string
	retriever ai.Retriever
}

func NewKnowledgeMemory(g *genkit.Genkit, db *sql.DB, cfg KnowledgeMemoryConfig) (*KnowledgeMemory, error) {
	tableName := fmt.Sprintf(TableNameFormat, cfg.Dimensions)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	// table
	query := fmt.Sprintf(CreateKnowledgeEmbeddingsQueryFormat, tableName, cfg.Dimensions)
	if _, err := tx.ExecContext(context.Background(), query); err != nil {
		return nil, fmt.Errorf("error creating table: %w", err)
	}

	// check
	query = fmt.Sprintf(CheckKnowledgeSchemaQueryFormat, tableName)
	if _, err := tx.ExecContext(context.Background(), query); err != nil {
		return nil, err
	}

	// index
	query = fmt.Sprintf(CreateKnowledgeLabelIndexQueryFormat, tableName, tableName)
	if _, err := tx.ExecContext(context.Background(), query); err != nil {
		return nil, fmt.Errorf("error creating index: %w", err)
	}

	query = fmt.Sprintf(CreateKnowledgeHashIndexQueryFormat, tableName, tableName)
	if _, err := tx.ExecContext(context.Background(), query); err != nil {
		return nil, fmt.Errorf("error creating index: %w", err)
	}

	query = fmt.Sprintf(CreateKnowledgeIvfflatIndexQueryFormat, tableName, tableName)
	if _, err := tx.ExecContext(context.Background(), query); err != nil {
		return nil, fmt.Errorf("error creating index: %w", err)
	}

	// KnowledgeMemory
	return &KnowledgeMemory{
		g:         g,
		db:        db,
		cfg:       &cfg,
		tableName: tableName,
		retriever: defineRetriever(g, db, tableName, &cfg),
	}, tx.Commit()
}

func (m *KnowledgeMemory) AsTool(agentID string, limit int) ai.Tool {
	return defineTool(m.g, m.retriever, m.cfg, agentID, limit)
}

func (m *KnowledgeMemory) DeleteKnowledge(ctx context.Context, agentID string, label string) error {
	if m.db == nil {
		return ErrDBNotInitialized
	}

	query := fmt.Sprintf(DeleteByLabelQueryFormat, m.tableName)
	_, err := m.db.ExecContext(ctx, query, agentID, m.cfg.Embedder.Name(), label)
	if err != nil {
		return fmt.Errorf("error deleting knowledge: %w", err)
	}
	return nil
}

func (m *KnowledgeMemory) IndexKnowledge(ctx context.Context, agentID string, label string, docs []*ai.Document) error {
	if m.db == nil {
		return ErrDBNotInitialized
	}

	var (
		docsToEmbed   []*ai.Document
		hashesToEmbed []string
	)

	for _, doc := range docs {
		content := documentToText(doc)
		if content == "" {
			continue
		}

		cHash := calculateHash(content)

		exists, err := m.isIndexed(ctx, agentID, label, cHash)
		if err != nil {
			return err
		}

		if !exists {
			docsToEmbed = append(docsToEmbed, doc)
			hashesToEmbed = append(hashesToEmbed, cHash)
		}
	}

	if len(docsToEmbed) == 0 {
		return nil
	}

	res, err := genkit.Embed(
		ctx,
		m.g,
		m.cfg.getEmbedderOptions(
			ai.WithDocs(docsToEmbed...),
		)...,
	)

	if err != nil {
		return err
	}

	var (
		query        = fmt.Sprintf(IndexKnowledgeQueryFormat, m.tableName)
		embedderName = m.cfg.Embedder.Name()
	)

	for i, emb := range res.Embeddings {
		content := documentToText(docsToEmbed[i])
		currentHash := hashesToEmbed[i]
		embedding := pgv.NewVector(emb.Embedding)

		_, err := m.db.ExecContext(ctx, query, agentID, embedderName, label, content, currentHash, embedding)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *KnowledgeMemory) isIndexed(ctx context.Context, agentID string, label string, content_hash string) (bool, error) {
	if m.db == nil {
		return false, ErrDBNotInitialized
	}

	var (
		query  = fmt.Sprintf(IsIndexedQueryFormat, m.tableName)
		exists bool
	)
	err := m.db.QueryRowContext(ctx, query, agentID, m.cfg.Embedder.Name(), label, content_hash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("database error: %v", err)
	}
	return exists, nil
}

func calculateHash(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func defineRetriever(g *genkit.Genkit, db *sql.DB, tableName string, cfg *KnowledgeMemoryConfig) ai.Retriever {
	f := func(ctx context.Context, req *ai.RetrieverRequest) (*ai.RetrieverResponse, error) {
		opts, ok := req.Options.(*RetrieveOptions)
		if !ok || opts == nil {
			return nil, ErrInvalidRetrieveOptions
		}

		if opts.Limit <= 0 {
			// Default limit if not specified or invalid
			opts.Limit = 3
		}

		eres, err := genkit.Embed(
			ctx,
			g,
			cfg.getEmbedderOptions(ai.WithDocs(req.Query))...,
		)

		if err != nil {
			return nil, err
		}

		query := fmt.Sprintf(RetrieveKnowledgeQueryFormat, tableName)
		rows, err := db.QueryContext(
			ctx,
			query,
			opts.AgentID,
			cfg.Embedder.Name(),
			pgv.NewVector(eres.Embeddings[0].Embedding),
			opts.Limit,
		)

		if err != nil {
			return nil, err
		}
		defer rows.Close()

		res := &ai.RetrieverResponse{}
		for rows.Next() {
			var label, content string
			if err := rows.Scan(&label, &content); err != nil {
				return nil, err
			}

			res.Documents = append(
				res.Documents,
				ai.DocumentFromText(content, map[string]any{
					labelKey: label,
				}),
			)
		}

		return res, rows.Err()
	}

	return genkit.DefineRetriever(g, api.NewName(Provider, cfg.Name), cfg.RetrieverOptions, f)
}
