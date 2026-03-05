package agens

import (
	"net/http"
)

// WebhookTriggerRoute represents a specific HTTP endpoint mapping for a webhook.
type WebhookTriggerRoute struct {
	// Method specifies the required HTTP verb (GET, POST, etc.).
	Method string

	// Path defines the URL pattern that matches this route.
	Path string

	// Handler contains the logic to process the incoming HTTP request.
	Handler http.HandlerFunc
}

// Trigger is an interface for components that can initiate an agent's flow.
type Trigger interface {
	// ID returns a unique identifier for the trigger.
	ID() string

	// Name returns the user-friendly name of the trigger.
	Name() string

	// RegisterAgent associates an agent with this trigger.
	RegisterAgent(*Agent) error

	// WithBatcher configures a MessageBatcher for batch processing.
	WithBatcher(MessageBatcher) error
}

// WebhookTrigger defines a component that exposes HTTP endpoints to react to external events (webhooks).
type WebhookTrigger interface {
	Trigger

	// GetRoutes returns the collection of WebhookTriggerRoute definitions
	// to be registered in an HTTP server.
	GetRoutes() []WebhookTriggerRoute
}
