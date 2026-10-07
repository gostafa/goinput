// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"context"
	"errors"
)

// Devices discovers accessible endpoints.
func (m *ManagerOperations[I, ID, D]) Devices(ctx context.Context) ([]I, error) {
	wrappedValue0, wrappedErr := m.Operations.Devices(ctx)

	return wrappedValue0, errors.Join(wrappedErr)
}

// Open creates an independent event consumer.
func (m *ManagerOperations[I, ID, D]) Open(ctx context.Context, id ID) (D, error) {
	wrappedValue0, wrappedErr := m.Operations.Open(ctx, id)

	return wrappedValue0, errors.Join(wrappedErr)
}
