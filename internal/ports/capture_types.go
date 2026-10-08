// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"github.com/gostafa/goinput/internal/domain"
)

type (
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
