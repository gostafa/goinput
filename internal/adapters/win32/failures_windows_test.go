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

func testNativeLateFailures(t *testing.T) {
	fixture := newNativeFixture(t)
	owner := fixtureBackend(t)
	owner.registrations[topLevel{1, 2}] = 0
	nativeOK(t, backendCleanupWindow(owner))
	subscription := fixture.capture(t, 2)
	fixture.inventory = []input.RAWINPUTDEVICELIST{{HDevice: 1, DwType: input.RIM_TYPEHID}}
	fixture.status = 0
	_, err := backendOpen(t.Context(), owner, &backendOpenArguments{id: "win32:test-device", sink: subscription.sink})
	assertCoreError(t, err, domain.ErrUnsupported)
	fixture.status = hid.HIDP_STATUS_SUCCESS
	fixture.err = domain.ErrUnsupported
	replaceNative(t, &winFindProcedure, func(*native.Proc) error { return nil })
	assertCoreError(t, captureLoadHIDCapabilities(t.Context(), subscription), domain.ErrUnsupported)
	fixture.err = nil
	fixture.caps.NumberInputButtonCaps = 1
	replaceNative(t, &winHidP_GetButtonCaps, func(hid.HIDP_REPORT_TYPE, *hid.HIDP_BUTTON_CAPS, *uint16, hid.PHIDP_PREPARSED_DATA) foundation.NTSTATUS {
		return 0
	})
	assertCoreError(t, captureBuildHIDCapabilities(subscription, []byte{1}), domain.ErrUnsupported)

	owner = fixtureBackend(t)
	owner.hwnd = 2
	device := &nativeDevice{kind: 1, handle: 1, tlc: topLevel{1, 6}}
	subscription = backendMakeCapture(owner, device, subscription.sink)
	nativeOK(t, subscription.backend.retry(t.Context(), func(context.Context) error { return nil }))
	ctx, cancel := context.WithCancel(t.Context())
	replaceNative(t, &winRegisterRawInputDevices, func([]input.RAWINPUTDEVICE, uint32) error { cancel(); return nil })
	replaceNative(t, &winSetEvent, func(foundation.HANDLE) error { go backendDrainCommands(owner); return nil })
	_, err = completeCaptureOpen(ctx, owner, subscription)
	assertCoreError(t, err, context.Canceled)
	assertCoreEqual(t, subscription.closed.Load(), true)
}
