package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/gonzxlezs/agens"
	"github.com/gonzxlezs/agens/extensions/pgmemory"
	"github.com/gonzxlezs/agens/extensions/timedbatcher"
	"github.com/gonzxlezs/agens/triggers/tgbot"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	_ "github.com/lib/pq"
)

const (
	CONN_STRING = ""

	GEMINI_API_KEY = ""

	TGBOT_TOKEN = ""
)

func main() {
	// Genkit
	ctx := context.Background()

	g := genkit.Init(ctx,
		genkit.WithPlugins(
			&googlegenai.GoogleAI{
				APIKey: GEMINI_API_KEY,
			},
		),
		genkit.WithDefaultModel("googleai/gemini-2.5-flash"),
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
		Description: "General AI assistant",
		Instructions: []string{
			"You are a general-purpose virtual assistant.",
			"You receive messages from users via a Telegram bot and must respond to their messages.",
		},
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
