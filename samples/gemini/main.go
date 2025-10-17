package main

import (
	"context"
	"database/sql"
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
		log.Fatalf("CONN_STRING environment variable is empty")
	}

	GEMINI_API_KEY := os.Getenv("GEMINI_API_KEY")
	if GEMINI_API_KEY == "" {
		log.Fatalf("GEMINI_API_KEY environment variable is empty")
	}

	TGBOT_TOKEN := os.Getenv("TGBOT_TOKEN")
	if TGBOT_TOKEN == "" {
		log.Fatalf("TGBOT_TOKEN environment variable is empty")
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
		log.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	pgm := pgmemory.NewHistoryMemory(db)

	if err := pgm.Init(ctx); err != nil {
		log.Fatal(err)
	}

	if err := pgm.SetMaxMessagesPerConversation(ctx, 10); err != nil {
		log.Fatal(err)
	}

	// Agent
	e21 := &agens.Agent{
		Name:        "e21",
		Description: "a general-purpose virtual assistant",
		Instructions: []string{
			"You receive messages from users via a Telegram bot and must respond to their messages.",
		},
		Model: model,
		Batcher: &timedbatcher.TimedBatcher{
			Duration: 5 * time.Second,
		},
		HistoryMemory: pgm,
	}

	if err := e21.Init(ctx, g); err != nil {
		log.Fatal(err)
	}

	// Telegram bot
	bot, err := gotgbot.NewBot(TGBOT_TOKEN, nil)
	if err != nil {
		log.Fatal(err)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println(err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})

	// Telegram bot trigger
	tgTrigger := tgbot.Trigger{
		Bot:        bot,
		Dispatcher: dispatcher,
	}

	if err := tgTrigger.RegisterAgent(e21); err != nil {
		log.Fatal(err)
	}

	// Updater
	updater := ext.NewUpdater(dispatcher, nil)

	updater.StartPolling(bot, &ext.PollingOpts{
		DropPendingUpdates: true,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout: 9,
			RequestOpts: &gotgbot.RequestOpts{
				Timeout: time.Second * 10,
			},
		},
	})
	if err != nil {
		panic("failed to start polling: " + err.Error())
	}
	log.Printf("%s has been started...\n", bot.User.Username)

	select {}
}
