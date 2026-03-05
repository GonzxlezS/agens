package tgbot

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/firebase/genkit/go/ai"
	"github.com/gonzxlezs/agens"
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

func (trigger *Trigger) TextHandler(_ *gotgbot.Bot, tgCtx *ext.Context) error {
	jsonMsg, err := json.Marshal(tgCtx.Update.Message)
	if err != nil {
		return err
	}

	var (
		chatID = tgCtx.EffectiveChat.Id
		source = strconv.FormatInt(chatID, 10)

		msg = ai.NewUserTextMessage(string(jsonMsg))

		ctx = context.Background()
	)

	// batch
	var batch = []*ai.Message{msg}

	if trigger.Batcher != nil {
		batch, err = trigger.Batcher.Add(ctx, source, msg)
		if err != nil {
			return err
		}

		// delegated
		if len(batch) == 0 {
			return nil
		}
	}

	// input
	input := &agens.Input{
		Trigger:  trigger.TriggerID,
		Source:   source,
		Messages: batch,
	}

	input = input.WithOutputType(MessageResponses{})

	// generate
	resp, err := trigger.Agent.Generate(ctx, input)
	if err != nil {
		return err
	}

	// output
	var params MessageResponses
	if err := resp.Output(&params); err != nil {
		return err
	}

	// send
	return sendMessage(trigger.Bot, chatID, params.Messages)
}

func sendMessage(bot *gotgbot.Bot, chatID int64, sendParams []*MessageResponse) error {
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
		lastMsg, err = bot.SendMessage(chatID, params.Text, params.sendMessageOpts())
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
