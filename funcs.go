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
	"github.com/gostafa/goinput/internal/platform"
	"github.com/gostafa/goinput/internal/ports"
)

// HID constructs a usage from its standard page and ID.
func HID(page, id uint16) Usage { return Usage(domain.HID(page, id)) }

// Page returns the HID usage page.
func (u Usage) Page() uint16 { return domain.Usage(u).Page() }

// ID returns the usage ID within its HID page.
func (u Usage) ID() uint16 { return domain.Usage(u).ID() }

func (u Usage) String() string { return fmt.Sprintf("%04x:%04x", u.Page(), u.ID()) }

// New creates a manager without opening devices or starting native resources.
// BufferSize zero selects 256 events per device.
func New(ctx context.Context, options Options, system *System) (*Manager, error) {
	if ctx == nil || system == nil || system.provider == nil {
		return nil, ErrInvalidOptions
	}

	impl, err := implementation.New(ctx, options, system.provider)
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
		result = slices.Grow([]DeviceInfo{}, len(infos))
	}

	for i := range infos {
		result = append(result, publicInfo(&infos[i]))
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
		result.Controls = slices.Grow([]Control{}, len(capabilities.Controls))
	}

	for i := range capabilities.Controls {
		result.Controls = append(result.Controls, publicControl(&capabilities.Controls[i]))
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

	operation, ok := errors.AsType[*domain.OpError](err)
	if ok && sameError(err, operation) {
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

	return reflect.TypeOf(a).Comparable() && reflect.ValueOf(a).Equal(reflect.ValueOf(b))
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

	result := slices.Grow([]DeviceClass{}, len(classes))
	for index := range classes {
		result = append(result, DeviceClass(classes[index]))
	}

	return result
}

func publicSentinel(err error) error {
	pairs := []struct{ internal, public error }{
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
		if sameError(err, pairs[index].internal) {
			return pairs[index].public
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
	converted := slices.Grow([]error{}, len(children))
	changed := false

	for index := range children {
		converted = append(converted, publicError(children[index]))
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

// NewSystem creates a reusable input system without opening native resources.
// The context bounds creation; it does not control subsequent manager lifetimes.
func NewSystem(ctx context.Context) (*System, error) {
	if ctx == nil {
		return nil, ErrInvalidOptions
	}

	provider, err := implementation.NewProvider(platform.Factory)

	err = errors.Join(err, context.Cause(ctx))

	system, err := systemFromProvider(provider, err)
	if err != nil {
		return nil, errors.Join(err)
	}

	return system, nil
}

func systemFromProvider(
	provider ports.Provider[*application.Coordinator],
	err error,
) (*System, error) {
	if err != nil {
		return nil, errors.Join(publicError(err))
	}

	return &System{provider: provider}, nil
}

func newManagerView(impl managerImpl) *Manager {
	view := new(Manager)

	view.Operations.Devices = func(ctx context.Context) ([]DeviceInfo, error) { return managerDevices(ctx, impl) }
	configureManagerOpen(view, impl)

	view.Operations.Close = func() error { return managerClose(impl) }

	return view
}

func newDeviceView(impl application.Device) *device {
	view := new(device)

	view.Operations.Info = func() DeviceInfo { return deviceInfo(impl) }
	view.Operations.Capabilities = func() Capabilities { return deviceCapabilities(impl) }
	view.Operations.Read = func(ctx context.Context) (Event, error) { return deviceRead(ctx, impl) }
	configureDeviceClose(view, impl)

	return view
}

func configureDeviceClose(view *device, impl application.Device) {
	view.Operations.Extension = func(target any) bool { return deviceExtension(impl, target) }
	view.Operations.Close = func() error { return deviceClose(impl) }
}

func configureManagerOpen(view *Manager, impl managerImpl) {
	view.Operations.Open = func(ctx context.Context, id DeviceID) (Device, error) {
		opened, err := managerOpen(ctx, impl, id)
		if err != nil {
			return nil, errors.Join(err)
		}

		return opened, nil
	}
}
