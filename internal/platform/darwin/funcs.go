// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build darwin && (amd64 || arm64)

package darwin

import (
	"github.com/gostafa/goinput/internal/adapters/iokit"
)

// Factory creates an independently owned native backend factory.
func init() { Factory = iokit.Factory }
