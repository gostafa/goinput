// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package platform

import (
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
)

// Register is exclusively for adapter package initialization. Duplicate or nil
// factories indicate a build/wiring bug and panic.
func Register(f ports.Factory) {
	if f == nil || factory != nil {
		panic("goinput: invalid or duplicate platform factory")
	}

	factory = f
}

func Factory() (ports.Factory, error) {
	if factory == nil {
		return nil, domain.ErrUnsupported
	}

	return factory, nil
}
