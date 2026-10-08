// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

import (
	"context"
	"testing"
	"unsafe"

	hid "github.com/deploymenttheory/go-bindings-win32/bindings/win32/devices/humaninterfacedevice"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/input"
	ext "github.com/gostafa/goinput/extensions/win32"
	"github.com/gostafa/goinput/internal/domain"
	captureview "github.com/gostafa/goinput/internal/ports/capture"
)

type (
	testNativeCapabilitiesState struct {
		ctx          func() context.Context
		fixture      *nativeFixture
		keyboard     *capture
		mouse        *capture
		subscription *capture
		cancel       context.CancelFunc
		button       hid.HIDP_BUTTON_CAPS
		axis         hid.HIDP_VALUE_CAPS
	}
	assertCapabilityBoundsState struct {
		values  []byte
		err     error
		builder *hidBuilder
		control hidControl
	}
	assertHIDDescriptorsState struct {
		ctrl  hidControl
		value hid.HIDP_VALUE_CAPS
	}
	assertCaptureViewsState struct {
		metadata ext.Metadata
		ctx      func() context.Context
		err      error
		view     *captureview.Operations[
			domain.DeviceInfo,
			domain.Capabilities,
		]
		cancel context.CancelFunc
		result *capture
		caps   domain.Capabilities
		info   ext.Info
	}
	assertHIDDescriptorsStep6ArgsRecord[
		ButtonValue any,
		StateValue any,
	] struct {
		state StateValue
	}
	assertHIDDescriptorsStep6Args = assertHIDDescriptorsStep6ArgsRecord[
		*hid.HIDP_BUTTON_CAPS,
		*assertHIDDescriptorsState,
	]
	assertCaptureViewsStep1ArgsRecord[
		SubscriptionValue any,
		ApiValue any,
		StateValue any,
	] struct {
		subscription SubscriptionValue
		api          ApiValue
		state        StateValue
	}
	assertCaptureViewsStep1Args = assertCaptureViewsStep1ArgsRecord[
		*capture,
		*nativeAPI,
		*assertCaptureViewsState,
	]
	assertCaptureViewsStep9ArgsRecord[
		SubscriptionValue any,
		ApiValue any,
		StateValue any,
	] struct {
		subscription SubscriptionValue
		state        StateValue
	}
	assertCaptureViewsStep9Args = assertCaptureViewsStep9ArgsRecord[
		*capture,
		*nativeAPI,
		*assertCaptureViewsState,
	]
	trimmedCase struct {
		err  error
		size uint32
	}
)

func assertCapabilityBounds(t *testing.T) {
	t.Helper()

	state := new(assertCapabilityBoundsState)
	assertCapabilityBoundsStep1(t, state)
}

func assertCapabilityBoundsStep1(t *testing.T, state *assertCapabilityBoundsState) {
	t.Helper()

	state.values, state.err = readCapabilities(noValue, func(*byte, *uint16) foundation.NTSTATUS {
		t.Fatal("zero count called driver")

		return noValue
	})
	nativeOK(t, state.err)
	assertCapabilityBoundsStep2(t, state)
}

func assertCapabilityBoundsStep2(t *testing.T, state *assertCapabilityBoundsState) {
	t.Helper()
	assertCoreEqual(t, len(state.values), noValue)
	assertCoreError(
		t,
		resultError(readCapabilities(singleValue, func(*byte, *uint16) foundation.NTSTATUS {
			return noValue
		})),
		domain.ErrUnsupported,
	)
	assertCapabilityBoundsStep2Continue(t, state)
}

func assertCapabilityBoundsStep3(t *testing.T, state *assertCapabilityBoundsState) {
	t.Helper()

	state.values, state.err = readCapabilities(
		singleValue,
		func(first *byte, _ *uint16) foundation.NTSTATUS {
			*first = testAxisValue

			return hid.HIDP_STATUS_SUCCESS
		},
	)
	nativeOK(t, state.err)
	nativeEqual(t, state.values, []byte{testAxisValue})
	assertCapabilityBoundsStep4(t, state)
}

func assertCapabilityBoundsStep4(t *testing.T, state *assertCapabilityBoundsState) {
	t.Helper()
	assertCoreError(t, addCapabilities([]byte{singleValue}, func(*byte) error {
		return domain.ErrUnsupported
	}), domain.ErrUnsupported)

	state.builder = nativeNew[hidBuilder](func(value *hidBuilder) {
		value.descriptor = nativeNew[descriptor](func(value *descriptor) {
			value.controls = make(map[hidIndex]hidControl)
			value.reportIDs = make(map[byte]bool)
		})
	})
	assertCapabilityBoundsStep4Continue(t, state)
}

func assertCapabilityBoundsStep5(t *testing.T, state *assertCapabilityBoundsState) {
	t.Helper()
	hidBuilderAdd(state.builder, &state.control)
	hidBuilderAdd(state.builder, &state.control)
	assertCoreEqual(t, len(state.builder.controls), singleValue)
	assertCapabilityBoundsStep6(t, state)
}

func assertCapabilityBoundsStep6(t *testing.T, state *assertCapabilityBoundsState) {
	t.Helper()
	addCapabilityRange(state.builder, nativeNew[capabilityRange](func(value *capabilityRange) {
		value.firstUsage = secondValue
		value.lastUsage = singleValue
	}), func(uint32, uint32) hidControl {
		t.Fatal("reversed span iterated")

		return nativeZero[hidControl]()
	})
	nativeOK(
		t,
		addNativeCapability(
			state.builder,
			nativeNew[nativeCapability](func(value *nativeCapability) {
				value.alias = singleValue
			}),
		),
	)
}

func assertCaptureViews(t *testing.T, fixture *nativeFixture, args *prepareCaptureArgs) {
	t.Helper()

	state := new(assertCaptureViewsState)
	assertCaptureViewsStep1(
		t,
		fixture,
		&assertCaptureViewsStep1Args{subscription: args.subscription, api: args.api, state: state},
	)
}

func assertCaptureViewsStep1(
	t *testing.T,
	fixture *nativeFixture,
	args *assertCaptureViewsStep1Args,
) {
	t.Helper()

	args.state.view = makeCaptureView(t.Context(), args.subscription)
	nativeEqual(t, args.state.view.Info(), args.subscription.info)
	assertCaptureViewsStep2(t, fixture, args)
}

func assertCaptureViewsStep10(t *testing.T, subscription *capture, state *assertCaptureViewsState) {
	t.Helper()
	nativeOK(t, state.err)
	assertCoreEqual(t, state.result, subscription)
}

func assertCaptureViewsStep2(
	t *testing.T,
	fixture *nativeFixture,
	args *assertCaptureViewsStep1Args,
) {
	t.Helper()

	args.state.caps = args.state.view.Capabilities()
	args.state.caps.Controls[noValue].Name = testChangedText
	assertCaptureViewsStep2Continue(t, fixture, args)
}

func assertCaptureViewsStep3(
	t *testing.T,
	fixture *nativeFixture,
	args *assertCaptureViewsStep1Args,
) {
	t.Helper()
	assertCoreEqual(t, args.state.view.Extension(&args.state.info), true)

	args.state.info.Controls[noValue].ID = testChangedText
	assertCaptureViewsStep3Continue(t, fixture, args)
}

func assertCaptureViewsStep4(
	t *testing.T,
	fixture *nativeFixture,
	args *assertCaptureViewsStep1Args,
) {
	t.Helper()
	assertCoreEqual(t, args.state.view.Extension(&args.state.metadata), true)
	nativeEqual(t, args.state.metadata.NativeInfo(), args.subscription.native)
	assertCoreEqual(t, args.state.view.Extension(new(string)), false)
	assertCaptureViewsStep5(t, fixture, args)
}

func assertCaptureViewsStep5(
	t *testing.T,
	fixture *nativeFixture,
	args *assertCaptureViewsStep1Args,
) {
	t.Helper()
	assertCoreEqual(t, args.state.view.Extension((*ext.Info)(nil)), false)
	assertCoreEqual(t, args.state.view.Extension((*ext.Metadata)(nil)), false)
	nativeOK(t, args.state.view.Close())
	assertCaptureViewsStep6(t, fixture, args)
}

func assertCaptureViewsStep6(
	t *testing.T,
	fixture *nativeFixture,
	args *assertCaptureViewsStep1Args,
) {
	t.Helper()
	assertCoreEqual(t, args.subscription.closed.Load(), true)
	nativeOK(t, args.state.view.Close())

	args.subscription = fixture.capture(t, noValue, args.api)
	assertCaptureViewsStep7(t, fixture, args)
}

func assertCaptureViewsStep7(
	t *testing.T,
	fixture *nativeFixture,
	args *assertCaptureViewsStep1Args,
) {
	t.Helper()

	args.subscription.backend.call = func(context.Context, func() error) error {
		return domain.ErrClosed
	}
	nativeOK(t, captureClose(t.Context(), args.subscription))

	args.subscription = fixture.capture(t, noValue, args.api)
	assertCaptureViewsStep8(
		t,
		&assertCaptureViewsStep9Args{subscription: args.subscription, state: args.state},
	)
}

func assertCaptureViewsStep8(t *testing.T, args *assertCaptureViewsStep9Args) {
	t.Helper()

	args.subscription.backend.call = func(context.Context, func() error) error {
		return domain.ErrPermissionDenied
	}
	assertCoreError(t, captureClose(t.Context(), args.subscription), domain.ErrPermissionDenied)

	args.state.ctx, args.state.cancel = nativeCancel(t.Context())
	assertCaptureViewsStep9(t, args)
}

func assertCaptureViewsStep9(t *testing.T, args *assertCaptureViewsStep9Args) {
	t.Helper()
	args.state.cancel()
	assertCoreError(
		t,
		resultError(captureCheckOpened(args.state.ctx(), args.subscription)),
		context.Canceled,
	)

	args.state.result, args.state.err = captureCheckOpened(t.Context(), args.subscription)
	assertCaptureViewsStep10(t, args.subscription, args.state)
}

func assertHIDDescriptors(t *testing.T, axis *hid.HIDP_VALUE_CAPS) {
	t.Helper()

	state := new(assertHIDDescriptorsState)
	assertHIDDescriptorsStep1(t, axis, &assertHIDDescriptorsStep6Args{state: state})
}

func assertHIDDescriptorsStep1(
	t *testing.T,
	axis *hid.HIDP_VALUE_CAPS,
	args *assertHIDDescriptorsStep6Args,
) {
	t.Helper()
	assertCoreEqual(t, buttonKind(seventhValue), domain.ControlKey)
	assertCoreEqual(t, buttonKind(twelfthValue), domain.ControlKey)
	assertHIDDescriptorsStep2(t, axis, args)
}

func assertHIDDescriptorsStep10(t *testing.T, args *assertHIDDescriptorsStep6Args) {
	t.Helper()

	args.state.value.PhysicalMax = singleValue
	assertCoreEqual(t, conventionalHat(&args.state.value, noValue, seventhValue), false)

	args.state.value.Units = singleValue
	assertHIDDescriptorsStep11(t, args.state)
}

func assertHIDDescriptorsStep11(t *testing.T, state *assertHIDDescriptorsState) {
	t.Helper()
	assertCoreEqual(t, conventionalHat(&state.value, noValue, seventhValue), false)
}

func assertHIDDescriptorsStep2(
	t *testing.T,
	axis *hid.HIDP_VALUE_CAPS,
	args *assertHIDDescriptorsStep6Args,
) {
	t.Helper()
	assertCoreEqual(t, buttonKind(testButtonPage), domain.ControlButton)
	assertCoreEqual(t, buttonSupport(noValue), domain.SupportUnsupported)
	assertCoreEqual(t, buttonSupport(singleValue), domain.SupportSupported)
	assertHIDDescriptorsStep3(t, axis, args)
}

func assertHIDDescriptorsStep3(
	t *testing.T,
	axis *hid.HIDP_VALUE_CAPS,
	args *assertHIDDescriptorsStep6Args,
) {
	t.Helper()

	args.state.value = *axis
	args.state.value.IsAbsolute = noValue
	args.state.ctrl = makeHIDValue(&args.state.value, testAxisUsage, secondValue)
	assertHIDDescriptorsStep4(t, axis, args)
}

func assertHIDDescriptorsStep4(
	t *testing.T,
	axis *hid.HIDP_VALUE_CAPS,
	args *assertHIDDescriptorsStep6Args,
) {
	t.Helper()
	assertCoreEqual(t, args.state.ctrl.control.Mode, domain.AxisRelative)

	args.state.value.BitSize = noValue
	args.state.ctrl = makeHIDValue(&args.state.value, testAxisUsage, secondValue)
	assertHIDDescriptorsStep5(t, axis, args)
}

func assertHIDDescriptorsStep5(
	t *testing.T,
	axis *hid.HIDP_VALUE_CAPS,
	args *assertHIDDescriptorsStep6Args,
) {
	t.Helper()
	assertCoreEqual(t, args.state.ctrl.control.Support, domain.SupportUnsupported)

	args.state.value = *axis
	args.state.value.LogicalMax = singleValue
	assertHIDDescriptorsStep6(t, args)
}

func assertHIDDescriptorsStep6(t *testing.T, args *assertHIDDescriptorsStep6Args) {
	t.Helper()

	args.state.ctrl = makeHIDValue(&args.state.value, testAxisUsage, secondValue)
	assertCoreEqual(t, args.state.ctrl.control.Kind, domain.ControlSwitch)

	args.state.value.LogicalMax = seventhValue
	assertHIDDescriptorsStep7(t, args)
}

func assertHIDDescriptorsStep7(t *testing.T, args *assertHIDDescriptorsStep6Args) {
	t.Helper()

	args.state.ctrl = makeHIDValue(&args.state.value, hatUsage, secondValue)
	assertCoreEqual(t, args.state.ctrl.control.Kind, domain.ControlHat)
	assertCoreEqual(t, args.state.ctrl.hat, true)
	assertHIDDescriptorsStep8(t, args)
}

func assertHIDDescriptorsStep8(t *testing.T, args *assertHIDDescriptorsStep6Args) {
	t.Helper()
	assertCoreEqual(t, conventionalHat(&args.state.value, noValue, fifthValue), false)

	args.state.value.Units = angularUnits
	args.state.value.PhysicalMax = compassExtent
	assertHIDDescriptorsStep9(t, args)
}

func assertHIDDescriptorsStep9(t *testing.T, args *assertHIDDescriptorsStep6Args) {
	t.Helper()
	assertCoreEqual(t, conventionalHat(&args.state.value, noValue, seventhValue), true)

	args.state.value.PhysicalMax = cardinalExtent
	assertCoreEqual(t, conventionalHat(&args.state.value, noValue, thirdValue), true)
	assertHIDDescriptorsStep10(t, args)
}

func assertTrimmed(t *testing.T) {
	t.Helper()

	cases := trimmedCases()
	for index := range cases {
		assertTrimmedCase(t, &cases[index])
	}
}

func trimmedCases() []trimmedCase {
	return []trimmedCase{
		{size: noValue, err: domain.ErrUnsupported},
		{size: secondValue, err: domain.ErrEventLoss},
		{size: singleValue, err: nil},
	}
}

func assertTrimmedCase(t *testing.T, test *trimmedCase) {
	t.Helper()

	data, err := trimPreparsedData([]byte{singleValue}, test.size)
	assertCoreError(t, err, test.err)

	if test.err == nil {
		nativeEqual(t, data, []byte{singleValue})
	}
}

func testNativeCapabilities(t *testing.T, api *nativeAPI) {
	t.Helper()

	state := new(testNativeCapabilitiesState)
	testNativeCapabilitiesStep1(t, api, state)
}

func testNativeCapabilitiesStep1(t *testing.T, api *nativeAPI, state *testNativeCapabilitiesState) {
	t.Helper()

	state.fixture = newNativeFixture(t, api)
	state.keyboard = state.fixture.capture(t, singleValue, api)
	nativeOK(t, captureLoadCapabilities(t.Context(), state.keyboard, api))
	testNativeCapabilitiesStep2(t, api, state)
}

func testNativeCapabilitiesStep10(
	t *testing.T,
	api *nativeAPI,
	state *testNativeCapabilitiesState,
) {
	t.Helper()

	state.ctx, state.cancel = nativeCancel(t.Context())
	state.cancel()
	assertCoreError(
		t,
		captureLoadHIDCapabilities(state.ctx(), state.subscription, api),
		context.Canceled,
	)
	testNativeCapabilitiesStep11(t, api, state)
}

func testNativeCapabilitiesStep11(
	t *testing.T,
	api *nativeAPI,
	state *testNativeCapabilitiesState,
) {
	t.Helper()

	state.fixture.err = domain.ErrUnsupported
	assertCoreError(
		t,
		captureLoadHIDCapabilities(t.Context(), state.subscription, api),
		domain.ErrUnsupported,
	)

	state.fixture.err = nil
	testNativeCapabilitiesStep12(t, api, state)
}

func testNativeCapabilitiesStep12(
	t *testing.T,
	api *nativeAPI,
	state *testNativeCapabilitiesState,
) {
	t.Helper()

	state.fixture.status = noValue
	assertCoreError(
		t,
		captureBuildHIDCapabilities(state.subscription, []byte{singleValue}, api),
		domain.ErrUnsupported,
	)

	state.fixture.status = hid.HIDP_STATUS_SUCCESS
	testNativeCapabilitiesStep13(t, api, state)
}

func testNativeCapabilitiesStep13(
	t *testing.T,
	api *nativeAPI,
	state *testNativeCapabilitiesState,
) {
	t.Helper()

	state.fixture.maxData = maxDevices + singleValue
	assertCoreError(t, resultError(makeHIDBuilder([]byte{singleValue}, api)), domain.ErrUnsupported)

	state.fixture.maxData = fourthValue
	testNativeCapabilitiesStep14(t, api, state)
}

func testNativeCapabilitiesStep14(
	t *testing.T,
	api *nativeAPI,
	state *testNativeCapabilitiesState,
) {
	t.Helper()

	state.fixture.caps.InputReportByteLength = noValue

	assertCoreError(t, resultError(makeHIDBuilder([]byte{singleValue}, api)), domain.ErrUnsupported)

	state.fixture.caps.InputReportByteLength = secondValue
	testNativeCapabilitiesStep15(t, api, state)
}

func testNativeCapabilitiesStep15(
	t *testing.T,
	api *nativeAPI,
	state *testNativeCapabilitiesState,
) {
	t.Helper()

	state.fixture.buttons[noValue].IsRange = singleValue
	state.fixture.buttons[noValue].Anonymous.Data[noValue] = secondValue
	state.fixture.buttons[noValue].Anonymous.Data[singleValue] = singleValue
	assertCoreError(
		t,
		captureBuildHIDCapabilities(state.subscription, []byte{singleValue}, api),
		domain.ErrUnsupported,
	)
	testNativeCapabilitiesStep16(t, api, state)
}

func testNativeCapabilitiesStep16(
	t *testing.T,
	api *nativeAPI,
	state *testNativeCapabilitiesState,
) {
	t.Helper()

	state.fixture.buttons[noValue] = state.button
	state.fixture.values[noValue].IsRange = singleValue
	state.fixture.values[noValue].Anonymous.Data[noValue] = secondValue
	state.fixture.values[noValue].Anonymous.Data[singleValue] = singleValue
	testNativeCapabilitiesStep17(t, api, state)
}

func testNativeCapabilitiesStep17(
	t *testing.T,
	api *nativeAPI,
	state *testNativeCapabilitiesState,
) {
	t.Helper()
	assertCoreError(
		t,
		captureBuildHIDCapabilities(state.subscription, []byte{singleValue}, api),
		domain.ErrUnsupported,
	)

	state.fixture.values[noValue] = state.axis
	state.fixture.err = domain.ErrUnsupported
	testNativeCapabilitiesStep18(t, api, state)
}

func testNativeCapabilitiesStep18(
	t *testing.T,
	api *nativeAPI,
	state *testNativeCapabilitiesState,
) {
	t.Helper()
	assertCoreError(
		t,
		resultError(captureReadPreparsedData(state.subscription, api)),
		domain.ErrUnsupported,
	)
	assertCoreError(
		t,
		resultError(captureReadPreparsedBuffer(state.subscription, singleValue, api)),
		domain.ErrUnsupported,
	)

	state.fixture.err = nil
	testNativeCapabilitiesStep19(t, api, state)
}

func testNativeCapabilitiesStep19(
	t *testing.T,
	api *nativeAPI,
	state *testNativeCapabilitiesState,
) {
	t.Helper()
	replaceNative(t, &api.inventory.getRawInputDeviceInfo, func(
		_ foundation.HANDLE,
		_ input.RAW_INPUT_DEVICE_INFO_COMMAND,
		_ unsafe.Pointer,
		count *uint32,
	) (uint32, error) {
		*count = noValue

		return noValue, nil
	})
	testNativeCapabilitiesStep19Finish(t, api, state)
}

func testNativeCapabilitiesStep2(t *testing.T, api *nativeAPI, state *testNativeCapabilitiesState) {
	t.Helper()
	assertCoreEqual(t, state.keyboard.caps.Repeat, domain.SupportSupported)

	if len(state.keyboard.caps.Controls) < keyboardScanCount {
		t.Fatal("missing keyboard controls")
	}

	state.keyboard.native.Controls = append(
		state.keyboard.native.Controls,
		state.keyboard.native.Controls[noValue],
	)
	testNativeCapabilitiesStep3(t, api, state)
}

func testNativeCapabilitiesStep20(
	t *testing.T,
	api *nativeAPI,
	state *testNativeCapabilitiesState,
) {
	t.Helper()
	assertCapabilityBounds(t)
	assertHIDDescriptors(t, &state.axis)
	assertCaptureViews(
		t,
		state.fixture,
		&prepareCaptureArgs{subscription: state.keyboard, api: api},
	)
}

func testNativeCapabilitiesStep3(t *testing.T, api *nativeAPI, state *testNativeCapabilitiesState) {
	t.Helper()
	sortKeyboardControls(state.keyboard)

	state.mouse = state.fixture.capture(t, noValue, api)
	state.mouse.device.buttons, state.mouse.device.hwheel = thirdValue, true
	testNativeCapabilitiesStep4(t, api, state)
}

func testNativeCapabilitiesStep4(t *testing.T, api *nativeAPI, state *testNativeCapabilitiesState) {
	t.Helper()
	nativeOK(t, captureLoadCapabilities(t.Context(), state.mouse, api))
	assertCoreEqual(t, len(state.mouse.caps.Controls), testEleventhValue)
	assertCoreEqual(t, state.mouse.caps.Controls[noValue].Support, domain.SupportSupported)
	testNativeCapabilitiesStep5(t, api, state)
}

func testNativeCapabilitiesStep5(t *testing.T, api *nativeAPI, state *testNativeCapabilitiesState) {
	t.Helper()
	assertCoreEqual(t, state.mouse.caps.Controls[fourthValue].Support, domain.SupportUnknown)
	assertCoreEqual(t, state.mouse.caps.Controls[testTenthValue].Support, domain.SupportSupported)
	assertCoreError(
		t,
		captureLoadCapabilities(t.Context(), state.fixture.capture(t, thirdValue, api), api),
		domain.ErrUnsupported,
	)
	testNativeCapabilitiesStep6(t, api, state)
}

func testNativeCapabilitiesStep6(t *testing.T, api *nativeAPI, state *testNativeCapabilitiesState) {
	t.Helper()

	state.subscription = state.fixture.capture(t, secondValue, api)
	state.button = nativeValue[hid.HIDP_BUTTON_CAPS](func(value *hid.HIDP_BUTTON_CAPS) {
		value.UsagePage = testButtonPage
		value.ReportID = singleValue
		value.IsAbsolute = singleValue
	})
	state.button.Anonymous.Data[noValue], state.button.Anonymous.Data[sixthValue] = singleValue, singleValue
	testNativeCapabilitiesStep7(t, api, state)
}

func testNativeCapabilitiesStep7(t *testing.T, api *nativeAPI, state *testNativeCapabilitiesState) {
	t.Helper()

	state.axis = nativeValue[hid.HIDP_VALUE_CAPS](func(value *hid.HIDP_VALUE_CAPS) {
		value.UsagePage = singleValue
		value.ReportID = singleValue
		value.BitSize = eighthValue
		value.ReportCount = singleValue
		value.IsAbsolute = singleValue
		value.LogicalMax = byteMask
	})
	state.axis.Anonymous.Data[noValue], state.axis.Anonymous.Data[sixthValue] = testAxisUsage, secondValue
	state.fixture.buttons, state.fixture.values = []hid.HIDP_BUTTON_CAPS{
		state.button,
	}, []hid.HIDP_VALUE_CAPS{
		state.axis,
	}
	testNativeCapabilitiesStep8(t, api, state)
}

func testNativeCapabilitiesStep8(t *testing.T, api *nativeAPI, state *testNativeCapabilitiesState) {
	t.Helper()

	state.fixture.caps.NumberInputButtonCaps, state.fixture.caps.NumberInputValueCaps = singleValue, singleValue
	nativeOK(t, captureLoadCapabilities(t.Context(), state.subscription, api))
	assertCoreEqual(t, state.subscription.caps.Complete, true)
	testNativeCapabilitiesStep9(t, api, state)
}

func testNativeCapabilitiesStep9(t *testing.T, api *nativeAPI, state *testNativeCapabilitiesState) {
	t.Helper()
	assertCoreEqual(t, len(state.subscription.caps.Controls), secondValue)
	assertCoreEqual(t, len(state.subscription.native.Controls), secondValue)
	assertCoreEqual(t, state.subscription.hid.reportIDs[singleValue], true)
	testNativeCapabilitiesStep10(t, api, state)
}

func assertCapabilityBoundsStep2Continue(t *testing.T, state *assertCapabilityBoundsState) {
	t.Helper()
	assertCoreError(
		t,
		resultError(readCapabilities(singleValue, func(_ *byte, count *uint16) foundation.NTSTATUS {
			*count = secondValue

			return hid.HIDP_STATUS_SUCCESS
		})),
		domain.ErrEventLoss,
	)
	assertCapabilityBoundsStep3(t, state)
}

func assertCapabilityBoundsStep4Continue(t *testing.T, state *assertCapabilityBoundsState) {
	t.Helper()

	state.control = nativeValue[hidControl](func(value *hidControl) {
		value.native = nativeValue[ext.NativeControl](func(value *ext.NativeControl) {
			value.ReportID = singleValue
			value.DataIndex = secondValue
		})
	})
	assertCapabilityBoundsStep5(t, state)
}

func assertCaptureViewsStep2Continue(
	t *testing.T,
	fixture *nativeFixture,
	args *assertCaptureViewsStep1Args,
) {
	t.Helper()
	assertFixtureCopy(t, args.subscription.caps.Controls[noValue].Name, testCapabilitiesText)
	assertCaptureViewsStep3(t, fixture, args)
}

func assertCaptureViewsStep3Continue(
	t *testing.T,
	fixture *nativeFixture,
	args *assertCaptureViewsStep1Args,
) {
	t.Helper()
	assertFixtureCopy(t, args.subscription.native.Controls[noValue].ID, "native controls")
	assertCaptureViewsStep4(t, fixture, args)
}

func testNativeCapabilitiesStep19Continue(
	t *testing.T,
	api *nativeAPI,
	state *testNativeCapabilitiesState,
) {
	t.Helper()
	assertTrimmed(t)
	testNativeCapabilitiesStep20(t, api, state)
}

func assertFixtureCopy(t *testing.T, value, subject string) {
	t.Helper()

	if value == testChangedText {
		t.Fatal(subject + " alias internal state")
	}
}

func testNativeCapabilitiesStep19Finish(
	t *testing.T,
	api *nativeAPI,
	state *testNativeCapabilitiesState,
) {
	t.Helper()
	assertCoreError(
		t,
		resultError(captureReadPreparsedData(state.subscription, api)),
		domain.ErrUnsupported,
	)
	testNativeCapabilitiesStep19Continue(t, api, state)
}
