package goinput

import (
	"context"
	"reflect"
	"slices"

	"fmt"
	"math"

	"github.com/gostafa/goinput/internal/application"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/implementation"
)

// HID constructs a usage from its standard page and ID.
func HID(page, id uint16) Usage { return Usage(uint32(page)<<16 | uint32(id)) }

func (u Usage) Page() uint16   { return uint16(uint32(u) >> 16) }
func (u Usage) ID() uint16     { return uint16(u) }
func (u Usage) String() string { return fmt.Sprintf("%04x:%04x", u.Page(), u.ID()) }

// Normalize explicitly scales a logical absolute axis into [0,1]. It does not
// infer a centered axis, deadzone, or physical unit. Invalid/null values, relative
// axes, hats, and unknown or degenerate ranges return false.
func (c Control) Normalize(value float64) (float64, bool) {
	if c.Kind != ControlAxis || c.Mode != AxisAbsolute || c.Range == nil ||
		c.Range.Max <= c.Range.Min || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	lo, hi := float64(c.Range.Min), float64(c.Range.Max)
	if hi <= lo || value < lo || value > hi {
		return 0, false
	}
	return (value - lo) / (hi - lo), true
}

func (e *OpError) Error() string {
	if e.DeviceID == "" {
		return fmt.Sprintf("goinput: %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("goinput: %s %s: %v", e.Op, e.DeviceID, e.Err)
}

func (e *OpError) Unwrap() error { return e.Err }

// New creates a manager without opening devices or starting native resources.
// BufferSize zero selects 256 events per device.
func New(options Options) (*Manager, error) {
	impl, err := implementation.New(domain.Options(options))
	if err != nil {
		return nil, publicError(err)
	}
	return &Manager{impl: impl}, nil
}

// Devices returns accessible endpoints together with any partial discovery diagnostics.
func (m *Manager) Devices(ctx context.Context) ([]DeviceInfo, error) {
	infos, err := m.impl.Devices(ctx)
	var result []DeviceInfo
	if infos != nil {
		result = make([]DeviceInfo, len(infos))
	}
	for i, info := range infos {
		result[i] = publicInfo(info)
	}
	return result, publicError(err)
}

// Open starts an independent event stream.
func (m *Manager) Open(ctx context.Context, id DeviceID) (Device, error) {
	impl, err := m.impl.Open(ctx, domain.DeviceID(id))
	if err != nil {
		return nil, publicError(err)
	}
	return &device{impl: impl}, nil
}

// Close cancels pending operations, closes captures, and releases the session lease.
func (m *Manager) Close() error              { return publicError(m.impl.Close()) }
func (d *device) Info() DeviceInfo           { return publicInfo(d.impl.Info()) }
func (d *device) Capabilities() Capabilities { return publicCapabilities(d.impl.Capabilities()) }
func (d *device) Read(ctx context.Context) (Event, error) {
	e, err := d.impl.Read(ctx)
	return Event{DeviceID: DeviceID(e.DeviceID), ControlID: ControlID(e.ControlID),
		Action: EventAction(e.Action), Value: e.Value,
		Timestamp: Timestamp{Time: e.Timestamp.Time, ReceivedAt: e.Timestamp.ReceivedAt, Source: TimestampSource(e.Timestamp.Source)}}, publicError(err)
}
func (d *device) Close() error { return publicError(d.impl.Close()) }
func (d *device) Extension(target any) bool {
	provider, ok := d.impl.(application.ExtensionProvider)
	return ok && provider.Extension(target)
}

func publicInfo(i domain.DeviceInfo) DeviceInfo {
	r := DeviceInfo{ID: DeviceID(i.ID), Name: i.Name, Path: i.Path, Manufacturer: i.Manufacturer,
		Serial: i.Serial, Transport: Transport(i.Transport)}
	if i.VendorID != nil {
		v := *i.VendorID
		r.VendorID = &v
	}
	if i.ProductID != nil {
		v := *i.ProductID
		r.ProductID = &v
	}
	if i.Classes != nil {
		r.Classes = make([]DeviceClass, len(i.Classes))
	}
	for n, c := range i.Classes {
		r.Classes[n] = DeviceClass(c)
	}
	return r
}
func publicCapabilities(c domain.Capabilities) Capabilities {
	r := Capabilities{Complete: c.Complete, Repeat: Support(c.Repeat)}
	if c.Controls != nil {
		r.Controls = make([]Control, len(c.Controls))
	}
	for i, v := range c.Controls {
		r.Controls[i] = Control{ID: ControlID(v.ID), Name: v.Name, Kind: ControlKind(v.Kind), Usage: Usage(v.Usage),
			Mapping: MappingSource(v.Mapping), Mode: AxisMode(v.Mode), Unit: Unit(v.Unit), Support: Support(v.Support)}
		if v.Range != nil {
			r.Controls[i].Range = &Range{Min: v.Range.Min, Max: v.Range.Max}
		}
	}
	return r
}

func (e *translatedError) Error() string   { return e.original.Error() }
func (e *translatedError) Unwrap() []error { return slices.Clone(e.children) }
func (e *translatedError) Is(target error) bool {
	matcher, ok := e.original.(interface{ Is(error) bool })
	return sameError(e.original, target) || ok && matcher.Is(target)
}
func (e *translatedError) As(target any) bool {
	matcher, ok := e.original.(interface{ As(any) bool })
	return ok && matcher.As(target)
}
func publicError(err error) error {
	if err == nil {
		return nil
	}
	switch err {
	case domain.ErrUnsupported:
		return ErrUnsupported
	case domain.ErrPermissionDenied:
		return ErrPermissionDenied
	case domain.ErrNotFound:
		return ErrNotFound
	case domain.ErrClosed:
		return ErrClosed
	case domain.ErrDisconnected:
		return ErrDisconnected
	case domain.ErrEventLoss:
		return ErrEventLoss
	case domain.ErrRegistrationConflict:
		return ErrRegistrationConflict
	case domain.ErrInvalidOptions:
		return ErrInvalidOptions
	}
	if e, ok := err.(*domain.OpError); ok {
		return &OpError{Op: e.Op, DeviceID: DeviceID(e.DeviceID), Err: publicError(e.Err)}
	}
	var children []error
	switch e := err.(type) {
	case interface{ Unwrap() []error }:
		children = e.Unwrap()
	case interface{ Unwrap() error }:
		children = []error{e.Unwrap()}
	default:
		return err
	}
	changed := false
	converted := make([]error, len(children))
	for i, c := range children {
		converted[i] = publicError(c)
		changed = changed || !sameError(converted[i], c)
	}
	if !changed {
		return err
	}
	return &translatedError{original: err, children: converted}
}

func sameError(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return reflect.TypeOf(a).Comparable() && a == b
}
