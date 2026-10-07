// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"context"

	"github.com/gostafa/goinput/internal/domain"
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
