// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package iokit

import (
	"github.com/gostafa/goinput/internal/domain"
)

type (
	// MetadataProvider is the optional IOHID device metadata extension.
	MetadataProvider interface {
		domain.MetadataProvider[Metadata]
	}

	// Metadata contains descriptive IOHID properties, never owning native handles.
	Metadata = MetadataRecord[Element]

	// MetadataRecord retains IOHID properties with a caller-defined element record.
	MetadataRecord[E any] struct {
		// LocationID is the optional IOHID location identifier.
		LocationID *uint32
		// Elements describes the native elements behind common controls.
		Elements []E
		// RegistryEntryID is the stable registry identifier for this endpoint.
		RegistryEntryID uint64
		// PrimaryUsagePage is the device's declared primary HID usage page.
		PrimaryUsagePage uint32
		// PrimaryUsage is the device's declared usage within PrimaryUsagePage.
		PrimaryUsage uint32
	}

	// Element describes the native element behind a common control. ControlID is
	// the string representation of the public device-local ControlID.
	Element ElementRecord[string]

	// ElementRecord retains native element properties with a caller-defined control identifier.
	ElementRecord[ID ~string] struct {
		// ControlID identifies the corresponding common control.
		ControlID ID
		// Cookie identifies the element within its IOHID device.
		Cookie uint32
		// ReportID selects the HID report containing the element.
		ReportID uint32
		// ReportSize is the number of bits per report value.
		ReportSize uint32
		// ReportCount is the number of values in the report item.
		ReportCount uint32
		// PhysicalMinimum is the descriptor's physical lower bound.
		PhysicalMinimum int64
		// PhysicalMaximum is the descriptor's physical upper bound.
		PhysicalMaximum int64
		// Unit is the encoded HID physical unit.
		Unit uint32
		// UnitExponent is the base-ten exponent applied to Unit.
		UnitExponent int32
		// HasNullState reports whether the descriptor permits a null value.
		HasNullState bool
	}
)
