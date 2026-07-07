package agens

import "github.com/firebase/genkit/go/ai"

// PipelineState encapsulates the state and context of a pipeline during its execution.
// It safely holds crucial data such as the gateway reference, original input, batch information,
// messages, and the final model response.
type PipelineState[T any] struct {
	gateway       GatewayRef
	originalInput T
	batchID       string
	message       *ai.Message
	batchMessages []*ai.Message
	input         *Input
	response      *ai.ModelResponse
}

// Gateway returns the gateway reference associated with the state.
func (s *PipelineState[T]) Gateway() GatewayRef { return s.gateway }

// OriginalInput returns the initial input provided to the pipeline.
func (s *PipelineState[T]) OriginalInput() T { return s.originalInput }

// BatchMessageID returns the identifier used for batching messages.
func (s *PipelineState[T]) BatchMessageID() string { return s.batchID }

// Message returns the AI message generated from the original input.
func (s *PipelineState[T]) Message() *ai.Message { return s.message }

// BatchMessages returns the collection of grouped messages.
func (s *PipelineState[T]) BatchMessages() []*ai.Message { return s.batchMessages }

// AgentInput returns the transformed input ready for the agent.
func (s *PipelineState[T]) AgentInput() *Input { return s.input }

// AgentResponse returns the final model response from the agent.
func (s *PipelineState[T]) AgentResponse() *ai.ModelResponse { return s.response }
