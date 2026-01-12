package agens

import "context"

// Trigger is an interface for a component that can trigger an agent's flow.
type Trigger interface {
	// Name returns the name of the trigger.
	Name() string

	// RegisterAgent registers an agent with the trigger.
	RegisterAgent(*Agent) error

	// Start begins the trigger's active listening or polling operation.
	Start(context.Context) error

	// Stop gracefully halts the trigger's operation and releases any resources.
	Stop(context.Context) error
}
