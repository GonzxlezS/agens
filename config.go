package agens

import (
	"fmt"
	"strings"

	"github.com/firebase/genkit/go/ai"
)

// DefaultSystemMessageFormat is the default template used to format the
// system message passed to the AI model.
const DefaultSystemMessageFormat = `You are %s, %s. 
instructions:
%s`

// AgentConfig contains the necessary configuration to define its behavior, including
// the AI model to use, the available tools, and the logic for managing
// conversation history memory.
type AgentConfig struct {
	// Name is the name of the agent, used to identify its flow in Genkit.
	Name string

	// Description is a brief description of the agent's purpose.
	Description string

	// Instructions are high-level instructions that guide the AI model's behavior.
	Instructions []string

	// Model is the AI model object that the agent will use to generate responses.
	// If specified, it takes precedence over ModelName.
	Model ai.ModelArg

	// ModelName is the name of the AI model that the agent will use to generate responses.
	// It is used only if Model is not defined (nil).
	ModelName string

	// Tools is a list of tools that the agent can use.
	Tools []ai.ToolRef

	// AdditionalOptions are extra options passed to the genkit.Generate function.
	AdditionalOptions []ai.GenerateOption

	// Batcher is responsible for batching incoming messages.
	Batcher MessageBatcher

	// HistoryProvider is responsible for persisting the conversation history.
	HistoryProvider HistoryProvider

	// MaxMessagesPerConversation defines the limit of messages to keep in context.
	MaxMessagesPerConversation int

	// KnowledgeProvider is responsible for managing and retrieving domain-specific
	// information to augment the agent's responses.
	KnowledgeProvider KnowledgeProvider

	// KnowledgeRetrieveLimit defines the maximum number of relevant documents
	// to retrieve from the knowledge base per query.
	KnowledgeRetrieveLimit int

	// SystemPromptFunc is an optional function for formatting the system message.
	// The system message is crucial for providing high-level instructions to the AI model.
	SystemPromptFunc func(*AgentConfig) string

	// ConversationIDFunc is an optional function for formatting the conversation id.
	ConversationIDFunc func(msg *ai.Message) (string, error)
}

// GetConversationID retrieves the conversation identifier for a given message.
// It uses the custom ConversationIDFunc if provided; otherwise, it falls back
// to DefaultConversationIDFunc.
func (cfg *AgentConfig) GetConversationID(msg *ai.Message) (string, error) {
	if cfg.ConversationIDFunc != nil {
		return cfg.ConversationIDFunc(msg)
	}
	return DefaultConversationIDFunc(msg)
}

// SystemMessage generates the system message for the agent.
// This function checks if a custom function is provided in the AgentConfig struct.
// If not, it uses the DefaultSystemPromptFunc function to generate the message.
func (cfg *AgentConfig) SystemMessage() string {
	if cfg.SystemPromptFunc != nil {
		return cfg.SystemPromptFunc(cfg)
	}
	return DefaultSystemPromptFunc(cfg)
}

// DefaultConversationIDFunc provides a standard way to generate a conversation ID
// by concatenating the message source and channel ID (format: "source:channel_id").
func DefaultConversationIDFunc(msg *ai.Message) (string, error) {
	source, err := GetSource(msg)
	if err != nil {
		return "", err
	}

	channel, err := GetChannelID(msg)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s", source, channel), nil
}

// DefaultSystemPromptFunc creates a formatted system message using the
// DefaultSystemMessageFormat constant, injecting the agent's name, description,
// and joining the instructions into a bulleted list.
func DefaultSystemPromptFunc(cfg *AgentConfig) string {
	var b strings.Builder
	for _, Instruction := range cfg.Instructions {
		fmt.Fprintf(&b, "- %s\n", Instruction)
	}

	return fmt.Sprintf(DefaultSystemMessageFormat,
		cfg.Name,
		cfg.Description,
		b.String(),
	)
}
