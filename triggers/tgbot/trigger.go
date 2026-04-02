package tgbot

import (
	"errors"
	"strings"

	"github.com/gonzxlezs/agens"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

var _ agens.Trigger = &Trigger{}

var ErrEmptyTriggerID = errors.New("trigger id cannot be empty")

type TriggerOpts struct {
	BotOpts        *gotgbot.BotOpts
	DispatcherOpts *ext.DispatcherOpts
	UpdaterOpts    *ext.UpdaterOpts
	PollingOpts    *ext.PollingOpts

	DisableDefaultHandlers bool
}

type Trigger struct {
	TriggerID string

	Bot        *gotgbot.Bot
	Dispatcher *ext.Dispatcher
	Updater    *ext.Updater

	PollingOpts *ext.PollingOpts

	Agent   *agens.Agent
	Batcher agens.MessageBatcher
}

func NewTrigger(triggerID string, token string, opts *TriggerOpts) (*Trigger, error) {
	triggerID = strings.TrimSpace(triggerID)
	if triggerID == "" {
		return nil, ErrEmptyTriggerID
	}

	var (
		trigger = &Trigger{
			TriggerID: triggerID,
		}

		err error
	)

	if opts == nil {
		opts = &TriggerOpts{}
	}

	trigger.Bot, err = gotgbot.NewBot(token, opts.BotOpts)
	if err != nil {
		return nil, err
	}

	trigger.Dispatcher = ext.NewDispatcher(opts.DispatcherOpts)

	if !opts.DisableDefaultHandlers {
		trigger.AddHandler(handlers.NewMessage(message.Text, trigger.TextHandler))
	}

	trigger.Updater = ext.NewUpdater(trigger.Dispatcher, opts.UpdaterOpts)

	trigger.PollingOpts = opts.PollingOpts

	return trigger, nil
}

func (trigger *Trigger) ID() string {
	return trigger.TriggerID
}

func (trigger *Trigger) Name() string {
	return "TelegramBot"
}

func (trigger *Trigger) RegisterAgent(agent *agens.Agent) error {
	trigger.Agent = agent
	return nil
}

func (trigger *Trigger) WithBatcher(batcher agens.MessageBatcher) error {
	trigger.Batcher = batcher
	return nil
}

func (trigger *Trigger) AddHandler(handler ext.Handler) {
	trigger.Dispatcher.AddHandler(handler)
}
