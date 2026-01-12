package agens

import (
	"context"

	"github.com/firebase/genkit/go/ai"
)

// KnowledgeProvider defines the interface for creating or retrieving
// a KnowledgeMemory instance for a specific agent.
type KnowledgeProvider interface {
	// ForAgent returns a KnowledgeMemory instance tailored for the given agent name
	// and sets a limit for the number of documents to retrieve.
	ForAgent(agentName string, limit int) (KnowledgeMemory, error)
}

// KnowledgeMemory defines the operations for managing and retrieving
// agent-specific knowledge used in RAG (Retrieval-Augmented Generation).
type KnowledgeMemory interface {
	// AsTool converts the knowledge retrieval capability into a Genkit tool
	// that the agent can call during a conversation.
	AsTool() ai.Tool

	// DeleteKnowledge removes stored knowledge associated with a specific label.
	DeleteKnowledge(ctx context.Context, label string) error

	// Index stores and indexes a set of documents under a specific label
	// to make them searchable by the agent.
	IndexKnowledge(ctx context.Context, label string, docs []*ai.Document) error
}
