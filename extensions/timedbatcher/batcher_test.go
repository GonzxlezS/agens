package timedbatcher

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"
)

func TestTimedBatcher_Add(t *testing.T) {
	var (
		batcher = &TimedBatcher{
			Duration: 100 * time.Millisecond,
		}

		ctx     = context.Background()
		batchID = "test-batch-1"

		wg            sync.WaitGroup
		receivedBatch []*ai.Message

		expectedTexts = []string{"msg1", "msg2"}
		msg1          = &ai.Message{
			Content: []*ai.Part{ai.NewTextPart(expectedTexts[0])},
		}
		msg2 = &ai.Message{
			Content: []*ai.Part{ai.NewTextPart(expectedTexts[1])},
		}
	)

	wg.Go(func() {
		bacth, err := batcher.Add(ctx, batchID, msg1)
		if err != nil {
			t.Errorf("unexpected error in Add 1: %v", err)
		}

		receivedBatch = bacth
	})

	time.Sleep(20 * time.Millisecond) // delay

	batch, err := batcher.Add(ctx, batchID, msg2)
	if err != nil {
		t.Errorf("unexpected error in Add 2: %v", err)
	} else if batch != nil {
		t.Error("batch must be nil")
	}

	wg.Wait()

	if len(receivedBatch) != 2 {
		t.Errorf("2 messages were expected; %d were received", len(receivedBatch))
	}

	for i, msg := range receivedBatch {
		txt := msg.Content[0].Text
		if txt != expectedTexts[i] {
			t.Errorf("Invalid message %d. Expected: %s, Received: %s", i, expectedTexts[i], txt)
		}
	}
}

func TestTimedBatcher_MultipleBatches(t *testing.T) {
	var (
		batcher = &TimedBatcher{Duration: 50 * time.Millisecond}

		ctx = context.Background()

		res1 = make(chan []*ai.Message)
		res2 = make(chan []*ai.Message)

		msgA = &ai.Message{Content: []*ai.Part{ai.NewTextPart("A1")}}
		msgB = &ai.Message{Content: []*ai.Part{ai.NewTextPart("B1")}}
	)

	go func() {
		m, _ := batcher.Add(ctx, "A", msgA)
		res1 <- m
	}()

	go func() {
		m, _ := batcher.Add(ctx, "B", msgB)
		res2 <- m
	}()

	r1 := <-res1
	r2 := <-res2

	if len(r1) != 1 || r1[0].Content[0].Text != "A1" {
		t.Errorf("Batch A was not processed correctly: %v", r1)
	}

	if len(r2) != 1 || r2[0].Content[0].Text != "B1" {
		t.Errorf("Batch B was not processed correctly: %v", r2)
	}
}
