package tgbot

import (
	"net/http"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/gonzxlezs/agens"
)

const DefaultSubPath = "/tgbot/"

var _ agens.HTTPTrigger = &HTTPTrigger{}

type HTTPTriggerOpts struct {
	BotOpts        *gotgbot.BotOpts
	DispatcherOpts *ext.DispatcherOpts
	UpdaterOpts    *ext.UpdaterOpts

	SubPath        string
	SecretToken    string
	SetWebhookOpts *gotgbot.SetWebhookOpts
}

type HTTPTrigger struct {
	BaseTrigger

	SubPath        string
	SecretToken    string
	SetWebhookOpts *gotgbot.SetWebhookOpts
}

func NewHTTPTrigger(token string, opts *HTTPTriggerOpts) (*HTTPTrigger, error) {
	var (
		trigger = &HTTPTrigger{}
		err     error
	)

	if opts == nil {
		opts = &HTTPTriggerOpts{}
	}

	trigger.Bot, err = gotgbot.NewBot(token, opts.BotOpts)
	if err != nil {
		return nil, err
	}

	trigger.Dispatcher = ext.NewDispatcher(opts.DispatcherOpts)

	trigger.Updater = ext.NewUpdater(trigger.Dispatcher, opts.UpdaterOpts)

	trigger.SubPath = DefaultSubPath
	if opts.SubPath != "" {
		trigger.SubPath = opts.SubPath
	}

	trigger.SecretToken = opts.SecretToken

	trigger.SetWebhookOpts = opts.SetWebhookOpts

	return trigger, nil
}

func (trigger *HTTPTrigger) Name() string {
	return trigger.BaseTrigger.Name() + "Webhook"
}

func (trigger *HTTPTrigger) RegisterAgent(agent *agens.Agent) error {
	err := trigger.BaseTrigger.RegisterAgent(agent)
	if err != nil {
		return err
	}

	return trigger.Updater.AddWebhook(
		trigger.Bot,
		trigger.Bot.Token,
		&ext.AddWebhookOpts{
			SecretToken: trigger.SecretToken,
		},
	)
}

func (trigger *HTTPTrigger) GetRoutes() []agens.HTTPTriggerRoute {
	return []agens.HTTPTriggerRoute{
		{
			Method:  http.MethodPost,
			Path:    trigger.SubPath,
			Handler: trigger.Updater.GetHandlerFunc(trigger.SubPath),
		},
	}
}

func (trigger *HTTPTrigger) SetWebhook(baseURL string) error {
	if trigger.SetWebhookOpts == nil {
		trigger.SetWebhookOpts = &gotgbot.SetWebhookOpts{}
	}

	trigger.SetWebhookOpts.SecretToken = trigger.SecretToken

	return trigger.Updater.SetAllBotWebhooks(
		baseURL+trigger.SubPath,
		trigger.SetWebhookOpts,
	)
}
