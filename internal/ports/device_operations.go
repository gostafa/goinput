// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"context"
	"errors"
)

// Info returns an endpoint snapshot.
func (d *DeviceOperations[I, C, E]) Info() I { return d.Operations.Info() }

// Capabilities returns a control-schema snapshot.
func (d *DeviceOperations[I, C, E]) Capabilities() C { return d.Operations.Capabilities() }

// Read consumes an event.
func (d *DeviceOperations[I, C, E]) Read(ctx context.Context) (E, error) {
	wrappedValue0, wrappedErr := d.Operations.Read(ctx)

	return wrappedValue0, errors.Join(wrappedErr)
}

// Extension queries a typed native extension.
func (d *DeviceOperations[I, C, E]) Extension(
	target any,
) bool {
	return d.Operations.Extension(target)
}
