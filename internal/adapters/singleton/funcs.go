package singleton

import (
	"context"

	"github.com/gostafa/goinput/internal/ports"
	engine "github.com/gostafa/singleton"
)

// New constructs a lazy provider. Its factory only allocates Go coordinator
// state, so one attempt prevents nesting initialization and discovery retries.
func New[T any](factory func(context.Context) (T, error)) ports.Provider[T] {
	return engine.MustNew(factory, engine.WithMaxAttempts(1))
}
