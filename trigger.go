package agens

// Trigger is an interface for a component that can trigger an agent's flow.
type Trigger interface {
	// Name returns the name of the trigger.
	Name() string

	// RegisterAgent registers an agent with the trigger.
	RegisterAgent(*Agent) error
}
