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
	*Trigger

	SubPath        string
	SecretToken    string
	SetWebhookOpts *gotgbot.SetWebhookOpts
}

func NewWebhookTrigger(triggerID string, token string, opts *WebhookTriggerOpts) (*WebhookTrigger, error) {
	var (
		trigger = &WebhookTrigger{}
		err     error
	)

	if opts == nil {
		opts = &WebhookTriggerOpts{}
	}

	trigger.Trigger, err = NewTrigger(triggerID, token, &TriggerOpts{
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
	return trigger.Trigger.Name() + "Webhook"
}

func (trigger *WebhookTrigger) RegisterAgent(agent *agens.Agent) error {
	err := trigger.Trigger.RegisterAgent(agent)
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

func (trigger *WebhookTrigger) GetRoutes() []agens.WebhookTriggerRoute {
	return []agens.WebhookTriggerRoute{
		{
			Method:  http.MethodPost,
			Path:    trigger.SubPath,
			Handler: trigger.Updater.GetHandlerFunc(trigger.SubPath),
		},
	}
}
