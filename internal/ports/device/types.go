// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package device

import (
	"context"
)

type (
	// InfoProvider defines the corresponding resource operation.
	InfoProvider[I any] interface{ Info() I }

	// CapabilitiesProvider defines the corresponding resource operation.
	CapabilitiesProvider[C any] interface{ Capabilities() C }

	// ExtensionProvider defines the corresponding resource operation.
	ExtensionProvider interface{ Extension(target any) bool }

	// EventReader defines the corresponding resource operation.
	EventReader[E any] interface {
		Read(ctx context.Context) (E, error)
	}

	// Closer defines the corresponding resource operation.
	Closer interface{ Close() error }
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
