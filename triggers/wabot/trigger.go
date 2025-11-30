package wabot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/gonzxlezs/agens"

	"github.com/firebase/genkit/go/ai"
	"github.com/labstack/echo/v4"
	wapi "github.com/wapikit/wapi.go/pkg/client"
	"github.com/wapikit/wapi.go/pkg/components"
	"github.com/wapikit/wapi.go/pkg/events"
)

const DefaultSubPath = "/wabot/"

var _ agens.HTTPTrigger = &Trigger{}

type Trigger struct {
	Client  *wapi.Client
	Echo    *echo.Echo
	Logger  *slog.Logger
	SubPath string
}

func NewTrigger(config *wapi.ClientConfig) *Trigger {
	return &Trigger{
		Client:  wapi.New(config),
		Echo:    echo.New(),
		Logger:  slog.New(slog.NewTextHandler(os.Stdout, nil)),
		SubPath: DefaultSubPath,
	}
}

func (trigger *Trigger) Name() string {
	return "WABot"
}

func (trigger *Trigger) RegisterAgent(agent *agens.Agent) error {
	trigger.Client.On(
		events.TextMessageEventType,
		func(event events.BaseEvent) {
			textMessageEvent := event.(*events.TextMessageEvent)

			jsonMsg, err := json.Marshal(textMessageEvent)
			if err != nil {
				trigger.Logger.Error(err.Error())
				return
			}

			var (
				aiMsg          = ai.NewUserTextMessage(string(jsonMsg))
				from           = textMessageEvent.From
				conversationID = fmt.Sprintf("%s:%s", trigger.Name(), from)

				ctx = context.Background()
			)

			agens.SetSource(aiMsg, trigger.Name())
			agens.SetUserID(aiMsg, from)
			agens.SetConversationID(aiMsg, conversationID)

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

		},
	)

	return nil
}
