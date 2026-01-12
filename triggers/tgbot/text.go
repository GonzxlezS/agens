package tgbot

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/gonzxlezs/agens"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/firebase/genkit/go/ai"
)

const MaxLengthMessageText = 4096

type MessageResponse struct {
	// Text text of the message to be sent, 1-4096 characters after entities parsing.
	Text string `json:"text" jsonschema_description:"Text of the message to be sent, 1-4096 characters after entities parsing."`

	// ParseMode mode for parsing entities in the message text.
	ParseMode string `json:"parse_mode,omitempty" jsonschema:"enum=HTML,enum=MarkdownV2,enum=Markdown,description=Mode for parsing entities in the message text."`

	// ReplyTo identifier of the message that will be replied to in the current chat.
	ReplyTo int64 `json:"reply_to,omitempty" jsonschema_description:"Identifier of the message that will be replied to in the current chat."`
}

func (params *MessageResponse) sendMessageOpts() *gotgbot.SendMessageOpts {
	return &gotgbot.SendMessageOpts{
		ParseMode: params.ParseMode,
		ReplyParameters: &gotgbot.ReplyParameters{
			MessageId: params.ReplyTo,
		},
	}
}

type MessageResponses struct {
	// Messages list of messages to be sent via the Telegram bot
	Messages []*MessageResponse `json:"messages" jsonschema:"description=List of messages to be sent via the Telegram bot,minItems=1,required"`
}

var outputType = MessageResponses{}

func (trigger *Trigger) TextHandler(agent *agens.Agent) ext.Handler {
	return handlers.NewMessage(
		message.Text,
		func(b *gotgbot.Bot, tgCtx *ext.Context) error {
			msg := tgCtx.Update.Message
			jsonMsg, err := json.Marshal(msg)
			if err != nil {
				return err
			}

			var (
				aiMsg  = ai.NewUserTextMessage(string(jsonMsg))
				userID = strconv.FormatInt(tgCtx.EffectiveUser.Id, 10)

				chatID    = tgCtx.EffectiveChat.Id
				channelID = strconv.FormatInt(chatID, 10)

				ctx = agens.WithOutputOption(
					context.Background(),
					ai.WithOutputType(outputType),
				)
			)

			agens.SetSource(aiMsg, trigger.Name())
			agens.SetUserID(aiMsg, userID)
			agens.SetChannelID(aiMsg, channelID)

			resp, err := agent.Run(ctx, aiMsg)
			if err != nil {
				return err
			}

			if resp.FinishReason == agens.FinishReasonDelegated {
				return nil
			}

			var params MessageResponses
			if err := resp.Output(&params); err != nil {
				return err
			}

			return trigger.SendMessage(chatID, params.Messages)
		},
	)
}

func (trigger *Trigger) SendMessage(chatID int64, sendParams []*MessageResponse) error {
	var (
		lastMsg *gotgbot.Message
		err     error
	)

	for _, params := range splitMessageText(sendParams) {
		// reply to
		if params.ReplyTo == -1 {
			if lastMsg != nil {
				params.ReplyTo = lastMsg.MessageId
			} else {
				params.ReplyTo = 0
			}
		}

		// send
		lastMsg, err = trigger.Bot.SendMessage(chatID, params.Text, params.sendMessageOpts())
		if err != nil {
			return err
		}
	}
	return nil
}

func splitMessageText(sendParams []*MessageResponse) []*MessageResponse {
	var result []*MessageResponse

	for _, params := range sendParams {
		var (
			runes      = []rune(params.Text)
			runeLength = len(runes)

			replyTo = params.ReplyTo
		)

		if runeLength <= MaxLengthMessageText {
			result = append(result, params)
			continue
		}

		// split
		for i := 0; i < runeLength; i += MaxLengthMessageText {
			end := min(i+MaxLengthMessageText, runeLength)

			result = append(result, &MessageResponse{
				Text:      string(runes[i:end]),
				ParseMode: params.ParseMode,
				ReplyTo:   replyTo,
			})

			replyTo = -1 // last msg
		}
	}

	return result
}
