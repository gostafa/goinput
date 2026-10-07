// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

type (
	// DeviceClass classifies an input endpoint.
	DeviceClass = code[interface{ deviceClass() }]
	// Transport identifies a device connection medium.
	Transport = code[interface{ transport() }]
	// ControlKind classifies a device control.
	ControlKind = code[interface{ controlKind() }]
	// AxisMode distinguishes absolute and relative axes.
	AxisMode = code[interface{ axisMode() }]
	// MappingSource identifies how a control usage was obtained.
	MappingSource = code[interface{ mappingSource() }]

	// code retains nominal enum identity through a distinct, non-instantiated tag.
	code[Tag any] uint8
)
