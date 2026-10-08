// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"slices"
	"syscall"
	"testing"
	"unsafe"

	native "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	hid "github.com/deploymenttheory/go-bindings-win32/bindings/win32/devices/humaninterfacedevice"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/security"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/storage/filesystem"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/input"
	wm "github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/windowsandmessaging"
	"github.com/gostafa/goinput/internal/domain"
	sinkview "github.com/gostafa/goinput/internal/ports/sink"
)

type (
	nativeFixture struct {
		t             *testing.T
		err           error
		callback      func(foundation.HWND, uint32, foundation.WPARAM, foundation.LPARAM) foundation.LRESULT
		path          string
		inventory     []input.RAWINPUTDEVICELIST
		buttons       []hid.HIDP_BUTTON_CAPS
		values        []hid.HIDP_VALUE_CAPS
		data          []hid.HIDP_DATA
		packet        []byte
		registrations []input.RAWINPUTDEVICE
		events        []domain.Event
		failures      []error
		words         [sixthValue]uint32
		status        foundation.NTSTATUS
		maxData       uint32
		caps          hid.HIDP_CAPS
	}
	installHIDFixtureState struct {
		getString func(_ foundation.HANDLE, data []byte) foundation.BOOLEAN
	}
	installHIDFixtureStep1ArgsRecord[
		ApiValue any,
		StateValue any,
	] struct {
		api   ApiValue
		state StateValue
	}
	installHIDFixtureStep1Args = installHIDFixtureStep1ArgsRecord[
		*nativeAPI,
		*installHIDFixtureState,
	]
	installHIDFixtureStep3ArgsRecord[
		ApiValue any,
		StateValue any,
	] struct {
		api ApiValue
	}
	installHIDFixtureStep3Args = installHIDFixtureStep3ArgsRecord[
		*nativeAPI,
		*installHIDFixtureState,
	]
	fixtureListReadArguments[Value any] struct {
		first *Value
		count *uint32
		err   error
	}
	fixtureDeviceInfoArguments struct {
		buffer  nativeBuffer
		command input.RAW_INPUT_DEVICE_INFO_COMMAND
	}
	nativeScenario = nativeScenarioRecord[
		*nativeFixture,
		*backend,
		func() context.Context,
		context.CancelFunc,
	]
	nativeScenarioRecord[
		Fixture,
		Owner,
		Context,
		Cancel any,
	] struct {
		fixture Fixture
		owner   Owner
		ctx     Context
		cancel  Cancel
	}
	nativeWindowCallback = func(
		foundation.HWND,
		uint32,
		foundation.WPARAM,
		foundation.LPARAM,
	) foundation.LRESULT
)

func TestNativeAdapter(t *testing.T) {
	t.Parallel()

	rangeValues1 := nativeScenarios()
	for name := range rangeValues1 {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rangeValues1[name](t, newNativeAPI())
		})
	}
}

func fixtureCast[Value any](t *testing.T, value any) Value {
	t.Helper()

	result, valid := value.(Value)
	if !valid {
		t.Fatal("unexpected native callback type")
	}

	return result
}

func fixtureLength16(t *testing.T, length int) uint16 {
	t.Helper()

	if length < noValue || length > math.MaxUint16 {
		t.Fatal(testNativeCountFailure)
	}

	return uint16(length & math.MaxUint16)
}

func fixtureLength32(t *testing.T, length int) uint32 {
	t.Helper()

	if length < noValue || uint64(length) > math.MaxUint32 {
		t.Fatal(testNativeCountFailure)
	}

	return uint32(length & math.MaxUint32)
}

func installHIDFixture(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()

	state := new(installHIDFixtureState)
	installHIDFixtureStep1(t, fixture, &installHIDFixtureStep1Args{api: api, state: state})
}

func installHIDFixtureStep1(
	t *testing.T,
	fixture *nativeFixture,
	args *installHIDFixtureStep1Args,
) {
	t.Helper()
	replaceNative(
		t,
		&args.api.hid.hidDGetAttributes,
		func(_ foundation.HANDLE, attributes *hid.HIDD_ATTRIBUTES) foundation.BOOLEAN {
			attributes.VendorID, attributes.ProductID, attributes.VersionNumber = fourthValue, fifthValue, sixthValue

			return singleValue
		},
	)
	installHIDFixtureStep1Continue(t, fixture, args)
}

func installHIDFixtureStep2(
	t *testing.T,
	fixture *nativeFixture,
	args *installHIDFixtureStep1Args,
) {
	t.Helper()
	replaceNative(t, &args.api.hid.hidDGetProductString, args.state.getString)
	replaceNative(t, &args.api.hid.hidDGetManufacturerString, args.state.getString)
	replaceNative(t, &args.api.hid.hidDGetSerialNumberString, args.state.getString)
	installHIDFixtureStep3(t, fixture, &installHIDFixtureStep3Args{api: args.api})
}

func installHIDFixtureStep3(
	t *testing.T,
	fixture *nativeFixture,
	args *installHIDFixtureStep3Args,
) {
	t.Helper()
	replaceNative(
		t,
		&args.api.hid.hidPGetCaps,
		func(_ hid.PHIDP_PREPARSED_DATA, caps *hid.HIDP_CAPS) foundation.NTSTATUS {
			*caps = fixture.caps

			return fixture.status
		},
	)
	installHIDFixtureStep3Finish(t, fixture, args)
}

func installHIDFixtureStep4(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()
	replaceNative(
		t,
		&api.hid.hidPGetValueCaps,
		fixtureCapabilitiesReader(t, fixture, &fixture.values),
	)
	installHIDFixtureStep4Continue(t, fixture, api)
}

func installInventoryFixture(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()
	replaceNative(
		t,
		&api.inventory.getRawInputDeviceList,
		fixtureListReader(t, fixture, &fixture.inventory),
	)
	replaceNative(
		t,
		&api.inventory.getRegisteredRawInputDevices,
		fixtureListReader(t, fixture, &fixture.registrations),
	)
	replaceNative(
		t,
		&api.inventory.registerRawInputDevices,
		func(devices []input.RAWINPUTDEVICE, _ uint32) error {
			return registerFixtureDevices(fixture, devices)
		},
	)
	installInventoryFixtureFinish(t, fixture, api)
}

func installWindowFixture(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()
	installWindowFixtureStep1(t, fixture, api)
}

func installWindowFixtureStep1(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()
	replaceNative(t, &api.read.findProcedure, func(*native.Proc) error {
		return fixture.err
	})
	replaceNative(t, &api.window.newCallback, func(value any) uintptr {
		fixture.callback = fixtureCast[nativeWindowCallback](t, value)

		return singleValue
	})
	installWindowFixtureStep2(t, fixture, api)
}

func installWindowFixtureStep2(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()
	replaceNative(
		t,
		&api.messages.createEvent,
		func(*security.SECURITY_ATTRIBUTES, bool, bool, *string) (foundation.HANDLE, error) {
			if fixture.err != nil {
				return nativeFailure[foundation.HANDLE](fixture.err)
			}

			return singleValue, nil
		},
	)
	replaceNative(t, &api.window.getModuleHandle, func(*string) (foundation.HMODULE, error) {
		if fixture.err != nil {
			return nativeFailure[foundation.HMODULE](fixture.err)
		}

		return singleValue, nil
	})
	installWindowFixtureStep2Continue(t, fixture, api)
}

func installWindowFixtureStep3(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()
	replaceNative(t, &api.window.createWindowEx, func(
		wm.WINDOW_EX_STYLE,
		*string,
		*string,
		wm.WINDOW_STYLE,
		int32,
		int32,
		int32,
		int32,
		foundation.HWND,
		wm.HMENU,
		foundation.HINSTANCE,
		unsafe.Pointer,
	) (foundation.HWND, error) {
		return fixtureWindowResult(fixture)
	})
	installWindowFixtureStep3Finish(t, fixture, api)
}

func installWindowFixtureStep4(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()
	replaceNative(t, &api.window.unregisterClass, func(string, foundation.HINSTANCE) error {
		return fixture.err
	})
	replaceNative(t, &api.messages.closeHandle, func(foundation.HANDLE) error {
		return fixture.err
	})
	replaceNative(t, &api.messages.setEvent, func(foundation.HANDLE) error {
		return fixture.err
	})
	installWindowFixtureStep5(t, fixture, api)
}

func installWindowFixtureStep5(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()
	replaceNative(t, &api.messages.msgWaitForMultipleObjectsEx, func(
		[]foundation.HANDLE,
		uint32,
		wm.QUEUE_STATUS_FLAGS,
		wm.MSG_WAIT_FOR_MULTIPLE_OBJECTS_EX_FLAGS,
	) (foundation.WAIT_EVENT, error) {
		if fixture.err != nil {
			return noValue, fixture.err
		}

		return singleValue, nil
	})
	installWindowFixtureStep5Continue(t, api)
}

func nativeEqual(t *testing.T, actual, expected any) {
	t.Helper()

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("got %#v, want %#v", actual, expected)
	}
}

func nativeOK(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func nativeScenarios() map[string]func(*testing.T, *nativeAPI) {
	return map[string]func(*testing.T, *nativeAPI){
		"commands":           testNativeCommands,
		"lifecycle":          testNativeLifecycle,
		"inventory":          testNativeInventory,
		"registration":       testNativeRegistration,
		"metadata":           testNativeMetadata,
		testCapabilitiesText: testNativeCapabilities,
		"input":              testNativeInput,
		"hid":                testNativeHID,
		"public operations":  testNativeViews,
		"late failures":      testNativeLateFailures,
	}
}

func nativeSlice[Value any](first *Value, count int) []Value {
	var result []Value

	reflect.ValueOf(&result).
		Elem().
		Set(reflect.SliceAt(reflect.TypeFor[Value](), nativePointer(first), count))

	return result
}

func newNativeFixture(t *testing.T, api *nativeAPI) *nativeFixture {
	t.Helper()

	fixture := nativeNew[nativeFixture](func(value *nativeFixture) {
		value.t = t
		value.path = testNativePath
		value.status = hid.HIDP_STATUS_SUCCESS
		value.maxData = fourthValue
	})

	fixture.caps.InputReportByteLength = secondValue
	fixture.words[singleValue], fixture.words[thirdValue] = thirdValue, singleValue
	installWindowFixture(t, fixture, api)
	installInventoryFixture(t, fixture, api)
	installHIDFixture(t, fixture, api)

	return fixture
}

func replaceNative[Value any](t *testing.T, target *Value, value Value) {
	t.Helper()

	previous := *target

	*target = value

	t.Cleanup(func() {
		*target = previous
	})
}

func (fixture *nativeFixture) capture(t *testing.T, kind uint32, api *nativeAPI) *capture {
	t.Helper()

	subscription := backendMakeCapture(
		fixtureBackend(t),
		fixtureDevice(kind),
		&backendMakeCaptureArgs{sink: fixtureSink(fixture), api: api},
	)
	configureFixtureCapture(subscription)

	return subscription
}

func fixtureDevice(kind uint32) *nativeDevice {
	return nativeNew[nativeDevice](func(device *nativeDevice) {
		device.kind = kind
		device.handle = singleValue
		device.tlc = topLevel{page: singleValue, usage: secondValue}
		device.info.ID = testCaptureID
	})
}

func fixtureSink(fixture *nativeFixture) *sinkview.Operations[domain.Event] {
	view := new(sinkview.Operations[domain.Event])

	view.Operations.Publish = func(event *domain.Event) bool {
		fixture.events = append(fixture.events, *event)

		return true
	}
	view.Operations.Fail = func(err error) {
		fixture.failures = append(fixture.failures, err)
	}

	return view
}

func configureFixtureCapture(subscription *capture) {
	subscription.backend.call = func(_ context.Context, operation func() error) error {
		return errors.Join(operation())
	}
	subscription.backend.retry = func(ctx context.Context, operation func(context.Context) error) error {
		return errors.Join(operation(ctx))
	}
}

func nativeCancel(ctx context.Context) (func() context.Context, context.CancelFunc) {
	scoped, cancel := context.WithCancel(ctx)

	return func() context.Context {
		return scoped
	}, cancel
}

func nativeZero[Value any]() Value {
	var value Value

	return value
}

func nativeNew[Value any](initialize func(*Value)) *Value {
	value := new(Value)
	initialize(value)

	return value
}

func nativeValue[Value any](initialize func(*Value)) Value {
	return *nativeNew(initialize)
}

func fixtureListReader[Value any](
	t *testing.T,
	fixture *nativeFixture,
	list *[]Value,
) func(*Value, *uint32, uint32) (uint32, error) {
	t.Helper()

	return func(first *Value, count *uint32, _ uint32) (uint32, error) {
		return readFixtureList(
			t,
			*list,
			nativeNew[fixtureListReadArguments[Value]](
				func(value *fixtureListReadArguments[Value]) {
					value.first = first
					value.count = count
					value.err = fixture.err
				},
			),
		)
	}
}

func readFixtureList[Value any](
	t *testing.T,
	list []Value,
	args *fixtureListReadArguments[Value],
) (uint32, error) {
	t.Helper()

	if args.err != nil {
		return infiniteWait, errors.Join(args.err)
	}

	if args.first == nil {
		*args.count = fixtureLength32(t, len(list))

		return noValue, nil
	}

	copy(nativeSlice(args.first, int(*args.count)), list)

	return fixtureLength32(t, len(list)), nil
}

func registerFixtureDevices(fixture *nativeFixture, devices []input.RAWINPUTDEVICE) error {
	if fixture.err != nil {
		return errors.Join(fixture.err)
	}

	for index := range devices {
		registerFixtureDevice(fixture, &devices[index])
	}

	return nil
}

func registerFixtureDevice(fixture *nativeFixture, device *input.RAWINPUTDEVICE) {
	fixture.registrations = slices.DeleteFunc(
		fixture.registrations,
		func(current input.RAWINPUTDEVICE) bool {
			return current.UsUsagePage == device.UsUsagePage && current.UsUsage == device.UsUsage
		},
	)

	if uint32(device.DwFlags)&singleValue == noValue {
		fixture.registrations = append(fixture.registrations, *device)
	}
}

func fixtureDeviceInfoReader(t *testing.T, fixture *nativeFixture) nativeGetRawInputDeviceInfoCall {
	t.Helper()

	return func(
		_ foundation.HANDLE,
		command input.RAW_INPUT_DEVICE_INFO_COMMAND,
		data unsafe.Pointer,
		count *uint32,
	) (uint32, error) {
		return readFixtureDeviceInfo(
			t,
			fixture,
			nativeNew[fixtureDeviceInfoArguments](func(value *fixtureDeviceInfoArguments) {
				value.command = command
				value.buffer = nativeValue[nativeBuffer](func(value *nativeBuffer) {
					value.data = data
					value.size = count
				})
			}),
		)
	}
}

func readFixtureDeviceInfo(
	t *testing.T,
	fixture *nativeFixture,
	args *fixtureDeviceInfoArguments,
) (uint32, error) {
	t.Helper()

	if fixture.err != nil {
		return infiniteWait, errors.Join(fixture.err)
	}

	value, err := readFixtureDeviceCommand(fixture, args)

	return value, errors.Join(err)
}

func readFixtureDeviceCommand(
	fixture *nativeFixture,
	args *fixtureDeviceInfoArguments,
) (uint32, error) {
	switch args.command {
	case input.RIDI_DEVICEINFO:
		return readFixtureDeviceWords(fixture, &args.buffer), nil
	case input.RIDI_DEVICENAME:
		value, err := readFixtureDeviceName(fixture.t, fixture.path, &args.buffer)

		return value, errors.Join(err)
	case input.RIDI_PREPARSEDDATA:
		return readFixturePreparsedData(&args.buffer), nil
	default:
		return infiniteWait, domain.ErrUnsupported
	}
}

func readFixtureDeviceWords(fixture *nativeFixture, buffer *nativeBuffer) uint32 {
	(*input.RID_DEVICE_INFO)(buffer.data).Anonymous.Data = fixture.words

	return *buffer.size
}

func readFixtureDeviceName(t *testing.T, path string, buffer *nativeBuffer) (uint32, error) {
	t.Helper()

	wide, err := syscall.UTF16FromString(path)
	if err != nil {
		return infiniteWait, errors.Join(err)
	}

	if buffer.data == nil {
		*buffer.size = fixtureLength32(t, len(wide))

		return noValue, nil
	}

	copy(nativeSlice((*uint16)(buffer.data), int(*buffer.size)), wide)

	return *buffer.size, nil
}

func readFixturePreparsedData(buffer *nativeBuffer) uint32 {
	if buffer.data == nil {
		*buffer.size = singleValue

		return noValue
	}

	*(*byte)(buffer.data) = singleValue

	return *buffer.size
}

func fixtureInputReader(t *testing.T, fixture *nativeFixture) nativeGetRawInputDataCall {
	t.Helper()

	return func(
		_ input.HRAWINPUT,
		_ input.RAW_INPUT_DATA_COMMAND_FLAGS,
		data unsafe.Pointer,
		count *uint32,
		_ uint32,
	) uint32 {
		return readFixtureInput(t, fixture, nativeNew[nativeBuffer](func(value *nativeBuffer) {
			value.data = data
			value.size = count
		}))
	}
}

func readFixtureInput(t *testing.T, fixture *nativeFixture, buffer *nativeBuffer) uint32 {
	t.Helper()

	if fixture.err != nil {
		return infiniteWait
	}

	if buffer.data == nil {
		*buffer.size = fixtureLength32(t, len(fixture.packet))

		return noValue
	}

	copy(nativeSlice((*byte)(buffer.data), int(*buffer.size)), fixture.packet)

	return fixtureLength32(t, len(fixture.packet))
}

func fixtureFileOpener(fixture *nativeFixture) nativeCreateFileCall {
	return func(
		string,
		uint32,
		filesystem.FILE_SHARE_MODE,
		*security.SECURITY_ATTRIBUTES,
		filesystem.FILE_CREATION_DISPOSITION,
		filesystem.FILE_FLAGS_AND_ATTRIBUTES,
		foundation.HANDLE,
	) (foundation.HANDLE, error) {
		if fixture.err != nil {
			return nativeFailure[foundation.HANDLE](fixture.err)
		}

		return thirdValue, nil
	}
}

func installHIDFixtureStep1Continue(
	t *testing.T,
	fixture *nativeFixture,
	args *installHIDFixtureStep1Args,
) {
	t.Helper()

	args.state.getString = func(_ foundation.HANDLE, data []byte) foundation.BOOLEAN {
		binary.LittleEndian.PutUint16(data, 'A')

		return singleValue
	}
	installHIDFixtureStep2(t, fixture, args)
}

func installHIDFixtureStep3Continue(
	t *testing.T,
	fixture *nativeFixture,
	args *installHIDFixtureStep3Args,
) {
	t.Helper()
	replaceNative(
		t,
		&args.api.hid.hidPGetButtonCaps,
		fixtureCapabilitiesReader(t, fixture, &fixture.buttons),
	)
	installHIDFixtureStep4(t, fixture, args.api)
}

func installHIDFixtureStep4Continue(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()
	replaceNative(t, &api.hid.hidPGetData, func(
		_ hid.HIDP_REPORT_TYPE,
		first *hid.HIDP_DATA,
		count *uint32,
		_ hid.PHIDP_PREPARSED_DATA,
		_ foundation.PSTR,
		_ uint32,
	) foundation.NTSTATUS {
		copyFixtureData(first, *count, fixture.data)

		*count = fixtureLength32(t, len(fixture.data))

		return fixture.status
	})
}

func installWindowFixtureStep2Continue(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()
	replaceNative(t, &api.window.registerClass, func(*wm.WNDCLASSW) (uint16, error) {
		if fixture.err != nil {
			return noValue, fixture.err
		}

		return singleValue, nil
	})
	installWindowFixtureStep3(t, fixture, api)
}

func installWindowFixtureStep3Continue(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()
	replaceNative(t, &api.window.destroyWindow, func(foundation.HWND) error {
		return fixture.err
	})
	installWindowFixtureStep4(t, fixture, api)
}

func installWindowFixtureStep5Continue(t *testing.T, api *nativeAPI) {
	t.Helper()
	replaceNative(
		t,
		&api.messages.peekMessage,
		func(msg *wm.MSG, _ foundation.HWND, _, _ uint32, _ wm.PEEK_MESSAGE_REMOVE_TYPE) bool {
			msg.Message = messageQuit

			return true
		},
	)
	replaceNative(t, &api.messages.dispatchMessage, func(*wm.MSG) foundation.LRESULT {
		return noValue
	})
}

func fixtureSubscription(
	t *testing.T,
	api *nativeAPI,
	kind uint32,
) (fixture *nativeFixture, subscription *capture, stamp *domain.Timestamp) {
	t.Helper()

	fixture = newNativeFixture(t, api)
	subscription = fixture.capture(t, kind, api)

	return fixture, subscription, new(domain.Timestamp)
}

func fixtureScenario(t *testing.T, api *nativeAPI) *nativeScenario {
	t.Helper()

	ctx, cancel := nativeCancel(t.Context())

	return &nativeScenario{
		fixture: newNativeFixture(t, api),
		owner:   fixtureBackend(t),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func fixtureWindowScenario(t *testing.T, api *nativeAPI) (fixture *nativeFixture, owner *backend) {
	t.Helper()

	owner = fixtureBackend(t)
	owner.hwnd = secondValue

	return newNativeFixture(t, api), owner
}

func fixtureCapabilitiesReader[Value any](
	t *testing.T,
	fixture *nativeFixture,
	values *[]Value,
) func(hid.HIDP_REPORT_TYPE, *Value, *uint16, hid.PHIDP_PREPARSED_DATA) foundation.NTSTATUS {
	t.Helper()

	return func(_ hid.HIDP_REPORT_TYPE, first *Value, count *uint16, _ hid.PHIDP_PREPARSED_DATA) foundation.NTSTATUS {
		copyFixtureData(first, *count, *values)

		*count = fixtureLength16(t, len(*values))

		return fixture.status
	}
}

func copyFixtureData[Value any, Count ~uint16 | ~uint32](
	first *Value,
	count Count,
	values []Value,
) {
	copy(nativeSlice(first, int(count)), values)
}

func installHIDFixtureStep3Finish(
	t *testing.T,
	fixture *nativeFixture,
	args *installHIDFixtureStep3Args,
) {
	t.Helper()
	replaceNative(
		t,
		&args.api.hid.hidPMaxDataListLength,
		func(hid.HIDP_REPORT_TYPE, hid.PHIDP_PREPARSED_DATA) uint32 {
			return fixture.maxData
		},
	)
	installHIDFixtureStep3Continue(t, fixture, args)
}

func installInventoryFixtureFinish(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()
	replaceNative(t, &api.inventory.getRawInputDeviceInfo, fixtureDeviceInfoReader(t, fixture))
	replaceNative(t, &api.read.getRawInputData, fixtureInputReader(t, fixture))
	replaceNative(t, &api.read.getLastError, func() error {
		return fixture.err
	})
	replaceNative(t, &api.inventory.createFile, fixtureFileOpener(fixture))
}

func installWindowFixtureStep3Finish(t *testing.T, fixture *nativeFixture, api *nativeAPI) {
	t.Helper()
	replaceNative(
		t,
		&api.window.defWindowProc,
		func(foundation.HWND, uint32, foundation.WPARAM, foundation.LPARAM) foundation.LRESULT {
			return seventhValue
		},
	)
	installWindowFixtureStep3Continue(t, fixture, api)
}

func fixtureWindowResult(fixture *nativeFixture) (foundation.HWND, error) {
	if fixture.err != nil {
		return nativeFailure[foundation.HWND](fixture.err)
	}

	return secondValue, nil
}
