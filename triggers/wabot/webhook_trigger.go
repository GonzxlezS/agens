package wabot

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/gonzxlezs/agens"

	"github.com/labstack/echo/v4"
	wapi "github.com/wapikit/wapi.go/pkg/client"
	"github.com/wapikit/wapi.go/pkg/events"
)

const (
	TriggerName = "WABot"

	DefaultSubPath = "/wabot/"
)

var _ agens.WebhookTrigger = &WebhookTrigger{}

type WebhookTrigger struct {
	Client  *wapi.Client
	Echo    *echo.Echo
	Logger  *slog.Logger
	SubPath string
}

func NewWebhookTrigger(config *wapi.ClientConfig) *WebhookTrigger {
	return &WebhookTrigger{
		Client:  wapi.New(config),
		Echo:    echo.New(),
		Logger:  slog.New(slog.NewTextHandler(os.Stdout, nil)),
		SubPath: DefaultSubPath,
	}
}

func (trigger *WebhookTrigger) Name() string {
	return TriggerName
}

func (trigger *WebhookTrigger) RegisterAgent(agent *agens.Agent) error {
	trigger.Client.On(events.TextMessageEventType, trigger.TextHandler(agent))
	return nil
}

func (trigger *WebhookTrigger) GetRoutes() []agens.WebhookTriggerRoute {
	var (
		path        = trigger.SubPath + trigger.Client.Business.BusinessAccountId
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

func (_ *WebhookTrigger) SetWebhook(_ string) error {
	return nil
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
