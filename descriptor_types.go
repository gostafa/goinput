// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput

import (
	"github.com/gostafa/goinput/internal/domain"
)

type (
	// Options configures a manager. Zero BufferSize selects the default capacity.
	Options = domain.Options

	// Range is the inclusive logical range of an absolute control.
	Range = domain.Range

	// DeviceInfo describes one endpoint; empty metadata means unavailable data.
	DeviceInfo = domain.DeviceInfoRecord[DeviceID, DeviceClass, Transport]

	// Control describes a device-local input. Range is nil when unavailable.
	Control = domain.ControlRecord[Range, ControlID, Usage, ControlKind, MappingSource, AxisMode, Unit, Support]

	// Capabilities is a snapshot of discoverable input controls.
	Capabilities = domain.CapabilitiesRecord[Control, Support]
)
