// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

import (
	"syscall"
	"unsafe"

	native "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	hid "github.com/deploymenttheory/go-bindings-win32/bindings/win32/devices/humaninterfacedevice"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/security"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/storage/filesystem"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/libraryloader"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/threading"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/input"
	wm "github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/windowsandmessaging"
)

// Each factory and test owns its native bindings.
type (
	nativeWindowCallsRecord[
		GetModuleHandleCall any,
		RegisterClassCall any,
		CreateWindowExCall any,
		DefWindowProcCall any,
		DestroyWindowCall any,
		UnregisterClassCall any,
		NewCallbackCall any,
	] struct {
		getModuleHandle GetModuleHandleCall
		registerClass   RegisterClassCall
		createWindowEx  CreateWindowExCall
		defWindowProc   DefWindowProcCall
		destroyWindow   DestroyWindowCall
		unregisterClass UnregisterClassCall
		newCallback     NewCallbackCall
	}
	nativeWindowCalls = nativeWindowCallsRecord[
		nativeGetModuleHandleCall,
		nativeRegisterClassCall,
		nativeCreateWindowExCall,
		nativeDefWindowProcCall,
		nativeDestroyWindowCall,
		nativeUnregisterClassCall,
		nativeNewCallbackCall,
	]
	nativeMessagesCallsRecord[
		CreateEventCall any,
		CloseHandleCall any,
		SetEventCall any,
		MsgWaitForMultipleObjectsExCall any,
		PeekMessageCall any,
		DispatchMessageCall any,
	] struct {
		createEvent                 CreateEventCall
		closeHandle                 CloseHandleCall
		setEvent                    SetEventCall
		msgWaitForMultipleObjectsEx MsgWaitForMultipleObjectsExCall
		peekMessage                 PeekMessageCall
		dispatchMessage             DispatchMessageCall
	}
	nativeMessagesCalls = nativeMessagesCallsRecord[
		nativeCreateEventCall,
		nativeCloseHandleCall,
		nativeSetEventCall,
		nativeMsgWaitForMultipleObjectsExCall,
		nativePeekMessageCall,
		nativeDispatchMessageCall,
	]
	nativeHidCallsRecord[
		HidDGetAttributesCall any,
		HidDGetManufacturerStringCall any,
		HidDGetProductStringCall any,
		HidDGetSerialNumberStringCall any,
		HidPGetButtonCapsCall any,
		HidPGetCapsCall any,
		HidPGetDataCall any,
		HidPGetValueCapsCall any,
		HidPMaxDataListLengthCall any,
	] struct {
		hidDGetAttributes         HidDGetAttributesCall
		hidDGetManufacturerString HidDGetManufacturerStringCall
		hidDGetProductString      HidDGetProductStringCall
		hidDGetSerialNumberString HidDGetSerialNumberStringCall
		hidPGetButtonCaps         HidPGetButtonCapsCall
		hidPGetCaps               HidPGetCapsCall
		hidPGetData               HidPGetDataCall
		hidPGetValueCaps          HidPGetValueCapsCall
		hidPMaxDataListLength     HidPMaxDataListLengthCall
	}
	nativeHidCalls = nativeHidCallsRecord[
		nativeHidDGetAttributesCall,
		nativeHidDGetManufacturerStringCall,
		nativeHidDGetProductStringCall,
		nativeHidDGetSerialNumberStringCall,
		nativeHidPGetButtonCapsCall,
		nativeHidPGetCapsCall,
		nativeHidPGetDataCall,
		nativeHidPGetValueCapsCall,
		nativeHidPMaxDataListLengthCall,
	]
	nativeInventoryCallsRecord[
		GetRawInputDeviceListCall any,
		GetRegisteredRawInputDevicesCall any,
		RegisterRawInputDevicesCall any,
		GetRawInputDeviceInfoCall any,
		CreateFileCall any,
	] struct {
		getRawInputDeviceList        GetRawInputDeviceListCall
		getRegisteredRawInputDevices GetRegisteredRawInputDevicesCall
		registerRawInputDevices      RegisterRawInputDevicesCall
		getRawInputDeviceInfo        GetRawInputDeviceInfoCall
		createFile                   CreateFileCall
	}
	nativeInventoryCalls = nativeInventoryCallsRecord[
		nativeGetRawInputDeviceListCall,
		nativeGetRegisteredRawInputDevicesCall,
		nativeRegisterRawInputDevicesCall,
		nativeGetRawInputDeviceInfoCall,
		nativeCreateFileCall,
	]
	nativeReadCallsRecord[
		FindProcedureCall any,
		GetRawInputDataCall any,
		GetLastErrorCall any,
	] struct {
		findProcedure   FindProcedureCall
		getRawInputData GetRawInputDataCall
		getLastError    GetLastErrorCall
	}
	nativeReadCalls = nativeReadCallsRecord[
		nativeFindProcedureCall,
		nativeGetRawInputDataCall,
		nativeGetLastErrorCall,
	]
	nativeAPIRecord[
		Window any,
		Messages any,
		HID any,
		Inventory any,
		Read any,
	] struct {
		window    Window
		messages  Messages
		hid       HID
		inventory Inventory
		read      Read
	}
	nativeAPI = nativeAPIRecord[
		nativeWindowCalls,
		nativeMessagesCalls,
		nativeHidCalls,
		nativeInventoryCalls,
		nativeReadCalls,
	]
	nativeFindProcedureCall = func(*native.Proc) error
	nativeCreateFileCall    = func(
		string,
		uint32,
		filesystem.FILE_SHARE_MODE,
		*security.SECURITY_ATTRIBUTES,
		filesystem.FILE_CREATION_DISPOSITION,
		filesystem.FILE_FLAGS_AND_ATTRIBUTES,
		foundation.HANDLE,
	) (foundation.HANDLE, error)
	nativeCloseHandleCall       = func(foundation.HANDLE) error
	nativeHidDGetAttributesCall = func(
		foundation.HANDLE,
		*hid.HIDD_ATTRIBUTES,
	) foundation.BOOLEAN
	nativeHidDGetManufacturerStringCall = func(
		foundation.HANDLE,
		[]byte,
	) foundation.BOOLEAN
	nativeHidDGetProductStringCall = func(
		foundation.HANDLE,
		[]byte,
	) foundation.BOOLEAN
	nativeHidDGetSerialNumberStringCall = func(
		foundation.HANDLE,
		[]byte,
	) foundation.BOOLEAN
	nativeHidPGetButtonCapsCall = func(
		hid.HIDP_REPORT_TYPE,
		*hid.HIDP_BUTTON_CAPS,
		*uint16,
		hid.PHIDP_PREPARSED_DATA,
	) foundation.NTSTATUS
	nativeHidPGetCapsCall = func(
		hid.PHIDP_PREPARSED_DATA,
		*hid.HIDP_CAPS,
	) foundation.NTSTATUS
	nativeHidPGetDataCall = func(
		hid.HIDP_REPORT_TYPE,
		*hid.HIDP_DATA,
		*uint32,
		hid.PHIDP_PREPARSED_DATA,
		foundation.PSTR,
		uint32,
	) foundation.NTSTATUS
	nativeHidPGetValueCapsCall = func(
		hid.HIDP_REPORT_TYPE,
		*hid.HIDP_VALUE_CAPS,
		*uint16,
		hid.PHIDP_PREPARSED_DATA,
	) foundation.NTSTATUS
	nativeHidPMaxDataListLengthCall = func(
		hid.HIDP_REPORT_TYPE,
		hid.PHIDP_PREPARSED_DATA,
	) uint32
	nativeGetRawInputDataCall = func(
		input.HRAWINPUT,
		input.RAW_INPUT_DATA_COMMAND_FLAGS,
		unsafe.Pointer,
		*uint32,
		uint32,
	) uint32
	nativeGetRawInputDeviceInfoCall = func(
		foundation.HANDLE,
		input.RAW_INPUT_DEVICE_INFO_COMMAND,
		unsafe.Pointer,
		*uint32,
	) (uint32, error)
	nativeGetRawInputDeviceListCall = func(
		*input.RAWINPUTDEVICELIST,
		*uint32,
		uint32,
	) (uint32, error)
	nativeGetRegisteredRawInputDevicesCall = func(
		*input.RAWINPUTDEVICE,
		*uint32,
		uint32,
	) (uint32, error)
	nativeRegisterRawInputDevicesCall = func(
		[]input.RAWINPUTDEVICE,
		uint32,
	) error
	nativeGetModuleHandleCall = func(*string) (foundation.HMODULE, error)
	nativeCreateEventCall     = func(
		*security.SECURITY_ATTRIBUTES,
		bool,
		bool,
		*string,
	) (foundation.HANDLE, error)
	nativeSetEventCall       = func(foundation.HANDLE) error
	nativeCreateWindowExCall = func(
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
	) (foundation.HWND, error)
	nativeDefWindowProcCall = func(
		foundation.HWND,
		uint32,
		foundation.WPARAM,
		foundation.LPARAM,
	) foundation.LRESULT
	nativeDestroyWindowCall               = func(foundation.HWND) error
	nativeDispatchMessageCall             = func(*wm.MSG) foundation.LRESULT
	nativeMsgWaitForMultipleObjectsExCall = func(
		[]foundation.HANDLE,
		uint32,
		wm.QUEUE_STATUS_FLAGS,
		wm.MSG_WAIT_FOR_MULTIPLE_OBJECTS_EX_FLAGS,
	) (foundation.WAIT_EVENT, error)
	nativePeekMessageCall = func(
		*wm.MSG,
		foundation.HWND,
		uint32,
		uint32,
		wm.PEEK_MESSAGE_REMOVE_TYPE,
	) bool
	nativeRegisterClassCall   = func(*wm.WNDCLASSW) (uint16, error)
	nativeUnregisterClassCall = func(
		string,
		foundation.HINSTANCE,
	) error
	nativeGetLastErrorCall = func() error
	nativeNewCallbackCall  = func(any) uintptr
)

func newReadCalls() nativeReadCalls {
	return nativeReadCalls{
		findProcedure:   (*native.Proc).Find,
		getRawInputData: input.GetRawInputData,
		getLastError:    syscall.GetLastError,
	}
}

func newInventoryCalls() nativeInventoryCalls {
	return nativeInventoryCalls{
		createFile:                   filesystem.CreateFile,
		getRawInputDeviceInfo:        input.GetRawInputDeviceInfo,
		getRawInputDeviceList:        input.GetRawInputDeviceList,
		getRegisteredRawInputDevices: input.GetRegisteredRawInputDevices,
		registerRawInputDevices:      input.RegisterRawInputDevices,
	}
}

func newMessagesCalls() nativeMessagesCalls {
	return nativeMessagesCalls{
		closeHandle:                 foundation.CloseHandle,
		createEvent:                 threading.CreateEvent,
		setEvent:                    threading.SetEvent,
		dispatchMessage:             wm.DispatchMessage,
		msgWaitForMultipleObjectsEx: wm.MsgWaitForMultipleObjectsEx,
		peekMessage:                 wm.PeekMessage,
	}
}

func newHidCalls() nativeHidCalls {
	return nativeHidCalls{
		hidDGetAttributes:         hid.HidD_GetAttributes,
		hidDGetManufacturerString: hid.HidD_GetManufacturerString,
		hidDGetProductString:      hid.HidD_GetProductString,
		hidDGetSerialNumberString: hid.HidD_GetSerialNumberString,
		hidPGetButtonCaps:         hid.HidP_GetButtonCaps,
		hidPGetCaps:               hid.HidP_GetCaps,
		hidPGetData:               hid.HidP_GetData,
		hidPGetValueCaps:          hid.HidP_GetValueCaps,
		hidPMaxDataListLength:     hid.HidP_MaxDataListLength,
	}
}

func newWindowCalls() nativeWindowCalls {
	return nativeWindowCalls{
		getModuleHandle: libraryloader.GetModuleHandle,
		createWindowEx:  wm.CreateWindowEx,
		defWindowProc:   wm.DefWindowProc,
		destroyWindow:   wm.DestroyWindow,
		registerClass:   wm.RegisterClass,
		unregisterClass: wm.UnregisterClass,
		newCallback:     syscall.NewCallback,
	}
}

func newNativeAPI() *nativeAPI {
	return &nativeAPI{
		read:      newReadCalls(),
		inventory: newInventoryCalls(),
		messages:  newMessagesCalls(),
		hid:       newHidCalls(),
		window:    newWindowCalls(),
	}
}
