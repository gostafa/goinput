// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package platform

import (
	"runtime"

	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/platform/darwin"
	"github.com/gostafa/goinput/internal/platform/linux"
	"github.com/gostafa/goinput/internal/platform/windows"
	"github.com/gostafa/goinput/internal/ports"
)

// Factory creates an independently owned native backend factory on supported platforms.
func Factory() (ports.Factory, error) {
	var factory ports.Factory

	switch runtime.GOOS {
	case "linux":
		factory = linux.Factory()
	case "darwin":
		factory = darwin.Factory()
	case "windows":
		factory = windows.Factory()
	}

	if factory == nil {
		return nil, domain.ErrUnsupported
	}

	return factory, nil
}
