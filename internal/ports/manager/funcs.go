// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package manager

import (
	"context"
	"errors"
)

// Devices discovers accessible endpoints.
func (view *Operations[I, ID, D]) Devices(ctx context.Context) ([]I, error) {
	value, err := view.Operations.Devices(ctx)

	return value, errors.Join(err)
}

// Open creates an independent event consumer.
func (view *Operations[I, ID, D]) Open(ctx context.Context, id ID) (D, error) {
	value, err := view.Operations.Open(ctx, id)

	return value, errors.Join(err)
}

// Close releases the view resources.
func (view *Operations[I, ID, D]) Close() error { return errors.Join(view.Operations.Close()) }
