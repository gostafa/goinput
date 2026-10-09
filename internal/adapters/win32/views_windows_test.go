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
	"github.com/gostafa/goinput/internal/ports"
	backendview "github.com/gostafa/goinput/internal/ports/backend"
)

type (
	testNativeViewsState struct {
		err         error
		opened      ports.Capture
		ctx         func() context.Context
		backendView ports.Backend
		fixture     *nativeFixture
		owner       *backend
		view        *backendview.Operations[
			domain.DeviceInfo,
			domain.DeviceID,
			ports.EventSink,
			ports.Capture,
		]
		subscription *capture
		cancel       context.CancelFunc
		infos        []domain.DeviceInfo
	}
)

func testNativeViews(t *testing.T, api *nativeAPI) {
	t.Helper()

	state := new(testNativeViewsState)
	testNativeViewsStep1(t, api, state)
}

func testNativeViewsStep1(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()

	state.fixture, state.owner = fixtureWindowScenario(t, api)
	testNativeViewsStep2(t, api, state)
}

func testNativeViewsStep10(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()
	nativeOK(t, state.subscription.backend.retry(t.Context(), func(context.Context) error {
		return nil
	}))
	nativeOK(t, state.subscription.backend.call(t.Context(), func() error {
		return nil
	}))
	nativeOK(t, state.view.Close())
	testNativeViewsStep11(t, api, state)
}

func testNativeViewsStep11(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()
	assertCoreEqual(t, state.owner.closed, true)
	assertCoreError(
		t,
		resultError(makeFactory(state.owner.native, domain.ErrUnsupported, api)(t.Context(), nil)),
		domain.ErrUnsupported,
	)
	assertCoreError(t, resultError(Factory()(state.ctx(), nil)), context.Canceled)
	testNativeViewsStep12(t, api, state)
}

func testNativeViewsStep12(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()
	replaceNative(t, &api.messages.setEvent, func(foundation.HANDLE) error {
		return nil
	})

	state.backendView, state.err = factoryWithNative(api)(t.Context(), nil)
	nativeOK(t, state.err)
	testNativeViewsStep13(t, state)
}

func testNativeViewsStep13(t *testing.T, state *testNativeViewsState) {
	t.Helper()
	nativeOK(t, state.backendView.Close())
}

func testNativeViewsStep2(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()
	replaceNative(t, &api.messages.setEvent, func(foundation.HANDLE) error {
		go func() {
			backendDrainCommands(state.owner)

			if state.owner.stopping {
				backendFinish(state.owner, nil, api)
			}
		}()

		return nil
	})

	state.fixture.inventory = []input.RAWINPUTDEVICELIST{
		nativeValue[input.RAWINPUTDEVICELIST](func(value *input.RAWINPUTDEVICELIST) {
			value.HDevice = singleValue
			value.DwType = input.RIM_TYPEKEYBOARD
		}),
	}
	testNativeViewsStep2Continue(t, api, state)
}

func testNativeViewsStep3(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()

	state.infos, state.err = state.view.Discover(t.Context())
	nativeOK(t, state.err)
	assertCoreEqual(t, len(state.infos), singleValue)
	testNativeViewsStep4(t, api, state)
}

func testNativeViewsStep4(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()

	state.subscription = state.fixture.capture(t, singleValue, api)
	state.opened, state.err = state.view.Open(
		t.Context(),
		state.infos[noValue].ID,
		state.subscription.sink,
	)
	nativeOK(t, state.err)
	testNativeViewsStep5(t, api, state)
}

func testNativeViewsStep5(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()
	assertCoreEqual(t, state.opened.Info().ID, state.infos[noValue].ID)
	nativeOK(t, state.opened.Close())
	assertCoreError(
		t,
		resultError(state.view.Open(t.Context(), testMissingID, state.subscription.sink)),
		domain.ErrNotFound,
	)
	testNativeViewsStep6(t, api, state)
}

func testNativeViewsStep6(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()

	state.fixture.err = domain.ErrUnsupported

	openArgs := backendOpenArgs{
		args: &backendOpenArguments{id: state.infos[noValue].ID, sink: state.subscription.sink},
		api:  api,
	}
	assertCoreError(
		t,
		resultError(backendOpen(t.Context(), state.owner, &openArgs)),
		domain.ErrUnsupported,
	)
	testNativeViewsStep6Finish(t, api, state)
}

func testNativeViewsStep7(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()

	state.subscription.device.kind = thirdValue
	assertCoreError(
		t,
		resultError(
			completeCaptureOpen(
				t.Context(),
				state.owner,
				&prepareCaptureArgs{subscription: state.subscription, api: api},
			),
		),
		domain.ErrUnsupported,
	)

	state.subscription.device.kind = singleValue
	testNativeViewsStep8(t, api, state)
}

func testNativeViewsStep8(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()

	state.owner.closed = true
	assertCoreError(
		t,
		resultError(
			completeCaptureOpen(
				t.Context(),
				state.owner,
				&prepareCaptureArgs{subscription: state.subscription, api: api},
			),
		),
		domain.ErrClosed,
	)

	state.owner.closed = false
	testNativeViewsStep9(t, api, state)
}

func testNativeViewsStep9(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()

	state.ctx, state.cancel = nativeCancel(t.Context())
	state.cancel()
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
	testNativeViewsStep10(t, api, state)
}

func testNativeViewsStep2Continue(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()

	state.view = makeBackendView(t.Context(), state.owner, api)
	testNativeViewsStep3(t, api, state)
}

func testNativeViewsStep6Finish(t *testing.T, api *nativeAPI, state *testNativeViewsState) {
	t.Helper()

	state.fixture.err = nil
	testNativeViewsStep7(t, api, state)
}
