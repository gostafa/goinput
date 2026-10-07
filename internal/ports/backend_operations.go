// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"context"
	"errors"

	"github.com/gostafa/goinput/internal/domain"
)

// Discover enumerates accessible endpoints.
func (b *BackendOperations) Discover(ctx context.Context) ([]domain.DeviceInfo, error) {
	wrappedValue0, wrappedErr := b.Operations.Discover(ctx)

	return wrappedValue0, errors.Join(wrappedErr)
}

// Open subscribes a sink to an endpoint.
func (b *BackendOperations) Open(
	ctx context.Context,
	id domain.DeviceID,
	sink EventSink,
) (Capture, error) {
	wrappedValue0, wrappedErr := b.Operations.Open(ctx, id, sink)

	return wrappedValue0, errors.Join(wrappedErr)
}
