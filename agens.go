// Package agens provides a generic abstraction for creating and managing AI agents.
// It is designed to be integrated with the Genkit framework.

package agens

import (
	"context"
	"errors"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/genkit"
)

var (
	// ErrAgentNotInitialized is returned if an attempt is made to run an agent
	// that has not been properly initialized.
	ErrAgentNotInitialized = errors.New("agent not initialized")

	// ErrEmptyAgentDescription indicates that the agent's purpose or metadata
	// is missing, which is required for registration.
	ErrEmptyAgentDescription = errors.New("agent description cannot be empty")

	// ErrEmptyAgentID indicates that the unique identifier for the agent
	// is missing or contains only whitespace.
	ErrEmptyAgentID = errors.New("agent id cannot be empty")

	// ErrEmptyAgentInstructions indicates that the system prompts or core
	// logic guidelines for the agent have not been provided.
	ErrEmptyAgentInstructions = errors.New("agent instructions cannot be empty")

	// ErrEmptyAgentName indicates that the human-readable name for the
	// agent is missing.
	ErrEmptyAgentName = errors.New("agent name cannot be empty")

	// ErrKnowledgeMemoryNotConfigured is returned when an operation is attempted
	// on an agent that does not have a KnowledgeMemory initialized.
	ErrKnowledgeMemoryNotConfigured = errors.New("knowledge memory is not configured for this agent")
)

// Agent represents a high-level Genkit execution unit that orchestrates
// configurations, flows, and knowledge memory.
type Agent struct {
	cfg  *AgentConfig
	flow *core.Flow[*Input, *ai.ModelResponse, struct{}]
}

// NewAgent initializes a new Agent instance.
func NewAgent(g *genkit.Genkit, cfg AgentConfig) (*Agent, error) {
	// id
	cfg.ID = strings.TrimSpace(cfg.ID)
	if cfg.ID == "" {
		return nil, ErrEmptyAgentID
	}

	// name
	cfg.Name = strings.TrimSpace(cfg.Name)
	if cfg.Name == "" {
		return nil, ErrEmptyAgentName
	}

	// description
	cfg.Description = strings.TrimSpace(cfg.Description)
	if cfg.Description == "" {
		return nil, ErrEmptyAgentDescription
	}

	// instructions
	if len(cfg.Instructions) == 0 {
		return nil, ErrEmptyAgentInstructions
	}

	// agent
	return &Agent{
		cfg:  &cfg,
		flow: genkit.DefineFlow(g, cfg.ID, cfg.flowFn(g)),
	}, nil
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
	}
	return agent.flow.Run(ctx, input)
}

// IndexKnowledge persists documents into the agent's knowledge memory using a label.
// Fails if KnowledgeMemory is not configured in the AgentConfig.
func (agent *Agent) IndexKnowledge(ctx context.Context, label string, docs []*ai.Document) error {
	if agent.cfg == nil {
		return ErrAgentNotInitialized
	} else if agent.cfg.KnowledgeMemory == nil {
		return ErrKnowledgeMemoryNotConfigured
	}
	return agent.cfg.KnowledgeMemory.IndexKnowledge(ctx, agent.cfg.ID, label, docs)
}

// DeleteKnowledge removes specific labeled documents from the agent's knowledge memory.
// Fails if KnowledgeMemory is not configured in the AgentConfig.
func (agent *Agent) DeleteKnowledge(ctx context.Context, label string) error {
	if agent.cfg == nil {
		return ErrAgentNotInitialized
	} else if agent.cfg.KnowledgeMemory == nil {
		return ErrKnowledgeMemoryNotConfigured
	}
	return agent.cfg.KnowledgeMemory.DeleteKnowledge(ctx, agent.cfg.ID, label)
}
