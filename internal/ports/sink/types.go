// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package sink

type (
	// Publisher defines the corresponding resource operation.
	Publisher[E any] interface{ Publish(event *E) bool }

	// FailureReceiver defines the corresponding resource operation.
	FailureReceiver interface{ Fail(err error) }
	// EventSink accepts copied updates and terminal failures.
	EventSink[E any] interface {
		Publisher[E]
		FailureReceiver
	}
)

type (
	// Operations exposes immutable, typed sink callbacks.
	Operations[E any] struct {
		// Operations holds the immutable dispatch table.
		Operations dispatch[E]
	}
	dispatch[E any] = struct {
		// Publish copies an event into the consumer queue.
		Publish func(*E) bool
		// Fail terminates the consumer with an error.
		Fail func(error)
	}
)
