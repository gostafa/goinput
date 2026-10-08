// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput

import (
	"context"

	"github.com/gostafa/goinput/internal/application"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	deviceview "github.com/gostafa/goinput/internal/ports/device"
	managerview "github.com/gostafa/goinput/internal/ports/manager"
)

type (
	// DeviceClass classifies an input endpoint.
	DeviceClass uint8
	// Transport identifies a device connection medium.
	Transport uint8
	// ControlKind classifies a device control.
	ControlKind uint8
	// AxisMode distinguishes absolute and relative axes.
	AxisMode uint8
	// MappingSource identifies how a control usage was obtained.
	MappingSource uint8
)

type (
	// Unit describes the interpretation of a control value.
	Unit uint8
	// Support reports whether a capability is supported.
	Support uint8
	// EventAction describes a control transition.
	EventAction uint8
	// TimestampSource identifies the origin of an event timestamp.
	TimestampSource uint8
	// HatDirection encodes a compass direction or neutral hat position.
	HatDirection int8
)

type (
	// Timestamp distinguishes source event time from host receipt time.
	// Device timestamps need not be monotonic or comparable between devices.
	Timestamp = domain.TimestampRecord[TimestampSource]
	// Event is a control update: digital values are 0/1, axes carry positions or deltas,
	// and hats use HatDirection. Values retain fractional scrolling.
	Event = domain.EventRecord[Timestamp, DeviceID, ControlID, EventAction]
	// OpError preserves a cause with operation and device context.
	OpError = domain.OperationError[DeviceID]
)

type (
	// DeviceID identifies a current operating-system input endpoint.
	// IDs may be reused after disconnection and are not persistent hardware IDs.
	DeviceID string

	// ControlID identifies a control within a device, independently of its HID usage.
	ControlID string

	// Usage packs a HID usage page in its upper 16 bits and a usage ID in its lower 16.
	// Zero means no known usage. Vendor-defined usages are preserved.
	Usage uint32
)

type (
	// System owns the shared native session and its callback bindings. Construct with
	// NewSystem and reuse across managers; its zero value is not usable.
	System = systemProvider[ports.Provider[*application.Coordinator]]

	systemProvider[Provider any] struct {
		provider Provider
	}

	// Manager owns input captures. Construct with New; do not copy or use its zero value.
	Manager     = managerOperations[DeviceInfo, DeviceID, Device]
	managerImpl interface {
		Devices(ctx context.Context) ([]domain.DeviceInfo, error)
		Open(ctx context.Context, id domain.DeviceID) (application.Device, error)
		Close() error
	}

	// Device is a concurrency-safe consumer whose readers share one ordered queue.
	Device interface {
		Info() DeviceInfo
		Capabilities() Capabilities
		Read(ctx context.Context) (Event, error)
		Close() error
	}

	// ExtensionProvider queries native metadata through a pointer to a supported struct.
	ExtensionProvider interface{ Extension(target any) bool }
	device            = deviceOperations[DeviceInfo, Capabilities, Event]

	// translatedError retains a wrapper's text and custom matching while exposing
	// translated children to errors.Is and errors.As.
	translatedError struct {
		operations errorDispatch
	}

	errorDispatch struct {
		message func() string
		unwrap  func() []error
		match   func(error) bool
		assign  func(any) bool
	}
)

type (
	deviceOperations[I, C, E any]   = deviceview.Operations[I, C, E]
	managerOperations[I, ID, D any] = managerview.Operations[I, ID, D]
)

type (
	// Options configures a manager. Zero BufferSize selects the default capacity.
	Options = domain.Options
	// Range is the inclusive logical range of an absolute control.
	Range = domain.Range
	// DeviceInfo describes one endpoint; empty metadata means unavailable data.
	DeviceInfo = domain.DeviceInfoRecord[DeviceID, DeviceClass, Transport]
	// Control describes a device-local input. Range is nil when unavailable.
	Control = domain.ControlRecord[Range, ControlID, Usage, ControlKind, MappingSource, AxisMode, Unit, Support]
	// Capabilities is a snapshot of discoverable input controls.
	Capabilities = domain.CapabilitiesRecord[Control, Support]
)
