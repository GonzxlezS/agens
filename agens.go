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
	// FinishReasonDelegated is a custom finish reason indicating that the message
	// was delegated to another flow or component for handling.
	FinishReasonDelegated ai.FinishReason = "delegated"

	// DelegationMessage is a text message that accompanies the FinishReasonDelegated finish reason.
	DelegationMessage = "The message has been delegated to another flow for handling."
)

// ErrAgentNotInitialized is returned if an attempt is made to run an agent
// that has not been properly initialized.
var ErrAgentNotInitialized = errors.New("agent not initialized")

// Agent represents a generic AI agent.
// It contains the necessary configuration to define its behavior, including
// the AI model to use, the available tools, and the logic for managing
// conversation history memory.
type Agent struct {
	// Name is the name of the agent, used to identify its flow in Genkit.
	Name string

	// Description is a brief description of the agent's purpose.
	Description string

	// Instructions are high-level instructions that guide the AI model's behavior.
	Instructions []string

	// Model is the AI model that the agent will use to generate responses.
	Model ai.ModelArg

	// Tools is a list of tools that the agent can use.
	Tools []ai.ToolRef

	// AdditionalOptions are extra options passed to the genkit.Generate function.
	AdditionalOptions []ai.GenerateOption

	// Batcher is responsible for batching incoming messages.
	Batcher MessageBatcher

	// HistoryMemory is responsible for persisting the conversation history.
	HistoryMemory HistoryMemory

	// FormatSystemMessage is an optional function for formatting the system message.
	FormatSystemMessage func(*Agent) string

	g    *genkit.Genkit
	flow *core.Flow[*ai.Message, *ai.ModelResponse, struct{}]
}

// Init initializes the agent and registers its flow with the Genkit object.
// It must be called before the agent can be used.
func (agent *Agent) Init(ctx context.Context, g *genkit.Genkit) error {
	agent.g = g
	agent.flow = genkit.DefineFlow(g, agent.Name, agent.coreFunc)
	return nil
}

// Run executes the agent's flow with a given message.
// It returns a model response or an error.
func (agent *Agent) Run(ctx context.Context, msg *ai.Message) (*ai.ModelResponse, error) {
	if (agent.g == nil) || (agent.flow == nil) {
		return EmptyModelResponse(), ErrAgentNotInitialized
	}
	return agent.flow.Run(ctx, msg)
}

func (agent *Agent) coreFunc(ctx context.Context, msg *ai.Message) (*ai.ModelResponse, error) {
	if agent.g == nil {
		return EmptyModelResponse(), ErrAgentNotInitialized
	}

	// message batch
	batch, err := agent.messageBatch(ctx, msg)
	if err != nil {
		return EmptyModelResponse(), err
	}
	if len(batch) == 0 {
		return DelegatedModelResponse(), nil
	}

	// history
	conversationID, err := GetConversationID(msg)
	if err != nil {
		return EmptyModelResponse(), err
	}

	history, err := agent.retrieveHistory(ctx, conversationID)
	if err != nil {
		return EmptyModelResponse(), err
	}

	// messages
	messages := append(history, batch...)

	// options
	opts := append(agent.AdditionalOptions,
		ai.WithSystem(agent.SystemMessage()),
		ai.WithMessages(messages...),
	)

	if agent.Model != nil {
		opts = append(opts, ai.WithModel(agent.Model))
	}

	if len(agent.Tools) > 0 {
		opts = append(opts, ai.WithTools(agent.Tools...))
	}

	// output option
	outputOpt, err := agent.outputOption(ctx)
	if err != nil {
		return EmptyModelResponse(), err
	}

	if outputOpt != nil {
		opts = append(opts, outputOpt)
	}

	// generate
	resp, err := genkit.Generate(ctx, agent.g, opts...)
	if err != nil {
		return EmptyModelResponse(), err
	}

	// store history
	err = agent.storeHistory(ctx, conversationID, resp.History())

	return resp, err
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
