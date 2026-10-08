// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"github.com/gostafa/goinput/internal/domain"
)

type (
	// InfoProvider supplies endpoint metadata independently of capture ownership.
	InfoProvider[I any] interface{ domain.InfoProvider[I] }

	// CapabilitiesProvider supplies the discoverable input schema.
	CapabilitiesProvider[C any] interface{ domain.CapabilitiesProvider[C] }

	// EventReader consumes an ordered event queue.
	EventReader[E any] interface{ domain.EventReader[E] }

	// ExtensionProvider queries optional native metadata.
	ExtensionProvider interface{ domain.ExtensionProvider }

	// Closer releases a resource.
	Closer interface{ domain.Closer }
)
