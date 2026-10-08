// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

import (
	"context"
	"errors"
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

type (
	nativeRetrier struct {
		calls int
	}
	testNativeInventoryState struct {
		*nativeScenario

		err     error
		retry   *nativeRetrier
		infos   []domain.DeviceInfo
		devices []nativeDevice
		device  nativeDevice
	}
	testNativeRegistrationState struct {
		fixture      *nativeFixture
		owner        *backend
		subscription *capture
		usage        topLevel
	}
	testNativeMetadataState struct {
		fixture *nativeFixture
		device  *nativeDevice
		ctx     func() context.Context
		cancel  context.CancelFunc
	}
)

func testNativeInventory(t *testing.T, api *nativeAPI) {
	t.Helper()

	state := new(testNativeInventoryState)
	testNativeInventoryStep1(t, api, state)
}

func testNativeInventoryStep1(t *testing.T, api *nativeAPI, state *testNativeInventoryState) {
	t.Helper()

	state.nativeScenario = fixtureScenario(t, api)
	testNativeInventoryStep2(t, api, state)
}

func testNativeInventoryStep10(t *testing.T, api *nativeAPI, state *testNativeInventoryState) {
	t.Helper()
	nativeOK(t, state.err)
	assertCoreEqual(t, state.device.info.ID, domain.DeviceID(testNativeID))
	assertCoreError(
		t,
		resultError(
			backendFindDevice(
				t.Context(),
				state.owner,
				&backendFindDeviceArgs{id: testMissingID, api: api},
			),
		),
		domain.ErrNotFound,
	)
	testNativeInventoryStep11(t, api, state)
}

func testNativeInventoryStep11(t *testing.T, api *nativeAPI, state *testNativeInventoryState) {
	t.Helper()
	assertCoreError(
		t,
		resultError(selectDevice(nil, testMissingID, domain.ErrUnsupported)),
		domain.ErrUnsupported,
	)

	state.owner.closed = true
	assertCoreError(
		t,
		resultError(backendEnumerateDevices(t.Context(), state.owner, api)),
		domain.ErrClosed,
	)
	testNativeInventoryStep12(t, api, state)
}

func testNativeInventoryStep12(t *testing.T, api *nativeAPI, state *testNativeInventoryState) {
	t.Helper()

	state.owner.closed = false
	state.retry = new(nativeRetrier)
	state.owner.retrier = state.retry
	testNativeInventoryStep13(t, api, state)
}

func testNativeInventoryStep13(t *testing.T, api *nativeAPI, state *testNativeInventoryState) {
	t.Helper()
	nativeOK(t, backendRetry(t.Context(), state.owner, func(context.Context) error {
		return nil
	}))
	assertCoreEqual(t, state.retry.calls, singleValue)
	nativeOK(
		t,
		resultError(readDeviceInventory(noValue, singleValue, func(uint32, uint32) ([]byte, error) {
			t.Fatal("empty inventory read")

			return nil, nil
		})),
	)
	testNativeInventoryStep14(t, api)
}

func testNativeInventoryStep14(t *testing.T, api *nativeAPI) {
	t.Helper()
	assertCoreError(
		t,
		resultError(
			readDeviceInventory(
				maxDevices+singleValue,
				singleValue,
				func(uint32, uint32) ([]byte, error) {
					t.Fatal("oversized inventory read")

					return nil, nil
				},
			),
		),
		domain.ErrInvalidOptions,
	)
	testNativeInventoryStep14Finish(t, api)
}

func testNativeInventoryStep15(t *testing.T, api *nativeAPI) {
	t.Helper()
	replaceNative(
		t,
		&api.inventory.getRawInputDeviceList,
		func(*input.RAWINPUTDEVICELIST, *uint32, uint32) (uint32, error) {
			return infiniteWait, domain.ErrUnsupported
		},
	)
	assertCoreError(
		t,
		resultError(readRawDeviceList(singleValue, singleValue, api)),
		domain.ErrUnsupported,
	)
}

func testNativeInventoryStep2(t *testing.T, api *nativeAPI, state *testNativeInventoryState) {
	t.Helper()
	state.cancel()
	assertCoreError(t, resultError(enumerateRawDevices(state.ctx(), api)), context.Canceled)

	state.fixture.err = syscall.ERROR_ACCESS_DENIED
	testNativeInventoryStep3(t, api, state)
}

func testNativeInventoryStep3(t *testing.T, api *nativeAPI, state *testNativeInventoryState) {
	t.Helper()
	assertCoreError(
		t,
		resultError(backendEnumerateDevices(t.Context(), state.owner, api)),
		domain.ErrPermissionDenied,
	)

	state.fixture.err = nil
	state.infos, state.err = backendDiscover(t.Context(), state.owner, api)
	testNativeInventoryStep4(t, api, state)
}

func testNativeInventoryStep4(t *testing.T, api *nativeAPI, state *testNativeInventoryState) {
	t.Helper()
	nativeOK(t, state.err)
	assertCoreEqual(t, len(state.infos), noValue)

	state.fixture.inventory = []input.RAWINPUTDEVICELIST{
		nativeValue[input.RAWINPUTDEVICELIST](func(value *input.RAWINPUTDEVICELIST) {
			value.HDevice = singleValue
			value.DwType = input.RIM_TYPEKEYBOARD
		}),
		nativeValue[input.RAWINPUTDEVICELIST](func(value *input.RAWINPUTDEVICELIST) {
			value.HDevice = secondValue
			value.DwType = input.RIM_TYPEMOUSE
		}),
	}
	testNativeInventoryStep5(t, api, state)
}

func testNativeInventoryStep5(t *testing.T, api *nativeAPI, state *testNativeInventoryState) {
	t.Helper()

	state.infos, state.err = backendDiscover(t.Context(), state.owner, api)
	nativeOK(t, state.err)
	assertCoreEqual(t, len(state.infos), secondValue)
	testNativeInventoryStep6(t, api, state)
}

func testNativeInventoryStep6(t *testing.T, api *nativeAPI, state *testNativeInventoryState) {
	t.Helper()
	assertCoreEqual(t, state.infos[noValue].ID, domain.DeviceID(testNativeID))
	assertCoreEqual(t, state.infos[noValue].Name, "A")

	state.devices, state.err = backendDescribeDevices(
		state.ctx(),
		state.owner,
		&backendDescribeDevicesArgs{list: state.fixture.inventory, api: api},
	)
	testNativeInventoryStep7(t, api, state)
}

func testNativeInventoryStep7(t *testing.T, api *nativeAPI, state *testNativeInventoryState) {
	t.Helper()
	assertCoreError(t, state.err, context.Canceled)
	assertCoreEqual(t, len(state.devices), noValue)

	state.fixture.err = domain.ErrUnsupported
	testNativeInventoryStep8(t, api, state)
}

func testNativeInventoryStep8(t *testing.T, api *nativeAPI, state *testNativeInventoryState) {
	t.Helper()

	state.devices, state.err = backendDescribeDevices(
		t.Context(),
		state.owner,
		&backendDescribeDevicesArgs{list: state.fixture.inventory, api: api},
	)
	assertCoreError(t, state.err, domain.ErrUnsupported)
	assertCoreEqual(t, len(state.devices), noValue)
	testNativeInventoryStep9(t, api, state)
}

func testNativeInventoryStep9(t *testing.T, api *nativeAPI, state *testNativeInventoryState) {
	t.Helper()
	assertCoreError(
		t,
		resultError(
			backendFindDevice(
				t.Context(),
				state.owner,
				&backendFindDeviceArgs{id: testMissingID, api: api},
			),
		),
		domain.ErrUnsupported,
	)

	state.fixture.err = nil
	testNativeInventoryStep9Continue(t, api, state)
}

func testNativeMetadata(t *testing.T, api *nativeAPI) {
	t.Helper()

	state := new(testNativeMetadataState)
	testNativeMetadataStep1(t, api, state)
}

func testNativeMetadataStep1(t *testing.T, api *nativeAPI, state *testNativeMetadataState) {
	t.Helper()

	state.fixture = newNativeFixture(t, api)
	testNativeMetadataStep1Finish(t, api, state)
}

func testNativeMetadataStep10(t *testing.T, api *nativeAPI, state *testNativeMetadataState) {
	t.Helper()
	nativeDeviceLoadStrings(state.device, noValue, api)
	assertCoreEqual(t, state.device.info.Name, "")
	assertCoreError(t, nonzeroError(nil), syscall.EINVAL)
	testNativeMetadataStep11(t)
}

func testNativeMetadataStep11(t *testing.T) {
	t.Helper()
	assertCoreError(t, nonzeroError(domain.ErrUnsupported), domain.ErrUnsupported)
	testNativeMetadataStep11Finish(t)
}

func testNativeMetadataStep12(t *testing.T) {
	t.Helper()
	assertCoreError(t, normalizeError(domain.ErrUnsupported), domain.ErrUnsupported)
	assertCoreEqual(t, transient(syscall.Errno(errorMoreData)), true)
	assertCoreEqual(t, transient(syscall.Errno(errorNotReady)), true)
	testNativeMetadataStep13(t)
}

func testNativeMetadataStep13(t *testing.T) {
	t.Helper()
	assertCoreEqual(t, transient(syscall.Errno(errorInsufficientBuffer)), true)
	assertCoreEqual(t, transient(domain.ErrUnsupported), false)
}

func testNativeMetadataStep2(t *testing.T, api *nativeAPI, state *testNativeMetadataState) {
	t.Helper()
	assertCoreEqual(t, classFor(topLevel{secondValue, secondValue}), domain.ClassOther)
	assertCoreEqual(t, optionalWord(wordMask+singleValue) == nil, true)
	assertCoreEqual(t, *optionalWord(singleValue), uint16(singleValue))
	testNativeMetadataStep3(t, api, state)
}

func testNativeMetadataStep3(t *testing.T, api *nativeAPI, state *testNativeMetadataState) {
	t.Helper()

	state.device = new(nativeDevice)
	nativeOK(t, nativeDeviceLoadDevicePath(state.device, api))
	assertCoreEqual(t, state.device.info.Path, state.fixture.path)
	testNativeMetadataStep4(t, api, state)
}

func testNativeMetadataStep4(t *testing.T, api *nativeAPI, state *testNativeMetadataState) {
	t.Helper()

	state.fixture.err = domain.ErrUnsupported
	assertCoreError(t, nativeDeviceLoadDevicePath(state.device, api), domain.ErrUnsupported)
	assertCoreError(
		t,
		resultError(readDeviceName(noValue, secondValue, api)),
		domain.ErrUnsupported,
	)
	testNativeMetadataStep5(t, api, state)
}

func testNativeMetadataStep5(t *testing.T, api *nativeAPI, state *testNativeMetadataState) {
	t.Helper()
	nativeOK(t, enrichIdentity(state.device, api))

	state.fixture.err = nil
	testNativeMetadataStep5Finish(t, api, state)
}

func testNativeMetadataStep6(t *testing.T, api *nativeAPI, state *testNativeMetadataState) {
	t.Helper()
	nativeOK(t, enrichIdentity(state.device, api))

	state.fixture.path = ""

	assertCoreError(t, resultError(readDeviceName(noValue, secondValue, api)), domain.ErrNotFound)
	testNativeMetadataStep7(t, api, state)
}

func testNativeMetadataStep7(t *testing.T, api *nativeAPI, state *testNativeMetadataState) {
	t.Helper()
	replaceNative(
		t,
		&api.inventory.getRawInputDeviceInfo,
		func(foundation.HANDLE, input.RAW_INPUT_DEVICE_INFO_COMMAND, unsafe.Pointer, *uint32) (uint32, error) {
			return noValue, nil
		},
	)
	assertCoreError(t, resultError(readDevicePath(noValue, api)), domain.ErrNotFound)

	state.ctx, state.cancel = nativeCancel(t.Context())
	testNativeMetadataStep8(t, api, state)
}

func testNativeMetadataStep8(t *testing.T, api *nativeAPI, state *testNativeMetadataState) {
	t.Helper()
	state.cancel()
	assertCoreError(t, enrichDeviceIdentity(state.ctx(), state.device, api), context.Canceled)
	assertCoreEqual(t, readHIDString(noValue, func(foundation.HANDLE, []byte) foundation.BOOLEAN {
		return noValue
	}), "")
	testNativeMetadataStep9(t, api, state)
}

func testNativeMetadataStep9(t *testing.T, api *nativeAPI, state *testNativeMetadataState) {
	t.Helper()
	replaceNative(
		t,
		&api.hid.hidDGetAttributes,
		func(foundation.HANDLE, *hid.HIDD_ATTRIBUTES) foundation.BOOLEAN {
			return noValue
		},
	)
	nativeDeviceLoadAttributes(state.device, noValue, api)
	replaceNative(
		t,
		&api.hid.hidDGetProductString,
		func(foundation.HANDLE, []byte) foundation.BOOLEAN {
			return noValue
		},
	)
	testNativeMetadataStep10(t, api, state)
}

func testNativeRegistration(t *testing.T, api *nativeAPI) {
	t.Helper()

	state := new(testNativeRegistrationState)
	testNativeRegistrationStep1(t, api, state)
}

func testNativeRegistrationStep1(t *testing.T, api *nativeAPI, state *testNativeRegistrationState) {
	t.Helper()

	state.fixture, state.owner = fixtureWindowScenario(t, api)
	testNativeRegistrationStep2(t, api, state)
}

func testNativeRegistrationStep10(
	t *testing.T,
	api *nativeAPI,
	state *testNativeRegistrationState,
) {
	t.Helper()
	assertCoreError(
		t,
		backendRetainRegistration(state.owner, state.usage, api),
		domain.ErrUnsupported,
	)

	state.owner.registrations[state.usage] = singleValue
	assertCoreError(
		t,
		backendReleaseRegistration(state.owner, state.usage, api),
		domain.ErrUnsupported,
	)
	testNativeRegistrationStep11(t, api, state)
}

func testNativeRegistrationStep11(
	t *testing.T,
	api *nativeAPI,
	state *testNativeRegistrationState,
) {
	t.Helper()
	assertCoreEqual(t, state.owner.registrations[state.usage], noValue)
	assertCoreError(
		t,
		resultError(readRegisteredDevices(singleValue, singleValue, api)),
		domain.ErrUnsupported,
	)

	state.fixture.err = nil
	testNativeRegistrationStep12(t, api, state)
}

func testNativeRegistrationStep12(
	t *testing.T,
	api *nativeAPI,
	state *testNativeRegistrationState,
) {
	t.Helper()
	replaceNative(
		t,
		&api.inventory.getRegisteredRawInputDevices,
		func(*input.RAWINPUTDEVICE, *uint32, uint32) (uint32, error) {
			return infiniteWait, nil
		},
	)
	assertCoreError(t, resultError(registeredDevices(api)), syscall.EINVAL)
	testNativeRegistrationStep12Finish(t, api, state)
}

func testNativeRegistrationStep13(
	t *testing.T,
	api *nativeAPI,
	state *testNativeRegistrationState,
) {
	t.Helper()
	assertCoreError(
		t,
		resultError(readRegisteredDevices(singleValue, singleValue, api)),
		domain.ErrInvalidOptions,
	)

	state.owner = fixtureBackend(t)
	state.owner.captures[singleValue] = map[*capture]struct{}{
		state.subscription: nativeZero[struct{}](),
	}
	testNativeRegistrationStep14(t, api, state)
}

func testNativeRegistrationStep14(
	t *testing.T,
	api *nativeAPI,
	state *testNativeRegistrationState,
) {
	t.Helper()
	backendDisconnect(state.owner, foundation.HANDLE(singleValue), api)
	assertCoreEqual(t, len(state.owner.captures), noValue)
	assertCoreError(t, state.fixture.failures[noValue], domain.ErrDisconnected)
	testNativeRegistrationStep15(t, api, state)
}

func testNativeRegistrationStep15(
	t *testing.T,
	api *nativeAPI,
	state *testNativeRegistrationState,
) {
	t.Helper()

	state.owner.captures[singleValue] = map[*capture]struct{}{
		state.subscription: nativeZero[struct{}](),
	}
	backendDisconnect(state.owner, singleValue, api)
	assertCoreEqual(t, len(state.fixture.failures), singleValue)
}

func testNativeRegistrationStep2(t *testing.T, api *nativeAPI, state *testNativeRegistrationState) {
	t.Helper()

	state.usage = topLevel{singleValue, secondValue}
	state.subscription = state.fixture.capture(t, noValue, api)
	assertCoreError(
		t,
		backendAcquireRegistration(state.owner, nativeZero[topLevel](), api),
		domain.ErrUnsupported,
	)
	testNativeRegistrationStep3(t, api, state)
}

func testNativeRegistrationStep3(t *testing.T, api *nativeAPI, state *testNativeRegistrationState) {
	t.Helper()
	nativeOK(t, captureRegister(state.owner, state.subscription, api))
	assertCoreEqual(t, state.owner.registrations[state.usage], singleValue)
	nativeOK(t, captureRegister(state.owner, state.subscription, api))
	testNativeRegistrationStep4(t, api, state)
}

func testNativeRegistrationStep4(t *testing.T, api *nativeAPI, state *testNativeRegistrationState) {
	t.Helper()
	assertCoreEqual(t, state.owner.registrations[state.usage], secondValue)
	nativeOK(t, backendReleaseRegistration(state.owner, state.usage, api))
	assertCoreEqual(t, state.owner.registrations[state.usage], singleValue)
	testNativeRegistrationStep5(t, api, state)
}

func testNativeRegistrationStep5(t *testing.T, api *nativeAPI, state *testNativeRegistrationState) {
	t.Helper()
	nativeOK(t, captureUnregister(state.owner, state.subscription, api))
	assertCoreEqual(t, len(state.owner.captures), noValue)
	assertCoreEqual(t, len(state.fixture.registrations), noValue)
	testNativeRegistrationStep6(t, api, state)
}

func testNativeRegistrationStep6(t *testing.T, api *nativeAPI, state *testNativeRegistrationState) {
	t.Helper()
	nativeOK(t, captureUnregister(state.owner, state.subscription, api))
	nativeOK(t, backendReleaseRegistration(state.owner, state.usage, api))

	state.fixture.registrations = []input.RAWINPUTDEVICE{
		nativeValue[input.RAWINPUTDEVICE](func(value *input.RAWINPUTDEVICE) {
			value.UsUsagePage = singleValue
			value.UsUsage = secondValue
			value.HwndTarget = testButtonPage
		}),
	}
	testNativeRegistrationStep7(t, api, state)
}

func testNativeRegistrationStep7(t *testing.T, api *nativeAPI, state *testNativeRegistrationState) {
	t.Helper()
	assertCoreError(
		t,
		backendAcquireRegistration(state.owner, state.usage, api),
		domain.ErrRegistrationConflict,
	)
	assertCoreError(
		t,
		captureRegister(state.owner, state.subscription, api),
		domain.ErrRegistrationConflict,
	)
	assertCoreError(
		t,
		backendRemoveRegistration(state.owner, state.usage, api),
		domain.ErrRegistrationConflict,
	)
	testNativeRegistrationStep8(t, api, state)
}

func testNativeRegistrationStep8(t *testing.T, api *nativeAPI, state *testNativeRegistrationState) {
	t.Helper()
	assertCoreEqual(
		t,
		matchesRegistration(
			state.usage,
			nativeValue[input.RAWINPUTDEVICE](func(value *input.RAWINPUTDEVICE) {
				value.UsUsagePage = singleValue
				value.DwFlags = input.RAWINPUTDEVICE_FLAGS(thirtySecondValue)
			}),
		),
		true,
	)
	testNativeRegistrationStep8Finish(t, api, state)
}

func testNativeRegistrationStep9(t *testing.T, api *nativeAPI, state *testNativeRegistrationState) {
	t.Helper()

	state.fixture.err = domain.ErrUnsupported
	assertCoreError(
		t,
		backendAcquireRegistration(state.owner, state.usage, api),
		domain.ErrUnsupported,
	)
	assertCoreError(
		t,
		backendRemoveRegistration(state.owner, state.usage, api),
		domain.ErrUnsupported,
	)
	testNativeRegistrationStep10(t, api, state)
}

func (retry *nativeRetrier) Do(
	ctx context.Context,
	operation func(context.Context) error,
	_ func(error) bool,
) error {
	retry.calls++

	return errors.Join(operation(ctx))
}

func testNativeInventoryStep14Continue(t *testing.T, api *nativeAPI) {
	t.Helper()
	assertCoreError(
		t,
		resultError(readRawDeviceList(singleValue, singleValue, api)),
		domain.ErrInvalidOptions,
	)
	testNativeInventoryStep15(t, api)
}

func testNativeInventoryStep9Continue(
	t *testing.T,
	api *nativeAPI,
	state *testNativeInventoryState,
) {
	t.Helper()

	state.device, state.err = backendFindDevice(
		t.Context(),
		state.owner,
		&backendFindDeviceArgs{id: testNativeID, api: api},
	)
	testNativeInventoryStep10(t, api, state)
}

func testNativeMetadataStep1Continue(t *testing.T, api *nativeAPI, state *testNativeMetadataState) {
	t.Helper()

	rangeValues2 := fixtureClassCases()
	for index := range rangeValues2 {
		assertCoreEqual(
			t,
			classFor(topLevel{singleValue, rangeValues2[index].usage}),
			rangeValues2[index].class,
		)
	}

	testNativeMetadataStep1ContinueFinish(t, api, state)
}

func testNativeRegistrationStep8Continue(
	t *testing.T,
	api *nativeAPI,
	state *testNativeRegistrationState,
) {
	t.Helper()
	nativeOK(
		t,
		backendRemoveRegistration(state.owner, topLevel{page: singleValue, usage: sixthValue}, api),
	)
	testNativeRegistrationStep9(t, api, state)
}

func testNativeInventoryStep14Finish(t *testing.T, api *nativeAPI) {
	t.Helper()
	replaceNative(
		t,
		&api.inventory.getRawInputDeviceList,
		func(*input.RAWINPUTDEVICELIST, *uint32, uint32) (uint32, error) {
			return secondValue, nil
		},
	)
	testNativeInventoryStep14Continue(t, api)
}

func testNativeMetadataStep1Finish(t *testing.T, api *nativeAPI, state *testNativeMetadataState) {
	t.Helper()

	rangeValues1 := []uint32{noValue, singleValue, secondValue, thirdValue}
	for index := range rangeValues1 {
		assertFixtureDeviceKind(t, state.fixture, rangeValues1[index])
	}

	testNativeMetadataStep1Continue(t, api, state)
}

func testNativeMetadataStep11Finish(t *testing.T) {
	t.Helper()

	rangeValues3 := fixtureErrorCases()
	for index := range rangeValues3 {
		assertCoreError(t, normalizeError(rangeValues3[index].native), rangeValues3[index].domain)
		assertCoreError(t, normalizeError(rangeValues3[index].native), rangeValues3[index].native)
	}

	nativeOK(t, normalizeError(nil))
	testNativeMetadataStep12(t)
}

func testNativeMetadataStep5Finish(t *testing.T, api *nativeAPI, state *testNativeMetadataState) {
	t.Helper()
	replaceNative(t, &api.inventory.createFile, func(
		string,
		uint32,
		filesystem.FILE_SHARE_MODE,
		*security.SECURITY_ATTRIBUTES,
		filesystem.FILE_CREATION_DISPOSITION,
		filesystem.FILE_FLAGS_AND_ATTRIBUTES,
		foundation.HANDLE,
	) (foundation.HANDLE, error) {
		return nativeFailure[foundation.HANDLE](domain.ErrUnsupported)
	})
	testNativeMetadataStep6(t, api, state)
}

func testNativeRegistrationStep12Finish(
	t *testing.T,
	api *nativeAPI,
	state *testNativeRegistrationState,
) {
	t.Helper()
	replaceNative(
		t,
		&api.inventory.getRegisteredRawInputDevices,
		func(*input.RAWINPUTDEVICE, *uint32, uint32) (uint32, error) {
			return secondValue, nil
		},
	)
	testNativeRegistrationStep13(t, api, state)
}

func testNativeRegistrationStep8Finish(
	t *testing.T,
	api *nativeAPI,
	state *testNativeRegistrationState,
) {
	t.Helper()
	assertCoreEqual(
		t,
		matchesRegistration(
			state.usage,
			nativeValue[input.RAWINPUTDEVICE](func(value *input.RAWINPUTDEVICE) {
				value.UsUsagePage = testButtonPage
				value.UsUsage = secondValue
			}),
		),
		false,
	)
	testNativeRegistrationStep8Continue(t, api, state)
}

func testNativeMetadataStep1ContinueFinish(
	t *testing.T,
	api *nativeAPI,
	state *testNativeMetadataState,
) {
	t.Helper()
	testNativeMetadataStep2(t, api, state)
}

func assertFixtureDeviceKind(t *testing.T, fixture *nativeFixture, kind uint32) {
	t.Helper()

	device := nativeNew[nativeDevice](func(value *nativeDevice) {
		value.kind = kind
	})
	err := nativeDeviceApplyDeviceInfo(device, fixture.words)

	if kind == thirdValue {
		assertCoreError(t, err, domain.ErrUnsupported)

		return
	}

	nativeOK(t, err)
	assertCoreEqual(t, len(device.info.Classes), singleValue)
}

func fixtureClassCases() []struct {
	usage uint16
	class domain.DeviceClass
} {
	return []struct {
		usage uint16
		class domain.DeviceClass
	}{
		{secondValue, domain.ClassMouse},
		{fourthValue, domain.ClassJoystick},
		{fifthValue, domain.ClassGamepad},
		{sixthValue, domain.ClassKeyboard},
		{seventhValue, domain.ClassKeyboard},
		{testUnknownValue, domain.ClassOther},
	}
}

func fixtureErrorCases() []struct {
	native error
	domain error
} {
	return []struct {
		native error
		domain error
	}{
		{syscall.ERROR_ACCESS_DENIED, domain.ErrPermissionDenied},
		{syscall.Errno(thirtySecondValue), domain.ErrPermissionDenied},
		{syscall.ERROR_FILE_NOT_FOUND, domain.ErrNotFound},
		{syscall.ERROR_PATH_NOT_FOUND, domain.ErrNotFound},
		{syscall.Errno(sixthValue), domain.ErrNotFound},
		{syscall.Errno(errorDeviceDisconnected), domain.ErrDisconnected},
		{syscall.Errno(errorInvalidParameter), domain.ErrInvalidOptions},
	}
}
