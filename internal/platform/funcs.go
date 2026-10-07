// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build (!linux && !darwin && !windows) || (!amd64 && !arm64)

package platform

import (
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
)

// Factory reports platforms without a native backend.
func Factory() (ports.Factory, error) { return nil, domain.ErrUnsupported }
