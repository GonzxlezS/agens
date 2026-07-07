package pgmemory

import (
	"context"
	"errors"

	"github.com/firebase/genkit/go/ai"
	pgv "github.com/pgvector/pgvector-go"
)

const LabelKey = "label"

const DefaultRetrieveLimit = 3

var ErrInvalidRetrieveOptions = errors.New("pgmemory: invalid or missing retrieval options")

type RetrieveOptions struct {
	AgentID string
	Limit   int
}

func (m *KnowledgeMemory) Embed(ctx context.Context, opts ...ai.EmbedderOption) (*ai.EmbedResponse, error) {
	opts = append(opts, ai.WithEmbedder(m.cfg.Embedder))
	return ai.Embed(ctx, m.reg, opts...)
}

func (m *KnowledgeMemory) RetrieveKnowledge(ctx context.Context, agentID string, query string, limit int) ([]*ai.Document, error) {
	resp, err := ai.Retrieve(ctx, m.reg,
		ai.WithRetriever(m.retriever),
		ai.WithConfig(&RetrieveOptions{
			AgentID: agentID,
			Limit:   limit,
		}),
		ai.WithTextDocs(query),
	)

	if err != nil {
		return nil, err
	}
	return resp.Documents, nil
}

func (m *KnowledgeMemory) retrieverFn(ctx context.Context, req *ai.RetrieverRequest) (*ai.RetrieverResponse, error) {
	opts, ok := req.Options.(*RetrieveOptions)
	if !ok || opts == nil {
		return nil, ErrInvalidRetrieveOptions
	}

	if opts.Limit == 0 {
		opts.Limit = DefaultRetrieveLimit // Default limit if not specified or invalid
	}

	eres, err := m.Embed(ctx, ai.WithDocs(req.Query))
	if err != nil {
		return nil, err
	}

	// Timeout
	dbCtx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()

	rows, err := m.stmtRetrieve.QueryContext(
		dbCtx,
		opts.AgentID,
		m.cfg.Embedder.Name(),
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
