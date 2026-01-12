package agens

import (
	"context"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

const (
	MessageBatchStep = "messageBatch"

	RetrieveHistoryStep = "retrieveHistory"

	StoreHistoryStep = "storeHistory"
)

func baseFlow(g *genkit.Genkit, cfg *AgentConfig, historyMemory HistoryMemory) func(ctx context.Context, msg *ai.Message) (*ai.ModelResponse, error) {
	// base options
	baseOpts := make([]ai.GenerateOption, 0, len(cfg.AdditionalOptions)+3)
	baseOpts = append(baseOpts, cfg.AdditionalOptions...)

	baseOpts = append(baseOpts, ai.WithSystem(cfg.SystemMessage()))

	if cfg.Model != nil {
		baseOpts = append(baseOpts, ai.WithModel(cfg.Model))
	} else if cfg.ModelName != "" {
		baseOpts = append(baseOpts, ai.WithModelName(cfg.ModelName))
	}

	if len(cfg.Tools) > 0 {
		baseOpts = append(baseOpts, ai.WithTools(cfg.Tools...))
	}

	return func(ctx context.Context, msg *ai.Message) (*ai.ModelResponse, error) {
		// conversation id
		conversationID, err := cfg.GetConversationID(msg)
		if err != nil {
			return EmptyModelResponse(), err
		}

		// message batch
		batch, err := messageBatchStep(ctx, cfg.Batcher, conversationID, msg)
		if err != nil {
			return EmptyModelResponse(), err
		}
		if len(batch) == 0 {
			return DelegatedModelResponse(), nil
		}

		// history
		history, err := retrieveHistoryStep(ctx, historyMemory, conversationID)
		if err != nil {
			return EmptyModelResponse(), err
		}

		// output option
		outputOpt, err := GetOutputOption(ctx)
		if err != nil {
			return EmptyModelResponse(), err
		}

		// options
		opts := make([]ai.GenerateOption, len(baseOpts), len(baseOpts)+2)
		copy(opts, baseOpts)

		opts = append(
			opts,
			ai.WithMessages(append(history, batch...)...), /* messages */
		)

		if outputOpt != nil {
			opts = append(opts, outputOpt)
		}

		// generate
		resp, err := genkit.Generate(ctx, g, opts...)
		if err != nil {
			return EmptyModelResponse(), err
		}

		// store history
		err = storeHistoryStep(ctx, historyMemory, conversationID, resp.History())

		return resp, err
	}
}

func messageBatchStep(ctx context.Context, batcher MessageBatcher, conversationID string, msg *ai.Message) ([]*ai.Message, error) {
	if batcher == nil {
		return []*ai.Message{msg}, nil
	}

	return genkit.Run(ctx, MessageBatchStep, func() ([]*ai.Message, error) {
		messages, err := batcher.Add(conversationID, msg)
		if (err != nil) || (messages == nil) {
			return []*ai.Message{}, err
		}
		return messages, nil
	})
}

func retrieveHistoryStep(ctx context.Context, historyMemory HistoryMemory, conversationID string) ([]*ai.Message, error) {
	if historyMemory == nil {
		return nil, nil
	}

	return genkit.Run(ctx, RetrieveHistoryStep, func() ([]*ai.Message, error) {
		return historyMemory.RetrieveHistory(ctx, conversationID)
	})
}

func storeHistoryStep(ctx context.Context, historyMemory HistoryMemory, conversationID string, history []*ai.Message) error {
	if historyMemory == nil {
		return nil
	}

	_, err := genkit.Run(ctx, StoreHistoryStep, func() (struct{}, error) {
		err := historyMemory.StoreHistory(ctx, conversationID, history)
		return struct{}{}, err
	})
	return err
}
