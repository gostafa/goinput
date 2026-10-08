// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

import (
	"context"
	"syscall"
	"testing"
	"unsafe"

	hid "github.com/deploymenttheory/go-bindings-win32/bindings/win32/devices/humaninterfacedevice"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/security"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/storage/filesystem"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/input"
	"github.com/gostafa/goinput/internal/domain"
)

type nativeRetrier struct{ calls int }

func (retry *nativeRetrier) Do(
	ctx context.Context,
	operation func(context.Context) error,
	transient func(error) bool,
) error {
	retry.calls++
	return operation(ctx)
}

func testNativeInventory(t *testing.T) {
	fixture := newNativeFixture(t)
	owner := fixtureBackend(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := enumerateRawDevices(ctx)
	assertCoreError(t, err, context.Canceled)
	fixture.err = syscall.ERROR_ACCESS_DENIED
	_, err = backendEnumerateDevices(t.Context(), owner)
	assertCoreError(t, err, domain.ErrPermissionDenied)
	fixture.err = nil
	infos, err := backendDiscover(t.Context(), owner)
	nativeOK(t, err)
	assertCoreEqual(t, len(infos), 0)
	fixture.inventory = []input.RAWINPUTDEVICELIST{
		{HDevice: 1, DwType: input.RIM_TYPEKEYBOARD},
		{HDevice: 2, DwType: input.RIM_TYPEMOUSE},
	}
	infos, err = backendDiscover(t.Context(), owner)
	nativeOK(t, err)
	assertCoreEqual(t, len(infos), 2)
	assertCoreEqual(t, infos[0].ID, domain.DeviceID("win32:test-device"))
	assertCoreEqual(t, infos[0].Name, "A")
	devices, err := backendDescribeDevices(ctx, owner, fixture.inventory)
	assertCoreError(t, err, context.Canceled)
	assertCoreEqual(t, len(devices), 0)
	fixture.err = domain.ErrUnsupported
	devices, err = backendDescribeDevices(t.Context(), owner, fixture.inventory)
	assertCoreError(t, err, domain.ErrUnsupported)
	assertCoreEqual(t, len(devices), 0)
	_, err = backendFindDevice(t.Context(), owner, "missing")
	assertCoreError(t, err, domain.ErrUnsupported)
	fixture.err = nil
	device, err := backendFindDevice(t.Context(), owner, "win32:test-device")
	nativeOK(t, err)
	assertCoreEqual(t, device.info.ID, domain.DeviceID("win32:test-device"))
	_, err = backendFindDevice(t.Context(), owner, "missing")
	assertCoreError(t, err, domain.ErrNotFound)
	_, err = selectDevice(nil, "missing", domain.ErrUnsupported)
	assertCoreError(t, err, domain.ErrUnsupported)
	owner.closed = true
	_, err = backendEnumerateDevices(t.Context(), owner)
	assertCoreError(t, err, domain.ErrClosed)
	owner.closed = false
	retry := new(nativeRetrier)
	owner.retrier = retry
	nativeOK(t, backendRetry(t.Context(), owner, func(context.Context) error { return nil }))
	assertCoreEqual(t, retry.calls, 1)

	_, err = readDeviceInventory(
		0,
		1,
		func(uint32, uint32) ([]byte, error) { t.Fatal("empty inventory read"); return nil, nil },
	)
	nativeOK(t, err)
	_, err = readDeviceInventory(
		maxDevices+1,
		1,
		func(uint32, uint32) ([]byte, error) { t.Fatal("oversized inventory read"); return nil, nil },
	)
	assertCoreError(t, err, domain.ErrInvalidOptions)
	replaceNative(
		t,
		&winGetRawInputDeviceList,
		func(*input.RAWINPUTDEVICELIST, *uint32, uint32) (uint32, error) { return 2, nil },
	)
	_, err = readRawDeviceList(1, 1)
	assertCoreError(t, err, domain.ErrInvalidOptions)
	replaceNative(
		t,
		&winGetRawInputDeviceList,
		func(*input.RAWINPUTDEVICELIST, *uint32, uint32) (uint32, error) {
			return infiniteWait, domain.ErrUnsupported
		},
	)
	_, err = readRawDeviceList(1, 1)
	assertCoreError(t, err, domain.ErrUnsupported)
}

func testNativeRegistration(t *testing.T) {
	fixture := newNativeFixture(t)
	owner := fixtureBackend(t)
	owner.hwnd = 2
	usage := topLevel{1, 2}
	subscription := fixture.capture(t, 0)
	assertCoreError(t, backendAcquireRegistration(owner, topLevel{}), domain.ErrUnsupported)
	nativeOK(t, captureRegister(owner, subscription))
	assertCoreEqual(t, owner.registrations[usage], 1)
	nativeOK(t, captureRegister(owner, subscription))
	assertCoreEqual(t, owner.registrations[usage], 2)
	nativeOK(t, backendReleaseRegistration(owner, usage))
	assertCoreEqual(t, owner.registrations[usage], 1)
	nativeOK(t, captureUnregister(owner, subscription))
	assertCoreEqual(t, len(owner.captures), 0)
	assertCoreEqual(t, len(fixture.registrations), 0)
	nativeOK(t, captureUnregister(owner, subscription))
	nativeOK(t, backendReleaseRegistration(owner, usage))

	fixture.registrations = []input.RAWINPUTDEVICE{{UsUsagePage: 1, UsUsage: 2, HwndTarget: 9}}
	assertCoreError(t, backendAcquireRegistration(owner, usage), domain.ErrRegistrationConflict)
	assertCoreError(t, captureRegister(owner, subscription), domain.ErrRegistrationConflict)
	assertCoreError(t, backendRemoveRegistration(owner, usage), domain.ErrRegistrationConflict)
	assertCoreEqual(
		t,
		matchesRegistration(
			usage,
			input.RAWINPUTDEVICE{UsUsagePage: 1, DwFlags: input.RAWINPUTDEVICE_FLAGS(32)},
		),
		true,
	)
	assertCoreEqual(
		t,
		matchesRegistration(usage, input.RAWINPUTDEVICE{UsUsagePage: 9, UsUsage: 2}),
		false,
	)
	nativeOK(t, backendRemoveRegistration(owner, topLevel{1, 6}))
	fixture.err = domain.ErrUnsupported
	assertCoreError(t, backendAcquireRegistration(owner, usage), domain.ErrUnsupported)
	assertCoreError(t, backendRemoveRegistration(owner, usage), domain.ErrUnsupported)
	assertCoreError(t, backendRetainRegistration(owner, usage), domain.ErrUnsupported)
	owner.registrations[usage] = 1
	assertCoreError(t, backendReleaseRegistration(owner, usage), domain.ErrUnsupported)
	assertCoreEqual(t, owner.registrations[usage], 0)
	_, err := readRegisteredDevices(1, 1)
	assertCoreError(t, err, domain.ErrUnsupported)
	fixture.err = nil
	replaceNative(
		t,
		&winGetRegisteredRawInputDevices,
		func(*input.RAWINPUTDEVICE, *uint32, uint32) (uint32, error) { return infiniteWait, nil },
	)
	_, err = registeredDevices()
	assertCoreError(t, err, syscall.EINVAL)
	replaceNative(
		t,
		&winGetRegisteredRawInputDevices,
		func(*input.RAWINPUTDEVICE, *uint32, uint32) (uint32, error) { return 2, nil },
	)
	_, err = readRegisteredDevices(1, 1)
	assertCoreError(t, err, domain.ErrInvalidOptions)

	owner = fixtureBackend(t)
	owner.captures[1] = map[*capture]struct{}{subscription: {}}
	backendDisconnect(owner, foundation.HANDLE(1))
	assertCoreEqual(t, len(owner.captures), 0)
	assertCoreError(t, fixture.failures[0], domain.ErrDisconnected)
	owner.captures[1] = map[*capture]struct{}{subscription: {}}
	backendDisconnect(owner, 1)
	assertCoreEqual(t, len(fixture.failures), 1)
}

func testNativeMetadata(t *testing.T) {
	fixture := newNativeFixture(t)
	for _, kind := range []uint32{0, 1, 2, 3} {
		device := &nativeDevice{kind: kind}
		err := nativeDeviceApplyDeviceInfo(device, fixture.words)
		if kind == 3 {
			assertCoreError(t, err, domain.ErrUnsupported)
			continue
		}
		nativeOK(t, err)
		assertCoreEqual(t, len(device.info.Classes), 1)
	}
	for _, item := range []struct {
		usage uint16
		class domain.DeviceClass
	}{{2, domain.ClassMouse}, {4, domain.ClassJoystick}, {5, domain.ClassGamepad}, {6, domain.ClassKeyboard}, {7, domain.ClassKeyboard}, {99, domain.ClassOther}} {
		assertCoreEqual(t, classFor(topLevel{1, item.usage}), item.class)
	}
	assertCoreEqual(t, classFor(topLevel{2, 2}), domain.ClassOther)
	assertCoreEqual(t, optionalWord(wordMask+1) == nil, true)
	assertCoreEqual(t, *optionalWord(1), uint16(1))
	device := new(nativeDevice)
	nativeOK(t, nativeDeviceLoadDevicePath(device))
	assertCoreEqual(t, device.info.Path, fixture.path)
	fixture.err = domain.ErrUnsupported
	assertCoreError(t, nativeDeviceLoadDevicePath(device), domain.ErrUnsupported)
	_, err := readDeviceName(0, 2)
	assertCoreError(t, err, domain.ErrUnsupported)
	nativeOK(t, enrichIdentity(device))
	fixture.err = nil
	replaceNative(
		t,
		&winCreateFile,
		func(string, uint32, filesystem.FILE_SHARE_MODE, *security.SECURITY_ATTRIBUTES, filesystem.FILE_CREATION_DISPOSITION, filesystem.FILE_FLAGS_AND_ATTRIBUTES, foundation.HANDLE) (foundation.HANDLE, error) {
			return 0, domain.ErrUnsupported
		},
	)
	nativeOK(t, enrichIdentity(device))

	fixture.path = ""
	_, err = readDeviceName(0, 2)
	assertCoreError(t, err, domain.ErrNotFound)
	replaceNative(
		t,
		&winGetRawInputDeviceInfo,
		func(foundation.HANDLE, input.RAW_INPUT_DEVICE_INFO_COMMAND, unsafe.Pointer, *uint32) (uint32, error) {
			return 0, nil
		},
	)
	_, err = readDevicePath(0)
	assertCoreError(t, err, domain.ErrNotFound)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	assertCoreError(t, enrichDeviceIdentity(ctx, device), context.Canceled)
	assertCoreEqual(
		t,
		readHIDString(0, func(foundation.HANDLE, []byte) foundation.BOOLEAN { return 0 }),
		"",
	)
	replaceNative(
		t,
		&winHidD_GetAttributes,
		func(foundation.HANDLE, *hid.HIDD_ATTRIBUTES) foundation.BOOLEAN { return 0 },
	)
	nativeDeviceLoadAttributes(device, 0)
	replaceNative(
		t,
		&winHidD_GetProductString,
		func(foundation.HANDLE, []byte) foundation.BOOLEAN { return 0 },
	)
	nativeDeviceLoadStrings(device, 0)
	assertCoreEqual(t, device.info.Name, "")

	assertCoreError(t, nonzeroError(nil), syscall.EINVAL)
	assertCoreError(t, nonzeroError(domain.ErrUnsupported), domain.ErrUnsupported)
	for _, item := range []struct {
		native error
		domain error
	}{{syscall.ERROR_ACCESS_DENIED, domain.ErrPermissionDenied}, {syscall.Errno(32), domain.ErrPermissionDenied}, {syscall.ERROR_FILE_NOT_FOUND, domain.ErrNotFound}, {syscall.ERROR_PATH_NOT_FOUND, domain.ErrNotFound}, {syscall.Errno(6), domain.ErrNotFound}, {syscall.Errno(errorDeviceDisconnected), domain.ErrDisconnected}, {syscall.Errno(errorInvalidParameter), domain.ErrInvalidOptions}} {
		assertCoreError(t, normalizeError(item.native), item.domain)
		assertCoreError(t, normalizeError(item.native), item.native)
	}
	nativeOK(t, normalizeError(nil))
	assertCoreError(t, normalizeError(domain.ErrUnsupported), domain.ErrUnsupported)
	assertCoreEqual(t, transient(syscall.Errno(errorMoreData)), true)
	assertCoreEqual(t, transient(syscall.Errno(errorNotReady)), true)
	assertCoreEqual(t, transient(syscall.Errno(errorInsufficientBuffer)), true)
	assertCoreEqual(t, transient(domain.ErrUnsupported), false)
}
