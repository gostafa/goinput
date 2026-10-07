// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
)

func TestCanceledSessionCreationReleasesBackend(t *testing.T) {
	t.Parallel()

	backend := new(testBackend)
	coordinator := NewCoordinator(func(context.Context, ports.Retrier) (ports.Backend, error) {
		return backend, nil
	}, nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result, err := coordinatorCreateSession(ctx, coordinator)
	checkCanceledSession(t, result, err)

	if backend.closes != unitStep {
		t.Fatalf("backend closed %d times, want once", backend.closes)
	}
}

func checkCanceledSession(t *testing.T, result ports.Backend, err error) {
	t.Helper()

	if result != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled session = (%v, %v), want (nil, context.Canceled)", result, err)
	}
}

func TestSessionFactoryFailureReturnsNoBackend(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator(func(context.Context, ports.Retrier) (ports.Backend, error) {
		return nil, domain.ErrUnsupported
	}, nil)
	result, err := coordinatorCreateSession(t.Context(), coordinator)

	if result != nil || !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("missing session = (%v, %v), want (nil, ErrUnsupported)", result, err)
	}
}
