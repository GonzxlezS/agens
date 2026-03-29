package wabot

import "github.com/wapikit/wapi.go/pkg/events"

type Handler struct {
	EventType events.EventType
	HandlerFn func(events.BaseEvent)
}
