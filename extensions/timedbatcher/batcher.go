package timedbatcher

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gonzxlezs/agens"

	"github.com/firebase/genkit/go/ai"
)

var _ agens.MessageBatcher = &TimedBatcher{}

var (
	ErrEmptyBatchID = errors.New("batch id cannot be empty")

	ErrNilMessage = errors.New("message cannot be nil")
)

type batchState struct {
	messages []*ai.Message
	timer    *time.Timer
	out      chan []*ai.Message
}

type TimedBatcher struct {
	Duration time.Duration

	mu      sync.Mutex
	batches map[string]*batchState
}

func (b *TimedBatcher) Add(_ context.Context, batchID string, msg *ai.Message) ([]*ai.Message, error) {
	if batchID == "" {
		return nil, ErrEmptyBatchID
	} else if msg == nil {
		return nil, ErrNilMessage
	}

	b.mu.Lock()
	if b.batches == nil {
		b.batches = make(map[string]*batchState)
	}

	state, exists := b.batches[batchID]
	if !exists {
		state = &batchState{
			messages: make([]*ai.Message, 0),
			out:      make(chan []*ai.Message, 1),
		}
		b.batches[batchID] = state

		state.timer = time.AfterFunc(b.Duration, func() {
			b.mu.Lock()
			defer b.mu.Unlock()

			currentState, ok := b.batches[batchID]
			if ok && currentState == state {
				delete(b.batches, batchID)

				state.out <- state.messages
				close(state.out)
			}
		})
	} else {
		state.timer.Reset(b.Duration)
	}

	state.messages = append(state.messages, msg)
	b.mu.Unlock()

	if exists {
		return nil, nil
	}
	return <-state.out, nil
}
