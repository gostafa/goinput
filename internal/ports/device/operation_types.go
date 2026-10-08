// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package device

import (
	"context"
)

type (
	// Operations exposes immutable, typed device callbacks.
	Operations[I, C, E any] struct {
		// Operations holds the immutable dispatch table.
		Operations dispatch[I, C, E]
	}

	dispatch[I, C, E any] = struct {
		// Close releases resources owned by this view.
		Close func() error
		// Info copies endpoint metadata.
		Info func() I
		// Capabilities copies control capabilities.
		Capabilities func() C
		// Read consumes the next event.
		Read func(context.Context) (E, error)
		// Extension copies optional native metadata.
		Extension func(any) bool
	}
)
