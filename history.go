package agens

import (
	"context"

	"github.com/firebase/genkit/go/ai"
)

// HistoryManager handles the lifecycle strategy of the conversation history.
// It is responsible for applying techniques like sliding windows, message pruning,
// or generating LLM-powered summaries before the history is persisted or sent to the model.
type HistoryManager interface {
	// ProcessHistory applies the configured strategy (windowing, summarization, etc.)
	// to the entire conversation history. It returns the newly processed and optimized
	// slice of messages.
	ProcessHistory(ctx context.Context, history []*ai.Message) ([]*ai.Message, error)
}

// HistoryMemory defines the operations for absolute state persistence of chat histories.
// Implementing types are fully responsible for idempotency, message deduplication,
// and determining whether a message in the received slice is new or already stored.
type HistoryMemory interface {
	// StoreHistory reconciles and persists the processed history state for a specific session.
	// The implementation must handle the logic to identify new messages and avoid duplication.
	StoreHistory(ctx context.Context, agentID string, gatewayID string, sessionID string, messages []*ai.Message) error

	// RetrieveHistory fetches the chronological sequence of messages for a specific
	// session and agent. Returns an empty slice if no history is found.
	RetrieveHistory(ctx context.Context, agentID string, gatewayID string, sessionID string) ([]*ai.Message, error)

	// DeleteHistory removes all messages and metadata associated with a specific session
	// within a specific gateway and agent.
	DeleteHistory(ctx context.Context, agentID string, gatewayID string, sessionID string) error

	// DeleteGatewayHistories removes all conversation histories and sessions associated
	// with a specific communication gateway for a given agent.
	DeleteGatewayHistories(ctx context.Context, agentID string, gatewayID string) error

	// DeleteAgentHistories performs a bulk deletion of all conversation histories
	// belonging to a specific agent across all gateways.
	DeleteAgentHistories(ctx context.Context, agentID string) error
}
