package agens

import (
	"context"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
)

// KnowledgeMode defines how knowledge (RAG) is used in the agent.
type KnowledgeMode string

const (
	// KnowledgeModeNone disables knowledge retrieval entirely.
	KnowledgeModeNone KnowledgeMode = "none"

	// KnowledgeModeStep enables knowledge as a PRE-RETRIEVAL step before generation.
	KnowledgeModeStep KnowledgeMode = "step"

	// KnowledgeModeTool enables knowledge as a TOOL (LLM calls it on-demand).
	KnowledgeModeTool KnowledgeMode = "tool"

	// KnowledgeModeBoth enables knowledge as both a PRE-RETRIEVAL step and a TOOL.
	KnowledgeModeBoth KnowledgeMode = "both"
)

// KnowledgeQueryer defines the behavior for generating a search query for RAG systems based on the input.
type KnowledgeQueryer interface {
	GenerateQuery(ctx context.Context, input *Input) (string, error)
}

// KnowledgeMemory defines the operations for managing and retrieving
// agent-specific knowledge used in RAG (Retrieval-Augmented Generation).
type KnowledgeMemory interface {
	// Registerable allows a primitive to be registered with a registry.
	api.Registerable

	// RetrieveKnowledge fetches relevant documents from the knowledge base
	// matching a semantic query for a specific agent, capped by the limit parameter.
	RetrieveKnowledge(ctx context.Context, agentID string, query string, limit int) ([]*ai.Document, error)

	// ReassignKnowledge transfers ownership of all knowledge documents under a specific label
	// from one source agent to a target destination agent.
	ReassignKnowledge(ctx context.Context, fromAgentID string, label string, toAgentID string) error

	// IndexKnowledge stores documents in the knowledge base under a specific label
	// for the given agent.
	IndexKnowledge(ctx context.Context, agentID string, label string, docs []*ai.Document) error

	// DeleteKnowledge removes all stored documents associated with a specific label
	// for the given agent.
	DeleteKnowledge(ctx context.Context, agentID string, label string) error

	// AsTool converts the knowledge retrieval capability into a Genkit tool
	// that the agent can call during a conversation.
	AsTool(agentID string, limit int) ai.Tool
}
