// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"context"
	"time"
)

type (
	// DeviceClass classifies an input endpoint.
	DeviceClass = code[interface{ deviceClass() }]
	// Transport identifies a device connection medium.
	Transport = code[interface{ transport() }]
	// ControlKind classifies a device control.
	ControlKind = code[interface{ controlKind() }]
	// AxisMode distinguishes absolute and relative axes.
	AxisMode = code[interface{ axisMode() }]
	// MappingSource identifies how a control usage was obtained.
	MappingSource = code[interface{ mappingSource() }]

	// code retains nominal enum identity through a distinct, non-instantiated tag.
	code[Tag any] uint8
)

type (
	// Discoverer enumerates accessible endpoints with partial diagnostics.
	Discoverer[I any] interface {
		Discover(ctx context.Context) ([]I, error)
	}
	// Opener subscribes a sink to a native device.
	Opener[ID, S, C any] interface {
		Open(ctx context.Context, id ID, sink S) (C, error)
	}
	// DeviceOpener creates an independent event consumer.
	DeviceOpener[ID, D any] interface {
		Open(ctx context.Context, id ID) (D, error)
	}
	// Provider supplies a shared value with context-bounded initialization.
	Provider[T any] interface {
		Get(ctx context.Context) (T, error)
	}
	// Retrier bounds transient operations by context and policy.
	Retrier interface {
		Do(
			ctx context.Context,
			operation func(context.Context) error,
			transient func(error) bool,
		) error
	}
)

type (
	// ErrorMatcher supports custom errors.Is matching.
	ErrorMatcher interface{ Is(target error) bool }
	// ErrorAssigner supports custom errors.As assignment.
	ErrorAssigner interface{ As(target any) bool }
	// ErrorUnwrapper exposes a single underlying cause.
	ErrorUnwrapper interface{ Unwrap() error }
	// ErrorChildren exposes joined underlying causes.
	ErrorChildren interface{ Unwrap() []error }
	// ErrorReporter formats an error for a caller.
	ErrorReporter interface{ Error() string }
)

type (
	// EventReader consumes one event from an ordered queue.
	EventReader[E any] interface {
		Read(ctx context.Context) (E, error)
	}
	// EventPublisher copies a borrowed event without blocking.
	EventPublisher[E any] interface{ Publish(event *E) bool }
	// FailureSink terminates a stream with an underlying cause.
	FailureSink interface{ Fail(err error) }
	// Closer releases a resource; repeated calls are safe.
	Closer interface{ Close() error }
	// Cloner creates an independently mutable snapshot.
	Cloner[T any] interface{ Clone() T }
)

type (
	// Unit describes the interpretation of a control value.
	Unit = code[interface{ unit() }]
	// Support reports whether a capability is supported.
	Support = code[interface{ support() }]
	// EventAction describes a control transition.
	EventAction = code[interface{ eventAction() }]
	// TimestampSource identifies the origin of an event timestamp.
	TimestampSource = code[interface{ timestampSource() }]
	// HatDirection encodes a compass direction or neutral hat position.
	HatDirection int8
)

type (
	// TimestampRecord distinguishes source time from host receipt time.
	// Time is UTC; ReceivedAt retains its monotonic clock component.
	TimestampRecord[Source ~uint8] struct {
		// Time is the event time in UTC.
		Time time.Time
		// ReceivedAt is the host receipt time, including its monotonic component.
		ReceivedAt time.Time
		// Source identifies the origin of Time.
		Source Source
	}
	// EventRecord is one typed control update. Values preserve fractional scrolling.
	EventRecord[Stamp any, Device, Control ~string, Action ~uint8] struct {
		// Timestamp records the event and receipt clocks.
		Timestamp Stamp
		// DeviceID identifies the affected endpoint.
		DeviceID Device
		// ControlID identifies the changed device-local control.
		ControlID Control
		// Action describes the control transition.
		Action Action
		// Value preserves the reported scalar and fractional scrolling.
		Value float64
	}
	// OperationError adds operation and typed device context to an underlying cause.
	OperationError[ID ~string] struct {
		// Err retains the underlying cause.
		Err error
		// DeviceID identifies the affected endpoint.
		DeviceID ID
		// Op identifies the failed operation.
		Op string
	}
	// Timestamp is the domain timestamp record.
	Timestamp = TimestampRecord[TimestampSource]
	// Event is the domain event record.
	Event = EventRecord[Timestamp, DeviceID, ControlID, EventAction]
)

type (
	// DeviceID identifies a current operating-system input endpoint.
	// IDs may be reused after disconnection and are not persistent hardware IDs.
	DeviceID = identifier[interface{ deviceID() }]

	// ControlID identifies a control within a device, independently of its HID usage.
	ControlID = identifier[interface{ controlID() }]

	// Usage packs a HID usage page in its upper 16 bits and a usage ID in its lower 16.
	// Zero means no known usage. Vendor-defined usages are preserved.
	Usage uint32

	// identifier retains distinct device and control identities in generic records.
	identifier[Tag any] string
)

type (
	// InfoProvider supplies an immutable endpoint snapshot.
	InfoProvider[I any] interface{ Info() I }
	// CapabilitiesProvider supplies an immutable control-schema snapshot.
	CapabilitiesProvider[C any] interface{ Capabilities() C }
	// MetadataProvider supplies platform-specific metadata without owning handles.
	MetadataProvider[M any] interface{ NativeInfo() M }
	// ExtensionProvider queries an optional typed native extension.
	ExtensionProvider interface{ Extension(target any) bool }
	// NativeControlProvider supplies native control mappings as a fresh slice.
	NativeControlProvider[C any] interface{ NativeControls() []C }
)

type (
	// OpError is an operation error with a domain device identifier.
	OpError = OperationError[DeviceID]
)

type (
	// Device combines endpoint snapshots with an ordered event reader.
	Device[I, C, E any] interface {
		InfoProvider[I]
		CapabilitiesProvider[C]
		EventReader[E]
		Closer
	}
	// Capture owns a native subscription and exposes snapshots and extensions.
	Capture[I, C any] interface {
		InfoProvider[I]
		CapabilitiesProvider[C]
		ExtensionProvider
		Closer
	}
	// Backend discovers endpoints and opens native captures.
	Backend[I, ID, S, C any] interface {
		Discoverer[I]
		Opener[ID, S, C]
		Closer
	}
	// EventSink accepts copied updates and terminal failures.
	EventSink[E any] interface {
		EventPublisher[E]
		FailureSink
	}
	// Manager owns independently buffered consumers.
	Manager[I, ID, D any] interface {
		Devices(ctx context.Context) ([]I, error)
		DeviceOpener[ID, D]
		Closer
	}
)

type (
	// DeviceInfo is the domain specialization of an endpoint record.
	DeviceInfo = DeviceInfoRecord[DeviceID, DeviceClass, Transport]
	// Control is the domain specialization of a control record.
	Control = ControlRecord[Range, ControlID, Usage, ControlKind, MappingSource, AxisMode, Unit, Support]
	// Capabilities is the domain specialization of a capabilities record.
	Capabilities = CapabilitiesRecord[Control, Support]
)

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

type (
	// Normalizer explicitly scales supported absolute logical axes.
	Normalizer interface {
		Normalize(value float64) (float64, bool)
	}
	// UsageProvider exposes the standard HID page and usage identifiers.
	UsageProvider interface {
		Page() uint16
		ID() uint16
	}
	// OperationFailure supplies typed operation context independently of a wrapper.
	OperationFailure[ID any] interface {
		ErrorReporter
		Operation() string
		Device() ID
	}
	// LogicalBounds supplies inclusive absolute-axis limits.
	LogicalBounds interface{ Bounds() (int64, int64) }
)
