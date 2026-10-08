// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

import (
	"context"
	"testing"

	native "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	hid "github.com/deploymenttheory/go-bindings-win32/bindings/win32/devices/humaninterfacedevice"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/input"
	"github.com/gostafa/goinput/internal/domain"
)

type (
	testNativeLateFailuresState struct {
		fixture      *nativeFixture
		owner        *backend
		subscription *capture
		device       *nativeDevice
		ctx          func() context.Context
		cancel       context.CancelFunc
	}
)

func testNativeLateFailures(t *testing.T, api *nativeAPI) {
	t.Helper()

	state := new(testNativeLateFailuresState)
	testNativeLateFailuresStep1(t, api, state)
}

func testNativeLateFailuresStep1(t *testing.T, api *nativeAPI, state *testNativeLateFailuresState) {
	t.Helper()

	state.fixture = newNativeFixture(t, api)
	state.owner = fixtureBackend(t)
	state.owner.registrations[topLevel{singleValue, secondValue}] = noValue
	testNativeLateFailuresStep2(t, api, state)
}

func testNativeLateFailuresStep2(t *testing.T, api *nativeAPI, state *testNativeLateFailuresState) {
	t.Helper()
	nativeOK(t, backendCleanupWindow(state.owner, api))

	state.subscription = state.fixture.capture(t, secondValue, api)
	state.fixture.inventory = []input.RAWINPUTDEVICELIST{
		nativeValue[input.RAWINPUTDEVICELIST](func(value *input.RAWINPUTDEVICELIST) {
			value.HDevice = singleValue
			value.DwType = input.RIM_TYPEHID
		}),
	}
	testNativeLateFailuresStep3(t, api, state)
}

func testNativeLateFailuresStep3(t *testing.T, api *nativeAPI, state *testNativeLateFailuresState) {
	t.Helper()

	state.fixture.status = noValue

	openArgs := backendOpenArgs{
		args: &backendOpenArguments{id: testNativeID, sink: state.subscription.sink},
		api:  api,
	}
	assertCoreError(
		t,
		resultError(backendOpen(t.Context(), state.owner, &openArgs)),
		domain.ErrUnsupported,
	)
	testNativeLateFailuresStep3Finish(t, api, state)
}

func testNativeLateFailuresStep4(t *testing.T, api *nativeAPI, state *testNativeLateFailuresState) {
	t.Helper()

	state.fixture.err = domain.ErrUnsupported

	replaceNative(t, &api.read.findProcedure, func(*native.Proc) error {
		return nil
	})
	assertCoreError(
		t,
		captureLoadHIDCapabilities(t.Context(), state.subscription, api),
		domain.ErrUnsupported,
	)
	testNativeLateFailuresStep5(t, api, state)
}

func testNativeLateFailuresStep5(t *testing.T, api *nativeAPI, state *testNativeLateFailuresState) {
	t.Helper()

	state.fixture.err = nil
	state.fixture.caps.NumberInputButtonCaps = singleValue

	replaceNative(
		t,
		&api.hid.hidPGetButtonCaps,
		func(hid.HIDP_REPORT_TYPE, *hid.HIDP_BUTTON_CAPS, *uint16, hid.PHIDP_PREPARSED_DATA) foundation.NTSTATUS {
			return noValue
		},
	)
	testNativeLateFailuresStep6(t, api, state)
}

func testNativeLateFailuresStep6(t *testing.T, api *nativeAPI, state *testNativeLateFailuresState) {
	t.Helper()
	assertCoreError(
		t,
		captureBuildHIDCapabilities(state.subscription, []byte{singleValue}, api),
		domain.ErrUnsupported,
	)

	state.owner = fixtureBackend(t)
	state.owner.hwnd = secondValue
	testNativeLateFailuresStep7(t, api, state)
}

func testNativeLateFailuresStep7(t *testing.T, api *nativeAPI, state *testNativeLateFailuresState) {
	t.Helper()

	state.device = nativeNew[nativeDevice](func(value *nativeDevice) {
		value.kind = singleValue
		value.handle = singleValue
		value.tlc = topLevel{singleValue, sixthValue}
	})
	state.subscription = backendMakeCapture(
		state.owner,
		state.device,
		&backendMakeCaptureArgs{sink: state.subscription.sink, api: api},
	)
	nativeOK(t, state.subscription.backend.retry(t.Context(), func(context.Context) error {
		return nil
	}))
	testNativeLateFailuresStep8(t, api, state)
}

func testNativeLateFailuresStep8(t *testing.T, api *nativeAPI, state *testNativeLateFailuresState) {
	t.Helper()

	state.ctx, state.cancel = nativeCancel(t.Context())
	replaceNative(
		t,
		&api.inventory.registerRawInputDevices,
		func([]input.RAWINPUTDEVICE, uint32) error {
			state.cancel()

			return nil
		},
	)
	replaceNative(t, &api.messages.setEvent, func(foundation.HANDLE) error {
		go backendDrainCommands(state.owner)

		return nil
	})
	testNativeLateFailuresStep9(t, api, state)
}

func testNativeLateFailuresStep9(t *testing.T, api *nativeAPI, state *testNativeLateFailuresState) {
	t.Helper()
	assertCoreError(
		t,
		resultError(
			completeCaptureOpen(
				state.ctx(),
				state.owner,
				&prepareCaptureArgs{subscription: state.subscription, api: api},
			),
		),
		context.Canceled,
	)
	assertCoreEqual(t, state.subscription.closed.Load(), true)
}

func testNativeLateFailuresStep3Finish(
	t *testing.T,
	api *nativeAPI,
	state *testNativeLateFailuresState,
) {
	t.Helper()

	state.fixture.status = hid.HIDP_STATUS_SUCCESS
	testNativeLateFailuresStep4(t, api, state)
}
