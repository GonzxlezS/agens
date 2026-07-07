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
	"github.com/gonzxlezs/agens"
	"github.com/gonzxlezs/agens/extensions/pgmemory"
	"github.com/gonzxlezs/agens/extensions/tgbot"
	"github.com/gonzxlezs/agens/extensions/timedbatcher"
	_ "github.com/lib/pq"

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

	// Genkit
	ctx := context.Background()

	g := genkit.Init(ctx,
		genkit.WithPlugins(
			&googlegenai.GoogleAI{
				APIKey: GEMINI_API_KEY,
			},
		),
	)

	model := googlegenai.ModelRef("googleai/gemini-3.1-flash-lite", nil)
	imgModel := googlegenai.ModelRef("googleai/gemini-3.1-flash-lite-image", nil)

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

	// Sub Agent
	esse, err := agens.NewAgent(agens.AgentConfig{
		ID:          "agent_esse",
		Name:        "Esse",
		Description: "Responsible for generating images.",
		Instructions: []string{
			"Use “generateImage” tool to generate images.",
		},
		Model: model,
		Tools: []ai.ToolRef{
			defineImgGenTool(g, imgModel),
		},
	})

	if err != nil {
		panic(err)
	}
	genkit.RegisterAction(g, esse)

	// Agent
	eme, err := agens.NewAgent(agens.AgentConfig{
		ID:          "agent_eme",
		Name:        "Eme",
		Description: "General purpose virtual assistant.",
		Instructions: []string{
			"You receive messages from users via a Telegram bot.",
			"Use “Esse” to generate images.",
		},
		Model:         model,
		HistoryMemory: historyMemory,
		HistoryManager: agens.SlidingWindowManager{
			WindowSize: 10,
		},
		Tools: []ai.ToolRef{
			esse.AsTool(nil), // default cfg
		},
	})

	if err != nil {
		panic(err)
	}
	genkit.RegisterAction(g, eme)

	// Telegram bot
	gateway, err := tgbot.NewTgGateway(tgbot.TgGatewayConfig{
		ID:    "tgbot01",
		Token: TGBOT_TOKEN,
		DispatcherOpts: &ext.DispatcherOpts{
			Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
				log.Println(err.Error())
				return ext.DispatcherActionNoop
			},
			MaxRoutines: ext.DefaultMaxRoutines,
		},
	})
	if err != nil {
		panic(err)
	}

	if err := gateway.RegisterAgent(eme); err != nil {
		panic(err)
	}

	batcher := &timedbatcher.TimedBatcher{Duration: 5 * time.Second}
	if err := gateway.WithMessageBatcher(batcher); err != nil {
		panic(err)
	}

	// start
	if err := gateway.StartPolling(nil); err != nil {
		panic(err)
	}
	fmt.Printf("%s has been started...\n", gateway.Bot.User.Username)

	gateway.IDLE()
}

func defineImgGenTool(g *genkit.Genkit, model ai.ModelRef) ai.Tool {
	return genkit.DefineMultipartTool(
		g,
		"generateImage",
		"Generates an image from a text prompt",
		func(ctx *ai.ToolContext, in agens.DefaultToolInput) (*ai.MultipartToolResponse, error) {
			resp, err := genkit.Generate(ctx, g,
				ai.WithModel(model),
				ai.WithPrompt("Generate an image of %s", in.Input),
			)
			if err != nil {
				return nil, err
			}

			toolResp := &ai.MultipartToolResponse{
				Output: resp.Text(),
			}

			if resp.Message != nil {
				toolResp.Metadata = resp.Message.Metadata

				for _, part := range resp.Message.Content {
					if part.IsData() || part.IsMedia() {
						toolResp.Content = append(toolResp.Content, part)
					}
				}
			}

			return toolResp, nil
		},
	)
}
