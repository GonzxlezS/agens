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
	"github.com/gonzxlezs/agens/triggers/wabot"
	_ "github.com/lib/pq"
	wapi "github.com/wapikit/wapi.go/pkg/client"
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
			"You receive messages from users via a WhatsApp bot and must respond to their messages.",
		},
		Model: model,
		Batcher: &timedbatcher.TimedBatcher{
			Duration: 10 * time.Second,
		},
		HistoryMemory: pgm,
	}

	if err := e21.Init(ctx, g); err != nil {
		panic(err)
	}

	// Whatsapp bot trigger
	waTrigger := wabot.NewTrigger(&wapi.ClientConfig{
		BusinessAccountId: WA_BUSINESS_ID,
		ApiAccessToken:    WA_TOKEN,
		WebhookSecret:     WEBHOOK_SECRET,
	})

	if err := waTrigger.RegisterAgent(e21); err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	for _, route := range waTrigger.GetRoutes() {
		pattern := fmt.Sprintf("%s %s", route.Method, route.Path)
		fmt.Println(pattern)
		mux.HandleFunc(pattern, route.Handler)
	}

	server := &http.Server{
		Addr:    ":" + PORT,
		Handler: mux,
	}

	fmt.Printf("Listening for webhooks on port %s...\n", PORT)

	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic("HTTP server failed: " + err.Error())
	}

	select {}
}
