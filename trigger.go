package agens

import (
	"context"
	"net/http"
)

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

// HTTPTriggerRoute defines the necessary components for an HTTP route
// handled by the trigger.
type HTTPTriggerRoute struct {
	// Method is the HTTP method.
	Method string

	// Path is the URL path the route will match.
	Path string

	// Handler is the function that will process the incoming HTTP request.
	Handler http.HandlerFunc
}

// HTTPTrigger is an interface for a component that provides HTTP routes
// that can trigger an agent's flow.
type HTTPTrigger interface {
	// Name returns the name of the trigger.
	Name() string

	// RegisterAgent registers an agent with the trigger.
	RegisterAgent(*Agent) error

	// GetRoutes returns a list of all HTTP routes defined by this trigger.
	GetRoutes() []HTTPTriggerRoute

	// SetWebhook configures the trigger's webhook settings.
	SetWebhook(baseURL string) error
}
