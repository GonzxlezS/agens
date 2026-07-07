package wabot

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gonzxlezs/agens"

	"github.com/firebase/genkit/go/ai"
	"github.com/wapikit/wapi.go/pkg/components"
	"github.com/wapikit/wapi.go/pkg/events"
)

type TextProcessor struct{}

func NewTextProcessor() *TextProcessor { return &TextProcessor{} }

func (p *TextProcessor) OriginalInputToMessage(ctx context.Context, state *agens.PipelineState[events.BaseEvent]) (string, *ai.Message, error) {
	var (
		textMessageEvent = state.OriginalInput().(*events.TextMessageEvent)
		from             = textMessageEvent.From
	)

	jsonMsg, err := json.Marshal(textMessageEvent)
	if err != nil {
		return "", nil, err
	}

	return from, ai.NewUserTextMessage(string(jsonMsg)), nil
}

func (p *TextProcessor) BatchToInput(ctx context.Context, state *agens.PipelineState[events.BaseEvent]) (*agens.Input, error) {
	var (
		textMessageEvent = state.OriginalInput().(*events.TextMessageEvent)
		from             = textMessageEvent.From
	)

	return &agens.Input{
		GatewayID: state.Gateway().ID(),
		SessionID: from,
		SenderID:  from,
		Messages:  state.BatchMessages(),
	}, nil
}

func (p *TextProcessor) HandleAgentResponse(ctx context.Context, state *agens.PipelineState[events.BaseEvent]) error {
	var (
		textMessageEvent = state.OriginalInput().(*events.TextMessageEvent)
		resp             = state.AgentResponse()
	)

	msg, err := components.NewTextMessage(components.TextMessageConfigs{
		Text: resp.Text(),
	})

	if err != nil {
		return fmt.Errorf("error creating text message: %w", err)
	}

	_, err = textMessageEvent.Reply(msg)
	return err
}
