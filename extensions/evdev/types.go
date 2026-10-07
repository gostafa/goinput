package evdev

// Metadata provides information that has no universal input-device equivalent.
type Metadata interface {
	NativeInfo() Info
}

// Info describes an evdev endpoint without exposing its native file descriptor.
type Info struct {
	PhysicalLocation string
	BusType          uint16
	Version          uint16
	Properties       []uint16
	Controls         []NativeControl
	Axes             map[uint16]AxisInfo
}

// NativeControl associates a device-local control with Linux event codes.
// Several native controls may contribute to one normalized hat control.
type NativeControl struct {
	ID   string
	Type uint16
	Code uint16
}

// AxisInfo retains the kernel's absolute-axis information at capture opening.
type AxisInfo struct {
	Value      int32
	Minimum    int32
	Maximum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}
