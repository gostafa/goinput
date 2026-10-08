// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"context"
)

type (
	// Discoverer enumerates accessible endpoints with partial diagnostics.
	Discoverer[I any] interface {
		Discover(ctx context.Context) ([]I, error)
	}

	// Opener subscribes a sink to a native device.
	Opener[ID, S, C any] interface {
		Open(ctx context.Context, id ID, sink S) (C, error)
	}

	// DeviceOpener creates an independent event consumer.
	DeviceOpener[ID, D any] interface {
		Open(ctx context.Context, id ID) (D, error)
	}

	// Provider supplies a shared value with context-bounded initialization.
	Provider[T any] interface {
		Get(ctx context.Context) (T, error)
	}

	// Retrier bounds transient operations by context and policy.
	Retrier interface {
		Do(
			ctx context.Context,
			operation func(context.Context) error,
			transient func(error) bool,
		) error
	}
)
