// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gostafa/goinput/internal/application"
	"github.com/gostafa/goinput/internal/domain"
)

type (
	noncomparableError []byte
)

func (noncomparableError) Error() string { return "noncomparable cause" }

func TestOperationTranslationPreservesKnownCause(t *testing.T) {
	t.Parallel()

	original := &domain.OpError{
		Op:       testOpenOperation,
		DeviceID: viewDeviceID,
		Err:      domain.ErrClosed,
	}
	translated := publicError(original)
	operation, ok := errors.AsType[*OpError](translated)
	checkEqual(t, ok, true)
	checkEqual(t, operation.Op, original.Op)
	checkEqual(t, operation.DeviceID, DeviceID(viewDeviceID))
	checkCause(t, translated, ErrClosed)
}

func TestWrappedErrorTranslationRetainsText(t *testing.T) {
	t.Parallel()

	original := fmt.Errorf("capture: %w", domain.ErrClosed)
	translated := publicError(original)
	checkEqual(t, translated.Error(), original.Error())
	checkCause(t, translated, ErrClosed)

	joined := publicError(errors.Join(original, errDriverFailed))
	checkCause(t, joined, ErrClosed)
	checkCause(t, joined, errDriverFailed)
}

func TestTranslatedChildrenAreIndependent(t *testing.T) {
	t.Parallel()

	translated := newTranslatedError(errDriverFailed, []error{ErrClosed})
	wrapper, ok := errors.AsType[*translatedError](translated)
	checkEqual(t, ok, true)

	children := wrapper.Unwrap()

	children[normalizedMinimum] = ErrDisconnected

	checkCause(t, wrapper, ErrClosed)
	checkEqual(t, errors.Is(wrapper, ErrDisconnected), false)
	checkEqual(t, wrapper.As(new(bool)), false)
}

func TestTranslatedErrorRetainsCustomMatching(t *testing.T) {
	t.Parallel()

	translated := newTranslatedError(matchingError(), nil)

	var assigned noncomparableError

	checkCause(t, translated, ErrEventLoss)
	checkEqual(t, errors.As(translated, &assigned), true)
	checkEqual(t, len(assigned), normalizedMaximum)
}

func matchingError() *translatedError {
	original := new(translatedError)

	original.operations.message = errDriverFailed.Error
	original.operations.match = func(target error) bool { return errors.Is(target, ErrEventLoss) }
	original.operations.assign = assignNoncomparable

	return original
}

func assignNoncomparable(target any) bool {
	value, ok := target.(*noncomparableError)
	if ok {
		*value = noncomparableError{byte(normalizedMaximum)}
	}

	return ok
}

func TestErrorIdentityHandlesNil(t *testing.T) {
	t.Parallel()
	checkEqual(t, sameError(nil, nil), true)
	checkEqual(t, sameError(nil, errDriverFailed), false)
	checkEqual(t, sameError(errDriverFailed, nil), false)
	checkEqual(t, publicError(nil), error(nil))
}

func TestErrorIdentityHandlesNoncomparableErrors(t *testing.T) {
	t.Parallel()

	cause := noncomparableError{byte(normalizedMaximum)}
	checkEqual(t, sameError(cause, cause), false)
	checkEqual(t, originalAssigns(errDriverFailed, new(bool)), false)
	checkEqual(t, assignNoncomparable(new(bool)), false)
	checkEqual(t, publicError(cause).Error(), cause.Error())
}

var errDriverFailed = errors.New("driver failed")

func TestMissingDeviceClassesStayNil(t *testing.T) {
	t.Parallel()

	if publicClasses(nil) != nil {
		t.Fatal("missing device classes became an allocated slice")
	}
}

const (
	testOpenOperation  = "open"
	normalizedMinimum  = 0
	errorPairSize      = 2
	normalizedMidpoint = 0.5
	normalizedMaximum  = 1
	hatTestMaximum     = 7
	axisTestLimit      = 10
	axisOutsideLimit   = 11
)

func TestPublicErrorSentinels(t *testing.T) {
	t.Parallel()

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
		checkPublicSentinel(t, pairs[index][0], pairs[index][1])
	}
}

func TestPublicErrorPreservesUnknownCause(t *testing.T) {
	t.Parallel()

	cause := errDriverFailed
	original := &domain.OpError{Op: testOpenOperation, DeviceID: "test-device", Err: cause}
	translated := publicError(original)

	var operation *OpError

	if !errors.As(translated, &operation) || !errors.Is(translated, cause) {
		t.Fatalf("operation and cause were not preserved: %v", translated)
	}

	if operation.Op != original.Op || operation.DeviceID != DeviceID(original.DeviceID) {
		t.Fatalf("operation metadata changed: %+v", operation)
	}
}

func checkPublicSentinel(t *testing.T, original, expected error) {
	t.Helper()

	got := publicError(original)
	if !errors.Is(got, expected) {
		t.Fatalf("publicError(%v) = %v, want %v", original, got, expected)
	}
}

func TestSystemPreservesProviderFailure(t *testing.T) {
	t.Parallel()

	system, err := systemFromProvider(nil, domain.ErrUnsupported)
	if system != nil || !errors.Is(err, ErrUnsupported) {
		t.Fatalf("provider failure = (%v, %v)", system, err)
	}
}

const (
	viewDeviceID  = "view-device"
	viewControlID = "view-control"
)

func TestDeviceViewCopiesMetadata(t *testing.T) {
	t.Parallel()

	native := testNativeDevice()
	info := newDeviceView(native).Info()

	info.Classes[normalizedMinimum] = ClassUnknown
	*info.VendorID = normalizedMinimum
	checkEqual(t, native.Info().Classes[normalizedMinimum], domain.ClassGamepad)
	checkEqual(t, *native.Info().VendorID, uint16(normalizedMaximum))
}

func TestDeviceViewCopiesControlRanges(t *testing.T) {
	t.Parallel()

	native := testNativeDevice()
	capabilities := newDeviceView(native).Capabilities()

	capabilities.Controls[normalizedMinimum].Range.Min = axisOutsideLimit
	checkEqual(
		t,
		native.Capabilities().Controls[normalizedMinimum].Range.Min,
		int64(normalizedMinimum),
	)
}

func TestDeviceViewReadTranslatesErrors(t *testing.T) {
	t.Parallel()

	event, err := newDeviceView(testNativeDevice()).Read(t.Context())
	checkEqual(t, event.DeviceID, DeviceID(viewDeviceID))
	checkEqual(t, event.ControlID, ControlID(viewControlID))
	checkCause(t, err, ErrDisconnected)
}

func TestDeviceViewCloseAndExtension(t *testing.T) {
	t.Parallel()

	view := newDeviceView(testNativeDevice())
	checkCause(t, view.Close(), ErrClosed)
	checkEqual(t, view.Extension(new(bool)), true)
}

func TestManagerViewDiscoversDevices(t *testing.T) {
	t.Parallel()

	infos, err := newManagerView(testNativeManager()).Devices(t.Context())
	checkEqual(t, err, error(nil))
	checkEqual(t, len(infos), normalizedMaximum)
	checkEqual(t, infos[normalizedMinimum].ID, DeviceID(viewDeviceID))
}

func TestManagerViewOpensAndCloses(t *testing.T) {
	t.Parallel()

	view := newManagerView(testNativeManager())
	opened, err := view.Open(t.Context(), viewDeviceID)
	checkEqual(t, err, error(nil))
	checkEqual(t, opened.Info().ID, DeviceID(viewDeviceID))
	checkCause(t, view.Close(), ErrClosed)
}

func TestManagerViewPreservesDiscoveryDiagnostics(t *testing.T) {
	t.Parallel()

	native := testNativeManager()

	native.Operations.Devices = func(context.Context) ([]domain.DeviceInfo, error) {
		return nil, domain.ErrPermissionDenied
	}

	infos, err := newManagerView(native).Devices(t.Context())
	checkEqual(t, len(infos), normalizedMinimum)
	checkCause(t, err, ErrPermissionDenied)
}

func TestManagerViewReturnsNilOnOpenFailure(t *testing.T) {
	t.Parallel()

	native := testNativeManager()

	native.Operations.Open = func(context.Context, domain.DeviceID) (application.Device, error) {
		return nil, domain.ErrNotFound
	}

	opened, err := newManagerView(native).Open(t.Context(), viewDeviceID)
	checkEqual(t, opened, Device(nil))
	checkCause(t, err, ErrNotFound)
}

func checkEqual[Value comparable](t *testing.T, actual, expected Value) {
	t.Helper()

	if actual != expected {
		t.Fatalf("got %v, want %v", actual, expected)
	}
}

func checkCause(t *testing.T, actual, expected error) {
	t.Helper()

	if !errors.Is(actual, expected) {
		t.Fatalf("got %v, want cause %v", actual, expected)
	}
}

func testNativeDevice() *deviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event] {
	view := new(deviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event])
	configureNativeSnapshots(view)

	view.Operations.Read = func(context.Context) (domain.Event, error) {
		return domain.Event{
			DeviceID:  viewDeviceID,
			ControlID: viewControlID,
			Action:    domain.ActionUnknown,
			Value:     normalizedMinimum,
			Timestamp: domain.Timestamp{
				Time:       time.Time{},
				ReceivedAt: time.Time{},
				Source:     domain.TimestampUnknown,
			},
		}, domain.ErrDisconnected
	}
	view.Operations.Close = func() error { return domain.ErrClosed }
	view.Operations.Extension = func(any) bool { return true }

	return view
}

func testNativeManager() *managerOperations[domain.DeviceInfo, domain.DeviceID, application.Device] {
	view := new(managerOperations[domain.DeviceInfo, domain.DeviceID, application.Device])

	view.Operations.Devices = func(context.Context) ([]domain.DeviceInfo, error) {
		return []domain.DeviceInfo{nativeViewInfo()}, nil
	}
	view.Operations.Open = func(context.Context, domain.DeviceID) (application.Device, error) {
		return testNativeDevice(), nil
	}
	view.Operations.Close = func() error { return domain.ErrClosed }

	return view
}

func configureNativeSnapshots(
	view *deviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event],
) {
	info := nativeViewInfo()
	caps := nativeViewCapabilities()

	view.Operations.Info = func() domain.DeviceInfo { return info }
	view.Operations.Capabilities = func() domain.Capabilities { return caps }
}

func nativeViewInfo() domain.DeviceInfo {
	vendor := uint16(normalizedMaximum)

	return domain.DeviceInfo{
		ID:           viewDeviceID,
		VendorID:     &vendor,
		Classes:      []domain.DeviceClass{domain.ClassGamepad},
		Transport:    domain.TransportUnknown,
		ProductID:    nil,
		Name:         "",
		Path:         "",
		Manufacturer: "",
		Serial:       "",
	}
}

func nativeViewCapabilities() domain.Capabilities {
	control := new(domain.Control)

	control.ID, control.Range = viewControlID, &domain.Range{
		Min: normalizedMinimum,
		Max: axisTestLimit,
	}

	return domain.Capabilities{
		Controls: []domain.Control{*control},
		Repeat:   domain.SupportUnknown,
		Complete: false,
	}
}
