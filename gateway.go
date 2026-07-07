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

// Gateway defines the interface for an agent gateway, managing agent registration
// and message batching configurations.
type Gateway interface {
	// ID returns the unique identifier of the gateway.
	ID() string

	// Name returns the human-readable name of the gateway.
	Name() string

	// RegisterAgent registers a new agent with the gateway.
	RegisterAgent(*Agent) error

	// WithMessageBatcher configures the gateway to use a specific MessageBatcher.
	WithMessageBatcher(MessageBatcher) error
}

// GatewayRef defines the interface for a reference to a specific gateway instance,
// providing access to its identification, the associated agent, and its message batching configuration.
type GatewayRef interface {
	// ID returns the unique identifier of the referenced gateway.
	ID() string

	// Name returns the human-readable name of the referenced gateway.
	Name() string

	// Agent returns the pointer to the Agent associated with this gateway reference.
	Agent() *Agent

	// MessageBatcher returns the MessageBatcher configuration utilized by this gateway reference.
	MessageBatcher() MessageBatcher
}
