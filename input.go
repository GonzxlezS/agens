package agens

import (
	"fmt"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
)

// SessionIDFormat defines the template used to generate a unique session key.
const SessionIDFormat = "%s_%s"

type Input struct {
	Trigger  string        `json:"trigger"`
	Source   string        `json:"source"`
	Messages []*ai.Message `json:"messages"`

	AdditionalSystemPrompt string         `json:"additional_system_prompt,omitempty"`
	OutputSchema           map[string]any `json:"output_schema,omitempty"`
	OutputInstructions     *string        `json:"output_instructions,omitempty"`
}

// SessionID generates a composite key based on Trigger and Source.
// Returns an empty string if both fields are missing.
func (in *Input) SessionID() string {
	in.Trigger = strings.TrimSpace(in.Trigger)
	in.Source = strings.TrimSpace(in.Source)

	if (in.Trigger == "") && (in.Source == "") {
		return ""
	}
	return fmt.Sprintf(SessionIDFormat, in.Trigger, in.Source)
}

func (in *Input) WithOutputInstructions(instructions string) *Input {
	in.OutputInstructions = &instructions
	return in
}

func (in *Input) WithOutputType(output any) *Input {
	in.OutputSchema = core.InferSchemaMap(output)
	return in
}

func (in *Input) outputOptions() []ai.GenerateOption {
	var outputOpts []ai.GenerateOption

	if in.OutputSchema != nil {
		outputOpts = append(outputOpts, ai.WithOutputSchema(in.OutputSchema))
	}

	if in.OutputInstructions != nil {
		outputOpts = append(outputOpts, ai.WithOutputInstructions(*in.OutputInstructions))
	}

	return outputOpts
}
