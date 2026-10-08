// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package windows

import (
	"github.com/gostafa/goinput/internal/ports"
)

// Factory selects a native backend factory, or nil on unsupported targets.
var Factory = func() ports.Factory { return nil }
