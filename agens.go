// Package agens provides a generic abstraction for creating and managing AI agents.
// It is designed to be integrated with the Genkit framework.

package agens

import (
	"context"
	"errors"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/genkit"
)

const (
	// DelegationMessage is a text message that accompanies the FinishReasonDelegated finish reason.
	DelegationMessage = "The message has been delegated to another flow for handling."

	// FinishReasonDelegated is a custom finish reason indicating that the message
	// was delegated to another flow or component for handling.
	FinishReasonDelegated ai.FinishReason = "delegated"
)

var (
	// ErrAgentNotInitialized is returned if an attempt is made to run an agent
	// that has not been properly initialized.
	ErrAgentNotInitialized = errors.New("agent not initialized")

	// ErrKnowledgeMemoryNotConfigured is returned when an operation is attempted
	// on an agent that does not have a KnowledgeMemory initialized.
	ErrKnowledgeMemoryNotConfigured = errors.New("knowledge memory is not configured for this agent")
)

// Agent represents a generic AI agent that encapsulates execution logic (flow).
type Agent struct {
	config          *AgentConfig
	knowledgeMemory KnowledgeMemory
	historyMemory   HistoryMemory

	flow *core.Flow[*ai.Message, *ai.ModelResponse, struct{}]
}

// NewAgent initializes a new Agent instance. It defines a Genkit flow based on the provided AgentConfig.
func NewAgent(g *genkit.Genkit, cfg AgentConfig) (*Agent, error) {
	var (
		agent = &Agent{config: &cfg}
		err   error
	)

	// history memory
	if cfg.HistoryProvider != nil {
		agent.historyMemory, err = cfg.HistoryProvider.ForAgent(cfg.Name, cfg.MaxMessagesPerConversation)
		if err != nil {
			return nil, err
		}
	}

	// knowledge
	if cfg.KnowledgeProvider != nil {
		agent.knowledgeMemory, err = cfg.KnowledgeProvider.ForAgent(cfg.Name, cfg.KnowledgeRetrieveLimit)
		if err != nil {
			return nil, err
		}

		agent.config.Tools = append(
			agent.config.Tools,
			agent.knowledgeMemory.AsTool(),
		)
	}

	// flow
	agent.flow = genkit.DefineFlow(g, cfg.Name, baseFlow(g, &cfg, agent.historyMemory))

	return agent, nil
}

// DeleteKnowledge removes documents associated with a specific label from the agent's memory.
// It returns ErrKnowledgeMemoryNotConfigured if the agent was not initialized with knowledge capabilities.
func (agent *Agent) DeleteKnowledge(ctx context.Context, label string) error {
	if agent.knowledgeMemory == nil {
		return ErrKnowledgeMemoryNotConfigured
	}
	return agent.knowledgeMemory.DeleteKnowledge(ctx, label)
}

// IndexKnowledge adds and indexes a set of documents into the agent's memory under a given label.
// This allows the agent to retrieve this information later during conversations.
// It returns ErrKnowledgeMemoryNotConfigured if the agent was not initialized with knowledge capabilities.
func (agent *Agent) IndexKnowledge(ctx context.Context, label string, docs []*ai.Document) error {
	if agent.knowledgeMemory == nil {
		return ErrKnowledgeMemoryNotConfigured
	}
	return agent.knowledgeMemory.IndexKnowledge(ctx, label, docs)
}

// Name returns the identifier of the agent defined in its configuration.
// If the agent or its configuration is nil, it returns an empty string.
func (agent *Agent) Name() string {
	if agent == nil || agent.config == nil {
		return ""
	}
	return agent.config.Name
}

// Run executes the agent's internal flow with a given message within the provided context.
// It returns a *ai.ModelResponse containing the AI's output or an error if execution fails.
func (agent *Agent) Run(ctx context.Context, msg *ai.Message) (*ai.ModelResponse, error) {
	if agent.flow == nil {
		return EmptyModelResponse(), ErrAgentNotInitialized
	}
	return agent.flow.Run(ctx, msg)
}

// DelegatedModelResponse creates a model response that indicates the message
// was delegated. This is useful when an agent decides not to handle a message.
func DelegatedModelResponse() *ai.ModelResponse {
	return &ai.ModelResponse{
		FinishReason:  FinishReasonDelegated,
		FinishMessage: DelegationMessage,
	}
}

// EmptyModelResponse creates an empty model response.
func EmptyModelResponse() *ai.ModelResponse {
	return &ai.ModelResponse{}
}
