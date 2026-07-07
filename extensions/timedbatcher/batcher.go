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
	// ErrEmptyBatchID is returned when the provided batch identifier is an empty string.
	ErrEmptyBatchID = errors.New("batch id cannot be empty")

	// ErrNilMessage is returned when a nil pointer is passed instead of a valid AI message.
	ErrNilMessage = errors.New("message cannot be nil")
)

type batchState struct {
	messages []*ai.Message
	timer    *time.Timer
	out      chan []*ai.Message
}

// TimedBatcher automatically aggregates messages by ID and flushes them
// after a configurable Duration has elapsed.
type TimedBatcher struct {
	Duration time.Duration

	mu      sync.Mutex
	batches map[string]*batchState
}

// Add appends an AI message to a specific batch. The first caller that creates
// the batch blocks until either the Duration timer expires or the Context is canceled.
func (b *TimedBatcher) Add(ctx context.Context, batchID string, msg *ai.Message) ([]*ai.Message, error) {
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

	select {
	case msgs := <-state.out:
		return msgs, nil
	case <-ctx.Done():
		b.mu.Lock()
		currentState, ok := b.batches[batchID]
		if ok && currentState == state {
			state.timer.Stop()
			delete(b.batches, batchID)
			close(state.out)
		}
		b.mu.Unlock()

		return nil, ctx.Err()
	}
}
