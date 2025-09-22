package tgbot

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gonzxlezs/agens"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/firebase/genkit/go/ai"
)

const TextMessageFormat = "Telegram Bot: message %d, chat %d, user %d (%s %s, %s): %s (%d unix)"

func NewUserTextMessage(msg *gotgbot.Message) *ai.Message {
	if msg == nil {
		return &ai.Message{}
	}

	s := fmt.Sprintf(TextMessageFormat,
		msg.MessageId,
		msg.Chat.Id,
		msg.From.Id,
		msg.From.FirstName, msg.From.LastName,
		msg.From.Username,
		msg.Text,
		msg.Date,
	)

	return ai.NewUserTextMessage(s)
}

func (trigger *Trigger) TextHandler(agent *agens.Agent) ext.Handler {
	return handlers.NewMessage(
		message.Text,
		func(b *gotgbot.Bot, tgCtx *ext.Context) error {
			var (
				msg            = tgCtx.Update.Message
				aiMsg          = NewUserTextMessage(msg)
				userID         = strconv.FormatInt(msg.From.Id, 10)
				conversationID = strconv.FormatInt(msg.Chat.Id, 10)
			)

			agens.SetSource(aiMsg, trigger.Name())
			agens.SetUserID(aiMsg, userID)
			agens.SetConversationID(aiMsg, conversationID)

			resp, err := agent.Run(context.Background(), aiMsg)
			if err != nil {
				return err
			}

			if resp.FinishReason == agens.FinishReasonDelegated {
				return nil
			}

			return trigger.SendMessage(tgCtx, resp.Message)
		},
	)
}
