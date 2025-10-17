package tgbot

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const MaxLengthMessageText = 4096

type SendMessageParameters struct {
	ChatID  int64  `json:"chat_id,omitempty"`
	Text    string `json:"text"`
	ReplyTo int64  `json:"reply_to,omitempty"`
}

func (trigger *Trigger) SendMessage(tgCtx *ext.Context, params []SendMessageParameters) error {
	var err error

	for _, msgParams := range params {
		// chat id
		if msgParams.ChatID == 0 {
			msgParams.ChatID = tgCtx.Message.Chat.Id
		}

		// text length
		if len(msgParams.Text) <= MaxLengthMessageText {
			_, err = trigger.sendMessage(&msgParams)
			continue
		}

		// split message
		var replyTo = msgParams.ReplyTo

		for _, chunk := range splitMessageText(msgParams.Text) {
			NewMsgParams := &SendMessageParameters{
				ChatID:  msgParams.ChatID,
				Text:    chunk,
				ReplyTo: replyTo,
			}

			replyTo, err = trigger.sendMessage(NewMsgParams)
			if err != nil {
				break
			}
		}
	}

	return err
}

func (trigger *Trigger) sendMessage(params *SendMessageParameters) (int64, error) {
	msg, err := trigger.Bot.SendMessage(
		params.ChatID,
		params.Text,
		&gotgbot.SendMessageOpts{
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId: params.ReplyTo,
			},
		},
	)

	if err != nil {
		return 0, err
	}
	return msg.MessageId, nil
}

func splitMessageText(text string) []string {
	var (
		textLength = len(text)
		chunks     []string
	)

	for i := 0; i < textLength; i += MaxLengthMessageText {
		end := i + MaxLengthMessageText
		if end > textLength {
			end = textLength
		}

		chunks = append(chunks, text[i:end])
	}
	return chunks
}
