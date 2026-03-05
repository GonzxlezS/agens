package wabot

import (
	"context"
	"encoding/json"

	"github.com/firebase/genkit/go/ai"
	"github.com/gonzxlezs/agens"
	"github.com/wapikit/wapi.go/pkg/components"
	"github.com/wapikit/wapi.go/pkg/events"
)

func (trigger *WebhookTrigger) TextHandler(event events.BaseEvent) {
	textMessageEvent := event.(*events.TextMessageEvent)

	jsonMsg, err := json.Marshal(textMessageEvent)
	if err != nil {
		trigger.Logger.Error(err.Error())
		return
	}

	var (
		aiMsg  = ai.NewUserTextMessage(string(jsonMsg))
		source = textMessageEvent.From

		ctx = context.Background()
	)

	// batch
	var batch = []*ai.Message{aiMsg}

	if trigger.Batcher != nil {
		batch, err = trigger.Batcher.Add(ctx, source, aiMsg)
		if err != nil {
			trigger.Logger.Error(err.Error())
			return
		}

		// delegated
		if len(batch) == 0 {
			return
		}
	}

	// input
	input := &agens.Input{
		Trigger:  trigger.TriggerID,
		Source:   source,
		Messages: batch,
	}

	// generate
	resp, err := trigger.Agent.Generate(ctx, input)
	if err != nil {
		trigger.Logger.Error(err.Error())
		return
	}

	// output
	msg, err := components.NewTextMessage(components.TextMessageConfigs{
		Text: resp.Text(),
	})

	if err != nil {
		trigger.Logger.Error("error creating text message: " + err.Error())
		return
	}

	// send
	_, err = textMessageEvent.Reply(msg)
	if err != nil {
		trigger.Logger.Error(err.Error())
	}
}
