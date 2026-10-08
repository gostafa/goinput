// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package implementation

import (
	"context"
	"errors"
	"testing"

	"github.com/gostafa/goinput/internal/domain"
)

const (
	invalidTestBuffer = -1
)

func TestNewValidatesManagerInputs(t *testing.T) {
	t.Parallel()

	manager, err := New(t.Context(), domain.Options{BufferSize: invalidTestBuffer}, nil)
	if manager != nil || !errors.Is(err, domain.ErrInvalidOptions) {
		t.Fatalf("negative buffer = (%v, %v)", manager, err)
	}

	checkNilManagerContext(t)
	checkLazyManager(t)
}

func checkNilManagerContext(t *testing.T) {
	t.Helper()

	var ctx context.Context

	manager, err := New(ctx, domain.Options{BufferSize: defaultBufferSize}, nil)

	if manager != nil || !errors.Is(err, domain.ErrInvalidOptions) {
		t.Fatalf("nil manager lifetime = (%v, %v)", manager, err)
	}
}

func checkLazyManager(t *testing.T) {
	t.Helper()

	manager, err := New(t.Context(), domain.Options{BufferSize: defaultBufferSize}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if manager == nil {
		t.Fatal("lazy manager was not constructed")
	}
}
