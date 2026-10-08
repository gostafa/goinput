// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backend

import (
	"context"
)

type (
	// Discoverer defines the corresponding resource operation.
	Discoverer[I any] interface {
		Discover(ctx context.Context) ([]I, error)
	}

	// Opener defines the corresponding resource operation.
	Opener[ID, S, C any] interface {
		Open(ctx context.Context, id ID, sink S) (C, error)
	}

	// Closer defines the corresponding resource operation.
	Closer interface{ Close() error }

	// Operations exposes immutable, typed backend callbacks.
	Operations[I, ID, S, C any] struct {
		// Operations holds the immutable dispatch table.
		Operations dispatch[I, ID, S, C]
	}
	dispatch[I, ID, S, C any] = struct {
		// Close releases resources owned by this view.
		Close func() error
		// Discover enumerates endpoints and discovery diagnostics.
		Discover func(context.Context) ([]I, error)
		// Open subscribes to an endpoint.
		Open func(context.Context, ID, S) (C, error)
	}
)
