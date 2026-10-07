// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package win32

// Metadata is the optional Windows device metadata interface.
type Metadata interface {
	NativeInfo() Info
}

// Info describes one Windows Raw Input endpoint, usually a HID top-level
// collection. DeviceType is 0 for mouse, 1 for keyboard, or 2 for other HID.
type Info struct {
	Controls       []NativeControl
	RawInputHandle uintptr
	DeviceType     uint32
	Version        uint32
	UsagePage      uint16
	Usage          uint16
}

// NativeControl preserves descriptor details outside the common input model.
// ID matches the corresponding normalized control. Keyboard ScanCode combines
// the make code with its E0/E1 prefix; VirtualKey identifies keyboard inputs with
// no scan code. Generic HID controls use data indices.
type NativeControl struct {
	ID             string
	Units          uint32
	LogicalMin     int32
	UnitsExponent  uint32
	LogicalMax     int32
	PhysicalMin    int32
	PhysicalMax    int32
	VirtualKey     uint16
	BitSize        uint16
	ReportCount    uint16
	LinkCollection uint16
	DataIndex      uint16
	ScanCode       uint16
	ReportID       byte
	HasNull        bool
	Absolute       bool
}
