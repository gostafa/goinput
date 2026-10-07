// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build darwin && (amd64 || arm64)

package platform

import (
	"github.com/gostafa/goinput/internal/adapters/iokit"
	"github.com/gostafa/goinput/internal/ports"
)

// Factory creates an independently owned native backend factory.
func Factory() (ports.Factory, error) { return iokit.Factory(), nil }
