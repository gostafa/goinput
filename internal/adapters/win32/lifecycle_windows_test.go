// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

import (
	"context"
	"errors"
	"testing"
	"time"

	native "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/security"
	wm "github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/windowsandmessaging"
	"github.com/gostafa/goinput/internal/domain"
)

type (
	testNativeCommandsState struct {
		*nativeScenario

		cmd     *command
		blocked *backend
		started *command
		replied *command
		wait    *commandWait
	}
	testNativeLifecycleState struct {
		ctx     func() context.Context
		err     error
		fixture *nativeFixture
		owner   *backend
		cancel  context.CancelFunc
		started *backend
		result  *backend
		peeked  bool
	}
)

func fixtureBackend(t *testing.T) *backend {
	t.Helper()

	return makeBackend(nativeNew(func(value *nativeState) {
		value.tables = testKeyTables(t)
	}), nil)
}

func fixtureCommand(ctx context.Context, operation func() error) *command {
	return nativeNew(func(value *command) {
		value.commandState = makeCommand(ctx, operation)
		value.reply = make(chan error, singleValue)
	})
}

func testNativeCommands(t *testing.T, api *nativeAPI) {
	t.Helper()

	state := new(testNativeCommandsState)
	testNativeCommandsStep1(t, api, state)
}

func testNativeCommandsStep1(t *testing.T, api *nativeAPI, state *testNativeCommandsState) {
	t.Helper()

	state.nativeScenario = fixtureScenario(t, api)
	testNativeCommandsStep2(t, api, state)
}

func testNativeCommandsStep10(t *testing.T, api *nativeAPI, state *testNativeCommandsState) {
	t.Helper()
	assertCoreError(t, backendSignalCommand(state.owner, fixtureCommand(t.Context(), func() error {
		return nil
	}), api), domain.ErrPermissionDenied)

	state.started = fixtureCommand(t.Context(), func() error {
		return nil
	})
	state.started.commandState.state.Store(singleValue)
	testNativeCommandsStep11(t, api, state)
}

func testNativeCommandsStep11(t *testing.T, api *nativeAPI, state *testNativeCommandsState) {
	t.Helper()
	nativeOK(t, backendSignalCommand(state.owner, state.started, api))
	close(state.owner.done)
	assertCoreError(t, backendSignalCommand(state.owner, fixtureCommand(t.Context(), func() error {
		return nil
	}), api), domain.ErrClosed)
	testNativeCommandsStep12(t, api, state)
}

func testNativeCommandsStep12(t *testing.T, api *nativeAPI, state *testNativeCommandsState) {
	t.Helper()

	state.owner = fixtureBackend(t)
	assertCoreError(t, submitCommand(state.owner, fixtureCommand(t.Context(), func() error {
		return nil
	}), api), domain.ErrPermissionDenied)
	assertCoreError(
		t,
		backendCall(t.Context(), state.owner, &backendCallArgs{operation: func() error {
			return nil
		}, api: api}),
		domain.ErrPermissionDenied,
	)
	testNativeCommandsStep13(t, api, state)
}

func testNativeCommandsStep13(t *testing.T, api *nativeAPI, state *testNativeCommandsState) {
	t.Helper()

	state.fixture.err = nil

	replaceNative(t, &api.messages.setEvent, func(foundation.HANDLE) error {
		backendDrainCommands(state.owner)

		return nil
	})
	nativeOK(t, backendCall(t.Context(), state.owner, &backendCallArgs{operation: func() error {
		return nil
	}, api: api}))
	testNativeCommandsStep14(t, state)
}

func testNativeCommandsStep14(t *testing.T, state *testNativeCommandsState) {
	t.Helper()

	state.replied = fixtureCommand(t.Context(), func() error {
		return nil
	})
	state.replied.reply <- domain.ErrUnsupported

	assertCoreError(t, backendAwaitCommand(state.owner, state.replied), domain.ErrUnsupported)
	testNativeCommandsStep15(t, state)
}

func testNativeCommandsStep15(t *testing.T, state *testNativeCommandsState) {
	t.Helper()

	state.wait = nativeNew(func(value *commandWait) {
		value.command = fixtureCommand(state.ctx(), func() error {
			return nil
		})
		value.contextDone = state.ctx().Done()
	})
	backendWaitCommandStep(state.owner, state.wait)
	assertCoreError(t, state.wait.err, context.Canceled)
	testNativeCommandsStep16(t, state)
}

func testNativeCommandsStep16(t *testing.T, state *testNativeCommandsState) {
	t.Helper()
	assertCoreEqual(t, state.wait.complete, true)

	state.wait = nativeNew(func(value *commandWait) {
		value.command = state.started
		value.contextDone = state.ctx().Done()
	})
	backendWaitCommandStep(state.owner, state.wait)
	testNativeCommandsStep17(t, state)
}

func testNativeCommandsStep17(t *testing.T, state *testNativeCommandsState) {
	t.Helper()
	assertCoreEqual(t, state.wait.complete, false)
	assertCoreEqual(t, state.wait.contextDone == nil, true)
	close(state.owner.done)
	testNativeCommandsStep18(t, state)
}

func testNativeCommandsStep18(t *testing.T, state *testNativeCommandsState) {
	t.Helper()
	assertCoreError(t, backendAwaitCommand(state.owner, state.started), domain.ErrClosed)
}

func testNativeCommandsStep2(t *testing.T, api *nativeAPI, state *testNativeCommandsState) {
	t.Helper()
	state.cancel()
	assertCoreError(t, backendCheckCall(state.ctx(), state.owner), context.Canceled)

	state.owner.closed = true
	testNativeCommandsStep3(t, api, state)
}

func testNativeCommandsStep3(t *testing.T, api *nativeAPI, state *testNativeCommandsState) {
	t.Helper()
	assertCoreError(t, backendCheckCall(t.Context(), state.owner), domain.ErrClosed)
	assertCoreError(
		t,
		backendCall(t.Context(), state.owner, &backendCallArgs{operation: func() error {
			t.Fatal("closed command ran")

			return nil
		}, api: api}),
		domain.ErrClosed,
	)
	assertCoreError(t, backendSignalWake(state.owner, api), domain.ErrClosed)
	testNativeCommandsStep4(t, api, state)
}

func testNativeCommandsStep4(t *testing.T, api *nativeAPI, state *testNativeCommandsState) {
	t.Helper()

	state.owner.closed = false
	nativeOK(t, backendCheckCall(t.Context(), state.owner))

	state.cmd = fixtureCommand(t.Context(), func() error {
		return domain.ErrUnsupported
	})
	testNativeCommandsStep5(t, api, state)
}

func testNativeCommandsStep5(t *testing.T, api *nativeAPI, state *testNativeCommandsState) {
	t.Helper()
	nativeOK(t, backendEnqueue(state.owner, state.cmd))
	backendDrainCommands(state.owner)
	assertCoreError(t, <-state.cmd.reply, domain.ErrUnsupported)
	testNativeCommandsStep6(t, api, state)
}

func testNativeCommandsStep6(t *testing.T, api *nativeAPI, state *testNativeCommandsState) {
	t.Helper()
	assertCoreError(t, backendExecuteCommand(state.owner, state.cmd), context.Canceled)

	state.owner.stopping = true
	assertCoreError(t, backendExecuteCommand(state.owner, fixtureCommand(t.Context(), func() error {
		return nil
	})), domain.ErrClosed)
	testNativeCommandsStep7(t, api, state)
}

func testNativeCommandsStep7(t *testing.T, api *nativeAPI, state *testNativeCommandsState) {
	t.Helper()

	state.owner.stopping = false
	assertCoreError(t, backendExecuteCommand(state.owner, fixtureCommand(state.ctx(), func() error {
		return nil
	})), context.Canceled)

	state.blocked = fixtureBackend(t)
	testNativeCommandsStep8(t, api, state)
}

func testNativeCommandsStep8(t *testing.T, api *nativeAPI, state *testNativeCommandsState) {
	t.Helper()

	state.blocked.commands = nil
	assertCoreError(t, backendEnqueue(state.blocked, fixtureCommand(state.ctx(), func() error {
		return nil
	})), context.Canceled)
	close(state.blocked.done)
	testNativeCommandsStep9(t, api, state)
}

func testNativeCommandsStep9(t *testing.T, api *nativeAPI, state *testNativeCommandsState) {
	t.Helper()
	assertCoreError(t, backendEnqueue(state.blocked, fixtureCommand(t.Context(), func() error {
		return nil
	})), domain.ErrClosed)
	assertCoreError(t, submitCommand(state.blocked, fixtureCommand(t.Context(), func() error {
		return nil
	}), api), domain.ErrClosed)

	state.fixture.err = domain.ErrPermissionDenied
	testNativeCommandsStep10(t, api, state)
}

func testNativeLifecycle(t *testing.T, api *nativeAPI) {
	t.Helper()

	state := new(testNativeLifecycleState)
	testNativeLifecycleStep1(t, api, state)
}

func testNativeLifecycleStep1(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	t.Log("startup")

	state.fixture = newNativeFixture(t, api)
	state.owner = fixtureBackend(t)
	testNativeLifecycleStep2(t, api, state)
}

func testNativeLifecycleStep10(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	assertCoreEqual(
		t,
		backendHandleWindowMessage(
			state.owner,
			nativeNew(func(value *windowMessage) {
				value.message = byteMask
				value.wParam = singleValue
			}),
			api,
		),
		foundation.LRESULT(noValue),
	)
	testNativeLifecycleStep10Finish(t, api, state)
}

func testNativeLifecycleStep11(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	replaceNative(t, &api.messages.msgWaitForMultipleObjectsEx, func(
		[]foundation.HANDLE,
		uint32,
		wm.QUEUE_STATUS_FLAGS,
		wm.MSG_WAIT_FOR_MULTIPLE_OBJECTS_EX_FLAGS,
	) (foundation.WAIT_EVENT, error) {
		return noValue, nil
	})
	nativeOK(t, backendWaitForMessages(state.owner, nil, api))
	assertCoreEqual(t, backendDispatchMessages(state.owner, api), true)
	testNativeLifecycleStep12(t, api, state)
}

func testNativeLifecycleStep12(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()

	state.peeked = false

	replaceNative(
		t,
		&api.messages.peekMessage,
		func(msg *wm.MSG, _ foundation.HWND, _, _ uint32, _ wm.PEEK_MESSAGE_REMOVE_TYPE) bool {
			if state.peeked {
				return false
			}

			state.peeked = true
			msg.Message = messageWake

			return true
		},
	)
	assertCoreEqual(t, backendDispatchMessages(state.owner, api), false)
	testNativeLifecycleStep13(t, api, state)
}

func testNativeLifecycleStep13(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()

	state.owner.stopping = true
	nativeOK(t, backendMessageLoop(state.owner, api))

	state.owner.stopping = false
	testNativeLifecycleStep14(t, api, state)
}

func testNativeLifecycleStep14(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()

	state.fixture.err = domain.ErrPermissionDenied
	assertCoreError(t, backendCreateWindow(state.owner, api), domain.ErrPermissionDenied)
	t.Log("window failures")
	testNativeLifecycleStep15(t, api, state)
}

func testNativeLifecycleStep15(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	replaceNative(t, &api.window.getModuleHandle, func(*string) (foundation.HMODULE, error) {
		return singleValue, nil
	})
	replaceNative(t, &api.window.registerClass, func(*wm.WNDCLASSW) (uint16, error) {
		return noValue, nil
	})

	if backendCreateWindow(state.owner, api) == nil {
		t.Fatal("zero class atom accepted")
	}

	testNativeLifecycleStep16(t, api, state)
}

func testNativeLifecycleStep16(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	assertCoreEqual(t, state.owner.className, "")
	replaceNative(t, &api.window.registerClass, func(*wm.WNDCLASSW) (uint16, error) {
		return singleValue, nil
	})
	assertCoreError(t, backendCreateWindow(state.owner, api), domain.ErrPermissionDenied)
	testNativeLifecycleStep17(t, api, state)
}

func testNativeLifecycleStep17(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	replaceNative(t, &api.read.findProcedure, func(*native.Proc) error {
		return nil
	})
	assertCoreError(t, backendInitialize(state.owner, api), domain.ErrPermissionDenied)
	replaceNative(
		t,
		&api.messages.createEvent,
		func(*security.SECURITY_ATTRIBUTES, bool, bool, *string) (foundation.HANDLE, error) {
			return singleValue, nil
		},
	)
	testNativeLifecycleStep18(t, api, state)
}

func testNativeLifecycleStep18(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	assertCoreError(t, backendInitialize(state.owner, api), domain.ErrPermissionDenied)

	state.fixture.err = nil
	state.owner = fixtureBackend(t)
	testNativeLifecycleStep19(t, api, state)
}

func testNativeLifecycleStep19(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	nativeOK(t, backendCleanupWindow(state.owner, api))
	nativeOK(t, backendCloseWakeEvent(state.owner, api))

	state.owner.hwnd, state.owner.wakeEvent, state.owner.className = secondValue, singleValue, "class"
	testNativeLifecycleStep20(t, api, state)
}

func testNativeLifecycleStep2(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()

	state.ctx, state.cancel = nativeCancel(t.Context())
	state.cancel()
	assertCoreError(
		t,
		resultError(
			newBackend(state.ctx(), state.owner.native, &newBackendArgs{retrier: nil, api: api}),
		),
		context.Canceled,
	)
	testNativeLifecycleStep3(t, api, state)
}

func testNativeLifecycleStep20(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	nativeOK(t, backendCleanupWindow(state.owner, api))
	nativeOK(t, backendCloseWakeEvent(state.owner, api))
	assertCoreEqual(t, state.owner.wakeEvent, foundation.HANDLE(noValue))
	testNativeLifecycleStep21(t, api, state)
}

func testNativeLifecycleStep21(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()

	state.owner = fixtureBackend(t)
	state.owner.ready <- domain.ErrUnsupported

	t.Log("startup completion")
	testNativeLifecycleStep22(t, api, state)
}

func testNativeLifecycleStep22(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	close(state.owner.done)
	state.err = resultError(finishBackendStartup(t.Context(), state.owner, api))
	assertCoreError(t, state.err, domain.ErrUnsupported)

	state.owner = fixtureBackend(t)
	testNativeLifecycleStep23(t, api, state)
}

func testNativeLifecycleStep23(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()

	state.owner.closed = true
	close(state.owner.done)

	state.owner.closeErr = domain.ErrUnsupported
	testNativeLifecycleStep24(t, api, state)
}

func testNativeLifecycleStep24(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	assertCoreError(
		t,
		resultError(backendCheckStartup(state.ctx(), state.owner, api)),
		context.Canceled,
	)
	assertCoreError(t, state.err, domain.ErrUnsupported)

	state.owner.ready <- nil

	testNativeLifecycleStep25(t, api, state)
}

func testNativeLifecycleStep25(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	assertCoreError(
		t,
		resultError(finishBackendStartup(state.ctx(), state.owner, api)),
		context.Canceled,
	)

	state.owner = fixtureBackend(t)
	state.owner.ready <- nil

	testNativeLifecycleStep26(t, api, state)
}

func testNativeLifecycleStep26(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()

	state.result, state.err = finishBackendStartup(t.Context(), state.owner, api)
	nativeOK(t, state.err)
	assertCoreEqual(t, state.result, state.owner)
	testNativeLifecycleStep27(t, api, state)
}

func testNativeLifecycleStep27(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()

	state.owner = fixtureBackend(t)
	state.fixture.err = domain.ErrUnsupported

	t.Log("close")
	testNativeLifecycleStep28(t, api, state)
}

func testNativeLifecycleStep28(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	assertCoreError(t, backendCloseContext(t.Context(), state.owner, api), domain.ErrUnsupported)

	state.fixture.err = nil

	replaceNative(t, &api.messages.setEvent, func(foundation.HANDLE) error {
		go func() {
			backendDrainCommands(state.owner)
			backendFinish(state.owner, nil, api)
		}()

		return nil
	})
	testNativeLifecycleStep29(t, api, state)
}

func testNativeLifecycleStep29(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	nativeOK(t, backendCloseContext(t.Context(), state.owner, api))
	assertCoreEqual(t, state.owner.stopping, true)
	assertCoreEqual(t, state.owner.closed, true)
	testNativeLifecycleStep30(t)
}

func testNativeLifecycleStep3(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()

	state.fixture.err = domain.ErrUnsupported
	assertCoreError(
		t,
		resultError(
			newBackend(t.Context(), state.owner.native, &newBackendArgs{retrier: nil, api: api}),
		),
		domain.ErrUnsupported,
	)

	state.fixture.err = nil
	testNativeLifecycleStep4(t, api, state)
}

func testNativeLifecycleStep30(t *testing.T) {
	t.Helper()

	if !errors.Is(normalizeError(domain.ErrUnsupported), domain.ErrUnsupported) {
		t.Fatal("error lost")
	}
}

func testNativeLifecycleStep4(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()

	state.started, state.err = newBackend(
		t.Context(),
		state.owner.native,
		&newBackendArgs{retrier: nil, api: api},
	)
	t.Log("backend created")
	nativeOK(t, state.err)
	testNativeLifecycleStep5(t, api, state)
}

func testNativeLifecycleStep5(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()

	select {
	case <-state.started.done:
	case <-time.After(time.Second):
		t.Fatal("message loop did not stop")
	}

	assertCoreEqual(t, state.started.closed, true)

	state.owner = fixtureBackend(t)
	testNativeLifecycleStep6(t, api, state)
}

func testNativeLifecycleStep6(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	nativeOK(t, backendInitialize(state.owner, api))
	t.Log("window initialized")
	assertCoreEqual(t, state.owner.hwnd, foundation.HWND(secondValue))
	testNativeLifecycleStep7(t, api, state)
}

func testNativeLifecycleStep7(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	assertCoreEqual(t, state.owner.wakeEvent, foundation.HANDLE(singleValue))
	assertCoreEqual(
		t,
		state.fixture.callback(testUnknownValue, noValue, noValue, noValue),
		foundation.LRESULT(seventhValue),
	)
	state.owner.native.windows.Store(foundation.HWND(testUnknownValue), "wrong type")
	testNativeLifecycleStep8(t, api, state)
}

func testNativeLifecycleStep8(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	assertCoreEqual(
		t,
		state.fixture.callback(testUnknownValue, noValue, noValue, noValue),
		foundation.LRESULT(seventhValue),
	)
	assertCoreEqual(
		t,
		state.fixture.callback(state.owner.hwnd, messageWake, noValue, noValue),
		foundation.LRESULT(noValue),
	)
	testNativeLifecycleStep8Continue(t, api, state)
}

func testNativeLifecycleStep9(t *testing.T, api *nativeAPI, state *testNativeLifecycleState) {
	t.Helper()
	assertCoreEqual(
		t,
		backendHandleWindowMessage(
			state.owner,
			nativeNew(func(value *windowMessage) {
				value.message = messageDeviceChange
				value.wParam = secondValue
			}),
			api,
		),
		foundation.LRESULT(noValue),
	)
	backendHandleDeviceChange(state.owner, new(windowMessage), api)
	testNativeLifecycleStep9Continue(t, api, state)
}

func testNativeLifecycleStep10Continue(
	t *testing.T,
	api *nativeAPI,
	state *testNativeLifecycleState,
) {
	t.Helper()

	if backendMessageLoop(state.owner, api) == nil {
		t.Fatal("failed wait accepted")
	}

	testNativeLifecycleStep11(t, api, state)
}

func testNativeLifecycleStep8Continue(
	t *testing.T,
	api *nativeAPI,
	state *testNativeLifecycleState,
) {
	t.Helper()
	testNativeLifecycleUnhandledMessage(t, api, state, keyboardScanCount)
	testNativeLifecycleStep9(t, api, state)
}

func testNativeLifecycleStep9Continue(
	t *testing.T,
	api *nativeAPI,
	state *testNativeLifecycleState,
) {
	t.Helper()
	testNativeLifecycleUnhandledMessage(t, api, state, byteMask)
	testNativeLifecycleStep10(t, api, state)
}

func testNativeLifecycleUnhandledMessage(
	t *testing.T,
	api *nativeAPI,
	state *testNativeLifecycleState,
	message uint32,
) {
	t.Helper()
	assertCoreEqual(
		t,
		backendHandleWindowMessage(
			state.owner,
			nativeNew(func(value *windowMessage) {
				value.message = message
			}),
			api,
		),
		foundation.LRESULT(seventhValue),
	)
}

func testNativeLifecycleStep10Finish(
	t *testing.T,
	api *nativeAPI,
	state *testNativeLifecycleState,
) {
	t.Helper()
	replaceNative(t, &api.messages.msgWaitForMultipleObjectsEx, func(
		[]foundation.HANDLE,
		uint32,
		wm.QUEUE_STATUS_FLAGS,
		wm.MSG_WAIT_FOR_MULTIPLE_OBJECTS_EX_FLAGS,
	) (foundation.WAIT_EVENT, error) {
		return infiniteWait, nil
	})
	testNativeLifecycleStep10Continue(t, api, state)
}
