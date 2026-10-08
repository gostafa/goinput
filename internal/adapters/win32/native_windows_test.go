// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

import (
	"context"
	"encoding/binary"
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

type nativeFixture struct {
	inventory     []input.RAWINPUTDEVICELIST
	registrations []input.RAWINPUTDEVICE
	packet        []byte
	path          string
	words         [sixthValue]uint32
	caps          hid.HIDP_CAPS
	buttons       []hid.HIDP_BUTTON_CAPS
	values        []hid.HIDP_VALUE_CAPS
	data          []hid.HIDP_DATA
	err           error
	status        foundation.NTSTATUS
	maxData       uint32
	callback      func(foundation.HWND, uint32, foundation.WPARAM, foundation.LPARAM) foundation.LRESULT
	events        []domain.Event
	failures      []error
}

// All native tests run serially inside one parallel top-level test. Portable
// tests do not use these bindings, and every replacement is restored at cleanup.
func TestNativeAdapter(t *testing.T) {
	t.Parallel()
	t.Run("commands", testNativeCommands)
	t.Run("lifecycle", testNativeLifecycle)
	t.Run("inventory", testNativeInventory)
	t.Run("registration", testNativeRegistration)
	t.Run("metadata", testNativeMetadata)
	t.Run("capabilities", testNativeCapabilities)
	t.Run("input", testNativeInput)
	t.Run("hid", testNativeHID)
	t.Run("public operations", testNativeViews)
	t.Run("late failures", testNativeLateFailures)
}

func replaceNative[Value any](t *testing.T, target *Value, value Value) {
	t.Helper()
	previous := *target
	*target = value
	t.Cleanup(func() { *target = previous })
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

func newNativeFixture(t *testing.T) *nativeFixture {
	t.Helper()
	fixture := &nativeFixture{path: "TEST-DEVICE", status: hid.HIDP_STATUS_SUCCESS, maxData: 4}
	fixture.caps.InputReportByteLength = 2
	fixture.words[1], fixture.words[3] = 3, 1
	installWindowFixture(t, fixture)
	installInventoryFixture(t, fixture)
	installHIDFixture(t, fixture)
	return fixture
}

func installWindowFixture(t *testing.T, fixture *nativeFixture) {
	t.Helper()
	replaceNative(t, &winFindProcedure, func(*native.Proc) error { return fixture.err })
	replaceNative(t, &winNewCallback, func(value any) uintptr {
		fixture.callback = value.(func(foundation.HWND, uint32, foundation.WPARAM, foundation.LPARAM) foundation.LRESULT)
		return 1
	})
	replaceNative(
		t,
		&winCreateEvent,
		func(*security.SECURITY_ATTRIBUTES, bool, bool, *string) (foundation.HANDLE, error) {
			return 1, fixture.err
		},
	)
	replaceNative(
		t,
		&winGetModuleHandle,
		func(*string) (foundation.HMODULE, error) { return 1, fixture.err },
	)
	replaceNative(
		t,
		&winRegisterClass,
		func(*wm.WNDCLASSW) (uint16, error) { return 1, fixture.err },
	)
	replaceNative(
		t,
		&winCreateWindowEx,
		func(wm.WINDOW_EX_STYLE, *string, *string, wm.WINDOW_STYLE, int32, int32, int32, int32, foundation.HWND, wm.HMENU, foundation.HINSTANCE, unsafe.Pointer) (foundation.HWND, error) {
			return 2, fixture.err
		},
	)
	replaceNative(
		t,
		&winDefWindowProc,
		func(foundation.HWND, uint32, foundation.WPARAM, foundation.LPARAM) foundation.LRESULT { return 7 },
	)
	replaceNative(t, &winDestroyWindow, func(foundation.HWND) error { return fixture.err })
	replaceNative(
		t,
		&winUnregisterClass,
		func(string, foundation.HINSTANCE) error { return fixture.err },
	)
	replaceNative(t, &winCloseHandle, func(foundation.HANDLE) error { return fixture.err })
	replaceNative(t, &winSetEvent, func(foundation.HANDLE) error { return fixture.err })
	replaceNative(
		t,
		&winMsgWaitForMultipleObjectsEx,
		func([]foundation.HANDLE, uint32, wm.QUEUE_STATUS_FLAGS, wm.MSG_WAIT_FOR_MULTIPLE_OBJECTS_EX_FLAGS) (foundation.WAIT_EVENT, error) {
			return 1, fixture.err
		},
	)
	replaceNative(
		t,
		&winPeekMessage,
		func(msg *wm.MSG, _ foundation.HWND, _, _ uint32, _ wm.PEEK_MESSAGE_REMOVE_TYPE) bool {
			msg.Message = messageQuit
			return true
		},
	)
	replaceNative(t, &winDispatchMessage, func(*wm.MSG) foundation.LRESULT { return 0 })
}

func installInventoryFixture(t *testing.T, fixture *nativeFixture) {
	t.Helper()
	replaceNative(
		t,
		&winGetRawInputDeviceList,
		func(first *input.RAWINPUTDEVICELIST, count *uint32, _ uint32) (uint32, error) {
			if fixture.err != nil {
				return infiniteWait, fixture.err
			}
			if first == nil {
				*count = uint32(len(fixture.inventory))
				return 0, nil
			}
			copy(unsafe.Slice(first, int(*count)), fixture.inventory)
			return uint32(len(fixture.inventory)), nil
		},
	)
	replaceNative(
		t,
		&winGetRegisteredRawInputDevices,
		func(first *input.RAWINPUTDEVICE, count *uint32, _ uint32) (uint32, error) {
			if fixture.err != nil {
				return infiniteWait, fixture.err
			}
			if first == nil {
				*count = uint32(len(fixture.registrations))
				return 0, nil
			}
			copy(unsafe.Slice(first, int(*count)), fixture.registrations)
			return uint32(len(fixture.registrations)), nil
		},
	)
	replaceNative(
		t,
		&winRegisterRawInputDevices,
		func(devices []input.RAWINPUTDEVICE, _ uint32) error {
			if fixture.err != nil {
				return fixture.err
			}
			for _, device := range devices {
				fixture.registrations = slices.DeleteFunc(
					fixture.registrations,
					func(current input.RAWINPUTDEVICE) bool {
						return current.UsUsagePage == device.UsUsagePage &&
							current.UsUsage == device.UsUsage
					},
				)
				if uint32(device.DwFlags)&1 == 0 {
					fixture.registrations = append(fixture.registrations, device)
				}
			}
			return nil
		},
	)
	replaceNative(
		t,
		&winGetRawInputDeviceInfo,
		func(_ foundation.HANDLE, command input.RAW_INPUT_DEVICE_INFO_COMMAND, data unsafe.Pointer, count *uint32) (uint32, error) {
			if fixture.err != nil {
				return infiniteWait, fixture.err
			}
			switch command {
			case deviceInfoCommand:
				(*input.RID_DEVICE_INFO)(data).Anonymous.Data = fixture.words
			case deviceNameCommand:
				wide, err := syscall.UTF16FromString(fixture.path)
				if err != nil {
					return infiniteWait, err
				}
				if data == nil {
					*count = uint32(len(wide))
					return 0, nil
				}
				copy(unsafe.Slice((*uint16)(data), int(*count)), wide)
			default:
				if data == nil {
					*count = 1
					return 0, nil
				}
				*(*byte)(data) = 1
			}
			return *count, nil
		},
	)
	replaceNative(
		t,
		&winGetRawInputData,
		func(_ input.HRAWINPUT, _ input.RAW_INPUT_DATA_COMMAND_FLAGS, data unsafe.Pointer, count *uint32, _ uint32) uint32 {
			if fixture.err != nil {
				return infiniteWait
			}
			if data == nil {
				*count = uint32(len(fixture.packet))
				return 0
			}
			copy(unsafe.Slice((*byte)(data), int(*count)), fixture.packet)
			return uint32(len(fixture.packet))
		},
	)
	replaceNative(t, &winGetLastError, func() error { return fixture.err })
	replaceNative(
		t,
		&winCreateFile,
		func(string, uint32, filesystem.FILE_SHARE_MODE, *security.SECURITY_ATTRIBUTES, filesystem.FILE_CREATION_DISPOSITION, filesystem.FILE_FLAGS_AND_ATTRIBUTES, foundation.HANDLE) (foundation.HANDLE, error) {
			return 3, fixture.err
		},
	)
}

func installHIDFixture(t *testing.T, fixture *nativeFixture) {
	t.Helper()
	replaceNative(
		t,
		&winHidD_GetAttributes,
		func(_ foundation.HANDLE, attributes *hid.HIDD_ATTRIBUTES) foundation.BOOLEAN {
			attributes.VendorID, attributes.ProductID, attributes.VersionNumber = 4, 5, 6
			return 1
		},
	)
	getString := func(_ foundation.HANDLE, data []byte) foundation.BOOLEAN {
		binary.LittleEndian.PutUint16(data, 'A')
		return 1
	}
	replaceNative(t, &winHidD_GetProductString, getString)
	replaceNative(t, &winHidD_GetManufacturerString, getString)
	replaceNative(t, &winHidD_GetSerialNumberString, getString)
	replaceNative(
		t,
		&winHidP_GetCaps,
		func(_ hid.PHIDP_PREPARSED_DATA, caps *hid.HIDP_CAPS) foundation.NTSTATUS {
			*caps = fixture.caps
			return fixture.status
		},
	)
	replaceNative(
		t,
		&winHidP_MaxDataListLength,
		func(hid.HIDP_REPORT_TYPE, hid.PHIDP_PREPARSED_DATA) uint32 { return fixture.maxData },
	)
	replaceNative(
		t,
		&winHidP_GetButtonCaps,
		func(_ hid.HIDP_REPORT_TYPE, first *hid.HIDP_BUTTON_CAPS, count *uint16, _ hid.PHIDP_PREPARSED_DATA) foundation.NTSTATUS {
			copy(unsafe.Slice(first, int(*count)), fixture.buttons)
			*count = uint16(len(fixture.buttons))
			return fixture.status
		},
	)
	replaceNative(
		t,
		&winHidP_GetValueCaps,
		func(_ hid.HIDP_REPORT_TYPE, first *hid.HIDP_VALUE_CAPS, count *uint16, _ hid.PHIDP_PREPARSED_DATA) foundation.NTSTATUS {
			copy(unsafe.Slice(first, int(*count)), fixture.values)
			*count = uint16(len(fixture.values))
			return fixture.status
		},
	)
	replaceNative(
		t,
		&winHidP_GetData,
		func(_ hid.HIDP_REPORT_TYPE, first *hid.HIDP_DATA, count *uint32, _ hid.PHIDP_PREPARSED_DATA, _ foundation.PSTR, _ uint32) foundation.NTSTATUS {
			copy(unsafe.Slice(first, int(*count)), fixture.data)
			*count = uint32(len(fixture.data))
			return fixture.status
		},
	)
}

func (fixture *nativeFixture) capture(t *testing.T, kind uint32) *capture {
	t.Helper()
	owner := makeBackend(&nativeState{tables: testKeyTables(t)}, nil)
	device := &nativeDevice{
		kind:   kind,
		handle: 1,
		tlc:    topLevel{1, 2},
		info:   domain.DeviceInfo{ID: "device"},
	}
	view := new(sinkview.Operations[domain.Event])
	view.Operations.Publish = func(event *domain.Event) bool { fixture.events = append(fixture.events, *event); return true }
	view.Operations.Fail = func(err error) { fixture.failures = append(fixture.failures, err) }
	subscription := backendMakeCapture(owner, device, view)
	subscription.backend.call = func(ctx context.Context, operation func() error) error { return operation() }
	subscription.backend.retry = func(ctx context.Context, operation func(context.Context) error) error { return operation(ctx) }
	return subscription
}
