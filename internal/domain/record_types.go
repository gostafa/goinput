// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

type (
	// Options configures a manager. Zero BufferSize selects the default capacity.
	Options struct {
		// BufferSize is the maximum number of queued events per capture.
		BufferSize int
	}

	// Range is an inclusive logical range.
	Range struct {
		// Min is the inclusive lower bound.
		Min int64
		// Max is the inclusive upper bound.
		Max int64
	}

	// DeviceInfoRecord describes an endpoint using the caller's identifier and enums.
	DeviceInfoRecord[ID ~string, Class, Medium ~uint8] struct {
		// ID identifies the endpoint or device-local control.
		ID        ID
		Transport Medium
		VendorID  *uint16
		ProductID *uint16
		// Name is the reported human-readable name.
		Name         string
		Path         string
		Manufacturer string
		Serial       string
		Classes      []Class
	}

	// ControlRecord describes a device-local control with caller-defined enums.
	// Usage is semantic identity and need not uniquely identify a control.
	ControlRecord[
		R ~struct{ Min, Max int64 },
		// ID identifies the endpoint or device-local control.
		ID ~string,
		U ~uint32,
		Kind, Mapping, Mode, ValueUnit, Supported ~uint8,
	] struct {
		// ID identifies the endpoint or device-local control.
		ID ID
		// Usage is the semantic HID page and usage identifier.
		Usage U
		// Kind classifies the control semantics.
		Kind Kind
		// Mapping identifies how the usage was obtained.
		Mapping Mapping
		// Mode distinguishes absolute positions from relative deltas.
		Mode Mode
		// Unit describes the value interpretation.
		Unit ValueUnit
		// Support reports whether the backend supports this control.
		Support Supported
		// Range contains inclusive logical bounds, when known.
		Range *R
		// Name is the reported human-readable name.
		Name string
	}

	// CapabilitiesRecord is a snapshot of discoverable controls.
	CapabilitiesRecord[C Cloner[C], Supported ~uint8] struct {
		// Repeat reports whether key-repeat events are available.
		Repeat Supported
		// Controls contains independent control descriptors.
		Controls []C
		// Complete reports whether all discoverable controls were enumerated.
		Complete bool
	}
)
