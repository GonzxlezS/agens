package agens

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// DefaultSystemMessageFormat is the default template used to format the
// system message passed to the AI model.
const DefaultSystemMessageFormat = `You are %s, %s.
instructions:
%s
---
%s`

// ErrNoValidMessages is returned when the message sequence fails validation or is empty.
var ErrNoValidMessages = errors.New("no valid messages in sequence")

// AgentConfig holds the execution parameters, memory providers, and behavior
// definitions for an Agent.
type AgentConfig struct {
	// ID is the unique internal identifier for the agent used in Genkit flow tracing.
	ID string

	// Name is a user-friendly name of the agent.
	Name string

	// Description is a brief description of the agent's purpose.
	Description string

	// Instructions are high-level instructions that guide the AI model's behavior.
	Instructions []string

	// Model is the AI model object that the agent will use to generate responses.
	// If specified, it takes precedence over ModelName.
	Model ai.ModelArg

	// Tools is a list of tools that the agent can use.
	Tools []ai.ToolRef

	// GenerateOptions are extra options passed to the genkit.Generate function.
	GenerateOptions []ai.GenerateOption

	// HistoryMemory is responsible for persisting the conversation history.
	HistoryMemory HistoryMemory

	// HistoryMemorySize limits the number of messages kept in the context window.
	HistoryMemorySize int

	// KnowledgeMemory enables RAG (Retrieval-Augmented Generation) capabilities.
	KnowledgeMemory KnowledgeMemory

	// KnowledgeRetrieveLimit sets the maximum number of documents fetched per query.
	KnowledgeRetrieveLimit int

	// SystemPromptFn allows for a custom implementation of the system instruction builder.
	SystemPromptFn func(ctx context.Context, cfg *AgentConfig, in *Input) (string, error)

	// SortMessagesFn provides a hook to filter or reorder messages before LLM inference.
	SortMessagesFn func(msgs []*ai.Message) ([]*ai.Message, error)
}

// GenerateSystemPrompt determines and executes the strategy for building
// the agent's system instructions.
func (cfg *AgentConfig) GenerateSystemPrompt(ctx context.Context, input *Input) (string, error) {
	if cfg.SystemPromptFn != nil {
		return cfg.SystemPromptFn(ctx, cfg, input)
	}
	return DefaultSystemPromptFn(ctx, cfg, input)
}

func (cfg *AgentConfig) flowFn(g *genkit.Genkit) func(ctx context.Context, input *Input) (*ai.ModelResponse, error) {
	var baseOpts = cfg.generateOptions()

	return func(ctx context.Context, input *Input) (*ai.ModelResponse, error) {
		// system prompt
		systemPrompt, err := cfg.GenerateSystemPrompt(ctx, input)
		if err != nil {
			return &ai.ModelResponse{}, err
		}

		// history
		history, err := cfg.retrieveHistory(ctx, input.SessionID())
		if err != nil {
			return &ai.ModelResponse{}, err
		}

		// messages
		messages, err := cfg.sortMessages(ctx, append(history, input.Messages...))
		if err != nil {
			return &ai.ModelResponse{}, err
		}

		// generate options
		generateOpts := append(baseOpts,
			ai.WithSystem(systemPrompt),
			ai.WithMessages(messages...),
		)

		generateOpts = append(generateOpts, input.outputOptions()...)

		// generate
		resp, err := genkit.Generate(ctx, g, generateOpts...)

		// store history
		if err == nil {
			err = cfg.storeHistory(ctx, input.SessionID(), resp.History())
		}

		// response
		return resp, err
	}
}

func (cfg *AgentConfig) generateOptions() []ai.GenerateOption {
	opts := append([]ai.GenerateOption(nil), cfg.GenerateOptions...)

	// Model
	if cfg.Model != nil {
		opts = append(opts,
			ai.WithModel(cfg.Model),
		)
	}

	// Tools
	tools := append([]ai.ToolRef(nil), cfg.Tools...)

	if cfg.KnowledgeMemory != nil {
		tools = append(tools,
			cfg.KnowledgeMemory.AsTool(cfg.ID, cfg.KnowledgeRetrieveLimit),
		)
	}

	if len(tools) > 0 {
		opts = append(opts,
			ai.WithTools(tools...),
		)
	}

	return opts
}

func (cfg *AgentConfig) sortMessages(ctx context.Context, msgs []*ai.Message) ([]*ai.Message, error) {
	return genkit.Run(ctx, "sortMessages", func() ([]*ai.Message, error) {
		if cfg.SortMessagesFn != nil {
			return cfg.SortMessagesFn(msgs)
		}
		return DefaultSortMessagesFn(msgs)
	})
}

func (cfg *AgentConfig) retrieveHistory(ctx context.Context, sessionID string) ([]*ai.Message, error) {
	if cfg.HistoryMemory == nil {
		return []*ai.Message{}, nil
	}

	return genkit.Run(ctx, "history", func() ([]*ai.Message, error) {
		return cfg.HistoryMemory.RetrieveHistory(ctx, cfg.ID, sessionID)
	})
}

func (cfg *AgentConfig) storeHistory(ctx context.Context, sessionID string, messages []*ai.Message) error {
	if cfg.HistoryMemory == nil {
		return nil
	}

	_, err := genkit.Run(ctx, "storeHistory", func() (bool, error) {
		err := cfg.HistoryMemory.StoreHistory(ctx,
			cfg.ID,
			sessionID,
			messages,
			cfg.HistoryMemorySize,
		)

		return (err == nil), err
	})

	return err
}

// DefaultSystemPromptFn constructs a standardized system prompt by aggregating
// agent metadata, core instructions, and dynamic runtime prompts.
func DefaultSystemPromptFn(ctx context.Context, cfg *AgentConfig, input *Input) (string, error) {
	var b strings.Builder
	for _, Instruction := range cfg.Instructions {
		fmt.Fprintf(&b, "- %s\n", Instruction)
	}

	return fmt.Sprintf(DefaultSystemMessageFormat,
		cfg.Name,
		cfg.Description,
		b.String(),
		input.AdditionalSystemPrompt,
	), nil
}

// DefaultSortMessagesFn filters and validates the message sequence to ensure
// it follows a valid role-based order (User -> Model -> Tool).
func DefaultSortMessagesFn(msgs []*ai.Message) ([]*ai.Message, error) {
	if msgs == nil {
		return nil, nil
	}

	var (
		result       = make([]*ai.Message, 0, len(msgs))
		previousRole ai.Role

		msg     *ai.Message
		nextMsg *ai.Message
	)

	for i := 0; i < len(msgs); i++ {
		msg = msgs[i]

		if len(result) > 0 {
			previousRole = result[len(result)-1].Role
		}

		if i+1 < len(msgs) {
			nextMsg = msgs[i+1]
		} else {
			nextMsg = nil
		}

		switch {
		case (previousRole == "") && (msg.Role == ai.RoleUser):
			result = append(result, msg)

		case (msg.Role == ai.RoleUser) && (previousRole == ai.RoleUser || previousRole == ai.RoleModel):
			result = append(result, msg)

		case (msg.Role == ai.RoleModel) && (previousRole == ai.RoleUser || previousRole == ai.RoleTool):
			if hasToolRequests(msg) {
				if (nextMsg != nil) && (nextMsg.Role == ai.RoleTool) {
					result = append(result, msg)

					result = append(result, nextMsg)
					i++
				}

				continue // discard messages
			}

			result = append(result, msg)

		default:
			continue // discard message
		}
	}

	if len(result) == 0 {
		return nil, ErrNoValidMessages
	}
	return result, nil
}

func hasToolRequests(msg *ai.Message) bool {
	for _, part := range msg.Content {
		if part.IsToolRequest() {
			return true
		}
	}
	return false
}
