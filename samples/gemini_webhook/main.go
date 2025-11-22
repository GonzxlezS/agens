package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

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

	PORT := os.Getenv("PORT")
	if PORT == "" {
		log.Fatalf("PORT environment variable is empty")
	}

	TGBOT_TOKEN := os.Getenv("TGBOT_TOKEN")
	if TGBOT_TOKEN == "" {
		log.Fatalf("TGBOT_TOKEN environment variable is empty")
	}

	TGBOT_WEBHOOK_DOMAIN := os.Getenv("TGBOT_WEBHOOK_DOMAIN")
	if TGBOT_WEBHOOK_DOMAIN == "" {
		log.Fatalf("TGBOT_WEBHOOK_DOMAIN environment variable is empty")
	}

	TGBOT_WEBHOOK_SECRET := os.Getenv("TGBOT_WEBHOOK_SECRET")
	if TGBOT_WEBHOOK_SECRET == "" {
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
		panic(err)
	}

	pgm, err := pgmemory.NewHistoryMemory(ctx, db)
	if err := pgm.Init(ctx); err != nil {
		panic(err)
	}

	if err := pgm.SetMaxMessagesPerConversation(ctx, 10); err != nil {
		panic(err)
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
		panic(err)
	}

	// Telegram bot trigger
	tgTrigger, err := tgbot.NewHTTPTrigger(
		TGBOT_TOKEN,
		&tgbot.HTTPTriggerOpts{
			SecretToken: TGBOT_WEBHOOK_SECRET,
		},
	)

	if err != nil {
		panic(err)
	}

	if err := tgTrigger.RegisterAgent(e21); err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	for _, route := range tgTrigger.GetRoutes() {
		pattern := fmt.Sprintf("%s %s", route.Method, route.Path)
		mux.HandleFunc(pattern, route.Handler)
	}

	server := &http.Server{
		Addr:    ":" + PORT,
		Handler: mux,
	}

	go func(server *http.Server) {
		fmt.Printf("Listening for webhooks on port %s...\n", PORT)

		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic("HTTP server failed: " + err.Error())
		}
	}(server)

	err = tgTrigger.SetWebhook(TGBOT_WEBHOOK_DOMAIN)
	if err != nil {
		panic(err)
	}

	select {}
}
