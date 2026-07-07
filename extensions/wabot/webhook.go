package wabot

import (
	"net/http"
	"path"

	"github.com/labstack/echo/v4"
)

type WebhookRoute struct {
	Method  string
	Path    string
	Handler echo.HandlerFunc
}

func (g *WAGateway) WebhookRoutes() []WebhookRoute {
	var path = path.Join(g.SubPath, g.SecretPath)

	return []WebhookRoute{
		{
			Method:  http.MethodGet,
			Path:    path,
			Handler: g.Client.GetWebhookGetRequestHandler(),
		},
		{
			Method:  http.MethodPost,
			Path:    path,
			Handler: g.Client.GetWebhookPostRequestHandler(),
		},
	}
}
