package agens

import (
	"fmt"
	"strings"
)

// DefaultSystemMessageFormat is the default template used to format the
// system message passed to the AI model.
const DefaultSystemMessageFormat = `You are %s, %s. 
instructions:
%s`

// SystemMessage generates the system message for the agent.
// This function checks if a custom formatter is provided in the Agent struct.
// If not, it uses the DefaultFormatSystemMessage function to generate the message.
// The system message is crucial for providing high-level instructions to the AI model.
func (agent *Agent) SystemMessage() string {
	if agent.FormatSystemMessage != nil {
		return agent.FormatSystemMessage(agent)
	}
	return DefaultFormatSystemMessage(agent)
}

// DefaultFormatSystemMessage creates a formatted system message using the
// DefaultSystemMessageFormat constant.
func DefaultFormatSystemMessage(agent *Agent) string {
	var b strings.Builder
	for _, Instruction := range agent.Instructions {
		fmt.Fprintf(&b, "- %s\n", Instruction)
	}

	return fmt.Sprintf(DefaultSystemMessageFormat,
		agent.Name,
		agent.Description,
		b.String(),
	)
}
