// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"context"

	"github.com/gostafa/goinput/internal/domain"
)

type (
	// Discoverer enumerates accessible endpoints.
	Discoverer[I any] interface{ domain.Discoverer[I] }
	// Opener subscribes an event sink to an endpoint.
	Opener[ID, S, C any] interface{ domain.Opener[ID, S, C] }
)

type (
	// Retrier retries only errors explicitly classified as transient, within a shared
	// discovery budget. Implementations must check context before the first attempt.
	Retrier interface {
		domain.Retrier
	}
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

type (
	// Factory starts a replaceable native session. Failed creation must release all
	// partially acquired resources. Startup is never automatically retried.
	Factory func(context.Context, Retrier) (Backend, error)
	// Backend discovers endpoints and owns a native session.
	Backend interface {
		Discoverer[domain.DeviceInfo]
		Opener[domain.DeviceID, EventSink, Capture]
		Closer
	}
	// Capture owns one native device subscription.
	Capture interface {
		InfoProvider[domain.DeviceInfo]
		CapabilitiesProvider[domain.Capabilities]
		ExtensionProvider
		Closer
	}
	// EventSink is concurrency-safe. Publish never blocks; false means the stream
	// has terminated. Native callbacks must copy borrowed data before publishing.
	// Publish copies the event before returning; callers may then reuse it.
	EventSink interface {
		domain.EventSink[domain.Event]
	}
	// Provider supplies a shared value with context-bounded initialization.
	Provider[T any] interface {
		domain.Provider[T]
	}
)
