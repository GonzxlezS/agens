package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/gonzxlezs/agens"
	"github.com/gonzxlezs/agens/extensions/pgmemory"
	"github.com/gonzxlezs/agens/extensions/timedbatcher"
	"github.com/gonzxlezs/agens/extensions/wabot"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
	wapi "github.com/wapikit/wapi.go/pkg/client"
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

	WA_TOKEN := os.Getenv("WA_TOKEN")
	if WA_TOKEN == "" {
		log.Fatalf("WA_TOKEN environment variable is empty")
	}

	WA_BUSINESS_ID := os.Getenv("WA_BUSINESS_ID")
	if WA_BUSINESS_ID == "" {
		log.Fatalf("WA_BUSINESS_ID environment variable is empty")
	}

	WEBHOOK_SECRET := os.Getenv("WEBHOOK_SECRET")
	if WEBHOOK_SECRET == "" {
		log.Fatalf("WEBHOOK_SECRET environment variable is empty")
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
		ID:          "wa_agent1",
		Name:        "e21",
		Description: "a general-purpose virtual assistant",
		Instructions: []string{
			"You receive messages from users via a WhatsApp bot and must respond to their messages.",
		},
		Model:         model,
		HistoryMemory: historyMemory,
		HistoryManager: agens.SlidingWindowManager{
			WindowSize: 10,
		},
	})

	if err != nil {
		panic(err)
	}

	genkit.RegisterAction(g, e21)

	// Whatsapp
	gateway, err := wabot.NewWAGateway(wabot.WAGatewayConfig{
		ID: "wa01",
		ClientConfig: &wapi.ClientConfig{
			BusinessAccountId: WA_BUSINESS_ID,
			ApiAccessToken:    WA_TOKEN,
			WebhookSecret:     WEBHOOK_SECRET,
		},
	})

	if err != nil {
		panic(err)
	}

	if err := gateway.RegisterAgent(e21); err != nil {
		panic(err)
	}

	batcher := &timedbatcher.TimedBatcher{Duration: 10 * time.Second}
	if err := gateway.WithMessageBatcher(batcher); err != nil {
		panic(err)
	}

	// server
	server := echo.New()

	for _, route := range gateway.WebhookRoutes() {
		server.Add(route.Method, route.Path, route.Handler)
	}

	server.Start(":8080")
}
