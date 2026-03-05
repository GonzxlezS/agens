package agens

import (
	"context"

	"github.com/firebase/genkit/go/ai"
)

// KnowledgeMemory defines the operations for managing and retrieving
// agent-specific knowledge used in RAG (Retrieval-Augmented Generation).
type KnowledgeMemory interface {
	// AsTool converts the knowledge retrieval capability into a Genkit tool
	// that the agent can call during a conversation.
	AsTool(agentID string, limit int) ai.Tool

	// IndexKnowledge stores documents in the knowledge base under a specific label
	// for the given agent.
	IndexKnowledge(ctx context.Context, agentID string, label string, docs []*ai.Document) error

	// DeleteKnowledge removes all stored documents associated with a specific label
	// for the given agent.
	DeleteKnowledge(ctx context.Context, agentID string, label string) error
}
