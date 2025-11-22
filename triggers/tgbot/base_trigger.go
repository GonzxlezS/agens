package tgbot

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/gonzxlezs/agens"
)

type BaseTrigger struct {
	Bot        *gotgbot.Bot
	Dispatcher *ext.Dispatcher
	Updater    *ext.Updater
}

func (trigger *BaseTrigger) Name() string {
	return "TelegramBot"
}

func (trigger *BaseTrigger) RegisterAgent(agent *agens.Agent) error {
	trigger.Dispatcher.AddHandler(trigger.TextHandler(agent))
	return nil
}
