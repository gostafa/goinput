// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

import (
	"encoding/binary"
	"testing"
	"unsafe"

	hid "github.com/deploymenttheory/go-bindings-win32/bindings/win32/devices/humaninterfacedevice"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/input"
	"github.com/gostafa/goinput/internal/domain"
)

type (
	rejectingNativeSink  struct{}
	testNativeInputState struct {
		fixture      *nativeFixture
		stamp        *domain.Timestamp
		subscription *capture
		packet       *inputPacket
		owner        *backend
		key          []byte
		mouse        []byte
	}
	testNativeHIDState struct {
		stamp        *domain.Timestamp
		subscription *capture
		fixture      *nativeFixture
		builder      *hidBuilder
		state        *hidReportState
		body         []byte
		unsupported  hidControl
		buttonCtrl   hidControl
		axisCtrl     hidControl
		direction    int64
		button       hid.HIDP_BUTTON_CAPS
		axis         hid.HIDP_VALUE_CAPS
		valid        bool
	}
)

func testNativeHID(t *testing.T, api *nativeAPI) {
	t.Helper()

	state := new(testNativeHIDState)
	testNativeHIDStep1(t, api, state)
}

func testNativeHIDStep1(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()

	state.fixture, state.subscription, state.stamp = fixtureSubscription(t, api, secondValue)
	testNativeHIDStep2(t, api, state)
}

func testNativeHIDStep10(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()
	assertCoreEqual(t, len(state.fixture.events), thirdValue)
	assertCoreEqual(t, state.fixture.events[secondValue].Action, domain.ActionRelease)

	state.axisCtrl.control.Mode = domain.AxisRelative
	testNativeHIDStep11(t, api, state)
}

func testNativeHIDStep11(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()
	captureEmitHIDValue(
		state.subscription,
		&state.axisCtrl.control,
		nativeNew[captureEmitHIDValueArguments](func(value *captureEmitHIDValueArguments) {
			value.value = testAxisValue
			value.stamp = state.stamp
		}),
	)
	assertCoreEqual(t, len(state.fixture.events), fourthValue)

	state.body[eighthValue] = secondValue
	testNativeHIDStep12(t, api, state)
}

func testNativeHIDStep12(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()
	captureReports(state.subscription, state.body, &captureReportArgs{stamp: state.stamp, api: api})
	assertCoreEqual(t, len(state.fixture.events), fourthValue)

	state.body[eighthValue] = singleValue
	testNativeHIDStep13(t, api, state)
}

func testNativeHIDStep13(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()

	state.subscription.hid.maxData = noValue
	captureReports(state.subscription, state.body, &captureReportArgs{stamp: state.stamp, api: api})

	state.subscription.hid.maxData = fourthValue
	testNativeHIDStep14(t, api, state)
}

func testNativeHIDStep14(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()

	state.fixture.status = noValue
	captureReports(state.subscription, state.body, &captureReportArgs{stamp: state.stamp, api: api})
	assertCoreEqual(t, state.subscription.closed.Load(), true)
	testNativeHIDStep15(t, api, state)
}

func testNativeHIDStep15(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()

	state.fixture.status = hid.HIDP_STATUS_SUCCESS
	state.subscription = state.fixture.capture(t, secondValue, api)
	state.subscription.hid = state.builder.descriptor
	testNativeHIDStep16(t, api, state)
}

func testNativeHIDStep16(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()

	state.fixture.data = allocateBuffer[hid.HIDP_DATA](fifthValue)
	assertCoreError(
		t,
		resultError(captureReadHIDReport(state.subscription, []byte{singleValue, noValue}, api)),
		domain.ErrEventLoss,
	)
	captureReport(state.subscription, nil, &captureReportArgs{stamp: state.stamp, api: api})
	testNativeHIDStep17(t, api, state)
}

func testNativeHIDStep17(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()
	assertCoreEqual(t, state.subscription.closed.Load(), true)

	state.subscription = state.fixture.capture(t, secondValue, api)
	state.subscription.hid = state.builder.descriptor
	testNativeHIDStep18(t, api, state)
}

func testNativeHIDStep18(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()

	state.body[noValue] = noValue
	captureReports(state.subscription, state.body, &captureReportArgs{stamp: state.stamp, api: api})
	assertCoreEqual(t, state.subscription.closed.Load(), true)
	testNativeHIDStep19(t, state)
}

func testNativeHIDStep19(t *testing.T, state *testNativeHIDState) {
	t.Helper()

	state.axisCtrl.native.HasNull = true
	state.axisCtrl.native.Bounds.LogicalMax = seventhValue
	state.direction, state.valid = hidControlReportValue(&state.axisCtrl, eighthValue)
	testNativeHIDStep20(t, state)
}

func testNativeHIDStep2(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()
	captureDispatchInput(state.subscription, nativeNew[inputPacket](func(value *inputPacket) {
		value.kind = secondValue
	}), api)
	assertCoreEqual(t, state.subscription.closed.Load(), true)

	state.subscription = state.fixture.capture(t, secondValue, api)
	testNativeHIDStep3(t, api, state)
}

func testNativeHIDStep20(t *testing.T, state *testNativeHIDState) {
	t.Helper()
	assertCoreEqual(t, state.valid, false)

	state.axisCtrl.hat = true
	state.direction, state.valid = hidControlReportValue(&state.axisCtrl, singleValue)
	testNativeHIDStep21(t, state)
}

func testNativeHIDStep21(t *testing.T, state *testNativeHIDState) {
	t.Helper()
	assertCoreEqual(t, state.valid, true)
	assertCoreEqual(t, state.direction, int64(domain.HatNorthEast))

	state.state = nativeNew[hidReportState](func(value *hidReportState) {
		value.reportID = singleValue
		value.pressed = make(map[domain.ControlID]bool)
	})
	testNativeHIDStep22(t, state)
}

func testNativeHIDStep22(t *testing.T, state *testNativeHIDState) {
	t.Helper()
	captureProcessHIDControl(
		state.subscription,
		&state.axisCtrl,
		nativeNew[captureProcessHIDControlArguments](
			func(value *captureProcessHIDControlArguments) {
				value.word = testUnknownValue
				value.state = state.state
			},
		),
	)

	state.unsupported = state.axisCtrl
	state.unsupported.control.Support = domain.SupportUnsupported
	testNativeHIDStep23(t, state)
}

func testNativeHIDStep23(t *testing.T, state *testNativeHIDState) {
	t.Helper()

	state.subscription.hid.controls[hidDataIndex(singleValue, thirdValue)] = state.unsupported
	captureProcessHIDData(
		state.subscription,
		[]hid.HIDP_DATA{nativeValue[hid.HIDP_DATA](func(value *hid.HIDP_DATA) {
			value.DataIndex = thirdValue
		})},
		state.state,
	)
	assertCoreEqual(t, len(state.state.pressed), noValue)
	testNativeHIDStep24(t)
}

func testNativeHIDStep24(t *testing.T) {
	t.Helper()
	assertCoreError(t, hidError("test", foundation.NTSTATUS(noValue)), domain.ErrUnsupported)
}

func testNativeHIDStep3(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()

	state.button = nativeValue[hid.HIDP_BUTTON_CAPS](func(value *hid.HIDP_BUTTON_CAPS) {
		value.UsagePage = testButtonPage
		value.ReportID = singleValue
		value.IsAbsolute = singleValue
	})
	state.axis = nativeValue[hid.HIDP_VALUE_CAPS](func(value *hid.HIDP_VALUE_CAPS) {
		value.UsagePage = singleValue
		value.ReportID = singleValue
		value.IsAbsolute = singleValue
		value.BitSize = eighthValue
		value.LogicalMax = byteMask
	})
	testNativeHIDStep3Continue(t, api, state)
}

func testNativeHIDStep4(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()

	state.buttonCtrl, state.axisCtrl = makeHIDButton(
		&state.button,
		singleValue,
		singleValue,
	), makeHIDValue(
		&state.axis,
		testAxisUsage,
		secondValue,
	)
	hidBuilderAdd(state.builder, &state.buttonCtrl)
	hidBuilderAdd(state.builder, &state.axisCtrl)
	testNativeHIDStep5(t, api, state)
}

func testNativeHIDStep5(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()
	captureCommitHID(state.subscription, state.builder)

	state.fixture.data = []hid.HIDP_DATA{nativeValue[hid.HIDP_DATA](func(value *hid.HIDP_DATA) {
		value.DataIndex = singleValue
	}), nativeValue[hid.HIDP_DATA](func(value *hid.HIDP_DATA) {
		value.DataIndex = secondValue
	}), nativeValue[hid.HIDP_DATA](func(value *hid.HIDP_DATA) {
		value.DataIndex = testUnknownValue
	})}
	state.fixture.data[noValue].Anonymous.Data[noValue] = singleValue
	testNativeHIDStep6(t, api, state)
}

func testNativeHIDStep6(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()

	state.fixture.data[singleValue].Anonymous.Data[noValue] = testAxisValue
	state.body = allocateBuffer[byte](testTenthValue)
	binary.LittleEndian.PutUint32(state.body, secondValue)
	testNativeHIDStep7(t, api, state)
}

func testNativeHIDStep7(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()
	binary.LittleEndian.PutUint32(state.body[fourthValue:], singleValue)

	state.body[eighthValue] = singleValue
	captureReports(state.subscription, state.body, &captureReportArgs{stamp: state.stamp, api: api})
	testNativeHIDStep8(t, api, state)
}

func testNativeHIDStep8(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()
	assertCoreEqual(t, len(state.fixture.events), secondValue)
	assertCoreEqual(t, state.subscription.values[state.axisCtrl.control.ID], int64(testAxisValue))
	captureReports(state.subscription, state.body, &captureReportArgs{stamp: state.stamp, api: api})
	testNativeHIDStep9(t, api, state)
}

func testNativeHIDStep9(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()
	assertCoreEqual(t, len(state.fixture.events), secondValue)

	state.fixture.data = state.fixture.data[singleValue:]
	captureReports(state.subscription, state.body, &captureReportArgs{stamp: state.stamp, api: api})
	testNativeHIDStep10(t, api, state)
}

func testNativeInput(t *testing.T, api *nativeAPI) {
	t.Helper()

	state := new(testNativeInputState)
	testNativeInputStep1(t, api, state)
}

func testNativeInputStep1(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()

	state.fixture, state.subscription, state.stamp = fixtureSubscription(t, api, singleValue)
	testNativeInputStep2(t, api, state)
}

func testNativeInputStep10(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()

	state.subscription = state.fixture.capture(t, thirdValue, api)
	captureDispatchInput(state.subscription, nativeNew[inputPacket](func(value *inputPacket) {
		value.kind = thirdValue
	}), api)
	assertCoreError(t, state.fixture.failures[thirdValue], domain.ErrEventLoss)
	testNativeInputStep11(t, api, state)
}

func testNativeInputStep11(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()

	state.subscription = state.fixture.capture(t, noValue, api)
	state.fixture.events = nil
	state.mouse = allocateBuffer[byte](mousePacketBytes)
	testNativeInputStep12(t, api, state)
}

func testNativeInputStep12(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	binary.LittleEndian.PutUint16(
		state.mouse[fourthValue:],
		mouseVerticalWheel|mouseHorizontalWheel|thirdValue,
	)
	binary.LittleEndian.PutUint16(state.mouse[sixthValue:], uint16(wheelDelta))
	binary.LittleEndian.PutUint32(state.mouse[twelfthValue:], secondValue)
	testNativeInputStep13(t, api, state)
}

func testNativeInputStep13(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	binary.LittleEndian.PutUint32(state.mouse[sixteenthValue:], thirdValue)
	captureDispatchInput(state.subscription, nativeNew[inputPacket](func(value *inputPacket) {
		value.body = state.mouse
	}), api)
	assertCoreEqual(t, len(state.fixture.events), sixthValue)
	testNativeInputStep14(t, api, state)
}

func testNativeInputStep14(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	assertCoreEqual(t, state.fixture.events[noValue].Action, domain.ActionPress)
	assertCoreEqual(t, state.fixture.events[singleValue].Action, domain.ActionRelease)
	assertCoreEqual(t, state.fixture.events[fourthValue].Value, float64(singleValue))
	testNativeInputStep15(t, api, state)
}

func testNativeInputStep15(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	binary.LittleEndian.PutUint16(state.mouse, singleValue)
	binary.LittleEndian.PutUint16(state.mouse[fourthValue:], noValue)
	captureMouse(state.subscription, state.mouse, state.stamp)
	testNativeInputStep16(t, api, state)
}

func testNativeInputStep16(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	assertCoreEqual(t, len(state.fixture.events), eighthValue)
	captureMouse(state.subscription, state.mouse, state.stamp)
	assertCoreEqual(t, len(state.fixture.events), eighthValue)
	testNativeInputStep17(t, api, state)
}

func testNativeInputStep17(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	binary.LittleEndian.PutUint16(state.mouse, noValue)
	binary.LittleEndian.PutUint32(state.mouse[twelfthValue:], noValue)
	binary.LittleEndian.PutUint32(state.mouse[sixteenthValue:], noValue)
	testNativeInputStep18(t, api, state)
}

func testNativeInputStep18(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	captureMouse(state.subscription, state.mouse, state.stamp)
	assertCoreEqual(t, len(state.fixture.events), eighthValue)
	captureMouse(state.subscription, nil, state.stamp)
	testNativeInputStep19(t, api, state)
}

func testNativeInputStep19(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	assertCoreEqual(t, state.subscription.closed.Load(), true)

	state.subscription = state.fixture.capture(t, noValue, api)
	state.subscription.sink = nativeZero[rejectingNativeSink]()
	testNativeInputStep20(t, api, state)
}

func testNativeInputStep2(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()

	state.key = allocateBuffer[byte](sixteenthValue)
	binary.LittleEndian.PutUint16(state.key, letterAScan)

	state.packet = nativeNew[inputPacket](func(value *inputPacket) {
		value.kind = singleValue
		value.body = state.key
	})
	testNativeInputStep3(t, api, state)
}

func testNativeInputStep20(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	captureEmit(state.subscription, new(domain.Event))
	assertCoreEqual(t, state.subscription.closed.Load(), true)
	captureEmit(state.subscription, new(domain.Event))
	testNativeInputStep21(t, api, state)
}

func testNativeInputStep21(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()

	state.owner = fixtureBackend(t)
	state.subscription = state.fixture.capture(t, singleValue, api)
	state.owner.captures[singleValue] = map[*capture]struct{}{
		state.subscription: nativeZero[struct{}](),
	}
	testNativeInputStep22(t, api, state)
}

func testNativeInputStep22(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()

	state.fixture.packet = allocateBuffer[byte](mousePacketBytes + sixteenthValue)
	binary.LittleEndian.PutUint32(state.fixture.packet, singleValue)
	binary.LittleEndian.PutUint32(
		state.fixture.packet[fourthValue:],
		fixtureLength32(t, len(state.fixture.packet)),
	)
	testNativeInputStep23(t, api, state)
}

func testNativeInputStep23(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	binary.LittleEndian.PutUint64(state.fixture.packet[eighthValue:], singleValue)
	binary.LittleEndian.PutUint16(state.fixture.packet[mousePacketBytes:], letterAScan)
	backendReadInput(state.owner, noValue, api)
	testNativeInputStep24(t, api, state)
}

func testNativeInputStep24(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	assertCoreEqual(t, state.subscription.closed.Load(), false)

	state.fixture.packet[fourthValue] = noValue
	backendReadInput(state.owner, noValue, api)
	testNativeInputStep25(t, api, state)
}

func testNativeInputStep25(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	assertCoreEqual(t, state.subscription.closed.Load(), true)

	state.subscription = state.fixture.capture(t, singleValue, api)
	state.owner.captures[singleValue] = map[*capture]struct{}{
		state.subscription: nativeZero[struct{}](),
	}
	testNativeInputStep26(t, api, state)
}

func testNativeInputStep26(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()

	state.fixture.err = domain.ErrUnsupported
	backendReadInput(state.owner, noValue, api)
	assertCoreEqual(t, state.subscription.closed.Load(), true)
	testNativeInputStep27(t, api, state)
}

func testNativeInputStep27(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	assertCoreError(
		t,
		resultError(readInputBuffer(noValue, mousePacketBytes, api)),
		domain.ErrEventLoss,
	)

	state.fixture.err = nil
	state.fixture.packet = nil

	testNativeInputStep28(t, api)
}

func testNativeInputStep28(t *testing.T, api *nativeAPI) {
	t.Helper()
	assertCoreError(t, resultError(readInputData(noValue, api)), domain.ErrEventLoss)
	assertCoreError(
		t,
		resultError(readInputBuffer(noValue, mousePacketBytes, api)),
		domain.ErrEventLoss,
	)
	testNativeInputStep28Finish(t, api)
}

func testNativeInputStep29(t *testing.T, api *nativeAPI) {
	t.Helper()
	assertCoreError(
		t,
		resultError(readInputBuffer(noValue, mousePacketBytes, api)),
		domain.ErrEventLoss,
	)
}

func testNativeInputStep3(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	captureHandlePacket(state.subscription, state.packet, api)
	captureHandlePacket(state.subscription, state.packet, api)
	binary.LittleEndian.PutUint16(state.key[secondValue:], singleValue)
	testNativeInputStep4(t, api, state)
}

func testNativeInputStep4(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	captureHandlePacket(state.subscription, state.packet, api)
	assertCoreEqual(t, len(state.fixture.events), thirdValue)

	rangeValues1 := []domain.EventAction{
		domain.ActionPress,
		domain.ActionRepeat,
		domain.ActionRelease,
	}
	for index := range rangeValues1 {
		assertCoreEqual(t, state.fixture.events[index].Action, rangeValues1[index])
		assertCoreEqual(t, state.fixture.events[index].DeviceID, state.subscription.info.ID)
	}

	testNativeInputStep5(t, api, state)
}

func testNativeInputStep5(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	binary.LittleEndian.PutUint16(state.key[sixthValue:], byteMask)
	captureKeyboard(state.subscription, state.key, state.stamp)
	assertCoreEqual(t, len(state.fixture.events), thirdValue)
	testNativeInputStep6(t, api, state)
}

func testNativeInputStep6(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	binary.LittleEndian.PutUint16(state.key, byteMask)
	captureKeyboard(state.subscription, state.key, state.stamp)
	assertCoreError(t, state.fixture.failures[noValue], domain.ErrEventLoss)
	testNativeInputStep7(t, api, state)
}

func testNativeInputStep7(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()
	captureHandlePacket(state.subscription, state.packet, api)
	captureFail(state.subscription, domain.ErrUnsupported)
	assertCoreEqual(t, len(state.fixture.failures), singleValue)
	testNativeInputStep8(t, api, state)
}

func testNativeInputStep8(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()

	state.subscription = state.fixture.capture(t, singleValue, api)
	captureKeyboard(state.subscription, nil, state.stamp)
	assertCoreError(t, state.fixture.failures[singleValue], domain.ErrEventLoss)
	testNativeInputStep9(t, api, state)
}

func testNativeInputStep9(t *testing.T, api *nativeAPI, state *testNativeInputState) {
	t.Helper()

	state.subscription = state.fixture.capture(t, noValue, api)
	captureHandlePacket(state.subscription, state.packet, api)
	assertCoreError(t, state.fixture.failures[secondValue], domain.ErrEventLoss)
	testNativeInputStep10(t, api, state)
}

func (rejectingNativeSink) Fail(error) {
}

func (rejectingNativeSink) Publish(*domain.Event) bool {
	return false
}

func testNativeHIDStep3Continue(t *testing.T, api *nativeAPI, state *testNativeHIDState) {
	t.Helper()

	state.builder = nativeNew[hidBuilder](func(value *hidBuilder) {
		value.descriptor = nativeNew[descriptor](func(value *descriptor) {
			value.preparsed = []byte{singleValue}
			value.reportLen = secondValue
			value.maxData = fourthValue
			value.controls = make(map[hidIndex]hidControl)
			value.reportIDs = make(map[byte]bool)
		})
	})
	testNativeHIDStep4(t, api, state)
}

func testNativeInputStep28Finish(t *testing.T, api *nativeAPI) {
	t.Helper()
	replaceNative(t, &api.read.getRawInputData, func(
		input.HRAWINPUT,
		input.RAW_INPUT_DATA_COMMAND_FLAGS,
		unsafe.Pointer,
		*uint32,
		uint32,
	) uint32 {
		return testOversizedPacket
	})
	testNativeInputStep29(t, api)
}
