// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package platform

import (
	"github.com/gostafa/goinput/internal/platform/darwin"
	"github.com/gostafa/goinput/internal/platform/linux"
	"github.com/gostafa/goinput/internal/platform/windows"
	"github.com/gostafa/goinput/internal/ports"
)

func nativeFactory(target string) ports.Factory {
	switch target {
	case "linux":
		return linux.Factory()
	case "darwin":
		return darwin.Factory()
	case "windows":
		return windows.Factory()
	default:
		return nil
	}
}
