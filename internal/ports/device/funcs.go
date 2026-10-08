// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package device

import (
	"context"
	"errors"
)

// Info returns an endpoint snapshot.
func (view *Operations[I, C, E]) Info() I { return view.Operations.Info() }

// Capabilities returns a control-schema snapshot.
func (view *Operations[I, C, E]) Capabilities() C { return view.Operations.Capabilities() }

// Extension queries a typed native extension.
func (view *Operations[I, C, E]) Extension(
	target any,
) bool {
	return view.Operations.Extension(target)
}

// Read consumes an event.
func (view *Operations[I, C, E]) Read(ctx context.Context) (E, error) {
	value, err := view.Operations.Read(ctx)

	return value, errors.Join(err)
}

// Close releases the view resources.
func (view *Operations[I, C, E]) Close() error { return errors.Join(view.Operations.Close()) }
