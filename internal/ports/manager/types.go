// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package manager

import (
	"context"
)

type (
	// Discoverer defines the corresponding resource operation.
	Discoverer[I any] interface {
		Devices(ctx context.Context) ([]I, error)
	}

	// Opener defines the corresponding resource operation.
	Opener[ID, D any] interface {
		Open(ctx context.Context, id ID) (D, error)
	}

	// Closer defines the corresponding resource operation.
	Closer interface{ Close() error }

	// Operations exposes immutable, typed manager callbacks.
	Operations[I, ID, D any] struct {
		// Operations holds the immutable dispatch table.
		Operations dispatch[I, ID, D]
	}
	dispatch[I, ID, D any] = struct {
		// Close releases resources owned by this view.
		Close func() error
		// Devices enumerates accessible endpoints.
		Devices func(context.Context) ([]I, error)
		// Open subscribes to an endpoint.
		Open func(context.Context, ID) (D, error)
	}
)
