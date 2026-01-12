package agens

import (
	"context"

	"github.com/firebase/genkit/go/ai"
)

// HistoryProvider defines an interface for providing history memory instances
// tailored for specific agents.
type HistoryProvider interface {
	// ForAgent initializes or retrieves a HistoryMemory implementation for the
	// specified agentName, limiting the history to maxMessagesPerConversation if applicable.
	ForAgent(agentName string, maxMessagesPerConversation int) (HistoryMemory, error)
}

// HistoryMemory is an interface for managing the history of a conversation.
type HistoryMemory interface {
	// RetrieveHistory fetches the history of messages for a given conversationID.
	// It should return the messages in chronological order.
	RetrieveHistory(ctx context.Context, conversationID string) ([]*ai.Message, error)

	// StoreHistory persists the given messages for a specific conversationID.
	// Implementations should handle adding new messages to the existing history.
	StoreHistory(ctx context.Context, conversationID string, messages []*ai.Message) error

	// DeleteHistory removes the entire conversation history associated with a conversationID.
	DeleteHistory(ctx context.Context, conversationID string) error

	// Close performs any necessary cleanup, such as closing database connections.
	Close() error
}
