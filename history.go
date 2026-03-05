package agens

import (
	"context"

	"github.com/firebase/genkit/go/ai"
)

// HistoryMemory defines the operations for persisting and retrieving
// chat message history between an agent and session.
type HistoryMemory interface {
	// RetrieveHistory fetches the chronological sequence of messages for a specific
	// session and agent. Returns an empty slice if no history is found.
	RetrieveHistory(ctx context.Context, agentID string, sessionID string) ([]*ai.Message, error)

	// StoreHistory persists a set of messages into the session storage.
	// It applies a retention policy based on maxMessages, typically keeping
	// only the most recent N messages to optimize context window usage.
	StoreHistory(ctx context.Context, agentID string, sessionID string, messages []*ai.Message, maxMessages int) error

	// DeleteHistory removes all messages and metadata associated with a specific session.
	DeleteHistory(ctx context.Context, agentID string, sessionID string) error
}
