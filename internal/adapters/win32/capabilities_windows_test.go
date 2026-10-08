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
)

func testNativeCapabilities(t *testing.T) {
	fixture := newNativeFixture(t)
	keyboard := fixture.capture(t, 1)
	nativeOK(t, captureLoadCapabilities(t.Context(), keyboard))
	assertCoreEqual(t, keyboard.caps.Repeat, domain.SupportSupported)
	if len(keyboard.caps.Controls) < keyboardScanCount {
		t.Fatal("missing keyboard controls")
	}
	keyboard.native.Controls = append(keyboard.native.Controls, keyboard.native.Controls[0])
	sortKeyboardControls(keyboard)
	mouse := fixture.capture(t, 0)
	mouse.device.buttons, mouse.device.hwheel = 3, true
	nativeOK(t, captureLoadCapabilities(t.Context(), mouse))
	assertCoreEqual(t, len(mouse.caps.Controls), 11)
	assertCoreEqual(t, mouse.caps.Controls[0].Support, domain.SupportSupported)
	assertCoreEqual(t, mouse.caps.Controls[4].Support, domain.SupportUnknown)
	assertCoreEqual(t, mouse.caps.Controls[10].Support, domain.SupportSupported)
	assertCoreError(
		t,
		captureLoadCapabilities(t.Context(), fixture.capture(t, 3)),
		domain.ErrUnsupported,
	)

	subscription := fixture.capture(t, 2)
	button := hid.HIDP_BUTTON_CAPS{UsagePage: 9, ReportID: 1, IsAbsolute: 1}
	button.Anonymous.Data[0], button.Anonymous.Data[6] = 1, 1
	axis := hid.HIDP_VALUE_CAPS{
		UsagePage:   1,
		ReportID:    1,
		BitSize:     8,
		ReportCount: 1,
		IsAbsolute:  1,
		LogicalMax:  255,
	}
	axis.Anonymous.Data[0], axis.Anonymous.Data[6] = 48, 2
	fixture.buttons, fixture.values = []hid.HIDP_BUTTON_CAPS{button}, []hid.HIDP_VALUE_CAPS{axis}
	fixture.caps.NumberInputButtonCaps, fixture.caps.NumberInputValueCaps = 1, 1
	nativeOK(t, captureLoadCapabilities(t.Context(), subscription))
	assertCoreEqual(t, subscription.caps.Complete, true)
	assertCoreEqual(t, len(subscription.caps.Controls), 2)
	assertCoreEqual(t, len(subscription.native.Controls), 2)
	assertCoreEqual(t, subscription.hid.reportIDs[1], true)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	assertCoreError(t, captureLoadHIDCapabilities(ctx, subscription), context.Canceled)
	fixture.err = domain.ErrUnsupported
	assertCoreError(t, captureLoadHIDCapabilities(t.Context(), subscription), domain.ErrUnsupported)
	fixture.err = nil
	fixture.status = 0
	assertCoreError(t, captureBuildHIDCapabilities(subscription, []byte{1}), domain.ErrUnsupported)
	fixture.status = hid.HIDP_STATUS_SUCCESS
	fixture.maxData = maxDevices + 1
	_, err := makeHIDBuilder([]byte{1})
	assertCoreError(t, err, domain.ErrUnsupported)
	fixture.maxData = 4
	fixture.caps.InputReportByteLength = 0
	_, err = makeHIDBuilder([]byte{1})
	assertCoreError(t, err, domain.ErrUnsupported)
	fixture.caps.InputReportByteLength = 2
	fixture.buttons[0].IsRange = 1
	fixture.buttons[0].Anonymous.Data[0], fixture.buttons[0].Anonymous.Data[1] = 2, 1
	assertCoreError(t, captureBuildHIDCapabilities(subscription, []byte{1}), domain.ErrUnsupported)
	fixture.buttons[0] = button
	fixture.values[0].IsRange = 1
	fixture.values[0].Anonymous.Data[0], fixture.values[0].Anonymous.Data[1] = 2, 1
	assertCoreError(t, captureBuildHIDCapabilities(subscription, []byte{1}), domain.ErrUnsupported)
	fixture.values[0] = axis

	fixture.err = domain.ErrUnsupported
	_, err = captureReadPreparsedData(subscription)
	assertCoreError(t, err, domain.ErrUnsupported)
	_, err = captureReadPreparsedBuffer(subscription, 1)
	assertCoreError(t, err, domain.ErrUnsupported)
	fixture.err = nil
	replaceNative(
		t,
		&winGetRawInputDeviceInfo,
		func(_ foundation.HANDLE, _ input.RAW_INPUT_DEVICE_INFO_COMMAND, _ unsafe.Pointer, count *uint32) (uint32, error) {
			*count = 0
			return 0, nil
		},
	)
	_, err = captureReadPreparsedData(subscription)
	assertCoreError(t, err, domain.ErrUnsupported)
	assertTrimmed(t)
	assertCapabilityBounds(t)
	assertHIDDescriptors(t, &axis, &button)
	assertCaptureViews(t, fixture, keyboard)
}

func assertTrimmed(t *testing.T) {
	t.Helper()
	for _, test := range []struct {
		size uint32
		err  error
	}{{0, domain.ErrUnsupported}, {2, domain.ErrEventLoss}, {1, nil}} {
		data, err := trimPreparsedData([]byte{1}, test.size)
		assertCoreError(t, err, test.err)
		if test.err == nil {
			nativeEqual(t, data, []byte{1})
		}
	}
}

func assertCapabilityBounds(t *testing.T) {
	t.Helper()
	values, err := readCapabilities(
		0,
		func(*byte, *uint16) foundation.NTSTATUS { t.Fatal("zero count called driver"); return 0 },
	)
	nativeOK(t, err)
	assertCoreEqual(t, len(values), 0)
	_, err = readCapabilities(1, func(*byte, *uint16) foundation.NTSTATUS { return 0 })
	assertCoreError(t, err, domain.ErrUnsupported)
	_, err = readCapabilities(
		1,
		func(_ *byte, count *uint16) foundation.NTSTATUS { *count = 2; return hid.HIDP_STATUS_SUCCESS },
	)
	assertCoreError(t, err, domain.ErrEventLoss)
	values, err = readCapabilities(
		1,
		func(first *byte, _ *uint16) foundation.NTSTATUS { *first = 42; return hid.HIDP_STATUS_SUCCESS },
	)
	nativeOK(t, err)
	nativeEqual(t, values, []byte{42})
	assertCoreError(
		t,
		addCapabilities([]byte{1}, func(*byte) error { return domain.ErrUnsupported }),
		domain.ErrUnsupported,
	)
	builder := &hidBuilder{
		descriptor: &descriptor{
			controls:  make(map[hidIndex]hidControl),
			reportIDs: make(map[byte]bool),
		},
	}
	control := hidControl{native: ext.NativeControl{ReportID: 1, DataIndex: 2}}
	hidBuilderAdd(builder, &control)
	hidBuilderAdd(builder, &control)
	assertCoreEqual(t, len(builder.controls), 1)
	addCapabilityRange(
		builder,
		&capabilityRange{firstUsage: 2, lastUsage: 1},
		func(uint32, uint32) hidControl { t.Fatal("reversed span iterated"); return hidControl{} },
	)
	nativeOK(t, addNativeCapability(builder, &nativeCapability{alias: 1}))
}

func assertHIDDescriptors(t *testing.T, axis *hid.HIDP_VALUE_CAPS, button *hid.HIDP_BUTTON_CAPS) {
	t.Helper()
	assertCoreEqual(t, buttonKind(7), domain.ControlKey)
	assertCoreEqual(t, buttonKind(12), domain.ControlKey)
	assertCoreEqual(t, buttonKind(9), domain.ControlButton)
	assertCoreEqual(t, buttonSupport(0), domain.SupportUnsupported)
	assertCoreEqual(t, buttonSupport(1), domain.SupportSupported)
	value := *axis
	value.IsAbsolute = 0
	ctrl := makeHIDValue(&value, 48, 2)
	assertCoreEqual(t, ctrl.control.Mode, domain.AxisRelative)
	value.BitSize = 0
	ctrl = makeHIDValue(&value, 48, 2)
	assertCoreEqual(t, ctrl.control.Support, domain.SupportUnsupported)
	value = *axis
	value.LogicalMax = 1
	ctrl = makeHIDValue(&value, 48, 2)
	assertCoreEqual(t, ctrl.control.Kind, domain.ControlSwitch)
	value.LogicalMax = 7
	ctrl = makeHIDValue(&value, hatUsage, 2)
	assertCoreEqual(t, ctrl.control.Kind, domain.ControlHat)
	assertCoreEqual(t, ctrl.hat, true)
	assertCoreEqual(t, conventionalHat(&value, 0, 5), false)
	value.Units = angularUnits
	value.PhysicalMax = compassExtent
	assertCoreEqual(t, conventionalHat(&value, 0, 7), true)
	value.PhysicalMax = cardinalExtent
	assertCoreEqual(t, conventionalHat(&value, 0, 3), true)
	value.PhysicalMax = 1
	assertCoreEqual(t, conventionalHat(&value, 0, 7), false)
	value.Units = 1
	assertCoreEqual(t, conventionalHat(&value, 0, 7), false)
}

func assertCaptureViews(t *testing.T, fixture *nativeFixture, subscription *capture) {
	t.Helper()
	view := makeCaptureView(t.Context(), subscription)
	nativeEqual(t, view.Info(), subscription.info)
	caps := view.Capabilities()
	caps.Controls[0].Name = "changed"
	if subscription.caps.Controls[0].Name == "changed" {
		t.Fatal("capabilities alias internal state")
	}
	var info ext.Info
	assertCoreEqual(t, view.Extension(&info), true)
	info.Controls[0].ID = "changed"
	if subscription.native.Controls[0].ID == "changed" {
		t.Fatal("native controls alias internal state")
	}
	var metadata ext.Metadata
	assertCoreEqual(t, view.Extension(&metadata), true)
	nativeEqual(t, metadata.NativeInfo(), subscription.native)
	assertCoreEqual(t, view.Extension(new(string)), false)
	assertCoreEqual(t, view.Extension((*ext.Info)(nil)), false)
	assertCoreEqual(t, view.Extension((*ext.Metadata)(nil)), false)
	nativeOK(t, view.Close())
	assertCoreEqual(t, subscription.closed.Load(), true)
	nativeOK(t, view.Close())
	subscription = fixture.capture(t, 0)
	subscription.backend.call = func(context.Context, func() error) error { return domain.ErrClosed }
	nativeOK(t, captureClose(t.Context(), subscription))
	subscription = fixture.capture(t, 0)
	subscription.backend.call = func(context.Context, func() error) error { return domain.ErrPermissionDenied }
	assertCoreError(t, captureClose(t.Context(), subscription), domain.ErrPermissionDenied)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := captureCheckOpened(ctx, subscription)
	assertCoreError(t, err, context.Canceled)
	result, err := captureCheckOpened(t.Context(), subscription)
	nativeOK(t, err)
	assertCoreEqual(t, result, subscription)
}
