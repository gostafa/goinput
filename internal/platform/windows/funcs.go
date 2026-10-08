// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package windows

import (
	"github.com/gostafa/goinput/internal/adapters/win32"
	"github.com/gostafa/goinput/internal/ports"
)

// Factory creates an independently owned native backend factory.
func Factory() ports.Factory { return win32.Factory() }
