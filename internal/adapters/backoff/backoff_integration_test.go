// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoff_test

import (
	"context"
	"errors"
	"testing"

	subject "github.com/gostafa/goinput/internal/adapters/backoff"
	"github.com/gostafa/goinput/internal/domain"
)

const (
	initialAttempt    = 0
	successfulAttempt = 2
)

func TestPermanentFailureIsPreserved(t *testing.T) {
	t.Parallel()

	err := subject.New().Do(t.Context(), closedOperation, nil)
	if !errors.Is(err, domain.ErrClosed) {
		t.Fatalf("permanent failure = %v", err)
	}
}

func closedOperation(context.Context) error { return domain.ErrClosed }

func TestTransientFailureCanRecover(t *testing.T) {
	t.Parallel()

	attempts := initialAttempt
	err := subject.New().
		Do(t.Context(), recoveringOperation(&attempts), func(error) bool { return true })

	if err != nil || attempts != successfulAttempt {
		t.Fatalf("retry = %v after %d attempts", err, attempts)
	}
}

func recoveringOperation(attempts *int) func(context.Context) error {
	return func(context.Context) error {
		*attempts++
		if *attempts < successfulAttempt {
			return domain.ErrNotFound
		}

		return nil
	}
}
