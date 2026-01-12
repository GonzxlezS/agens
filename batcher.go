package agens

import (
	"github.com/firebase/genkit/go/ai"
)

// MessageBatcher is an interface for managing message batches.
// Implementations of this interface can decide how to group or handle incoming
// messages before they are sent to the AI model. This is useful for
// optimizing API calls or handling message streams.
type MessageBatcher interface {
	// Add adds a new message to the batch. It returns the current batch of
	// messages, which can be a single message or multiple messages depending on
	// the batching logic. It may return an empty slice if the message should
	// not be processed at this time.
	Add(conversationID string, message *ai.Message) ([]*ai.Message, error)
}
