package tgbot

import (
	"net/http"
	"net/url"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type WebhookRoute struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

func (g *TgGateway) GetRoutes() []WebhookRoute {
	return []WebhookRoute{
		{
			Method:  http.MethodPost,
			Path:    g.SubPath,
			Handler: g.Updater.GetHandlerFunc(g.SubPath),
		},
	}
}

func (g *TgGateway) SetWebhook(domain string, opts *gotgbot.SetWebhookOpts) error {
	err := g.Updater.AddWebhook(
		g.Bot,
		g.SecretPath,
		&ext.AddWebhookOpts{
			SecretToken: g.WebhookSecret,
		},
	)
	if err != nil {
		return err
	}

	if opts == nil {
		opts = &gotgbot.SetWebhookOpts{}
	}
	opts.SecretToken = g.WebhookSecret

	baseURL, err := url.JoinPath(domain, g.SubPath)
	if err != nil {
		return err
	}

	return g.Updater.SetAllBotWebhooks(baseURL, opts)
}
