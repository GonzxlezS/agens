package wabot

import (
	"net/http"

	"github.com/gonzxlezs/agens"

	"github.com/labstack/echo/v4"
)

func (trigger *Trigger) GetRoutes() []agens.HTTPTriggerRoute {

	path := trigger.SubPath + trigger.Client.Business.BusinessAccountId

	getHandler := trigger.Client.GetWebhookGetRequestHandler()
	postHandler := trigger.Client.GetWebhookPostRequestHandler()

	return []agens.HTTPTriggerRoute{
		{
			Method:  http.MethodGet,
			Path:    path,
			Handler: trigger.wrapHandler(getHandler),
		},
		{
			Method:  http.MethodPost,
			Path:    path,
			Handler: trigger.wrapHandler(postHandler),
		},
	}
}

func (_ *Trigger) SetWebhook(_ string) error {
	return nil
}

func (trigger *Trigger) wrapHandler(handler echo.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := trigger.Echo.AcquireContext()
		c.Reset(r, w)

		err := handler(c)
		trigger.Echo.ReleaseContext(c)

		if err != nil {
			trigger.Echo.HTTPErrorHandler(err, c)
		}
	}
}
