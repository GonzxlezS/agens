package tgbot

import (
	"context"
	"fmt"

	"github.com/gonzxlezs/agens"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var _ agens.Trigger = &Trigger{}

type TriggerOpts struct {
	BotOpts        *gotgbot.BotOpts
	DispatcherOpts *ext.DispatcherOpts
	UpdaterOpts    *ext.UpdaterOpts

	PollingOpts *ext.PollingOpts
}

type Trigger struct {
	Bot        *gotgbot.Bot
	Dispatcher *ext.Dispatcher
	Updater    *ext.Updater

	PollingOpts *ext.PollingOpts
}

func NewTrigger(token string, opts *TriggerOpts) (*Trigger, error) {
	var (
		trigger = &Trigger{}
		err     error
	)

	if opts == nil {
		opts = &TriggerOpts{}
	}

	trigger.Bot, err = gotgbot.NewBot(token, opts.BotOpts)
	if err != nil {
		return nil, err
	}

	trigger.Dispatcher = ext.NewDispatcher(opts.DispatcherOpts)

	trigger.Updater = ext.NewUpdater(trigger.Dispatcher, opts.UpdaterOpts)

	trigger.PollingOpts = opts.PollingOpts

	return trigger, nil
}

func (trigger *Trigger) Name() string {
	return "TelegramBot"
}

func (trigger *Trigger) RegisterAgent(agent *agens.Agent) error {
	trigger.Dispatcher.AddHandler(trigger.TextHandler(agent))
	return nil
}

func (trigger *Trigger) Start(_ context.Context) error {
	err := trigger.Updater.StartPolling(trigger.Bot, trigger.PollingOpts)
	if err != nil {
		return fmt.Errorf("failed to start polling: %w", err)
	}
	return nil
}

func (trigger *Trigger) Stop(_ context.Context) error {
	return trigger.Updater.Stop()
}
