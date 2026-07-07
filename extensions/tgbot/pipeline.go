package tgbot

import (
	"context"

	"github.com/gonzxlezs/agens"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters"
)

func (g *TgGateway) AddPipeline(f filters.Message, processor agens.PipelineProcessor[*ext.Context]) error {
	pipeline, err := agens.NewPipeline(processor, nil)
	if err != nil {
		return err
	}

	g.Dispatcher.AddHandler(handlers.NewMessage(f,
		func(b *gotgbot.Bot, tgCtx *ext.Context) error {
			_, err := pipeline(context.Background(), g, tgCtx)
			if err != nil {
				return err
			}
			return nil
		},
	))

	return nil
}
