// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"github.com/gostafa/goinput/internal/domain"
)

// Info returns an endpoint snapshot.
func (c *CaptureOperations) Info() domain.DeviceInfo { return c.Operations.Info() }

// Capabilities returns a control-schema snapshot.
func (c *CaptureOperations) Capabilities() domain.Capabilities { return c.Operations.Capabilities() }

// Extension queries a typed native extension.
func (c *CaptureOperations) Extension(
	target any,
) bool {
	return c.Operations.Extension(target)
}
