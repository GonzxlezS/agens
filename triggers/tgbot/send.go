package tgbot

import (
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/firebase/genkit/go/ai"
)

func (trigger *Trigger) SendMessage(tgCtx *ext.Context, msg *ai.Message) error {
	_, err := trigger.Bot.SendMessage(
		tgCtx.Message.Chat.Id,
		msg.Text(),
		nil,
	)
	return err
}
