// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

// Publish copies an event without blocking.
func (s *SinkOperations[E]) Publish(event *E) bool { return s.Operations.Publish(event) }

// Fail terminates publication with a cause.
func (s *SinkOperations[E]) Fail(err error) { s.Operations.Fail(err) }
