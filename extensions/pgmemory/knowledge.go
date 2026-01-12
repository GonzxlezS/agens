package pgmemory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
	"github.com/gonzxlezs/agens"
	pgv "github.com/pgvector/pgvector-go"
)

const Provider = "pgmemory"

const (
	TableNameFormat = "knowledge_embeddings_%d"

	DeleteByLabelQueryFormat = `DELETE FROM %s WHERE agent_name = $1 AND embedder_name = $2 AND label = $3`

	IndexKnowledgeQueryFormat = `INSERT INTO %s (
	agent_name, 
	embedder_name,
	label, 
	content,
	content_hash,
	embedding
	) VALUES ($1, $2, $3, $4, $5, $6)`

	IsIndexedQueryFormat = `SELECT EXISTS(
        SELECT 1 FROM %s 
            WHERE agent_name = $1 
            AND embedder_name = $2
            AND label = $3
            AND content_hash = $4
    )`

	RetrieveKnowledgeQueryFormat = `SELECT label, content 
    FROM %s 
    WHERE agent_name = $1 
      AND embedder_name = $2
    ORDER BY embedding <#> $3 LIMIT $4`
)

const (
	StatusKnowledgeSuccess = "success"

	StatusKnowledgeNoResults = "no_results"
)

const labelKey = "label"

var (
	ErrDimensionNotSupported = errors.New("pgmemory: dimension not supported")

	ErrKnowledgeProviderFailure = fmt.Errorf("pgmemory: knowledge provider failure")

	ErrInvalidRetrieveOptions = errors.New("pgmemory: invalid or missing retrieval options")
)

var _ agens.KnowledgeProvider = &KnowledgeProvider{}
var _ agens.KnowledgeMemory = &knowledgeMemory{}

var supportedDimensions = map[int]struct{}{
	384:  {},
	768:  {},
	1024: {},
	1536: {},
}

type (
	KnowledgeQuery struct {
		Query string `json:"query" jsonschema_description:"The specific search query or keywords to retrieve relevant information from the knowledge base. Should be clear and focused on the topic."`
	}

	DocumentResult struct {
		Label   string `json:"label" jsonschema_description:"The category or source label of the retrieved document."`
		Content string `json:"content" jsonschema_description:"The text content of the retrieved document."`
	}

	KnowledgeResponse struct {
		Results []DocumentResult `json:"results" jsonschema_description:"List of relevant documents found."`
		Count   int              `json:"count" jsonschema_description:"Number of documents retrieved. 0 if nothing was found."`
		Status  string           `json:"status" jsonschema:"enum=success,enum=,description=The outcome of the retrieval operation."`
	}
)

type RetrieveOptions struct {
	AgentName string
	Limit     int
}

type KnowledgeProviderConfig struct {
	Name             string
	Description      string
	Embedder         ai.Embedder
	EmbedderName     string
	Dimensions       int
	RetrieverOptions *ai.RetrieverOptions
	EmbedderOptions  []ai.EmbedderOption
}

func (cfg *KnowledgeProviderConfig) resolveEmbedderName() string {
	if cfg.Embedder != nil {
		return cfg.Embedder.Name()
	}
	return cfg.EmbedderName
}

func (cfg *KnowledgeProviderConfig) resolveEmbedderOptions(additionalOptions ...ai.EmbedderOption) []ai.EmbedderOption {
	embedderOpts := make([]ai.EmbedderOption, 0, len(cfg.EmbedderOptions)+len(additionalOptions)+1)

	embedderOpts = append(embedderOpts, cfg.EmbedderOptions...)
	embedderOpts = append(embedderOpts, additionalOptions...)

	if cfg.Embedder != nil {
		embedderOpts = append(embedderOpts, ai.WithEmbedder(cfg.Embedder))
	} else if cfg.EmbedderName != "" {
		embedderOpts = append(embedderOpts, ai.WithEmbedderName(cfg.EmbedderName))
	}

	return embedderOpts
}

type KnowledgeProvider struct {
	g   *genkit.Genkit
	db  *sql.DB
	cfg *KnowledgeProviderConfig

	tableName string
	retriever ai.Retriever
}

func NewKnowledgeProvider(g *genkit.Genkit, db *sql.DB, cfg KnowledgeProviderConfig) (*KnowledgeProvider, error) {
	tableName, err := getTableName(cfg.Dimensions)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := runModuleMigration(db, "knowledge", "migrations_knowledge"); err != nil {
		return nil, fmt.Errorf("knowledge migrations failed: %w", err)
	}

	retriever := defineRetriever(g, db, tableName, &cfg)

	return &KnowledgeProvider{
		g:         g,
		db:        db,
		cfg:       &cfg,
		tableName: tableName,
		retriever: retriever,
	}, nil
}

func (p *KnowledgeProvider) ForAgent(agentName string, limit int) (agens.KnowledgeMemory, error) {
	return &knowledgeMemory{
		provider:  p,
		agentName: agentName,
		asTool:    defineTool(p.g, p.retriever, p.cfg, agentName, limit),
	}, nil
}

func (p *KnowledgeProvider) deleteKnowledge(ctx context.Context, agentName string, label string) error {
	if p.db == nil {
		return ErrDBNotInitialized
	}

	query := fmt.Sprintf(DeleteByLabelQueryFormat, p.tableName)
	_, err := p.db.ExecContext(ctx, query, agentName, p.cfg.resolveEmbedderName(), label)
	if err != nil {
		return fmt.Errorf("error deleting knowledge: %w", err)
	}
	return nil
}

func (p *KnowledgeProvider) indexKnowledge(ctx context.Context, agentName string, label string, docs []*ai.Document) error {
	if p.db == nil {
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

		exists, err := p.isIndexed(ctx, agentName, label, cHash)
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
		p.g,
		p.cfg.resolveEmbedderOptions(
			ai.WithDocs(docsToEmbed...),
		)...,
	)

	if err != nil {
		return err
	}

	var (
		query        = fmt.Sprintf(IndexKnowledgeQueryFormat, p.tableName)
		embedderName = p.cfg.resolveEmbedderName()
	)

	for i, emb := range res.Embeddings {
		content := documentToText(docsToEmbed[i])
		currentHash := hashesToEmbed[i]
		embedding := pgv.NewVector(emb.Embedding)

		_, err := p.db.ExecContext(ctx, query, agentName, embedderName, label, content, currentHash, embedding)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *KnowledgeProvider) isIndexed(ctx context.Context, agentName string, label string, content_hash string) (bool, error) {
	if p.db == nil {
		return false, ErrDBNotInitialized
	}

	var (
		query  = fmt.Sprintf(IsIndexedQueryFormat, p.tableName)
		exists bool
	)
	err := p.db.QueryRowContext(ctx, query, agentName, p.cfg.resolveEmbedderName(), label, content_hash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("database error: %v", err)
	}
	return exists, nil
}

type knowledgeMemory struct {
	provider  *KnowledgeProvider
	asTool    ai.Tool
	agentName string
}

func (k *knowledgeMemory) AsTool() ai.Tool {
	return k.asTool
}

func (k *knowledgeMemory) DeleteKnowledge(ctx context.Context, label string) error {
	return k.provider.deleteKnowledge(ctx, k.agentName, label)
}

func (k *knowledgeMemory) IndexKnowledge(ctx context.Context, label string, docs []*ai.Document) error {
	return k.provider.indexKnowledge(ctx, k.agentName, label, docs)
}

func calculateHash(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func defineRetriever(g *genkit.Genkit, db *sql.DB, tableName string, cfg *KnowledgeProviderConfig) ai.Retriever {
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
			cfg.resolveEmbedderOptions(ai.WithDocs(req.Query))...,
		)

		if err != nil {
			return nil, err
		}

		query := fmt.Sprintf(RetrieveKnowledgeQueryFormat, tableName)
		rows, err := db.QueryContext(
			ctx,
			query,
			opts.AgentName,
			cfg.resolveEmbedderName(),
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

func defineTool(g *genkit.Genkit, retriever ai.Retriever, cfg *KnowledgeProviderConfig, agentName string, limit int) ai.Tool {
	toolName := fmt.Sprintf("%s_%s_tool", agentName, cfg.Name)

	f := func(ctx *ai.ToolContext, query KnowledgeQuery) (KnowledgeResponse, error) {
		resp, err := genkit.Retrieve(
			ctx, g,
			ai.WithRetriever(retriever),
			ai.WithConfig(&RetrieveOptions{
				AgentName: agentName,
				Limit:     limit,
			}),
			ai.WithTextDocs(query.Query),
		)
		if err != nil {
			return KnowledgeResponse{}, errors.Join(ErrKnowledgeProviderFailure, err)
		}

		kResponse := KnowledgeResponse{
			Count:  len(resp.Documents),
			Status: StatusKnowledgeNoResults,
		}

		if kResponse.Count < 1 {
			return kResponse, nil
		}
		kResponse.Status = StatusKnowledgeSuccess

		for _, doc := range resp.Documents {
			label, _ := doc.Metadata[labelKey].(string)
			if label == "" {
				label = "unlabeled"
			}

			kResponse.Results = append(
				kResponse.Results,
				DocumentResult{
					Label:   label,
					Content: documentToText(doc),
				},
			)
		}
		return kResponse, nil
	}

	return genkit.DefineTool(g, toolName, cfg.Description, f)
}

func documentToText(doc *ai.Document) string {
	var b strings.Builder
	for _, part := range doc.Content {
		b.WriteString(part.Text)
		b.WriteString("\n")
	}
	return b.String()
}

func getTableName(dim int) (string, error) {
	if _, ok := supportedDimensions[dim]; !ok {
		dims := make([]string, 0, len(supportedDimensions))
		for d := range supportedDimensions {
			dims = append(dims, fmt.Sprintf("%d", d))
		}

		return "", fmt.Errorf("%w. Supported: [%s]", ErrDimensionNotSupported, strings.Join(dims, ", "))
	}

	return fmt.Sprintf(TableNameFormat, dim), nil
}
