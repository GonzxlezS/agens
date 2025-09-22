package tgbot

import (
	"github.com/gonzxlezs/agens"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var _ agens.Trigger = &Trigger{}

type Trigger struct {
	Bot        *gotgbot.Bot
	Dispatcher *ext.Dispatcher
}

func (trigger *Trigger) Name() string {
	return "TelegramBot"
}

func (trigger *Trigger) RegisterAgent(agent *agens.Agent) error {
	trigger.Dispatcher.AddHandler(trigger.TextHandler(agent))
	return nil
}
