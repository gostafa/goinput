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
