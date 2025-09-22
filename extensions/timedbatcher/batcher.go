package timedbatcher

import (
	"errors"
	"sync"
	"time"

	"github.com/gonzxlezs/agens"

	"github.com/firebase/genkit/go/ai"
)

var ErrCannotAddMessage = errors.New("cannot add message to batch")

var _ agens.MessageBatcher = &TimedBatcher{}

type TimedBatcher struct {
	Duration time.Duration

	mu       sync.Mutex
	channels map[string]chan *ai.Message
}

func (b *TimedBatcher) Add(msg *ai.Message) ([]*ai.Message, error) {
	conversationID, err := agens.GetConversationID(msg)
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	if b.channels == nil {
		b.channels = make(map[string]chan *ai.Message)
	}

	var out chan []*ai.Message

	ch, ok := b.channels[conversationID]
	if !ok {
		ch = make(chan *ai.Message, 100)
		b.channels[conversationID] = ch

		out = make(chan []*ai.Message)
		go b.start(conversationID, ch, out)
	}

	ch <- msg
	b.mu.Unlock()

	if ok {
		return nil, nil
	}
	return <-out, nil
}

func (b *TimedBatcher) start(conversationID string, ch chan *ai.Message, out chan []*ai.Message) {
	var (
		batch []*ai.Message
		timer = time.NewTimer(b.Duration)
	)

	defer func() {
		b.mu.Lock()
		delete(b.channels, conversationID)
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
