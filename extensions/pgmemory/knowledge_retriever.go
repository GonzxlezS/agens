package pgmemory

import (
	"context"
	"database/sql"
	"errors"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
	pgv "github.com/pgvector/pgvector-go"
)

const Provider = "pgmemory"

const LabelKey = "label"

const DefaultRetrieveLimit = 3

var ErrInvalidRetrieveOptions = errors.New("pgmemory: invalid or missing retrieval options")

type RetrieveOptions struct {
	AgentID string
	Limit   int
}

func (m *KnowledgeMemory) RetrieveKnowledge(ctx context.Context, agentID string, query string, limit int) (*ai.RetrieverResponse, error) {
	return genkit.Retrieve(ctx, m.g,
		ai.WithRetriever(m.retriever),
		ai.WithConfig(&RetrieveOptions{
			AgentID: agentID,
			Limit:   limit,
		}),
		ai.WithTextDocs(query),
	)
}

func defineRetriever(g *genkit.Genkit, cfg *KnowledgeMemoryConfig, stmtRetrieve *sql.Stmt) ai.Retriever {
	f := func(ctx context.Context, req *ai.RetrieverRequest) (*ai.RetrieverResponse, error) {
		opts, ok := req.Options.(*RetrieveOptions)
		if !ok || opts == nil {
			return nil, ErrInvalidRetrieveOptions
		}

		if opts.Limit == 0 {
			opts.Limit = DefaultRetrieveLimit // Default limit if not specified or invalid
		}

		eres, err := genkit.Embed(
			ctx,
			g,
			cfg.embedderOptions(ai.WithDocs(req.Query))...,
		)

		if err != nil {
			return nil, err
		}

		rows, err := stmtRetrieve.QueryContext(
			ctx,
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
					LabelKey: label,
				}),
			)
		}

		return res, rows.Err()
	}

	return genkit.DefineRetriever(g, api.NewName(Provider, cfg.Name), cfg.RetrieverOptions, f)
}
