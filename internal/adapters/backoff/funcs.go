package backoff

import (
	"context"
	"errors"

	engine "github.com/cenkalti/backoff/v7"
)

func (Retrier) Do(ctx context.Context, operation func(context.Context) error, transient func(error) bool) error {
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	policy := engine.NewExponentialBackOff()
	policy.InitialInterval = initialInterval
	policy.MaxInterval = maximumInterval
	policy.Multiplier = multiplier
	policy.RandomizationFactor = jitter
	_, err := engine.Retry(ctx, func() (struct{}, error) {
		if err := context.Cause(ctx); err != nil {
			return struct{}{}, engine.Permanent(err)
		}
		err := operation(ctx)
		if err != nil && (transient == nil || !transient(err)) {
			err = engine.Permanent(err)
		}
		return struct{}{}, err
	}, engine.WithBackOff(policy), engine.WithMaxTries(maxAttempts), engine.WithMaxElapsedTime(budget))
	if err == nil {
		return nil
	}
	// Translate engine-specific stop types; retain operation and cancellation causes.
	var failure *engine.RetryError
	if errors.As(err, &failure) {
		return errors.Join(failure.LastErr, context.Cause(ctx))
	}
	return err
}
