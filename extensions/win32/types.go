// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package win32

import (
	"github.com/gostafa/goinput/internal/domain"
)

type (
	// Metadata is the optional Windows device metadata interface.
	Metadata interface {
		domain.MetadataProvider[Info]
	}

	// Info describes one Windows Raw Input endpoint, usually a HID top-level
	// collection. DeviceType is 0 for mouse, 1 for keyboard, or 2 for other HID.
	Info = InfoRecord[NativeControl]

	// InfoRecord describes a Raw Input endpoint with caller-defined native controls.
	InfoRecord[C any] struct {
		// Controls contains the native descriptor mappings.
		Controls []C
		// RawInputHandle is a borrowed Raw Input endpoint identifier.
		RawInputHandle uintptr
		// DeviceType identifies mouse, keyboard, or generic HID endpoints.
		DeviceType uint32
		// Version is the reported device version.
		Version uint32
		// UsagePage identifies the primary HID usage page.
		UsagePage uint16
		// Usage is the primary usage within UsagePage.
		Usage uint16
	}

	// NativeControl preserves descriptor details outside the common input model.
	// ID matches the corresponding normalized control. Keyboard ScanCode combines
	// the make code with its E0/E1 prefix; VirtualKey identifies keyboard inputs with
	// no scan code. Generic HID controls use data indices.
	NativeControl struct {
		ID string
		DescriptorBounds
		Units          uint32
		UnitsExponent  uint32
		ReportCount    uint16
		BitSize        uint16
		VirtualKey     uint16
		LinkCollection uint16
		DataIndex      uint16
		ScanCode       uint16
		ReportID       byte
		HasNull        bool
		Absolute       bool
	}

	// DescriptorBounds groups logical and physical bounds of one report value.
	DescriptorBounds struct {
		// LogicalMin is the inclusive logical lower bound.
		LogicalMin int32
		// LogicalMax is the inclusive logical upper bound.
		LogicalMax int32
		// PhysicalMin is the descriptor's physical lower bound.
		PhysicalMin int32
		// PhysicalMax is the descriptor's physical upper bound.
		PhysicalMax int32
	}
)
