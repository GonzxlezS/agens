package tgbot

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const MaxLengthMessageText = 4096

type SendMessageParameters struct {
	// ChatID Unique identifier for the target chat
	ChatID int64 `json:"chat_id"`

	// Text text of the message to be sent, 1-4096 characters after entities parsing
	Text string `json:"text"`

	// ParseMode mode for parsing entities in the message text.
	ParseMode string `json:"parse_mode,omitempty" jsonschema:"enum=HTML,enum=MarkdownV2,enum=Markdown"`

	// ReplyParameters description of the message to reply to
	ReplyParameters *gotgbot.ReplyParameters `json:"reply_parameters,omitempty"`
}

func (params *SendMessageParameters) newWithText(text string) *SendMessageParameters {
	return &SendMessageParameters{
		ChatID:          params.ChatID,
		Text:            text,
		ParseMode:       params.ParseMode,
		ReplyParameters: params.ReplyParameters,
	}
}

func (params *SendMessageParameters) sendMessageOpts() *gotgbot.SendMessageOpts {
	return &gotgbot.SendMessageOpts{
		ParseMode:       params.ParseMode,
		ReplyParameters: params.ReplyParameters,
	}
}

func (trigger *BaseTrigger) SendMessage(tgCtx *ext.Context, sendParams []*SendMessageParameters) error {
	for _, params := range splitMessageText(sendParams) {
		// chat id
		if params.ChatID == 0 {
			params.ChatID = tgCtx.Message.Chat.Id
		}

		// send
		_, err := trigger.Bot.SendMessage(
			params.ChatID,
			params.Text,
			params.sendMessageOpts(),
		)

		if err != nil {
			return err
		}
	}

	return nil
}

func splitMessageText(sendParams []*SendMessageParameters) []*SendMessageParameters {
	var result []*SendMessageParameters

	for _, params := range sendParams {
		runes := []rune(params.Text)
		runeLength := len(runes)

		if runeLength <= MaxLengthMessageText {
			result = append(result, params)
			continue
		}

		// split
		for i := 0; i < runeLength; i += MaxLengthMessageText {
			end := min(i+MaxLengthMessageText, runeLength)

			result = append(result, params.newWithText(string(runes[i:end])))
		}
	}

	return result
}
