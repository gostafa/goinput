package goinput

import (
	"context"
	"time"

	"github.com/gostafa/goinput/internal/application"
	"github.com/gostafa/goinput/internal/domain"
)

// DeviceID identifies a current operating-system input endpoint.
// IDs may be reused after disconnection and are not persistent hardware IDs.
type DeviceID string

// ControlID identifies a control within a device, independently of its HID usage.
type ControlID string

// Usage packs a HID usage page in its upper 16 bits and a usage ID in its lower 16.
// Zero means no known usage. Vendor-defined usages are preserved.
type Usage uint32

type DeviceClass uint8
type Transport uint8
type ControlKind uint8
type AxisMode uint8
type MappingSource uint8
type Unit uint8
type Support uint8
type EventAction uint8
type TimestampSource uint8
type HatDirection int8

// Options configures a manager. A zero BufferSize selects the default capacity.
type Options struct {
	BufferSize int
}

// DeviceInfo describes one endpoint, which can have multiple device classes.
// Empty strings and nil identifiers mean unavailable metadata, not inferred data.
type DeviceInfo struct {
	ID           DeviceID
	Name         string
	Path         string
	Manufacturer string
	Serial       string
	VendorID     *uint16
	ProductID    *uint16
	Transport    Transport
	Classes      []DeviceClass
}

// Range is the inclusive logical range of an absolute control.
type Range struct {
	Min int64
	Max int64
}

// Control describes a device-local input. Range is nil when unavailable.
// Usage is semantic identity; it need not uniquely identify a control.
type Control struct {
	ID      ControlID
	Name    string
	Kind    ControlKind
	Usage   Usage
	Mapping MappingSource
	Mode    AxisMode
	Range   *Range
	Unit    Unit
	Support Support
}

// Capabilities is a snapshot of discoverable input controls. Complete means the
// backend obtained the entire supported input schema; it does not imply that every
// listed control can be decoded. Consult each control's Support field.
type Capabilities struct {
	Controls []Control
	Complete bool
	Repeat   Support
}

// Timestamp distinguishes source event time from host receipt time. Time is UTC;
// ReceivedAt comes from time.Now and retains its monotonic component. Device
// timestamps are not guaranteed monotonic or comparable between devices.
type Timestamp struct {
	Time       time.Time
	ReceivedAt time.Time
	Source     TimestampSource
}

// Event is one control update. Digital values are 0 or 1; absolute values are
// logical positions, relative values are deltas, and hats use HatDirection.
// Values use float64 for fractional scrolling. Backends mark native controls
// wider than their exact scalar decoding support as unsupported rather than round.
type Event struct {
	DeviceID  DeviceID
	ControlID ControlID
	Action    EventAction
	Value     float64
	Timestamp Timestamp
}

// OpError adds operation and device context while preserving an underlying cause.
type OpError struct {
	Op       string
	DeviceID DeviceID
	Err      error
}

// Manager owns input captures. Construct with New; do not copy or use its zero value.
type Manager struct{ impl managerImpl }
type managerImpl interface {
	Devices(context.Context) ([]domain.DeviceInfo, error)
	Open(context.Context, domain.DeviceID) (application.Device, error)
	Close() error
}

// Device is a concurrency-safe consumer whose readers share one ordered queue.
type Device interface {
	Info() DeviceInfo
	Capabilities() Capabilities
	Read(context.Context) (Event, error)
	Close() error
}

// ExtensionProvider queries native metadata through a pointer to a supported struct.
type ExtensionProvider interface{ Extension(target any) bool }
type device struct{ impl application.Device }

// translatedError retains a wrapper's text and custom matching while exposing
// translated children to errors.Is and errors.As.
type translatedError struct {
	original error
	children []error
}
