// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package manager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gostafa/goinput/internal/domain"
	subject "github.com/gostafa/goinput/internal/ports/manager"
)

const (
	firstSnapshotIndex = 0
	testSnapshot       = 1
	testIdentifier     = "endpoint"
)

func TestManagerDispatchRetainsValuesAndDiagnostics(t *testing.T) {
	t.Parallel()

	manager := new(subject.Operations[int, string, string])

	manager.Operations.Close = closedDispatch
	manager.Operations.Devices = discoverDispatch
	manager.Operations.Open = openDispatch

	values, err := manager.Devices(t.Context())
	checkDiagnostic(t, err)
	checkManagerOpen(t, manager, values)
}

func checkManagerOpen(
	t *testing.T,
	manager *subject.Operations[int, string, string],
	values []int,
) {
	t.Helper()

	value, err := manager.Open(t.Context(), testIdentifier)
	checkDiagnostic(t, err)
	checkDiagnostic(t, manager.Close())

	condition1 := value != testIdentifier || len(values) != testSnapshot ||
		values[firstSnapshotIndex] != testSnapshot

	if condition1 {
		t.Fatal("manager dispatch lost its return values")
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
