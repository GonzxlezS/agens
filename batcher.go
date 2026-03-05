package agens

import (
	"context"

	"github.com/firebase/genkit/go/ai"
)

// MessageBatcher defines the interface for grouping individual messages into
// a single processing unit (batch).
type MessageBatcher interface {
	// Add appends a message to a specific batch identified by batchID.
	// It returns the current collection of messages if a processing threshold
	// is met, or an error if the persistence or grouping fails.
	Add(ctx context.Context, batchID string, message *ai.Message) ([]*ai.Message, error)
}
