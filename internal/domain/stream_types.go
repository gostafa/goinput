// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"context"
)

type (
	// EventReader consumes one event from an ordered queue.
	EventReader[E any] interface {
		Read(ctx context.Context) (E, error)
	}

	// EventPublisher copies a borrowed event without blocking.
	EventPublisher[E any] interface{ Publish(event *E) bool }

	// FailureSink terminates a stream with an underlying cause.
	FailureSink interface{ Fail(err error) }

	// Closer releases a resource; repeated calls are safe.
	Closer interface{ Close() error }

	// Cloner creates an independently mutable snapshot.
	Cloner[T any] interface{ Clone() T }
)
