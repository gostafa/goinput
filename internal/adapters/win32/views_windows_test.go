// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

import (
	"context"
	"testing"

	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/input"
	"github.com/gostafa/goinput/internal/domain"
)

func testNativeViews(t *testing.T) {
	fixture := newNativeFixture(t)
	owner := fixtureBackend(t)
	owner.hwnd = 2
	replaceNative(t, &winSetEvent, func(foundation.HANDLE) error {
		go func() {
			backendDrainCommands(owner)
			if owner.stopping {
				backendFinish(owner, nil)
			}
		}()
		return nil
	})
	fixture.inventory = []input.RAWINPUTDEVICELIST{{HDevice: 1, DwType: input.RIM_TYPEKEYBOARD}}
	view := makeBackendView(t.Context(), owner)
	infos, err := view.Discover(t.Context())
	nativeOK(t, err)
	assertCoreEqual(t, len(infos), 1)
	subscription := fixture.capture(t, 1)
	opened, err := view.Open(t.Context(), infos[0].ID, subscription.sink)
	nativeOK(t, err)
	assertCoreEqual(t, opened.Info().ID, infos[0].ID)
	nativeOK(t, opened.Close())
	_, err = view.Open(t.Context(), "missing", subscription.sink)
	assertCoreError(t, err, domain.ErrNotFound)
	fixture.err = domain.ErrUnsupported
	_, err = backendOpen(t.Context(), owner, &backendOpenArguments{id: infos[0].ID, sink: subscription.sink})
	assertCoreError(t, err, domain.ErrUnsupported)
	fixture.err = nil
	subscription.device.kind = 3
	_, err = completeCaptureOpen(t.Context(), owner, subscription)
	assertCoreError(t, err, domain.ErrUnsupported)
	subscription.device.kind = 1
	owner.closed = true
	_, err = completeCaptureOpen(t.Context(), owner, subscription)
	assertCoreError(t, err, domain.ErrClosed)
	owner.closed = false
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = completeCaptureOpen(ctx, owner, subscription)
	assertCoreError(t, err, context.Canceled)
	nativeOK(t, subscription.backend.retry(t.Context(), func(context.Context) error { return nil }))
	nativeOK(t, subscription.backend.call(t.Context(), func() error { return nil }))
	nativeOK(t, view.Close())
	assertCoreEqual(t, owner.closed, true)

	_, err = makeFactory(owner.native, domain.ErrUnsupported)(t.Context(), nil)
	assertCoreError(t, err, domain.ErrUnsupported)
	_, err = Factory()(ctx, nil)
	assertCoreError(t, err, context.Canceled)
	replaceNative(t, &winSetEvent, func(foundation.HANDLE) error { return nil })
	backendView, err := Factory()(t.Context(), nil)
	nativeOK(t, err)
	nativeOK(t, backendView.Close())
}
