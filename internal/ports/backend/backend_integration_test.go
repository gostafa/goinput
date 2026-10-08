// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backend_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gostafa/goinput/internal/domain"
	subject "github.com/gostafa/goinput/internal/ports/backend"
)

const (
	testSnapshot   = 1
	testIdentifier = "endpoint"
)

func TestBackendDispatchRetainsValuesAndDiagnostics(t *testing.T) {
	t.Parallel()

	backend := new(subject.Operations[int, string, bool, string])

	backend.Operations.Close, backend.Operations.Discover = closedDispatch, discoverDispatch
	backend.Operations.Open = func(ctx context.Context, id string, _ bool) (string, error) {
		return openDispatch(ctx, id)
	}

	values, err := backend.Discover(t.Context())
	checkDiagnostic(t, err)
	checkBackendOpen(t, backend, values)
}

func checkBackendOpen(
	t *testing.T,
	backend *subject.Operations[int, string, bool, string],
	values []int,
) {
	t.Helper()

	value, err := backend.Open(t.Context(), testIdentifier, true)
	checkDiagnostic(t, err)
	checkDiagnostic(t, backend.Close())

	if value != testIdentifier || len(values) != testSnapshot {
		t.Fatal("backend dispatch lost its return values")
	}
}

func closedDispatch() error { return domain.ErrClosed }

func discoverDispatch(
	context.Context,
) ([]int, error) {
	return []int{testSnapshot}, domain.ErrClosed
}

func openDispatch(_ context.Context, id string) (string, error) { return id, domain.ErrClosed }

func checkDiagnostic(t *testing.T, err error) {
	t.Helper()

	if !errors.Is(err, domain.ErrClosed) {
		t.Fatalf("dispatch diagnostic = %v", err)
	}
}
