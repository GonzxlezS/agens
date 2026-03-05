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
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/firebase/genkit/go/plugins/ollama"
	"github.com/gonzxlezs/agens"
	"github.com/gonzxlezs/agens/extensions/pgmemory"
	"github.com/gonzxlezs/agens/extensions/timedbatcher"
	"github.com/gonzxlezs/agens/triggers/tgbot"
	_ "github.com/lib/pq"
	"google.golang.org/genai"

	_ "github.com/joho/godotenv/autoload"
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

	OLLAMA_HOST := os.Getenv("OLLAMA_HOST")
	if OLLAMA_HOST == "" {
		panic("OLLAMA_HOST environment variable is empty")
	}

	// ollama
	ollamaPlugin := &ollama.Ollama{
		ServerAddress: OLLAMA_HOST,
		Timeout:       120,
	}

	// Genkit
	ctx := context.Background()

	g := genkit.Init(ctx,
		genkit.WithPlugins(
			ollamaPlugin,
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

	embedder := ollamaPlugin.DefineEmbedder(g, OLLAMA_HOST, "embeddinggemma", nil)

	// PGMemory
	db, err := sql.Open("postgres", CONN_STRING)
	if err != nil {
		panic(err)
	}

	historyMemory, err := pgmemory.NewHistoryMemory("history", db)
	if err != nil {
		panic(err)
	}

	knowledgeProvider, err := pgmemory.NewKnowledgeMemory(
		g,
		db,
		pgmemory.KnowledgeMemoryConfig{
			Name: "knowledge",
			Description: "USE THIS TOOL when the user asks for information about errors in the agens framework;" +
				" it returns definitions of the errors.",
			Embedder:   embedder,
			Dimensions: 768,
		},
	)

	if err != nil {
		panic(err)
	}

	// Agent
	e21, err := agens.NewAgent(g, agens.AgentConfig{
		ID:          "agent02",
		Name:        "e21",
		Description: "General purpose virtual assistant for developers.",
		Instructions: []string{
			"You receive messages from users via a Telegram bot.",
			"Do not assume or invent errors; always consult the error knowledge tool.",
			"Explain the error clearly and suggest a fix based on the documentation found.",
		},
		Model:                  model,
		HistoryMemory:          historyMemory,
		HistoryMemorySize:      5,
		KnowledgeMemory:        knowledgeProvider,
		KnowledgeRetrieveLimit: 1,
	})

	if err != nil {
		panic(err)
	}

	err = e21.IndexKnowledge(ctx, "agens_errors", []*ai.Document{
		ai.DocumentFromText(agensErrorsP1, nil),
		ai.DocumentFromText(agensErrorsP2, nil),
		ai.DocumentFromText(agensErrorsP3, nil),
	})

	if err != nil {
		panic(err)
	}

	// Telegram bot trigger
	tgTrigger, err := tgbot.NewTrigger(
		"tgbot02",
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

	batcher := &timedbatcher.TimedBatcher{Duration: 5 * time.Second}
	if err := tgTrigger.WithBatcher(batcher); err != nil {
		panic(err)
	}

	// start
	if err := tgbot.StartPolling(tgTrigger); err != nil {
		panic(err)
	}
	fmt.Printf("%s has been started...\n", tgTrigger.Bot.User.Username)

	select {}
}

const agensErrorsP1 = `agens errors (v0.4.0):
	ErrAgentNotInitialized: "agent not initialized, is returned if an attempt 
	is made to run an agent that has not been properly initialized."

	ErrKnowledgeMemoryNotConfigured: "knowledge memory is not configured for this agent, 
	is returned when an operation is attempted on an agent that does not have a KnowledgeMemory initialized."

	ErrInvalidOutputOption: "output option in context is invalid is returned when 
	a value associated with the output option key exists in the context, 
	but it is not of the expected type (ai.OutputOption)."
`

const agensErrorsP2 = `
	ErrMetadataNotFound: "metadata not found, is returned if the message metadata map is nil."

	ErrChannelIDNotInMetadata: "channel ID not found in metadata, is returned if the channel ID 
	is not present in the message metadata."

	ErrSourceNotInMetadata: "source not found in metadata, is returned if the source is not 
	present in the message metadata."

	ErrUserIDNotInMetadata: "user ID not found in metadata, is returned if the user ID is 
	not present in the message metadata."
`

const agensErrorsP3 = `
	ErrChannelIDNotAString: "channel ID is not a string type, is returned if the channel ID in metadata is not a string."
 
	ErrSourceNotAString: "source is not a string type, is returned if the source in metadata is not a string."

	ErrStoredIDNotAString: "stored ID is not a string type, is returned if the stored ID in metadata is not a string."

	ErrUserIDNotAString: "user ID is not a string type, is returned if the user ID in metadata is not a string."
`
