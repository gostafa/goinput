// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package iokit

// MetadataProvider is the optional IOHID device metadata extension.
type MetadataProvider interface {
	IOKitMetadata() Metadata
}

// Metadata contains descriptive IOHID properties, never owning native handles.
type Metadata struct {
	LocationID       *uint32
	Elements         []Element
	RegistryEntryID  uint64
	PrimaryUsagePage uint32
	PrimaryUsage     uint32
}

// Element describes the native element behind a common control. ControlID is
// the string representation of the public device-local ControlID.
type Element struct {
	ControlID       string
	Cookie          uint32
	ReportID        uint32
	ReportSize      uint32
	ReportCount     uint32
	PhysicalMinimum int64
	PhysicalMaximum int64
	Unit            uint32
	UnitExponent    int32
	HasNullState    bool
}
