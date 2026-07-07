package agens

import (
	"context"
	"errors"

	"github.com/firebase/genkit/go/ai"
)

// ErrInvalidWindowSize is returned when the configured sliding window size is zero or negative.
var ErrInvalidWindowSize = errors.New("window size must be positive")

var _ HistoryManager = &SlidingWindowManager{}

// SlidingWindowManager implements HistoryManager using a sliding window algorithm.
// It keeps only the most recent messages up to a specified memory size limit,
// discarding older messages to control token usage and context length.
type SlidingWindowManager struct {
	// WindowSize specifies the maximum number of messages to retain in the history.
	// Must be positive. If zero or negative, returns error.
	WindowSize int
}

// ProcessHistory trims the conversation history to ensure only the most recent
// messages (up to WindowSize) are returned. If the history size is within
// the limit, it returns the original slice unmodified.
// Returns error if WindowSize is invalid or context is cancelled.
func (m SlidingWindowManager) ProcessHistory(ctx context.Context, history []*ai.Message) ([]*ai.Message, error) {
	if m.WindowSize <= 0 {
		return nil, ErrInvalidWindowSize
	}

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(history) <= m.WindowSize {
		return history, nil
	}

	// cut
	cutoff := len(history) - m.WindowSize

	// If our sliding window cutoff point lands exactly on a ToolResponse,
	// we must expand our window backward by one message to include its preceding ToolRequest.
	// Otherwise, Genkit will receive an orphaned Tool Response and fail.
	for (cutoff > 0) && (history[cutoff].Role == ai.RoleTool) {
		cutoff-- // Slide the window back to rescue the ToolRequest
	}

	return history[cutoff:], nil
}
