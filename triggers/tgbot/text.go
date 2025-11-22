package tgbot

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gonzxlezs/agens"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/firebase/genkit/go/ai"
)

var outputType = []*SendMessageParameters{}

func (trigger *BaseTrigger) TextHandler(agent *agens.Agent) ext.Handler {
	return handlers.NewMessage(
		message.Text,
		func(b *gotgbot.Bot, tgCtx *ext.Context) error {
			msg := tgCtx.Update.Message
			jsonMsg, err := json.Marshal(msg)
			if err != nil {
				return err
			}

			var (
				aiMsg          = ai.NewUserTextMessage(string(jsonMsg))
				userID         = strconv.FormatInt(msg.From.Id, 10)
				conversationID = fmt.Sprintf("%s:%d", trigger.Name(), msg.Chat.Id)

				ctx = agens.WithOutputOption(
					context.Background(),
					ai.WithOutputType(outputType),
				)
			)

			agens.SetSource(aiMsg, trigger.Name())
			agens.SetUserID(aiMsg, userID)
			agens.SetConversationID(aiMsg, conversationID)

			resp, err := agent.Run(ctx, aiMsg)
			if err != nil {
				return err
			}

			if resp.FinishReason == agens.FinishReasonDelegated {
				return nil
			}

			var params []*SendMessageParameters
			if err := resp.Output(&params); err != nil {
				return err
			}

			return trigger.SendMessage(tgCtx, params)
		},
	)
}
