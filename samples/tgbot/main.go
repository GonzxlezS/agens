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
	"github.com/gonzxlezs/agens/extensions/tgbot"
	"github.com/gonzxlezs/agens/extensions/timedbatcher"
	_ "github.com/lib/pq"
	"google.golang.org/genai"

	_ "github.com/joho/godotenv/autoload"
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

	model := googlegenai.ModelRef(
		"googleai/gemini-2.5-flash",
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

	historyMemory, err := pgmemory.NewHistoryMemory(pgmemory.HistoryMemoryConfig{
		TableName: "history",
		DB:        db,
		OwnsDB:    true,
	})
	if err != nil {
		panic(err)
	}
	defer historyMemory.Close()

	// Agent
	e21, err := agens.NewAgent(agens.AgentConfig{
		ID:          "tg_agent1",
		Name:        "e21",
		Description: "a general-purpose virtual assistant",
		Instructions: []string{
			"You receive messages from users via a Telegram bot and must respond to their messages.",
		},
		Model:         model,
		HistoryMemory: historyMemory,
		HistoryManager: agens.SlidingWindowManager{
			WindowSize: 20,
		},
	})

	if err != nil {
		panic(err)
	}

	genkit.RegisterAction(g, e21)

	// Telegram bot
	gateway, err := tgbot.NewTgGateway(tgbot.TgGatewayConfig{
		ID:            "tgbot01",
		Token:         TGBOT_TOKEN,
		WebhookSecret: TGBOT_WEBHOOK_SECRET,
	})
	if err != nil {
		panic(err)
	}

	if err := gateway.RegisterAgent(e21); err != nil {
		panic(err)
	}

	batcher := &timedbatcher.TimedBatcher{Duration: 5 * time.Second}
	if err := gateway.WithMessageBatcher(batcher); err != nil {
		panic(err)
	}

	// server
	mux := http.NewServeMux()
	for _, route := range gateway.GetRoutes() {
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

	err = gateway.SetWebhook(TGBOT_WEBHOOK_DOMAIN, nil)
	if err != nil {
		panic(err)
	}

	gateway.IDLE()
}
