// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build (darwin || linux || windows) && (amd64 || arm64)

package platform

import (
	"runtime"

	"github.com/gostafa/goinput/internal/ports"
)

// Factory creates an independently owned native backend factory on supported platforms.
func Factory() (ports.Factory, error) { return nativeFactory(runtime.GOOS), nil }
