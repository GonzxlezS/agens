package agens

import (
	"context"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

const (
	RetrieveHistoryStep = "retrieveHistory"

	StoreHistoryStep = "storeHistory"
)

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

func (agent *Agent) retrieveHistory(ctx context.Context, conversationID string) ([]*ai.Message, error) {
	if agent.HistoryMemory == nil {
		return nil, nil
	}

	return genkit.Run(ctx, RetrieveHistoryStep, func() ([]*ai.Message, error) {
		return agent.HistoryMemory.RetrieveHistory(ctx, conversationID)
	})
}

func (agent *Agent) storeHistory(ctx context.Context, conversationID string, history []*ai.Message) error {
	if agent.HistoryMemory == nil {
		return nil
	}

	_, err := genkit.Run(ctx, StoreHistoryStep, func() (struct{}, error) {
		err := agent.HistoryMemory.StoreHistory(ctx, conversationID, history)
		return struct{}{}, err
	})
	return err
}
