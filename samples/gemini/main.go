package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/gonzxlezs/agens"
	"github.com/gonzxlezs/agens/extensions/pgmemory"
	"github.com/gonzxlezs/agens/extensions/timedbatcher"
	"github.com/gonzxlezs/agens/triggers/tgbot"
	_ "github.com/lib/pq"
	"google.golang.org/genai"
)

func main() {
	// Environment variables
	CONN_STRING := os.Getenv("CONN_STRING")
	if CONN_STRING == "" {
		panic("CONN_STRING environment variable is empty")
	}

	GEMINI_API_KEY := os.Getenv("GEMINI_API_KEY")
	if GEMINI_API_KEY == "" {
		panic("GEMINI_API_KEY environment variable is empty")
	}

	TGBOT_TOKEN := os.Getenv("TGBOT_TOKEN")
	if TGBOT_TOKEN == "" {
		panic("TGBOT_TOKEN environment variable is empty")
	}

	// Genkit
	ctx := context.Background()

	g := genkit.Init(ctx,
		genkit.WithPlugins(
			&googlegenai.GoogleAI{
				APIKey: GEMINI_API_KEY,
			},
		),
	)

	model := googlegenai.GoogleAIModelRef(
		"gemini-2.5-flash",
		&genai.GenerateContentConfig{
			MaxOutputTokens: 500,
			Temperature:     genai.Ptr[float32](0.5),
			TopP:            genai.Ptr[float32](0.4),
			TopK:            genai.Ptr[float32](50),
		},
	)

	// PGMemory
	db, err := sql.Open("postgres", CONN_STRING)
	if err != nil {
		panic(err)
	}

	pgm, err := pgmemory.NewHistoryProvider(db)
	if err != nil {
		panic(err)
	}

	// Agent
	e21, err := agens.NewAgent(g, agens.AgentConfig{
		Name:        "e21",
		Description: "a general-purpose virtual assistant",
		Instructions: []string{
			"You receive messages from users via a Telegram bot and must respond to their messages.",
		},
		Model: model,
		Batcher: &timedbatcher.TimedBatcher{
			Duration: 5 * time.Second,
		},
		HistoryProvider:            pgm,
		MaxMessagesPerConversation: 5,
	})

	if err != nil {
		panic(err)
	}

	// Telegram bot trigger
	tgTrigger, err := tgbot.NewTrigger(
		TGBOT_TOKEN,
		&tgbot.TriggerOpts{
			DispatcherOpts: &ext.DispatcherOpts{
				Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
					log.Println(err.Error())
					return ext.DispatcherActionNoop
				},
				MaxRoutines: ext.DefaultMaxRoutines,
			},
			PollingOpts: &ext.PollingOpts{
				DropPendingUpdates: true,
				GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
					Timeout: 9,
					RequestOpts: &gotgbot.RequestOpts{
						Timeout: time.Second * 10,
					},
				},
			},
		},
	)

	if err != nil {
		panic(err)
	}

	if err := tgTrigger.RegisterAgent(e21); err != nil {
		panic(err)
	}

	if err := tgTrigger.Start(ctx); err != nil {
		panic(err)
	}
	fmt.Printf("%s has been started...\n", tgTrigger.Bot.User.Username)

	select {}
}
