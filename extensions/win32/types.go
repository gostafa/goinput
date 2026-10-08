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
	NativeControl = NativeControlRecord[DescriptorBounds]

	// NativeControlRecord preserves native fields with caller-defined bounds.
	NativeControlRecord[Bounds any] struct {
		// Bounds preserves the logical and physical report limits.
		Bounds Bounds
		// ID identifies the corresponding device-local control.
		ID string
		// Units contains the HID physical unit descriptor.
		Units uint32
		// UnitsExponent contains the signed HID unit exponent encoding.
		UnitsExponent uint32
		// VirtualKey identifies keyboard inputs without a scan code.
		VirtualKey uint16
		// BitSize is the report value width in bits.
		BitSize uint16
		// ReportCount is the number of values in the report item.
		ReportCount uint16
		// LinkCollection identifies the parent HID link collection.
		LinkCollection uint16
		// DataIndex identifies the control within its input report.
		DataIndex uint16
		// ScanCode combines the keyboard make code with its E0 or E1 prefix.
		ScanCode uint16
		// ReportID identifies the containing HID report.
		ReportID byte
		// HasNull reports whether the descriptor defines a null state.
		HasNull bool
		// Absolute distinguishes absolute values from relative deltas.
		Absolute bool
	}
)
