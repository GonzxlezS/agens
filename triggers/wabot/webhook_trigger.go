package wabot

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gonzxlezs/agens"

	"github.com/labstack/echo/v4"
	wapi "github.com/wapikit/wapi.go/pkg/client"
	"github.com/wapikit/wapi.go/pkg/events"
)

const DefaultSubPath = "/wabot/"

var _ agens.WebhookTrigger = &WebhookTrigger{}

var ErrEmptyTriggerID = errors.New("trigger id cannot be empty")

type WebhookTrigger struct {
	TriggerID string

	Client *wapi.Client
	Echo   *echo.Echo
	Logger *slog.Logger

	// SubPath is the base routing path for the webhook.
	SubPath string

	// SecretPath is a unique identifier or token appended to the URL
	// to prevent unauthorized update injections. Defaults to the business id.
	SecretPath string

	Agent   *agens.Agent
	Batcher agens.MessageBatcher
}

func NewWebhookTrigger(triggerID string, config *wapi.ClientConfig) (*WebhookTrigger, error) {
	triggerID = strings.TrimSpace(triggerID)
	if triggerID == "" {
		return nil, ErrEmptyTriggerID
	}

	trigger := &WebhookTrigger{
		TriggerID:  triggerID,
		Client:     wapi.New(config),
		Echo:       echo.New(),
		Logger:     slog.New(slog.NewTextHandler(os.Stdout, nil)),
		SubPath:    DefaultSubPath,
		SecretPath: config.BusinessAccountId,
	}

	trigger.AddHandler(&Handler{
		EventType: events.TextMessageEventType,
		HandlerFn: trigger.TextHandler,
	})

	return trigger, nil
}

func (trigger *WebhookTrigger) ID() string {
	return trigger.TriggerID
}

func (trigger *WebhookTrigger) Name() string {
	return "WABot"
}

func (trigger *WebhookTrigger) RegisterAgent(agent *agens.Agent) error {
	trigger.Agent = agent
	return nil
}

func (trigger *WebhookTrigger) WithBatcher(batcher agens.MessageBatcher) error {
	trigger.Batcher = batcher
	return nil
}

func (trigger *WebhookTrigger) GetRoutes() []agens.WebhookTriggerRoute {
	var (
		path        = trigger.SubPath + trigger.SecretPath
		getHandler  = trigger.Client.GetWebhookGetRequestHandler()
		postHandler = trigger.Client.GetWebhookPostRequestHandler()
	)

	return []agens.WebhookTriggerRoute{
		{
			Method:  http.MethodGet,
			Path:    path,
			Handler: wrapHandler(trigger.Echo, getHandler),
		},
		{
			Method:  http.MethodPost,
			Path:    path,
			Handler: wrapHandler(trigger.Echo, postHandler),
		},
	}
}

func (trigger *WebhookTrigger) AddHandler(handler *Handler) {
	trigger.Client.On(handler.EventType, handler.HandlerFn)
}

func wrapHandler(e *echo.Echo, handler echo.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := e.AcquireContext()
		c.Reset(r, w)

		err := handler(c)
		e.ReleaseContext(c)

		if err != nil {
			e.HTTPErrorHandler(err, c)
		}
	}
}
