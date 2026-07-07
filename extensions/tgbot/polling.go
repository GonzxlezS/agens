package tgbot

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func (g *TgGateway) StartPolling(opts *ext.PollingOpts) error {
	err := g.Updater.StartPolling(g.Bot, opts)
	if err != nil {
		return fmt.Errorf("failed to start polling: %w", err)
	}
	return nil
}

func (g *TgGateway) StopPolling() error {
	return g.Updater.Stop()
}

func (g *TgGateway) IDLE() {
	g.Updater.Idle()
}
