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

type TimedBatcher struct {
	Duration time.Duration

	mu       sync.Mutex
	channels map[string]chan *ai.Message
}

func (b *TimedBatcher) Add(_ context.Context, batchID string, msg *ai.Message) ([]*ai.Message, error) {
	if batchID == "" {
		return nil, ErrEmptyBatchID
	} else if msg == nil {
		return nil, ErrNilMessage
	}

	b.mu.Lock()
	if b.channels == nil {
		b.channels = make(map[string]chan *ai.Message)
	}

	var out chan []*ai.Message

	ch, ok := b.channels[batchID]
	if !ok {
		ch = make(chan *ai.Message, 100)
		b.channels[batchID] = ch

		out = make(chan []*ai.Message)
		go b.start(batchID, ch, out)
	}

	ch <- msg
	b.mu.Unlock()

	if ok {
		return nil, nil
	}
	return <-out, nil
}

func (b *TimedBatcher) start(batchID string, ch chan *ai.Message, out chan []*ai.Message) {
	var (
		batch []*ai.Message
		timer = time.NewTimer(b.Duration)
	)

	defer func() {
		b.mu.Lock()
		delete(b.channels, batchID)
		close(ch)
		b.mu.Unlock()

		out <- batch
	}()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}

			batch = append(batch, msg)

			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(b.Duration)

		case <-timer.C:
			return
		}
	}
}
