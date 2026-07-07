package agens

import (
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
)

type Input struct {
	// GatewayID specifies the communication platform or channel where the input originated.
	GatewayID string `json:"gateway_id,omitempty"`

	// SessionID identifies the unique logic conversation space or session within the gateway.
	// Used by the agent to isolate session states and track conversation history.
	SessionID string `json:"session_id,omitempty"`

	// SenderID identifies the specific entity (user, system, or another agent) that sent the input.
	// Useful for permission checks and personalization.
	SenderID string `json:"sender_id,omitempty"`

	// Messages contains the conversation history for the model.
	Messages []*ai.Message `json:"messages,omitempty"`

	// Documents provides retrieved documents to be used as context for this generation.
	Documents []*ai.Document `json:"documents,omitempty"`

	// ToolChoice controls how the model uses tools (auto, required, or none).
	ToolChoice *ai.ToolChoice `json:"tool_choice,omitempty"`

	// ReturnToolRequests, when true, returns tool calls for manual processing instead of
	// automatically resolving them.
	ReturnToolRequests *bool `json:"return_tool_requests,omitempty"`

	// ToolResponses contains tool response parts to send to the model when resuming.
	ToolResponses []*ai.Part `json:"tool_responses,omitempty"`

	// ToolRestarts contains tool request parts to restart when resuming.
	ToolRestarts []*ai.Part `json:"tool_restarts,omitempty"`

	// OutputSchema JSON schema of the output.
	OutputSchema map[string]any `json:"output_schema,omitempty"`

	// OutputFormat Format of the output. If OutputSchema is set, this is set to OutputFormatJSON.
	OutputFormat *string `json:"output_format,omitempty"`

	// OutputInstructions Instructions to add to conform the output to a schema.
	// If nil, default instructions will be added. If empty string, no instructions will be added.
	OutputInstructions *string `json:"output_instructions,omitempty"`

	// Metadata contains arbitrary key-value data associated with this input.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// WithMessages sets the messages.
func (in *Input) WithMessages(messages ...*ai.Message) *Input {
	in.Messages = messages
	return in
}

// WithDocs sets the documents to be used as context for generation or as input to an embedder.
func (in *Input) WithDocs(docs ...*ai.Document) *Input {
	in.Documents = docs
	return in
}

// WithToolChoice configures whether by default tool calls are required, disabled, or optional for the prompt.
func (in *Input) WithToolChoice(toolChoice ai.ToolChoice) *Input {
	in.ToolChoice = &toolChoice
	return in
}

// WithReturnToolRequests configures whether to return tool requests instead of making the tool calls and continuing the generation.
func (in *Input) WithReturnToolRequests(returnReqs bool) *Input {
	in.ReturnToolRequests = &returnReqs
	return in
}

// WithToolResponses provides resolved responses for interrupted tool calls.
// Use this when you already have the result and want to skip re-executing the tool.
func (in *Input) WithToolResponses(parts ...*ai.Part) *Input {
	in.ToolResponses = parts
	return in
}

// WithToolRestarts re-executes interrupted tool calls with additional metadata.
// Use this when the original call lacked required context (e.g., auth, user confirmation)
// that should now allow the tool to complete successfully.
func (in *Input) WithToolRestarts(parts ...*ai.Part) *Input {
	in.ToolRestarts = parts
	return in
}

// WithOutputType sets the output format to JSON and the schema derived from the given value.
func (in *Input) WithOutputType(output any) *Input {
	in.OutputSchema = core.InferSchemaMap(output)
	return in.WithOutputFormat(ai.OutputFormatJSON)
}

// WithOutputFormat sets the format of the output.
func (in *Input) WithOutputFormat(format string) *Input {
	in.OutputFormat = &format
	return in
}

// WithOutputEnums sets the output format to enum and the schema based on the given values.
func (in *Input) WithOutputEnums(values ...string) *Input {
	in.OutputSchema = map[string]any{
		"type": "string",
		"enum": values,
	}
	return in.WithOutputFormat(ai.OutputFormatEnum)
}

// WithOutputInstructions sets custom instructions for constraining output format in the prompt.
func (in *Input) WithOutputInstructions(instructions string) *Input {
	in.OutputInstructions = &instructions
	return in
}

func (in *Input) genOptions(history []*ai.Message, docs []*ai.Document) []ai.GenerateOption {
	var opts []ai.GenerateOption

	// Messages
	if (len(history) > 0) || (len(in.Messages) > 0) {
		opts = append(opts, ai.WithMessages(
			append(history, in.Messages...)...,
		))
	}

	// Documents
	if (len(docs) > 0) || (len(in.Documents) > 0) {
		opts = append(opts, ai.WithDocs(
			append(docs, in.Documents...)...,
		))
	}

	// Tools
	if in.ToolChoice != nil {
		opts = append(opts, ai.WithToolChoice(*in.ToolChoice))
	}

	if in.ReturnToolRequests != nil {
		opts = append(opts, ai.WithReturnToolRequests(*in.ReturnToolRequests))
	}

	if in.ToolResponses != nil {
		opts = append(opts, ai.WithToolResponses(in.ToolResponses...))
	}

	if in.ToolRestarts != nil {
		opts = append(opts, ai.WithToolRestarts(in.ToolRestarts...))
	}

	// Output
	if in.OutputSchema != nil {
		opts = append(opts, ai.WithOutputSchema(in.OutputSchema))
	}

	if in.OutputFormat != nil {
		opts = append(opts, ai.WithOutputFormat(*in.OutputFormat))
	}

	if in.OutputInstructions != nil {
		opts = append(opts, ai.WithOutputInstructions(*in.OutputInstructions))
	}

	return opts
}
