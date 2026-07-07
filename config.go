package agens

import (
	"context"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/core/api"
)

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
	Model ai.ModelArg

	// Tools is a list of tools that the agent can use.
	Tools []ai.ToolRef

	// MaxTurns Set maximum tool call iterations
	// Default: 5 (Genkit)
	MaxTurns int

	// GenerateOptions are extra options passed to the genkit.Generate function.
	GenerateOptions []ai.GenerateOption

	// HistoryMemory is responsible for persisting the conversation history.
	HistoryMemory HistoryMemory

	// HistoryManager handles the lifecycle strategy of the conversation history.
	// If not explicitly provided, it defaults to a SlidingWindowManager initialized
	// with a standard default window size.
	HistoryManager HistoryManager

	// KnowledgeMode defines how knowledge is used in the agent.
	// If not explicitly set, the mode is determined dynamically according to the following rules:
	// - KnowledgeModeNone: the default value when neither KnowledgeMemory nor KnowledgeQueryer has been configured.
	// - KnowledgeModeStep: the default value when KnowledgeQueryer has been configured.
	// - KnowledgeModeTool: the default value when KnowledgeMemory has been configured.
	//
	// KnowledgeModeTool requires a valid KnowledgeMemory; KnowledgeModeStep and KnowledgeModeBoth require both a valid
	// KnowledgeMemory and a valid KnowledgeQueryer; otherwise, the agent configuration will fail during the validation phase.
	KnowledgeMode KnowledgeMode

	// KnowledgeMemory enables RAG (Retrieval-Augmented Generation) capabilities.
	// Required if KnowledgeMode is Step, Tool, or Both.
	KnowledgeMemory KnowledgeMemory

	// KnowledgeQueryer generates search queries for RAG systems based on the input.
	// Required if KnowledgeMode is Step or Both.
	KnowledgeQueryer KnowledgeQueryer

	// KnowledgeRetrieveLimit sets the maximum number of documents fetched per query.
	// Used in both Step and Tool modes if applicable.
	KnowledgeRetrieveLimit int

	// SystemPromptFn allows for a custom implementation of the system instruction builder.
	SystemPromptFn func(ctx context.Context, cfg *AgentConfig, in *Input) (string, error)
}

// Prepare sets certain configuration values before they are validated.
// It is used when initializing a new Agent.
func (cfg *AgentConfig) Prepare() error {
	// Basic data
	cfg.ID = strings.TrimSpace(cfg.ID)
	cfg.Name = strings.TrimSpace(cfg.Name)
	cfg.Description = strings.TrimSpace(cfg.Description)

	// Instructions
	cleanedInstructions := cfg.Instructions[:0]
	for _, value := range cfg.Instructions {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			cleanedInstructions = append(cleanedInstructions, trimmed)
		}
	}

	cfg.Instructions = cleanedInstructions

	// HistoryManager
	if (cfg.HistoryMemory != nil) && (cfg.HistoryManager == nil) {
		cfg.HistoryManager = SlidingWindowManager{
			WindowSize: DefaultWindowSize,
		}
	}

	// KnowledgeMode
	if cfg.KnowledgeMode == "" {
		cfg.KnowledgeMode = KnowledgeModeNone

		if cfg.KnowledgeMemory != nil {
			cfg.KnowledgeMode = KnowledgeModeTool
		}

		if cfg.KnowledgeQueryer != nil {
			cfg.KnowledgeMode = KnowledgeModeStep
		}
	}

	// KnowledgeRetrieveLimit
	if cfg.KnowledgeRetrieveLimit == 0 {
		cfg.KnowledgeRetrieveLimit = DefaultKnowledgeRetrieveLimit
	}

	return cfg.Validate()
}

// SystemPrompt determines and executes the strategy for building
// the agent's system instructions.
func (cfg *AgentConfig) SystemPrompt(ctx context.Context, input *Input) (string, error) {
	if cfg.SystemPromptFn != nil {
		return cfg.SystemPromptFn(ctx, cfg, input)
	}
	return DefaultSystemPromptFn(ctx, cfg, input)
}

// Validate checks that the configuration is correct without modifying its values.
func (cfg *AgentConfig) Validate() error {
	if cfg.ID == "" {
		return ErrAgentIDEmpty
	}

	if cfg.Name == "" {
		return ErrAgentNameEmpty
	}

	if cfg.Description == "" {
		return ErrAgentDescriptionEmpty
	}

	if len(cfg.Instructions) == 0 {
		return ErrAgentInstructionsEmpty
	}

	if (cfg.HistoryMemory != nil) && (cfg.HistoryManager == nil) {
		return ErrHistoryManagerRequired
	}

	switch cfg.KnowledgeMode {
	case KnowledgeModeNone:
		// pass

	case KnowledgeModeTool:
		if cfg.KnowledgeMemory == nil {
			return ErrKnowledgeMemoryRequired
		}

	case KnowledgeModeStep, KnowledgeModeBoth:
		if cfg.KnowledgeMemory == nil {
			return ErrKnowledgeMemoryRequired
		}
		if cfg.KnowledgeQueryer == nil {
			return ErrKnowledgeQueryerRequired
		}

	default:
		return ErrInvalidKnowledgeMode
	}
	return nil
}

func (cfg *AgentConfig) genFlow(ctx context.Context, r api.Registry, input *Input) (*ai.ModelResponse, error) {
	// Validate
	if err := cfg.Validate(); err != nil {
		return &ai.ModelResponse{}, err
	}

	// System prompt
	systemPrompt, err := cfg.SystemPrompt(ctx, input)
	if err != nil {
		return &ai.ModelResponse{}, err
	}

	// Retrieve history
	stateless, _ := StatelessFromContext(ctx)

	history, err := cfg.retrieveHistory(ctx, input.GatewayID, input.SessionID, stateless)
	if err != nil {
		return &ai.ModelResponse{}, err
	}

	// Retrieve knowledge
	docs, err := cfg.retrieveKnowledge(ctx, input)
	if err != nil {
		return &ai.ModelResponse{}, err
	}

	// Build generation options for the LLM
	genOpts := cfg.genOptions(input, systemPrompt, history, docs)

	// Execute LLM generation
	ctx = cfg.setGenContext(ctx, input)

	resp, err := ai.Generate(ctx, r, genOpts...)
	if err != nil {
		return &ai.ModelResponse{}, err
	}

	// Store history
	err = cfg.storeHistory(ctx, input.GatewayID, input.SessionID, stateless, resp.History())

	return resp, err
}

func (cfg *AgentConfig) retrieveHistory(ctx context.Context, gatewayID string, sessionID string, stateless bool) ([]*ai.Message, error) {
	if (cfg.HistoryMemory == nil) || stateless {
		return []*ai.Message{}, nil
	}

	// Empty data implies an anonymous or stateless call.
	// We bypass the storage layer and return an empty history immediately.
	if (gatewayID == "") || (sessionID == "") {
		return []*ai.Message{}, nil
	}

	return core.Run(ctx, "history", func() ([]*ai.Message, error) {
		history, err := cfg.HistoryMemory.RetrieveHistory(ctx, cfg.ID, gatewayID, sessionID)
		if (err != nil) || (history == nil) {
			return []*ai.Message{}, err
		}
		return history, nil
	})
}

func (cfg *AgentConfig) retrieveKnowledge(ctx context.Context, input *Input) ([]*ai.Document, error) {
	// Only retrieve knowledge if mode is Step or Both
	if (cfg.KnowledgeMode != KnowledgeModeStep) && (cfg.KnowledgeMode != KnowledgeModeBoth) {
		return []*ai.Document{}, nil
	}

	// Skip knowledge retrieval on tool call continuations/restarts.
	if (input.ToolResponses != nil) || (input.ToolRestarts != nil) {
		return []*ai.Document{}, nil
	}

	return core.Run(ctx, "knowledge", func() ([]*ai.Document, error) {
		query, err := cfg.KnowledgeQueryer.GenerateQuery(ctx, input)
		if err != nil {
			return []*ai.Document{}, err
		}

		docs, err := cfg.KnowledgeMemory.RetrieveKnowledge(ctx, cfg.ID, query, cfg.KnowledgeRetrieveLimit)
		if (err != nil) || (docs == nil) {
			return []*ai.Document{}, err
		}

		return docs, nil
	})
}

func (cfg *AgentConfig) genOptions(input *Input, systemPrompt string, history []*ai.Message, docs []*ai.Document) []ai.GenerateOption {
	// Base options + System
	opts := append(cfg.GenerateOptions, ai.WithSystem(systemPrompt))

	// Model
	if cfg.Model != nil {
		opts = append(opts, ai.WithModel(cfg.Model))
	}

	// Tools: include KnowledgeMemory as a tool if mode is Tool or Both
	tools := append([]ai.ToolRef(nil), cfg.Tools...)

	if (cfg.KnowledgeMode == KnowledgeModeTool) || (cfg.KnowledgeMode == KnowledgeModeBoth) {
		tools = append(tools,
			cfg.KnowledgeMemory.AsTool(cfg.ID, cfg.KnowledgeRetrieveLimit),
		)
	}

	if len(tools) > 0 {
		opts = append(opts, ai.WithTools(tools...))
	}

	// MaxTurns
	if cfg.MaxTurns != 0 {
		opts = append(opts, ai.WithMaxTurns(cfg.MaxTurns))
	}

	// Input
	opts = append(opts, input.genOptions(history, docs)...)
	return opts
}

func (cfg *AgentConfig) setGenContext(ctx context.Context, input *Input) context.Context {
	ctx = ContextWithGatewayID(ctx, input.GatewayID)
	ctx = ContextWithSessionID(ctx, input.SessionID)
	ctx = ContextWithSenderID(ctx, input.SenderID)

	return ContextWithAgentID(ctx, cfg.ID)
}

func (cfg *AgentConfig) storeHistory(ctx context.Context, gatewayID string, sessionID string, stateless bool, messages []*ai.Message) error {
	if (cfg.HistoryMemory == nil) || stateless {
		return nil
	}

	if (gatewayID == "") || (sessionID == "") {
		return nil
	}

	_, err := core.Run(ctx, "storeHistory", func() (bool, error) {
		messages, err := cfg.HistoryManager.ProcessHistory(ctx, messages)
		if err != nil {
			return false, err
		}

		err = cfg.HistoryMemory.StoreHistory(ctx, cfg.ID, gatewayID, sessionID, messages)
		return (err == nil), err
	})

	return err
}
