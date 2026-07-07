package wabot

import (
	"context"

	"github.com/gonzxlezs/agens"
	"github.com/wapikit/wapi.go/pkg/events"
)

func (g *WAGateway) AddPipeline(eventType events.EventType, processor agens.PipelineProcessor[events.BaseEvent]) error {
	pipeline, err := agens.NewPipeline(processor, nil)
	if err != nil {
		return err
	}

	g.Client.On(eventType, func(event events.BaseEvent) {
		ok, err := pipeline(context.Background(), g, event)
		if err != nil {
			g.Logger.Error("pipeline execution failed", "event_type", eventType, "error", err)
			return
		}

		if ok {
			g.Logger.Info("pipeline executed successfully and model response handled", "event_type", eventType)
		} else {
			g.Logger.Debug("message appended to batch, execution deferred", "event_type", eventType)
		}
	})

	return nil
}
