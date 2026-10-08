// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build !linux || (!amd64 && !arm64)

package linux

import (
	"github.com/gostafa/goinput/internal/ports"
)

// Factory selects a native backend factory, or nil on unsupported targets.
func Factory() ports.Factory { return nil }
