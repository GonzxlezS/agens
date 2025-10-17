package agens

import (
	"context"
	"errors"

	"github.com/firebase/genkit/go/ai"
)

// ErrInvalidOutputOption is returned when a value associated with the output option key exists in the context,
// but it is not of the expected type (ai.OutputOption).
var ErrInvalidOutputOption = errors.New("output option in context is invalid")

// OutputOptionKey is used as a context key to store and retrieve the ai.OutputOption value within a context.Context.
type OutputOptionKey struct{}

// WithOutputOption returns a new context.Context derived from the provided ctx,
// with the specified ai.OutputOption attached to it.
func WithOutputOption(ctx context.Context, opt ai.OutputOption) context.Context {
	return context.WithValue(ctx, OutputOptionKey{}, opt)
}

func (agent *Agent) outputOption(ctx context.Context) (ai.OutputOption, error) {
	v := ctx.Value(OutputOptionKey{})
	if v == nil {
		return nil, nil
	}

	opt, ok := v.(ai.OutputOption)
	if ok {
		return opt, nil
	}
	return nil, ErrInvalidOutputOption
}
