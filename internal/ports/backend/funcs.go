// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backend

import (
	"context"
	"errors"
)

// Discover enumerates accessible endpoints.
func (view *Operations[I, ID, S, C]) Discover(ctx context.Context) ([]I, error) {
	value, err := view.Operations.Discover(ctx)

	return value, errors.Join(err)
}

// Open subscribes a sink to an endpoint.
func (view *Operations[I, ID, S, C]) Open(ctx context.Context, id ID, sink S) (C, error) {
	value, err := view.Operations.Open(ctx, id, sink)

	return value, errors.Join(err)
}

// Close releases the view resources.
func (view *Operations[I, ID, S, C]) Close() error { return errors.Join(view.Operations.Close()) }
