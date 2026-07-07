package agens

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/firebase/genkit/go/ai"
)

func TestPipelineState_Getters(t *testing.T) {
	var (
		gw        = &mockGateway{}
		msg       = &ai.Message{}
		batchMsgs = []*ai.Message{msg}
		input     = &Input{}
		resp      = &ai.ModelResponse{}
	)

	state := &PipelineState[string]{
		gateway:       gw,
		originalInput: "test input",
		batchID:       "batch-123",
		message:       msg,
		batchMessages: batchMsgs,
		input:         input,
		response:      resp,
	}

	if state.Gateway() != gw {
		t.Errorf("Gateway() error")
	}
	if state.OriginalInput() != "test input" {
		t.Errorf("OriginalInput() error")
	}
	if state.BatchMessageID() != "batch-123" {
		t.Errorf("BatchMessageID() error")
	}
	if state.Message() != msg {
		t.Errorf("Message() error")
	}
	if !reflect.DeepEqual(state.BatchMessages(), batchMsgs) {
		t.Errorf("BatchMessages() error")
	}
	if state.AgentInput() != input {
		t.Errorf("AgentInput() error")
	}
	if state.AgentResponse() != resp {
		t.Errorf("AgentResponse() error")
	}
}

func TestNewPipeline(t *testing.T) {
	ctx := context.Background()
	gw := &mockGateway{}

	t.Run("nil_processor", func(t *testing.T) {
		_, err := NewPipeline[string](nil, nil)
		if !errors.Is(err, ErrNilPipelineProcessor) {
			t.Fatalf("expected ErrNilPipelineProcessor, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		processor := &mockPipelineProcessor{
			batchID: "1",
			msg:     &ai.Message{},
			input:   &Input{},
		}

		opts := &PipelineOptions[string]{
			BatchMessages: func(ctx context.Context, state *PipelineState[string]) (bool, []*ai.Message, error) {
				return true, []*ai.Message{{}}, nil
			},
			Generate: func(ctx context.Context, state *PipelineState[string]) (*ai.ModelResponse, error) {
				return &ai.ModelResponse{}, nil
			},
		}

		pipeline, err := NewPipeline[string](processor, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		continueFlow, err := pipeline(ctx, gw, "test")
		if err != nil {
			t.Errorf("unexpected error running pipeline: %v", err)
		}
		if !continueFlow {
			t.Errorf("expected flow to continue, got false")
		}
	})

	t.Run("originalInputToMessage_error", func(t *testing.T) {
		expectedErr := errors.New("input error")
		processor := &mockPipelineProcessor{err1: expectedErr}

		pipeline, _ := NewPipeline[string](processor, nil)
		_, err := pipeline(ctx, gw, "test")
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected %v, got %v", expectedErr, err)
		}
	})

	t.Run("batchMessages_error", func(t *testing.T) {
		processor := &mockPipelineProcessor{}
		expectedErr := errors.New("batch error")
		opts := &PipelineOptions[string]{
			BatchMessages: func(ctx context.Context, state *PipelineState[string]) (bool, []*ai.Message, error) {
				return false, nil, expectedErr
			},
		}

		pipeline, _ := NewPipeline[string](processor, opts)
		_, err := pipeline(ctx, gw, "test")
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected %v, got %v", expectedErr, err)
		}
	})

	t.Run("batchMessages_stop", func(t *testing.T) {
		processor := &mockPipelineProcessor{}
		opts := &PipelineOptions[string]{
			BatchMessages: func(ctx context.Context, state *PipelineState[string]) (bool, []*ai.Message, error) {
				return false, nil, nil // No debe continuar
			},
		}

		pipeline, _ := NewPipeline[string](processor, opts)
		continueFlow, err := pipeline(ctx, gw, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if continueFlow {
			t.Errorf("expected flow to stop (false), got true")
		}
	})

	t.Run("batchToInput_error", func(t *testing.T) {
		expectedErr := errors.New("batch to input error")
		processor := &mockPipelineProcessor{err2: expectedErr}
		opts := &PipelineOptions[string]{
			BatchMessages: func(ctx context.Context, state *PipelineState[string]) (bool, []*ai.Message, error) {
				return true, []*ai.Message{{}}, nil
			},
		}

		pipeline, _ := NewPipeline[string](processor, opts)
		_, err := pipeline(ctx, gw, "test")
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected %v, got %v", expectedErr, err)
		}
	})

	t.Run("generate_error", func(t *testing.T) {
		processor := &mockPipelineProcessor{}
		expectedErr := errors.New("generate error")
		opts := &PipelineOptions[string]{
			BatchMessages: func(ctx context.Context, state *PipelineState[string]) (bool, []*ai.Message, error) {
				return true, []*ai.Message{{}}, nil
			},
			Generate: func(ctx context.Context, state *PipelineState[string]) (*ai.ModelResponse, error) {
				return nil, expectedErr
			},
		}

		pipeline, _ := NewPipeline[string](processor, opts)
		_, err := pipeline(ctx, gw, "test")
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected %v, got %v", expectedErr, err)
		}
	})

	t.Run("handleAgentResponse_error", func(t *testing.T) {
		expectedErr := errors.New("handle response error")
		processor := &mockPipelineProcessor{err3: expectedErr}
		opts := &PipelineOptions[string]{
			BatchMessages: func(ctx context.Context, state *PipelineState[string]) (bool, []*ai.Message, error) {
				return true, []*ai.Message{{}}, nil
			},
			Generate: func(ctx context.Context, state *PipelineState[string]) (*ai.ModelResponse, error) {
				return &ai.ModelResponse{}, nil
			},
		}

		pipeline, _ := NewPipeline[string](processor, opts)
		_, err := pipeline(ctx, gw, "test")
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected %v, got %v", expectedErr, err)
		}
	})
}

func TestDefaultPipelineBatchMessages(t *testing.T) {
	ctx := context.Background()

	t.Run("nil", func(t *testing.T) {
		gw := &mockGateway{batcher: nil}
		msg := &ai.Message{}
		state := &PipelineState[string]{gateway: gw, message: msg}

		shouldContinue, msgs, err := DefaultPipelineBatchMessages(ctx, state)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !shouldContinue {
			t.Errorf("expected to continue")
		}
		if len(msgs) != 1 || msgs[0] != msg {
			t.Errorf("expected messages slice to contain original message")
		}
	})

	t.Run("batcher_error", func(t *testing.T) {
		expectedErr := errors.New("add error")
		batcher := &mockMessageBatcher{err: expectedErr}
		gw := &mockGateway{batcher: batcher}
		state := &PipelineState[string]{gateway: gw}

		_, _, err := DefaultPipelineBatchMessages(ctx, state)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("empty", func(t *testing.T) {
		batcher := &mockMessageBatcher{msgs: []*ai.Message{}}
		gw := &mockGateway{batcher: batcher}
		state := &PipelineState[string]{gateway: gw}

		shouldContinue, _, err := DefaultPipelineBatchMessages(ctx, state)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if shouldContinue {
			t.Errorf("expected to stop (false) on empty batch")
		}
	})

	t.Run("populated", func(t *testing.T) {
		batcher := &mockMessageBatcher{msgs: []*ai.Message{{}}}
		gw := &mockGateway{batcher: batcher}
		state := &PipelineState[string]{gateway: gw}

		shouldContinue, msgs, err := DefaultPipelineBatchMessages(ctx, state)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !shouldContinue {
			t.Errorf("expected to continue on populated batch")
		}
		if len(msgs) != 1 {
			t.Errorf("expected batch of size 1")
		}
	})
}

func TestDefaultPipelineGenerate(t *testing.T) {
	ctx := context.Background()

	t.Run("nil_Agent", func(t *testing.T) {
		gw := &mockGateway{agent: nil}
		state := &PipelineState[string]{gateway: gw}

		_, err := DefaultPipelineGenerate(ctx, state)
		if !errors.Is(err, ErrNilAgent) {
			t.Errorf("expected ErrNilAgent, got %v", err)
		}
	})
}

// MOCKS
type mockGateway struct {
	agent   *Agent
	batcher MessageBatcher
}

func (m *mockGateway) ID() string                     { return "mock-gw-id" }
func (m *mockGateway) Name() string                   { return "Mock Gateway" }
func (m *mockGateway) Agent() *Agent                  { return m.agent }
func (m *mockGateway) MessageBatcher() MessageBatcher { return m.batcher }

type mockMessageBatcher struct {
	msgs []*ai.Message
	err  error
}

func (m *mockMessageBatcher) Add(ctx context.Context, batchID string, message *ai.Message) ([]*ai.Message, error) {
	return m.msgs, m.err
}

type mockPipelineProcessor struct {
	batchID string
	msg     *ai.Message
	input   *Input
	err1    error
	err2    error
	err3    error
}

func (m *mockPipelineProcessor) OriginalInputToMessage(ctx context.Context, state *PipelineState[string]) (string, *ai.Message, error) {
	return m.batchID, m.msg, m.err1
}

func (m *mockPipelineProcessor) BatchToInput(ctx context.Context, state *PipelineState[string]) (*Input, error) {
	return m.input, m.err2
}

func (m *mockPipelineProcessor) HandleAgentResponse(ctx context.Context, state *PipelineState[string]) error {
	return m.err3
}
