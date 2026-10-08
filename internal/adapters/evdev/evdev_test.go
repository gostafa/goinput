// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build linux && (amd64 || arm64)

package evdev

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"testing/fstest"

	extension "github.com/gostafa/goinput/extensions/evdev"
	"github.com/gostafa/goinput/internal/domain"
	sinkview "github.com/gostafa/goinput/internal/ports/sink"
	native "github.com/holoplot/go-evdev"
)

type (
	observations = struct {
		events   []domain.Event
		failures []error
	}
	testRetrier func(context.Context, func(context.Context) error, func(error) bool) error
)

const (
	testDevicePath = "/dev/input/event0"
	testDeviceName = "Test keyboard"
	testOtherName  = "other"
	testSerial     = "serial"
	testLocation   = "usb/input"
	testFileMode   = 0o600
)

func (retry testRetrier) Do(
	ctx context.Context,
	operation func(context.Context) error,
	_ func(error) bool,
) error {
	return errors.Join(retry(ctx, operation, transient))
}

func equal[T comparable](t *testing.T, actual, expected T) {
	t.Helper()

	if actual != expected {
		t.Fatalf("got %v, want %v", actual, expected)
	}
}

func requireError(t *testing.T, actual, expected error) {
	t.Helper()

	if !errors.Is(actual, expected) {
		t.Fatalf("got error %v, want %v", actual, expected)
	}
}

func noError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func fixtureDevice() *device {
	endpoint := new(device)
	fixtureIdentity(endpoint)
	fixtureCapabilities(endpoint)

	endpoint.nonBlock, endpoint.release = func() error { return nil }, func() error { return nil }
	endpoint.readOne = func() (*native.InputEvent, error) { return nil, io.EOF }

	return endpoint
}

func fixtureIdentity(endpoint *device) {
	endpoint.path = func() string { return testDevicePath }
	endpoint.name = func() (string, error) { return testDeviceName, nil }
	fixtureInputID(endpoint)

	endpoint.uniqueID = func() (string, error) { return testSerial, nil }
	endpoint.physicalLocation = func() (string, error) { return testLocation, nil }
}

func fixtureCapabilities(endpoint *device) {
	endpoint.properties = func() []native.EvProp { return []native.EvProp{native.INPUT_PROP_POINTER} }
	endpoint.absInfos = func() (map[native.EvCode]native.AbsInfo, error) { return fixtureAxes(), nil }
	endpoint.capableTypes = fixtureEventTypes
	endpoint.capableEvents = fixtureEventCodes
}

func fixtureEventTypes() []native.EvType {
	return []native.EvType{
		native.EV_KEY,
		native.EV_REL,
		native.EV_ABS,
		native.EV_SW,
		native.EV_REP,
		native.EV_SYN,
	}
}

func fixtureEventCodes(eventType native.EvType) []native.EvCode {
	codes := map[native.EvType][]native.EvCode{
		native.EV_KEY: {native.KEY_A, native.BTN_LEFT, native.BTN_GAMEPAD},
		native.EV_REL: {native.REL_X, native.REL_WHEEL, native.REL_WHEEL_HI_RES},
		native.EV_ABS: {
			native.ABS_X,
			native.ABS_HAT0X,
			native.ABS_HAT0Y,
		},
		native.EV_SW: {native.SW_LID},
	}

	return codes[eventType]
}

func fixtureAxes() map[native.EvCode]native.AbsInfo {
	axis := new(native.AbsInfo)

	axis.Minimum, axis.Maximum = hatMinimum, nativeOne

	return map[native.EvCode]native.AbsInfo{
		native.ABS_X:     *axis,
		native.ABS_HAT0X: *axis,
		native.ABS_HAT0Y: *axis,
	}
}

func fixtureBackend(endpoint *device) *backend {
	state := new(backend)

	state.captures, state.closeDone = make(map[*capture]struct{}), make(chan struct{})
	state.environment = environment{
		readDirectory: func(string) ([]os.DirEntry, error) { return []os.DirEntry{}, nil },
		openDevice:    func(string, int) (*device, error) { return endpoint, nil },
	}

	return state
}

func fixtureSink(records *observations) *sinkview.Operations[domain.Event] {
	sink := new(sinkview.Operations[domain.Event])

	sink.Operations.Publish = func(event *domain.Event) bool {
		records.events = append(records.events, *event)

		return true
	}
	sink.Operations.Fail = func(err error) { records.failures = append(records.failures, err) }

	return sink
}

func fixtureCapture(t *testing.T, records *observations) *capture {
	t.Helper()

	endpoint := fixtureDevice()
	capture, err := backendPrepareCapture(fixtureBackend(endpoint), endpoint, fixtureSink(records))
	noError(t, err)

	return capture
}

func inputEvent(eventType native.EvType, code native.EvCode, value int32) *native.InputEvent {
	event := new(native.InputEvent)

	event.Type, event.Code, event.Value = eventType, code, value

	return event
}

func directoryEntries(t *testing.T) []os.DirEntry {
	t.Helper()

	files := fstest.MapFS{
		"event0":        new(fstest.MapFile),
		testOtherName:   new(fstest.MapFile),
		"eventdir/file": new(fstest.MapFile),
	}
	entries, err := files.ReadDir(".")
	noError(t, err)

	return entries
}

func TestDiscoveryMetadata(t *testing.T) {
	t.Parallel()

	state := fixtureBackend(fixtureDevice())

	state.environment.readDirectory = func(string) ([]os.DirEntry, error) { return directoryEntries(t), nil }

	infos, err := backendView(state).Discover(t.Context())
	noError(t, err)
	equal(t, len(infos), nativeOne)
	equal(t, infos[nativeZero].Name, testDeviceName)
	equal(t, infos[nativeZero].Transport, domain.TransportUSB)
}

func TestDiscoveryFailures(t *testing.T) {
	t.Parallel()

	values := []error{os.ErrNotExist, syscall.ENODEV, os.ErrPermission}
	for idx := range values {
		cause := values[idx]
		discoveryFailure(t, cause)
	}
}

func discoveryFailure(t *testing.T, cause error) {
	t.Helper()

	state := fixtureBackend(fixtureDevice())

	state.environment.openDevice = func(string, int) (*device, error) { return nil, errors.Join(cause) }

	infos, err := backendDiscoverEntries(t.Context(), state, directoryEntries(t))
	equal(t, len(infos), nativeZero)
	checkDiscoveryError(t, cause, err)
}

func checkDiscoveryError(t *testing.T, cause, actual error) {
	t.Helper()

	if disappeared(cause) {
		noError(t, actual)

		return
	}

	requireError(t, actual, domain.ErrPermissionDenied)
}

func TestDiscoveryDescriptionFailure(t *testing.T) {
	t.Parallel()

	endpoint := fixtureDevice()

	endpoint.name = func() (string, error) { return "", syscall.ENODEV }

	infos, err := backendDiscoverEntries(t.Context(), fixtureBackend(endpoint), directoryEntries(t))
	noError(t, err)
	equal(t, len(infos), nativeZero)

	checkDiscoveryPermission(t, endpoint)
}

func TestDiscoveryCloseFailure(t *testing.T) {
	t.Parallel()

	endpoint := fixtureDevice()

	endpoint.release = func() error { return os.ErrPermission }

	err := errorOf(
		backendDiscoverEntries(t.Context(), fixtureBackend(endpoint), directoryEntries(t)),
	)
	requireError(t, err, domain.ErrPermissionDenied)
}

func TestDirectoryFailuresAndRetry(t *testing.T) {
	t.Parallel()

	state := fixtureBackend(fixtureDevice())

	state.environment.readDirectory = func(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist }

	infos, err := backendDiscover(t.Context(), state)
	noError(t, err)
	equal(t, len(infos), nativeZero)

	checkDirectoryRetry(t, state)
}

func canceledContext(t *testing.T) func() context.Context {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	return func() context.Context { return ctx }
}

func TestCancellation(t *testing.T) {
	t.Parallel()

	ctx := canceledContext(t)()
	err := errorOf(newBackend(ctx, nil, defaultEnvironment()))
	requireError(t, err, context.Canceled)

	state := fixtureBackend(fixtureDevice())

	err = errorOf(backendDiscover(ctx, state))
	requireError(t, err, context.Canceled)
	checkCanceledOperations(ctx, t, state)
}

func checkCanceledOperations(ctx context.Context, t *testing.T, state *backend) {
	t.Helper()

	err := errorOf(backendDiscoverEntries(ctx, state, directoryEntries(t)))
	requireError(t, err, context.Canceled)

	var entries []os.DirEntry

	noError(t, errors.Join())
	requireError(t, backendReadDirectory(ctx, state, &entries), context.Canceled)

	err = errorOf(backendOpen(ctx, state, new(openRequest)))
	requireError(t, err, context.Canceled)
}

func TestBackendFactoryAndClose(t *testing.T) {
	t.Parallel()

	view, err := newBackend(t.Context(), nil, fixtureBackend(fixtureDevice()).environment)
	noError(t, err)
	noError(t, view.Close())
	noError(t, view.Close())

	err = errorOf(view.Discover(t.Context()))
	requireError(t, err, domain.ErrClosed)
	checkFactoryCanceled(t)
}

func checkFactoryCanceled(t *testing.T) {
	t.Helper()

	backend, err := Factory()(canceledContext(t)(), nil)
	equal(t, backend, nil)
	requireError(t, err, context.Canceled)
}

func TestNativeFactoryStartup(t *testing.T) {
	t.Parallel()

	backend, err := Factory()(t.Context(), nil)
	noError(t, err)
	noError(t, backend.Close())
}

func TestOpenLifecycle(t *testing.T) {
	t.Parallel()

	state := fixtureBackend(fixtureDevice())
	view := backendView(state)
	records := new(observations)
	capture, err := view.Open(t.Context(), deviceID(testDevicePath), fixtureSink(records))
	noError(t, err)
	noError(t, capture.Close())
	noError(t, view.Close())
	equal(t, capture.Info().Name, testDeviceName)
	equal(t, capture.Capabilities().Repeat, domain.SupportSupported)
}

func TestOpenPathsAndFailures(t *testing.T) {
	t.Parallel()

	state := fixtureBackend(fixtureDevice())

	values := []domain.DeviceID{testOtherName, "evdev:/tmp/event0", "evdev:/dev/input/mouse0"}
	for idx := range values {
		id := values[idx]
		err := errorOf(backendView(state).Open(t.Context(), id, fixtureSink(new(observations))))
		requireError(t, err, domain.ErrNotFound)
	}

	checkOpenPermission(t, state)
}

func TestOpenSetupFailures(t *testing.T) {
	t.Parallel()

	endpoint := fixtureDevice()

	endpoint.nonBlock = func() error { return syscall.EINVAL }

	err := errorOf(backendOpen(
		t.Context(),
		fixtureBackend(endpoint),
		&openRequest{id: deviceID(testDevicePath), sink: nil},
	))
	requireError(t, err, syscall.EINVAL)

	endpoint.name = func() (string, error) { return "", syscall.ENODEV }
	err = errorOf(backendOpen(
		t.Context(),
		fixtureBackend(endpoint),
		&openRequest{id: deviceID(testDevicePath), sink: nil},
	))
	requireError(t, err, domain.ErrDisconnected)
}

func TestRegistrationFailures(t *testing.T) {
	t.Parallel()

	state := fixtureBackend(fixtureDevice())
	capture := newCapture(state, fixtureDevice(), fixtureSink(new(observations)))
	err := errorOf(backendStartCapture(canceledContext(t)(), state, capture))
	requireError(t, err, context.Canceled)

	state.closed = true
	err = errorOf(backendStartCapture(t.Context(), state, capture))
	requireError(t, err, domain.ErrClosed)
}

func TestCloseActiveCapture(t *testing.T) {
	t.Parallel()

	state := fixtureBackend(fixtureDevice())
	capture := newCapture(state, fixtureDevice(), fixtureSink(new(observations)))

	state.captures[capture] = struct{}{}
	close(capture.done)
	noError(t, backendShutdown(state))
	equal(t, captureStopped(capture), true)
}

func TestCaptureCloseErrors(t *testing.T) {
	t.Parallel()

	values := []error{os.ErrClosed, os.ErrPermission}
	for idx := range values {
		cause := values[idx]
		captureCloseError(t, cause)
	}
}

func captureCloseError(t *testing.T, cause error) {
	t.Helper()

	capture := fixtureCapture(t, new(observations))

	capture.device.release = func() error { return errors.Join(cause) }
	close(capture.done)

	err := captureClose(capture)

	if errors.Is(cause, os.ErrClosed) {
		noError(t, err)

		return
	}

	requireError(t, err, cause)
}

func TestReaderErrorsAndTermination(t *testing.T) {
	t.Parallel()

	capture := fixtureCapture(t, new(observations))

	capture.device.readOne = func() (*native.InputEvent, error) { return nil, syscall.EINTR }
	equal(t, captureReadNext(capture), true)

	capture.device.readOne = func() (*native.InputEvent, error) { return nil, io.EOF }
	equal(t, captureReadNext(capture), false)
	captureStopReader(capture)
	equal(t, captureReadNext(capture), false)
}

func TestReaderPublishesEvents(t *testing.T) {
	t.Parallel()

	records := new(observations)
	capture := fixtureCapture(t, records)

	capture.device.readOne = func() (*native.InputEvent, error) {
		return inputEvent(native.EV_SYN, native.SYN_DROPPED, nativeZero), nil
	}
	captureRead(capture)
	equal(t, len(records.failures), nativeOne)
	requireError(t, records.failures[nativeZero], domain.ErrEventLoss)
}

func TestControlEvents(t *testing.T) {
	t.Parallel()

	records := new(observations)
	capture := fixtureCapture(t, records)

	keyTransitions(t, capture)

	equal(t, len(records.events), nativeThree)
	equal(t, records.events[nativeTwo].Value, float64(nativeOne))
	equal(t, records.events[nativeTwo].Action, domain.ActionRepeat)
	checkIgnoredControls(t, capture)
}

func checkIgnoredControls(t *testing.T, capture *capture) {
	t.Helper()
	equal(t, captureDispatch(capture, inputEvent(native.EV_MSC, native.MSC_SCAN, nativeOne)), true)

	control := unknownControl()

	control.Support = domain.SupportUnsupported
	capture.controls[eventCode{typeCode: native.EV_MSC, code: native.MSC_SCAN}] = *control
	equal(t, captureDispatch(capture, inputEvent(native.EV_MSC, native.MSC_SCAN, nativeOne)), true)
}

func TestHighResolutionWheel(t *testing.T) {
	t.Parallel()

	records := new(observations)
	capture := fixtureCapture(t, records)
	equal(t, captureDispatch(capture, inputEvent(native.EV_REL, native.REL_WHEEL, nativeOne)), true)
	equal(t, len(records.events), nativeZero)
	equal(
		t,
		captureDispatch(capture, inputEvent(native.EV_REL, native.REL_WHEEL_HI_RES, wheelDetent)),
		true,
	)
	equal(t, len(records.events), nativeOne)
	equal(t, records.events[nativeZero].Value, float64(nativeOne))
}

func TestHatReports(t *testing.T) {
	t.Parallel()

	records := new(observations)
	capture := fixtureCapture(t, records)
	moveHat(t, capture)
	equal(
		t,
		captureDispatch(capture, inputEvent(native.EV_SYN, native.SYN_REPORT, nativeZero)),
		true,
	)
	equal(t, len(records.events), nativeOne)
	equal(t, records.events[nativeZero].Value, float64(domain.HatNorthEast))
	equal(
		t,
		captureDispatch(capture, inputEvent(native.EV_SYN, native.SYN_CONFIG, nativeZero)),
		true,
	)
	moveHat(t, capture)
	equal(t, capturePublishHats(capture, new(domain.Timestamp)), true)
}

func moveHat(t *testing.T, capture *capture) {
	t.Helper()
	equal(
		t,
		captureDispatch(capture, inputEvent(native.EV_ABS, native.ABS_HAT0X, nativeOne)),
		true,
	)
	equal(
		t,
		captureDispatch(capture, inputEvent(native.EV_ABS, native.ABS_HAT0Y, hatMinimum)),
		true,
	)
}

func TestInvalidHatAndSinkTermination(t *testing.T) {
	t.Parallel()

	capture := fixtureCapture(t, new(observations))
	equal(
		t,
		captureDispatch(capture, inputEvent(native.EV_ABS, native.ABS_HAT0X, nativeTwo)),
		true,
	)
	equal(t, capturePublishHats(capture, new(domain.Timestamp)), false)

	capture.sink = rejectingSink()
	moveHat(t, capture)
	equal(t, capturePublishHats(capture, new(domain.Timestamp)), false)
	equal(t, hatValue(nativeTwo, nativeZero), int64(domain.HatNeutral))
}

func rejectingSink() *sinkview.Operations[domain.Event] {
	sink := new(sinkview.Operations[domain.Event])

	sink.Operations.Publish = func(*domain.Event) bool { return false }
	sink.Operations.Fail = func(error) {}

	return sink
}

func TestMetadataCopies(t *testing.T) {
	t.Parallel()

	capture := fixtureCapture(t, new(observations))
	info := captureNativeInfo(capture)

	info.Properties[nativeZero] = nativeZero
	delete(info.Axes, uint16(native.ABS_X))
	equal(t, len(captureNativeInfo(capture).Axes), nativeThree)
	equal(t, captureNativeInfo(capture).Properties[nativeZero], uint16(native.INPUT_PROP_POINTER))
	checkExtensions(t, capture)
}

func checkExtensions(t *testing.T, capture *capture) {
	t.Helper()

	var metadata extension.Metadata

	equal(t, captureView(capture).Extension(&metadata), true)
	equal(t, metadata.NativeInfo().PhysicalLocation, testLocation)
	equal(t, captureExtension(capture, new(extension.Info)), true)
	equal(t, captureExtension(capture, (*extension.Info)(nil)), false)
	equal(t, captureExtension(capture, (*extension.Metadata)(nil)), false)
	equal(t, captureExtension(capture, new(int)), false)
}

func TestDescriptionErrors(t *testing.T) {
	t.Parallel()

	endpoint := fixtureDevice()

	endpoint.inputID = func() (native.InputID, error) { return native.InputID{}, syscall.ENODEV }

	err := errorOf(describe(endpoint))
	requireError(t, err, syscall.ENODEV)

	checkAxesFailure(t)
}

func TestOptionalIdentity(t *testing.T) {
	t.Parallel()

	values := []error{syscall.ENOENT, syscall.ENOTTY, syscall.EINVAL}
	for idx := range values {
		cause := values[idx]
		value, err := optionalString(func() (string, error) { return "", errors.Join(cause) })
		noError(t, err)
		equal(t, value, "")
	}

	checkIdentityFailure(t)
}

func checkIdentityFailure(t *testing.T) {
	t.Helper()

	endpoint := fixtureDevice()

	endpoint.uniqueID = func() (string, error) { return "", syscall.ENODEV }

	err := errorOf(describe(endpoint))
	requireError(t, err, syscall.ENODEV)

	checkLocationFailure(t, endpoint)
}

func TestClassesAndUnsupportedControls(t *testing.T) {
	t.Parallel()
	equal(t, deviceClasses(nil)[nativeZero], domain.ClassOther)
	equal(
		t,
		deviceClasses(map[native.EvCode]bool{native.BTN_JOYSTICK: true})[nativeZero],
		domain.ClassJoystick,
	)

	control := controlFor(
		&controlRequest{eventType: native.EV_MSC, code: native.MSC_SCAN, axes: nil},
	)
	equal(t, control.Support, domain.SupportUnsupported)

	control = controlFor(
		&controlRequest{eventType: native.EV_ABS, code: native.ABS_MISC, axes: nil},
	)
	equal(t, control.Mapping, domain.MappingUnknown)
	equal(t, control.Range, nil)
}

func TestMissingHatAxes(t *testing.T) {
	t.Parallel()

	capture := fixtureCapture(t, new(observations))
	delete(capture.native.Axes, uint16(native.ABS_HAT0Y))
	capturePrepareHat(capture, nativeZero)

	axis := new(extension.AxisInfo)
	equal(t, hatAxis(axis), false)
}

func TestMappingsAndTransport(t *testing.T) {
	t.Parallel()

	values := []native.EvCode{native.REL_X, native.REL_DIAL, native.REL_WHEEL, native.REL_HWHEEL}
	for idx := range values {
		code := values[idx]
		equal(t, relativeAxisUsage(code) != domain.UsageUnknown, true)
	}

	checkAbsoluteMappings(t)

	checkUnknownMappings(t)
}

func checkUnknownMappings(t *testing.T) {
	t.Helper()
	equal(t, relativeAxisUsage(native.REL_MISC), domain.UsageUnknown)
	equal(t, keyUsage(native.KEY_RESERVED), domain.UsageUnknown)
	equal(t, transport(nativeZero), domain.TransportUnknown)
	equal(t, transient(syscall.EAGAIN), true)
	equal(t, transient(io.EOF), false)
	noError(t, errorKind(nil))
}

func TestKeyboardAndButtonMappings(t *testing.T) {
	t.Parallel()
	equal(t, keyUsage(native.KEY_A), domain.HID(nativeSeven, nativeFour))
	equal(t, keyUsage(native.KEY_PLAY), domain.HID(nativeTwelve, usageB0))
	equal(t, keyUsage(native.KEY_POWER), domain.HID(nativeOne, usage81))
	equal(t, keyUsage(native.BTN_LEFT), domain.HID(nativeNine, nativeOne))
	equal(t, keyUsage(native.BTN_SOUTH), domain.HID(nativeNine, nativeOne))
	equal(t, errorKind(os.ErrClosed), domain.ErrClosed)
}

func TestNativeDeviceBoundary(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), eventPrefix)
	noError(t, os.WriteFile(path, nil, testFileMode))
	requireNativeError(t, errorOf(defaultEnvironment().openDevice(path, os.O_RDONLY)))
	requireError(
		t,
		errorOf(defaultEnvironment().openDevice(path+devicePrefix, os.O_RDONLY)),
		os.ErrNotExist,
	)

	endpoint, err := openNativeDevice(
		testDevicePath,
		os.O_RDONLY,
		func(string, int) (*native.InputDevice, error) {
			return new(native.InputDevice), nil
		},
	)
	noError(t, err)
	equal(t, endpoint != nil, true)
}

func TestSortedCapabilities(t *testing.T) {
	t.Parallel()

	capture := fixtureCapture(t, new(observations))
	equal(t, sortedControls(capture.caps.Controls), true)
	equal(t, len(capture.caps.Controls) > nativeZero, true)
}

func sortedControls(controls []domain.Control) bool {
	ids := make([]domain.ControlID, nativeZero, len(controls))
	for idx := range controls {
		ids = append(ids, controls[idx].ID)
	}

	return slices.IsSorted(ids)
}

func errorOf[Value any](_ Value, err error) error { return errors.Join(err) }

func requireNativeError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("regular file accepted as an input device")
	}

	equal(t, strings.Contains(err.Error(), syscall.ENOTTY.Error()), true)
}

func fixtureInputID(endpoint *device) {
	endpoint.inputID = func() (native.InputID, error) {
		id := new(native.InputID)

		id.BusType = nativeThree

		return *id, nil
	}
}

func checkDiscoveryPermission(t *testing.T, endpoint *device) {
	t.Helper()

	endpoint.name = func() (string, error) { return "", os.ErrPermission }

	err := errorOf(
		backendDiscoverEntries(t.Context(), fixtureBackend(endpoint), directoryEntries(t)),
	)
	requireError(t, err, domain.ErrPermissionDenied)
}

func checkDirectoryRetry(t *testing.T, state *backend) {
	t.Helper()

	state.environment.readDirectory = func(string) ([]os.DirEntry, error) { return nil, syscall.EINTR }
	state.retrier = testRetrier(
		func(ctx context.Context, operation func(context.Context) error, _ func(error) bool) error {
			return errors.Join(operation(ctx))
		},
	)

	err := errorOf(backendDiscover(t.Context(), state))
	requireError(t, err, syscall.EINTR)
}

func checkAxesFailure(t *testing.T) {
	t.Helper()

	endpoint := fixtureDevice()

	endpoint.absInfos = func() (map[native.EvCode]native.AbsInfo, error) { return nil, syscall.EINVAL }

	err := errorOf(describe(endpoint))
	requireError(t, err, syscall.EINVAL)
}

func checkLocationFailure(t *testing.T, endpoint *device) {
	t.Helper()

	endpoint.uniqueID = func() (string, error) { return "", nil }
	endpoint.physicalLocation = func() (string, error) { return "", os.ErrPermission }

	err := errorOf(describe(endpoint))
	requireError(t, err, os.ErrPermission)
}

func checkOpenPermission(t *testing.T, state *backend) {
	t.Helper()

	state.environment.openDevice = func(string, int) (*device, error) { return nil, os.ErrPermission }

	err := errorOf(
		backendOpen(t.Context(), state, &openRequest{id: deviceID(testDevicePath), sink: nil}),
	)
	requireError(t, err, domain.ErrPermissionDenied)
}

func keyTransitions(t *testing.T, capture *capture) {
	t.Helper()

	values := []int32{nativeZero, nativeOne, nativeTwo, nativeThree}
	for idx := range values {
		value := values[idx]
		equal(t, captureDispatch(capture, inputEvent(native.EV_KEY, native.KEY_A, value)), true)
	}
}

func TestCancellationDuringOpening(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())

	defer cancel()

	endpoint := fixtureDevice()

	endpoint.nonBlock = func() error {
		cancel()

		return nil
	}

	state := fixtureBackend(endpoint)
	err := errorOf(backendOpen(ctx, state, &openRequest{id: deviceID(testDevicePath), sink: nil}))
	requireError(t, err, context.Canceled)
}

func checkAbsoluteMappings(t *testing.T) {
	t.Helper()

	values := []native.EvCode{
		native.ABS_X,
		native.ABS_THROTTLE,
		native.ABS_RUDDER,
		native.ABS_GAS,
		native.ABS_BRAKE,
	}
	for idx := range values {
		code := values[idx]
		equal(t, absoluteAxisUsage(code) != domain.UsageUnknown, true)
	}

	equal(t, absoluteAxisUsage(native.ABS_WHEEL), domain.HID(nativeOne, wheelUsage))
}
