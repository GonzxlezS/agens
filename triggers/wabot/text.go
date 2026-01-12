package wabot

import (
	"context"
	"encoding/json"

	"github.com/firebase/genkit/go/ai"
	"github.com/gonzxlezs/agens"
	"github.com/wapikit/wapi.go/pkg/components"
	"github.com/wapikit/wapi.go/pkg/events"
)

func (trigger *WebhookTrigger) TextHandler(agent *agens.Agent) func(event events.BaseEvent) {
	return func(event events.BaseEvent) {
		textMessageEvent := event.(*events.TextMessageEvent)

		jsonMsg, err := json.Marshal(textMessageEvent)
		if err != nil {
			trigger.Logger.Error(err.Error())
			return
		}

		var (
			aiMsg = ai.NewUserTextMessage(string(jsonMsg))
			from  = textMessageEvent.From

			ctx = context.Background()
		)

		agens.SetSource(aiMsg, trigger.Name())
		agens.SetUserID(aiMsg, from)
		agens.SetChannelID(aiMsg, from)

		resp, err := agent.Run(ctx, aiMsg)
		if err != nil {
			trigger.Logger.Error(err.Error())
			return
		}

		if resp.FinishReason == agens.FinishReasonDelegated {
			return
		}

		msg, err := components.NewTextMessage(components.TextMessageConfigs{
			Text: resp.Text(),
		})

		if err != nil {
			trigger.Logger.Error("error creating text message: " + err.Error())
			return
		}

		_, err = textMessageEvent.Reply(msg)
		if err != nil {
			trigger.Logger.Error(err.Error())
		}
	}
}
