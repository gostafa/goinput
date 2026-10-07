// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoff

import (
	"context"
	"errors"

	engine "github.com/cenkalti/backoff/v7"
)

// Do retries transient failures until the context, budget, or attempt limit expires.
func (retry retryFunc) Do(
	ctx context.Context,
	operation func(context.Context) error,
	transient func(error) bool,
) error {
	return errors.Join(retry(ctx, operation, transient))
}

// New supplies the default bounded retry policy.
func New() Retrier { return retryFunc(retry) }

func retry(
	ctx context.Context,
	operation func(context.Context) error,
	transient func(error) bool,
) error {
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	var err error = resultError(engine.Retry(
		ctx,
		retryOperation(ctx, operation, transient),
		engine.WithBackOff(
			retryPolicy(),
		),
		engine.WithMaxTries(maxAttempts),
		engine.WithMaxElapsedTime(budget),
	))

	return errors.Join(retryError(ctx, err))
}

func retryPolicy() *engine.ExponentialBackOff {
	policy := engine.NewExponentialBackOff()

	policy.InitialInterval = initialInterval
	policy.MaxInterval = maximumInterval
	policy.Multiplier = multiplier
	policy.RandomizationFactor = jitter

	return policy
}

func retryOperation(
	ctx context.Context,
	operation func(context.Context) error,
	transient func(error) bool,
) func() (struct{}, error) {
	return func() (struct{}, error) {
		err := context.Cause(ctx)
		if err != nil {
			return struct{}{}, engine.Permanent(err)
		}

		return struct{}{}, classifyRetry(operation(ctx), transient)
	}
}

func classifyRetry(err error, transient func(error) bool) error {
	if err != nil && (transient == nil || !transient(err)) {
		return errors.Join(engine.Permanent(err))
	}

	return err
}

func retryError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	if failure, ok := errors.AsType[*engine.RetryError](err); ok {
		return errors.Join(failure.LastErr, context.Cause(ctx))
	}

	return err
}
