// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

import (
	native "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	hid "github.com/deploymenttheory/go-bindings-win32/bindings/win32/devices/humaninterfacedevice"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/storage/filesystem"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/libraryloader"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/threading"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/input"
	wm "github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/windowsandmessaging"
	"syscall"
)

// Native calls are bound once in production; serial Windows tests replace them
// to exercise driver failures without changing process-wide input registrations.
var (
	winFindProcedure                = (*native.Proc).Find
	winCreateFile                   = filesystem.CreateFile
	winCloseHandle                  = foundation.CloseHandle
	winHidD_GetAttributes           = hid.HidD_GetAttributes
	winHidD_GetManufacturerString   = hid.HidD_GetManufacturerString
	winHidD_GetProductString        = hid.HidD_GetProductString
	winHidD_GetSerialNumberString   = hid.HidD_GetSerialNumberString
	winHidP_GetButtonCaps           = hid.HidP_GetButtonCaps
	winHidP_GetCaps                 = hid.HidP_GetCaps
	winHidP_GetData                 = hid.HidP_GetData
	winHidP_GetValueCaps            = hid.HidP_GetValueCaps
	winHidP_MaxDataListLength       = hid.HidP_MaxDataListLength
	winGetRawInputData              = input.GetRawInputData
	winGetRawInputDeviceInfo        = input.GetRawInputDeviceInfo
	winGetRawInputDeviceList        = input.GetRawInputDeviceList
	winGetRegisteredRawInputDevices = input.GetRegisteredRawInputDevices
	winRegisterRawInputDevices      = input.RegisterRawInputDevices
	winGetModuleHandle              = libraryloader.GetModuleHandle
	winCreateEvent                  = threading.CreateEvent
	winSetEvent                     = threading.SetEvent
	winCreateWindowEx               = wm.CreateWindowEx
	winDefWindowProc                = wm.DefWindowProc
	winDestroyWindow                = wm.DestroyWindow
	winDispatchMessage              = wm.DispatchMessage
	winMsgWaitForMultipleObjectsEx  = wm.MsgWaitForMultipleObjectsEx
	winPeekMessage                  = wm.PeekMessage
	winRegisterClass                = wm.RegisterClass
	winUnregisterClass              = wm.UnregisterClass
	winGetLastError                 = syscall.GetLastError
	winNewCallback                  = syscall.NewCallback
)
