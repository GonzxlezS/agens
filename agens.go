// Package agens provides a generic abstraction for creating and managing AI agents.
// It is designed to be integrated with the Genkit framework.

package agens

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/core/api"
)

// DefaultKnowledgeRetrieveLimit is the maximum number of relevant documents
// fetched from the vector database during a single RAG retrieval step.
// This ensures sufficient external context without overloading the LLM prompt.
const DefaultKnowledgeRetrieveLimit = 3

// DefaultSystemMessageFormat is the base template used to construct the agent's
// system prompt. It expects three string variables in the following sequence:
// 1. Agent Name (%s)
// 2. Agent Description (%s)
// 3. Formatted list of Agent Instructions (%s)
const DefaultSystemMessageFormat = `You are %s, %s.
instructions:
%s`

// DefaultWindowSize is the maximum number of recent messages preserved
// in the conversation history when using the default SlidingWindowManager.
// This helps balance conversation context retention with LLM token usage.
const DefaultWindowSize = 20

var (
	// ErrAgentNotInitialized is returned if an attempt is made to run an agent
	// that has not been properly initialized.
	ErrAgentNotInitialized = errors.New("agent not initialized")

	// ErrAgentDescriptionEmpty indicates that the agent's purpose or metadata
	// is missing, which is required for registration.
	ErrAgentDescriptionEmpty = errors.New("agent description cannot be empty")

	// ErrAgentIDEmpty indicates that the unique identifier for the agent
	// is missing or contains only whitespace.
	ErrAgentIDEmpty = errors.New("agent id cannot be empty")

	// ErrAgentInstructionsEmpty indicates that the system prompts or core
	// logic guidelines for the agent have not been provided.
	ErrAgentInstructionsEmpty = errors.New("agent instructions cannot be empty")

	// ErrAgentNameEmpty indicates that the human-readable name for the
	// agent is missing.
	ErrAgentNameEmpty = errors.New("agent name cannot be empty")

	// ErrAgentRegistryNil is returned if you attempt to run an agent
	// that has not been registered correctly.
	ErrAgentRegistryNil = errors.New("registry is nil, cannot generate")

	// ErrHistoryManagerRequired is returned when a HistoryMemory is set without a HistoryManager.
	ErrHistoryManagerRequired = errors.New("history manager is required")

	// ErrKnowledgeMemoryRequired indicates that KnowledgeMemory is mandatory
	// for the selected knowledge mode.
	ErrKnowledgeMemoryRequired = errors.New("knowledge memory is required for the selected knowledge mode")

	// ErrKnowledgeQueryerRequired indicates that KnowledgeQueryer is mandatory
	// for the selected knowledge mode.
	ErrKnowledgeQueryerRequired = errors.New("knowledge queryer is required for the selected knowledge mode")

	// ErrInvalidKnowledgeMode indicates that the knowledge mode is invalid.
	ErrInvalidKnowledgeMode = errors.New("invalid knowledge mode")
)

var _ api.Registerable = &Agent{}

// Agent represents a high-level Genkit execution unit that orchestrates
// configurations, flows, and knowledge memory.
type Agent struct {
	cfg  *AgentConfig
	reg  api.Registry
	flow *core.Flow[*Input, *ai.ModelResponse, struct{}]
}

// NewAgent initializes a new Agent instance.
func NewAgent(cfg AgentConfig) (*Agent, error) {
	if err := cfg.Prepare(); err != nil {
		return nil, err
	}

	agent := &Agent{cfg: &cfg}

	agent.flow = core.NewFlow(cfg.ID, func(ctx context.Context, input *Input) (*ai.ModelResponse, error) {
		return agent.cfg.genFlow(ctx, agent.reg, input)
	})
	return agent, nil
}

// ID returns the agent's unique identifier. Returns an empty string if the agent is not configured.
func (agent *Agent) ID() string {
	if agent.cfg != nil {
		return agent.cfg.ID
	}
	return ""
}

// Generate executes the agent's internal Genkit flow with the given input.
// Returns ErrAgentNotInitialized if the flow is not properly defined.
func (agent *Agent) Generate(ctx context.Context, input *Input) (*ai.ModelResponse, error) {
	if agent.flow == nil {
		return &ai.ModelResponse{}, ErrAgentNotInitialized
	} else if agent.reg == nil {
		return &ai.ModelResponse{}, ErrAgentRegistryNil
	}
	return agent.flow.Run(ctx, input)
}

// Register registers the agent (as a flow) in the specified registry.
func (agent *Agent) Register(r api.Registry) {
	agent.reg = r
	agent.flow.Register(r)
}

// DefaultSystemPromptFn constructs a standardized system prompt by aggregating
// agent metadata and core instructions.
func DefaultSystemPromptFn(ctx context.Context, cfg *AgentConfig, _ *Input) (string, error) {
	var b strings.Builder
	for _, instruction := range cfg.Instructions {
		fmt.Fprintf(&b, "- %s\n", instruction)
	}

	return fmt.Sprintf(DefaultSystemMessageFormat,
		cfg.Name,
		cfg.Description,
		b.String(),
	), nil
}
