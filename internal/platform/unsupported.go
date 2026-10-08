// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build (!darwin && !linux && !windows) || (!amd64 && !arm64)

package platform

import (
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
)

// Factory reports that this target has no native input backend.
func Factory() (ports.Factory, error) { return nil, domain.ErrUnsupported }
