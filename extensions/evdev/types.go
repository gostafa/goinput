// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package evdev

import (
	"github.com/gostafa/goinput/internal/domain"
)

type (
	// Metadata provides information that has no universal input-device equivalent.
	Metadata interface {
		domain.MetadataProvider[Info]
	}

	// Info describes an evdev endpoint without exposing its native file descriptor.
	Info = InfoRecord[AxisInfo, NativeControl]

	// InfoRecord retains Linux endpoint properties with typed axes and controls.
	InfoRecord[A, C any] struct {
		// Axes maps Linux absolute-axis codes to their calibration metadata.
		Axes map[uint16]A
		// PhysicalLocation is the kernel-reported connection path.
		PhysicalLocation string
		// Properties contains the supported Linux input-property codes.
		Properties []uint16
		// Controls maps common controls to native Linux event codes.
		Controls []C
		// BusType is the Linux input bus identifier.
		BusType uint16
		// Version is the kernel-reported device version.
		Version uint16
	}

	// NativeControl associates a device-local control with Linux event codes.
	// Several native controls may contribute to one normalized hat control.
	NativeControl struct {
		// ID identifies the corresponding common control.
		ID string
		// Type is the Linux event type.
		Type uint16
		// Code is the event code within Type.
		Code uint16
	}

	// AxisInfo retains the kernel's absolute-axis information at capture opening.
	AxisInfo struct {
		// Value is the axis position when metadata was captured.
		Value int32
		// Minimum is the inclusive logical lower bound.
		Minimum int32
		// Maximum is the inclusive logical upper bound.
		Maximum int32
		// Fuzz is the kernel noise-filter tolerance.
		Fuzz int32
		// Flat is the kernel dead-zone tolerance.
		Flat int32
		// Resolution is the kernel-reported units per physical unit.
		Resolution int32
	}
)
