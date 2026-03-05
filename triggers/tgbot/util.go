package tgbot

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func StartPolling(trigger *Trigger) error {
	err := trigger.Updater.StartPolling(trigger.Bot, trigger.PollingOpts)
	if err != nil {
		return fmt.Errorf("failed to start polling: %w", err)
	}
	return nil
}

func StopPolling(trigger *Trigger) error {
	return trigger.Updater.Stop()
}

func SetWebhook(trigger *WebhookTrigger, baseURL string) error {
	if trigger.SetWebhookOpts == nil {
		trigger.SetWebhookOpts = &gotgbot.SetWebhookOpts{}
	}

	trigger.SetWebhookOpts.SecretToken = trigger.SecretToken

	return trigger.Updater.SetAllBotWebhooks(
		baseURL+trigger.SubPath,
		trigger.SetWebhookOpts,
	)
}
