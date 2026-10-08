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

	// Retrier retries only errors explicitly classified as transient, within a shared
	// discovery budget. Implementations must check context before the first attempt.
	Retrier interface {
		domain.Retrier
	}

	// Factory starts a replaceable native session. Failed creation must release all
	// partially acquired resources. Startup is never automatically retried.
	Factory func(context.Context, Retrier) (Backend, error)

	// Backend discovers endpoints and owns a native session.
	Backend interface {
		Discoverer[domain.DeviceInfo]
		Opener[domain.DeviceID, EventSink, Capture]
		Closer
	}
)
