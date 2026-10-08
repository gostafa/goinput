// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package sink

// Publish copies an event without blocking.
func (view *Operations[E]) Publish(event *E) bool { return view.Operations.Publish(event) }

// Fail terminates publication with a cause.
func (view *Operations[E]) Fail(err error) { view.Operations.Fail(err) }
