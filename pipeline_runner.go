package agens

import "context"

type pipelineRunner[T any] struct {
	processor PipelineProcessor[T]
	cfg       *PipelineOptions[T]
}

func (r *pipelineRunner[T]) originalInputToMessage(ctx context.Context, state *PipelineState[T]) (bool, error) {
	batchID, msg, err := r.processor.OriginalInputToMessage(ctx, state)
	if err != nil {
		return false, err
	}

	state.batchID = batchID
	state.message = msg

	return r.batchMessages(ctx, state)
}

func (r *pipelineRunner[T]) batchMessages(ctx context.Context, state *PipelineState[T]) (bool, error) {
	shouldContinue, batchMsgs, err := r.cfg.BatchMessages(ctx, state)
	if err != nil {
		return false, err
	}

	if !shouldContinue {
		return false, nil
	}

	state.batchMessages = batchMsgs
	return r.batchToInput(ctx, state)
}

func (r *pipelineRunner[T]) batchToInput(ctx context.Context, state *PipelineState[T]) (bool, error) {
	in, err := r.processor.BatchToInput(ctx, state)
	if err != nil {
		return false, err
	}

	state.input = in
	return r.generate(ctx, state)
}

func (r *pipelineRunner[T]) generate(ctx context.Context, state *PipelineState[T]) (bool, error) {
	resp, err := r.cfg.Generate(ctx, state)
	if err != nil {
		return false, err
	}

	state.response = resp
	return r.handleAgentResponse(ctx, state)
}

func (r *pipelineRunner[T]) handleAgentResponse(ctx context.Context, state *PipelineState[T]) (bool, error) {
	err := r.processor.HandleAgentResponse(ctx, state)
	if err != nil {
		return false, err
	}
	return true, nil
}
