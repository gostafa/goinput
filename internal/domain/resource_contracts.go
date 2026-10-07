// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"context"
)

type (
	// Device combines endpoint snapshots with an ordered event reader.
	Device[I, C, E any] interface {
		InfoProvider[I]
		CapabilitiesProvider[C]
		EventReader[E]
		Closer
	}
	// Capture owns a native subscription and exposes snapshots and extensions.
	Capture[I, C any] interface {
		InfoProvider[I]
		CapabilitiesProvider[C]
		ExtensionProvider
		Closer
	}
	// Backend discovers endpoints and opens native captures.
	Backend[I, ID, S, C any] interface {
		Discoverer[I]
		Opener[ID, S, C]
		Closer
	}
	// EventSink accepts copied updates and terminal failures.
	EventSink[E any] interface {
		EventPublisher[E]
		FailureSink
	}
	// Manager owns independently buffered consumers.
	Manager[I, ID, D any] interface {
		Devices(ctx context.Context) ([]I, error)
		DeviceOpener[ID, D]
		Closer
	}
)
