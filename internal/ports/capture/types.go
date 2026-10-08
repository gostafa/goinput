// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package capture

type (
	// InfoProvider defines the corresponding resource operation.
	InfoProvider[I any] interface{ Info() I }

	// CapabilitiesProvider defines the corresponding resource operation.
	CapabilitiesProvider[C any] interface{ Capabilities() C }

	// ExtensionProvider defines the corresponding resource operation.
	ExtensionProvider interface{ Extension(target any) bool }

	// Closer defines the corresponding resource operation.
	Closer interface{ Close() error }

	// Operations exposes immutable, typed capture callbacks.
	Operations[I, C any] struct {
		// Operations holds the immutable dispatch table.
		Operations dispatch[I, C]
	}
	dispatch[I, C any] = struct {
		// Close releases resources owned by this view.
		Close func() error
		// Info copies endpoint metadata.
		Info func() I
		// Capabilities copies control capabilities.
		Capabilities func() C
		// Extension copies optional native metadata.
		Extension func(any) bool
	}
)
