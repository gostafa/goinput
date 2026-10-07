// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/gostafa/goinput/internal/application"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/implementation"
)

const (
	normalizedMinimum = 0
	errorPairSize     = 2
	publicErrorIndex  = 1
)

// HID constructs a usage from its standard page and ID.
func HID(page, id uint16) Usage { return Usage(uint32(page)<<16 | uint32(id)) }

// Page returns the HID usage page.
func (u Usage) Page() uint16 { return uint16(uint32(u) >> 16) }

// ID returns the usage ID within its HID page.
func (u Usage) ID() uint16 { return uint16(u) }

func (u Usage) String() string { return fmt.Sprintf("%04x:%04x", u.Page(), u.ID()) }

// New creates a manager without opening devices or starting native resources.
// BufferSize zero selects 256 events per device.
func New(options Options, system *System) (*Manager, error) {
	if system == nil || system.provider == nil {
		return nil, ErrInvalidOptions
	}

	impl, err := implementation.New(options, system.provider)
	if err != nil {
		return nil, errors.Join(publicError(err))
	}

	return newManagerView(application.ManagerView(impl)), nil
}

// Devices returns accessible endpoints together with any partial discovery diagnostics.
func managerDevices(ctx context.Context, impl managerImpl) ([]DeviceInfo, error) {
	infos, err := impl.Devices(ctx)

	var result []DeviceInfo

	if infos != nil {
		result = make([]DeviceInfo, len(infos))
	}

	for i := range infos {
		result[i] = publicInfo(&infos[i])
	}

	return result, errors.Join(publicError(err))
}

// Open starts an independent event stream.
func managerOpen(ctx context.Context, impl managerImpl, id DeviceID) (*device, error) {
	opened, err := impl.Open(ctx, domain.DeviceID(id))
	if err != nil {
		return nil, errors.Join(publicError(err))
	}

	return newDeviceView(opened), nil
}

// Close cancels pending operations, closes captures, and releases the session lease.
func managerClose(impl managerImpl) error { return errors.Join(publicError(impl.Close())) }

func deviceInfo(impl application.Device) DeviceInfo {
	info := impl.Info()

	return publicInfo(&info)
}

func deviceCapabilities(impl application.Device) Capabilities {
	capabilities := impl.Capabilities()

	return publicCapabilities(&capabilities)
}

func deviceRead(ctx context.Context, impl application.Device) (Event, error) {
	event, err := impl.Read(ctx)

	return Event{
		DeviceID:  DeviceID(event.DeviceID),
		ControlID: ControlID(event.ControlID),
		Action:    EventAction(event.Action),
		Value:     event.Value,
		Timestamp: Timestamp{
			Time:       event.Timestamp.Time,
			ReceivedAt: event.Timestamp.ReceivedAt,
			Source:     TimestampSource(event.Timestamp.Source),
		},
	}, errors.Join(publicError(err))
}

func deviceClose(impl application.Device) error { return errors.Join(publicError(impl.Close())) }

func deviceExtension(impl application.Device, target any) bool {
	provider, ok := impl.(application.ExtensionProvider)

	return ok && provider.Extension(target)
}

func publicInfo(info *domain.DeviceInfo) DeviceInfo {
	return DeviceInfo{
		ID: DeviceID(info.ID), Name: info.Name, Path: info.Path, Manufacturer: info.Manufacturer,
		Serial: info.Serial, Transport: Transport(info.Transport),
		VendorID: clonePointer(info.VendorID), ProductID: clonePointer(info.ProductID),
		Classes: publicClasses(info.Classes),
	}
}

func publicCapabilities(capabilities *domain.Capabilities) Capabilities {
	result := Capabilities{
		Controls: nil,
		Complete: capabilities.Complete,
		Repeat:   Support(capabilities.Repeat),
	}
	if capabilities.Controls != nil {
		result.Controls = make([]Control, len(capabilities.Controls))
	}

	for i := range capabilities.Controls {
		result.Controls[i] = publicControl(&capabilities.Controls[i])
	}

	return result
}

// Error retains the original wrapper's error text.
func (e *translatedError) Error() string { return e.operations.message() }

// Unwrap exposes independently copied translated children.
func (e *translatedError) Unwrap() []error { return e.operations.unwrap() }

// Is retains custom matching from the original wrapper.
func (e *translatedError) Is(target error) bool { return e.operations.match(target) }

// As retains custom assignment from the original wrapper.
func (e *translatedError) As(target any) bool { return e.operations.assign(target) }

func originalMatches(original, target error) bool {
	matcher, ok := original.(domain.ErrorMatcher)

	return sameError(original, target) || ok && matcher.Is(target)
}

func originalAssigns(original error, target any) bool {
	matcher, ok := original.(domain.ErrorAssigner)

	return ok && matcher.As(target)
}

func publicError(err error) error {
	if err == nil {
		return nil
	}

	translated := publicSentinel(err)
	if translated != nil {
		return errors.Join(translated)
	}

	if operation, ok := errors.AsType[*domain.OpError](err); ok {
		return &OpError{
			Op:       operation.Op,
			DeviceID: DeviceID(operation.DeviceID),
			Err:      publicError(operation.Err),
		}
	}

	return errors.Join(publicWrappedError(err))
}

func sameError(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return reflect.TypeOf(a).Comparable() && errors.Is(a, b)
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}

	copyValue := *value

	return &copyValue
}

func publicClasses(classes []domain.DeviceClass) []DeviceClass {
	if classes == nil {
		return nil
	}

	result := make([]DeviceClass, len(classes))
	for index := range classes {
		result[index] = DeviceClass(classes[index])
	}

	return result
}

func publicSentinel(err error) error {
	pairs := [][errorPairSize]error{
		{domain.ErrUnsupported, ErrUnsupported},
		{domain.ErrPermissionDenied, ErrPermissionDenied},
		{domain.ErrNotFound, ErrNotFound},
		{domain.ErrClosed, ErrClosed},
		{domain.ErrDisconnected, ErrDisconnected},
		{domain.ErrEventLoss, ErrEventLoss},
		{domain.ErrRegistrationConflict, ErrRegistrationConflict},
		{domain.ErrInvalidOptions, ErrInvalidOptions},
	}
	for index := range pairs {
		if errors.Is(err, pairs[index][0]) {
			return pairs[index][publicErrorIndex]
		}
	}

	return nil
}

func errorChildren(err error) []error {
	var (
		multiple domain.ErrorChildren
		single   domain.ErrorUnwrapper
	)

	switch {
	case errors.As(err, &multiple):
		return multiple.Unwrap()
	case errors.As(err, &single):
		return []error{single.Unwrap()}
	default:
		return nil
	}
}

func publicWrappedError(err error) error {
	children := errorChildren(err)
	converted := make([]error, len(children))
	changed := false

	for index := range children {
		converted[index] = publicError(children[index])
		changed = changed || !sameError(converted[index], children[index])
	}

	if !changed {
		return err
	}

	return errors.Join(newTranslatedError(err, converted))
}

func newTranslatedError(err error, converted []error) error {
	wrapper := new(translatedError)

	wrapper.operations.message = err.Error
	wrapper.operations.unwrap = func() []error { return slices.Clone(converted) }
	wrapper.operations.match = func(target error) bool { return originalMatches(err, target) }
	wrapper.operations.assign = func(target any) bool { return originalAssigns(err, target) }

	return wrapper
}

func publicControl(control *domain.Control) Control {
	result := Control{
		Range: nil, ID: ControlID(control.ID), Name: control.Name,
		Kind: ControlKind(control.Kind), Usage: Usage(control.Usage),
		Mapping: MappingSource(control.Mapping), Mode: AxisMode(control.Mode),
		Unit: Unit(control.Unit), Support: Support(control.Support),
	}
	if control.Range != nil {
		result.Range = &Range{Min: control.Range.Min, Max: control.Range.Max}
	}

	return result
}
