// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

type (
	// OpError is an operation error with a domain device identifier.
	OpError = OperationError[DeviceID]

	// DeviceInfo is the domain specialization of an endpoint record.
	DeviceInfo = DeviceInfoRecord[DeviceID, DeviceClass, Transport]

	// Control is the domain specialization of a control record.
	Control = ControlRecord[Range, ControlID, Usage, ControlKind, MappingSource, AxisMode, Unit, Support]

	// Capabilities is the domain specialization of a capabilities record.
	Capabilities = CapabilitiesRecord[Control, Support]
)
