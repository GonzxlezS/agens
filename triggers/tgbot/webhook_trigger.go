package tgbot

import (
	"net/http"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/gonzxlezs/agens"
)

const DefaultSubPath = "/tgbot/"

var _ agens.WebhookTrigger = &WebhookTrigger{}

type WebhookTriggerOpts struct {
	BotOpts        *gotgbot.BotOpts
	DispatcherOpts *ext.DispatcherOpts
	UpdaterOpts    *ext.UpdaterOpts

	SubPath        string
	SecretToken    string
	SetWebhookOpts *gotgbot.SetWebhookOpts
}

type WebhookTrigger struct {
	BaseTrigger *Trigger

	SubPath        string
	SecretToken    string
	SetWebhookOpts *gotgbot.SetWebhookOpts
}

func NewWebhookTrigger(token string, opts *WebhookTriggerOpts) (*WebhookTrigger, error) {
	var (
		trigger = &WebhookTrigger{}
		err     error
	)

	if opts == nil {
		opts = &WebhookTriggerOpts{}
	}

	trigger.BaseTrigger, err = NewTrigger(token, &TriggerOpts{
		BotOpts:        opts.BotOpts,
		DispatcherOpts: opts.DispatcherOpts,
		UpdaterOpts:    opts.UpdaterOpts,
	})

	if err != nil {
		return nil, err
	}

	trigger.SubPath = DefaultSubPath
	if opts.SubPath != "" {
		trigger.SubPath = opts.SubPath
	}

	trigger.SecretToken = opts.SecretToken

	trigger.SetWebhookOpts = opts.SetWebhookOpts

	return trigger, nil
}

func (trigger *WebhookTrigger) Name() string {
	return trigger.BaseTrigger.Name() + "Webhook"
}

func (trigger *WebhookTrigger) RegisterAgent(agent *agens.Agent) error {
	err := trigger.BaseTrigger.RegisterAgent(agent)
	if err != nil {
		return err
	}

	return trigger.BaseTrigger.Updater.AddWebhook(
		trigger.BaseTrigger.Bot,
		trigger.BaseTrigger.Bot.Token,
		&ext.AddWebhookOpts{
			SecretToken: trigger.SecretToken,
		},
	)
}

func (trigger *WebhookTrigger) GetRoutes() []agens.WebhookTriggerRoute {
	return []agens.WebhookTriggerRoute{
		{
			Method:  http.MethodPost,
			Path:    trigger.SubPath,
			Handler: trigger.BaseTrigger.Updater.GetHandlerFunc(trigger.SubPath),
		},
	}
}

func (trigger *WebhookTrigger) SetWebhook(baseURL string) error {
	if trigger.SetWebhookOpts == nil {
		trigger.SetWebhookOpts = &gotgbot.SetWebhookOpts{}
	}

	trigger.SetWebhookOpts.SecretToken = trigger.SecretToken

	return trigger.BaseTrigger.Updater.SetAllBotWebhooks(
		baseURL+trigger.SubPath,
		trigger.SetWebhookOpts,
	)
}
