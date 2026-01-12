package agens

import "net/http"

// WebhookTriggerRoute represents a specific HTTP endpoint mapping for a webhook.
type WebhookTriggerRoute struct {
	// Method specifies the required HTTP verb (GET, POST, etc.).
	Method string

	// Path defines the URL pattern that matches this route.
	Path string

	// Handler contains the logic to process the incoming HTTP request.
	Handler http.HandlerFunc
}

// WebhookTrigger defines a component that exposes HTTP endpoints to react to external events (webhooks).
type WebhookTrigger interface {
	// Name returns the name of the webhook trigger.
	Name() string

	// RegisterAgent associates an agent to handle incoming webhook data.
	RegisterAgent(*Agent) error

	// GetRoutes returns the collection of WebhookTriggerRoute definitions 
	// to be registered in an HTTP server.
	GetRoutes() []WebhookTriggerRoute

	// SetWebhook configures the external-facing connectivity settings, 
	// such as the base URL for the callback.
	SetWebhook(baseURL string) error
}
