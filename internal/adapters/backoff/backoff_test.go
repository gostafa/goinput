// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoff

import (
	"context"
	"errors"
	"testing"

	"github.com/gostafa/goinput/internal/domain"
)

func closedOperation(context.Context) error { return domain.ErrClosed }

func TestCanceledOperationRetainsCause(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result, err := retryOperation(ctx, closedOperation, nil)()
	if result != (struct{}{}) || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled retry = %v", err)
	}
}

func TestRetryErrorRetainsNonEngineFailures(t *testing.T) {
	t.Parallel()

	condition1 := retryError(t.Context(), nil) != nil ||
		!errors.Is(retryError(t.Context(), domain.ErrClosed), domain.ErrClosed)

	if condition1 {
		t.Fatal("retry error conversion lost a non-engine failure")
	}
}
