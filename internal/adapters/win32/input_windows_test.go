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

func testNativeInput(t *testing.T) {
	fixture := newNativeFixture(t)
	stamp := new(domain.Timestamp)
	subscription := fixture.capture(t, 1)
	key := make([]byte, 16)
	binary.LittleEndian.PutUint16(key, letterAScan)
	packet := &inputPacket{kind: 1, body: key}
	captureHandlePacket(subscription, packet)
	captureHandlePacket(subscription, packet)
	binary.LittleEndian.PutUint16(key[2:], 1)
	captureHandlePacket(subscription, packet)
	assertCoreEqual(t, len(fixture.events), 3)
	for index, action := range []domain.EventAction{domain.ActionPress, domain.ActionRepeat, domain.ActionRelease} {
		assertCoreEqual(t, fixture.events[index].Action, action)
		assertCoreEqual(t, fixture.events[index].DeviceID, subscription.info.ID)
	}
	binary.LittleEndian.PutUint16(key[6:], byteMask)
	captureKeyboard(subscription, key, stamp)
	assertCoreEqual(t, len(fixture.events), 3)
	binary.LittleEndian.PutUint16(key, byteMask)
	captureKeyboard(subscription, key, stamp)
	assertCoreError(t, fixture.failures[0], domain.ErrEventLoss)
	captureHandlePacket(subscription, packet)
	captureFail(subscription, domain.ErrUnsupported)
	assertCoreEqual(t, len(fixture.failures), 1)
	subscription = fixture.capture(t, 1)
	captureKeyboard(subscription, nil, stamp)
	assertCoreError(t, fixture.failures[1], domain.ErrEventLoss)
	subscription = fixture.capture(t, 0)
	captureHandlePacket(subscription, packet)
	assertCoreError(t, fixture.failures[2], domain.ErrEventLoss)
	subscription = fixture.capture(t, 3)
	captureDispatchInput(subscription, &inputPacket{kind: 3})
	assertCoreError(t, fixture.failures[3], domain.ErrEventLoss)

	subscription = fixture.capture(t, 0)
	fixture.events = nil
	mouse := make([]byte, mousePacketBytes)
	binary.LittleEndian.PutUint16(mouse[4:], mouseVerticalWheel|mouseHorizontalWheel|3)
	binary.LittleEndian.PutUint16(mouse[6:], uint16(wheelDelta))
	binary.LittleEndian.PutUint32(mouse[12:], 2)
	binary.LittleEndian.PutUint32(mouse[16:], 3)
	captureDispatchInput(subscription, &inputPacket{kind: 0, body: mouse})
	assertCoreEqual(t, len(fixture.events), 6)
	assertCoreEqual(t, fixture.events[0].Action, domain.ActionPress)
	assertCoreEqual(t, fixture.events[1].Action, domain.ActionRelease)
	assertCoreEqual(t, fixture.events[4].Value, float64(1))
	binary.LittleEndian.PutUint16(mouse, 1)
	binary.LittleEndian.PutUint16(mouse[4:], 0)
	captureMouse(subscription, mouse, stamp)
	assertCoreEqual(t, len(fixture.events), 8)
	captureMouse(subscription, mouse, stamp)
	assertCoreEqual(t, len(fixture.events), 8)
	binary.LittleEndian.PutUint16(mouse, 0)
	binary.LittleEndian.PutUint32(mouse[12:], 0)
	binary.LittleEndian.PutUint32(mouse[16:], 0)
	captureMouse(subscription, mouse, stamp)
	assertCoreEqual(t, len(fixture.events), 8)
	captureMouse(subscription, nil, stamp)
	assertCoreEqual(t, subscription.closed.Load(), true)

	subscription = fixture.capture(t, 0)
	subscription.sink = rejectingNativeSink{}
	captureEmit(subscription, &domain.Event{})
	assertCoreEqual(t, subscription.closed.Load(), true)
	captureEmit(subscription, &domain.Event{})

	owner := fixtureBackend(t)
	subscription = fixture.capture(t, 1)
	owner.captures[1] = map[*capture]struct{}{subscription: {}}
	fixture.packet = make([]byte, 24+16)
	binary.LittleEndian.PutUint32(fixture.packet, 1)
	binary.LittleEndian.PutUint32(fixture.packet[4:], uint32(len(fixture.packet)))
	binary.LittleEndian.PutUint64(fixture.packet[8:], 1)
	binary.LittleEndian.PutUint16(fixture.packet[24:], letterAScan)
	backendReadInput(owner, 0)
	assertCoreEqual(t, subscription.closed.Load(), false)
	fixture.packet[4] = 0
	backendReadInput(owner, 0)
	assertCoreEqual(t, subscription.closed.Load(), true)
	subscription = fixture.capture(t, 1)
	owner.captures[1] = map[*capture]struct{}{subscription: {}}
	fixture.err = domain.ErrUnsupported
	backendReadInput(owner, 0)
	assertCoreEqual(t, subscription.closed.Load(), true)
	_, err := readInputBuffer(0, 24)
	assertCoreError(t, err, domain.ErrEventLoss)
	fixture.err = nil
	fixture.packet = nil
	_, err = readInputData(0)
	assertCoreError(t, err, domain.ErrEventLoss)
	_, err = readInputBuffer(0, 24)
	assertCoreError(t, err, domain.ErrEventLoss)
	replaceNative(
		t,
		&winGetRawInputData,
		func(input.HRAWINPUT, input.RAW_INPUT_DATA_COMMAND_FLAGS, unsafe.Pointer, *uint32, uint32) uint32 {
			return 25
		},
	)
	_, err = readInputBuffer(0, 24)
	assertCoreError(t, err, domain.ErrEventLoss)
}

type rejectingNativeSink struct{}

func (rejectingNativeSink) Publish(*domain.Event) bool { return false }
func (rejectingNativeSink) Fail(error)                 {}

func testNativeHID(t *testing.T) {
	fixture := newNativeFixture(t)
	stamp := new(domain.Timestamp)
	subscription := fixture.capture(t, 2)
	captureDispatchInput(subscription, &inputPacket{kind: 2})
	assertCoreEqual(t, subscription.closed.Load(), true)
	subscription = fixture.capture(t, 2)
	button := hid.HIDP_BUTTON_CAPS{UsagePage: 9, ReportID: 1, IsAbsolute: 1}
	axis := hid.HIDP_VALUE_CAPS{
		UsagePage:  1,
		ReportID:   1,
		IsAbsolute: 1,
		BitSize:    8,
		LogicalMax: 255,
	}
	builder := &hidBuilder{
		descriptor: &descriptor{
			preparsed: []byte{1},
			reportLen: 2,
			maxData:   4,
			controls:  make(map[hidIndex]hidControl),
			reportIDs: make(map[byte]bool),
		},
	}
	buttonCtrl, axisCtrl := makeHIDButton(&button, 1, 1), makeHIDValue(&axis, 48, 2)
	hidBuilderAdd(builder, &buttonCtrl)
	hidBuilderAdd(builder, &axisCtrl)
	captureCommitHID(subscription, builder)
	fixture.data = []hid.HIDP_DATA{{DataIndex: 1}, {DataIndex: 2}, {DataIndex: 99}}
	fixture.data[0].Anonymous.Data[0] = 1
	fixture.data[1].Anonymous.Data[0] = 42
	body := make([]byte, 10)
	binary.LittleEndian.PutUint32(body, 2)
	binary.LittleEndian.PutUint32(body[4:], 1)
	body[8] = 1
	captureReports(subscription, body, stamp)
	assertCoreEqual(t, len(fixture.events), 2)
	assertCoreEqual(t, subscription.values[axisCtrl.control.ID], int64(42))
	captureReports(subscription, body, stamp)
	assertCoreEqual(t, len(fixture.events), 2)
	fixture.data = fixture.data[1:]
	captureReports(subscription, body, stamp)
	assertCoreEqual(t, len(fixture.events), 3)
	assertCoreEqual(t, fixture.events[2].Action, domain.ActionRelease)
	axisCtrl.control.Mode = domain.AxisRelative
	captureEmitHIDValue(
		subscription,
		&axisCtrl.control,
		&captureEmitHIDValueArguments{value: 42, stamp: stamp},
	)
	assertCoreEqual(t, len(fixture.events), 4)
	body[8] = 2
	captureReports(subscription, body, stamp)
	assertCoreEqual(t, len(fixture.events), 4)
	body[8] = 1
	subscription.hid.maxData = 0
	captureReports(subscription, body, stamp)
	subscription.hid.maxData = 4
	fixture.status = 0
	captureReports(subscription, body, stamp)
	assertCoreEqual(t, subscription.closed.Load(), true)
	fixture.status = hid.HIDP_STATUS_SUCCESS
	subscription = fixture.capture(t, 2)
	subscription.hid = builder.descriptor
	fixture.data = make([]hid.HIDP_DATA, 5)
	_, err := captureReadHIDReport(subscription, []byte{1, 0})
	assertCoreError(t, err, domain.ErrEventLoss)
	captureReport(subscription, nil, stamp)
	assertCoreEqual(t, subscription.closed.Load(), true)
	subscription = fixture.capture(t, 2)
	subscription.hid = builder.descriptor
	body[0] = 0
	captureReports(subscription, body, stamp)
	assertCoreEqual(t, subscription.closed.Load(), true)

	axisCtrl.native.HasNull = true
	axisCtrl.native.Bounds.LogicalMax = 7
	_, valid := hidControlReportValue(&axisCtrl, 8)
	assertCoreEqual(t, valid, false)
	axisCtrl.hat = true
	direction, valid := hidControlReportValue(&axisCtrl, 1)
	assertCoreEqual(t, valid, true)
	assertCoreEqual(t, direction, int64(domain.HatNorthEast))
	state := &hidReportState{reportID: 1, pressed: make(map[domain.ControlID]bool)}
	captureProcessHIDControl(
		subscription,
		&axisCtrl,
		&captureProcessHIDControlArguments{word: 99, state: state},
	)
	unsupported := axisCtrl
	unsupported.control.Support = domain.SupportUnsupported
	subscription.hid.controls[hidDataIndex(1, 3)] = unsupported
	captureProcessHIDData(subscription, []hid.HIDP_DATA{{DataIndex: 3}}, state)
	assertCoreEqual(t, len(state.pressed), 0)
	assertCoreError(t, hidError("test", foundation.NTSTATUS(0)), domain.ErrUnsupported)
}
