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
		ID           ID
		Transport    Medium
		VendorID     *uint16
		ProductID    *uint16
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
		ID ~string,
		U ~uint32,
		Kind, Mapping, Mode, ValueUnit, Supported ~uint8,
	] struct {
		ID      ID
		Usage   U
		Kind    Kind
		Mapping Mapping
		Mode    Mode
		Unit    ValueUnit
		Support Supported
		Range   *R
		Name    string
	}
	// CapabilitiesRecord is a snapshot of discoverable controls.
	CapabilitiesRecord[C Cloner[C], Supported ~uint8] struct {
		Repeat   Supported
		Controls []C
		Complete bool
	}
)
