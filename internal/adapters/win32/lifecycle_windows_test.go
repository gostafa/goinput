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

func fixtureCommand(ctx context.Context, operation func() error) *command {
	return &command{commandState: makeCommand(ctx, operation), reply: make(chan error, 1)}
}

func fixtureBackend(t *testing.T) *backend {
	t.Helper()
	return makeBackend(&nativeState{tables: testKeyTables(t)}, nil)
}

func testNativeCommands(t *testing.T) {
	fixture := newNativeFixture(t)
	owner := fixtureBackend(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	assertCoreError(t, backendCheckCall(ctx, owner), context.Canceled)
	owner.closed = true
	assertCoreError(t, backendCheckCall(t.Context(), owner), domain.ErrClosed)
	assertCoreError(t, backendCall(t.Context(), owner, func() error { t.Fatal("closed command ran"); return nil }), domain.ErrClosed)
	assertCoreError(t, backendSignalWake(owner), domain.ErrClosed)
	owner.closed = false
	nativeOK(t, backendCheckCall(t.Context(), owner))

	cmd := fixtureCommand(t.Context(), func() error { return domain.ErrUnsupported })
	nativeOK(t, backendEnqueue(owner, cmd))
	backendDrainCommands(owner)
	assertCoreError(t, <-cmd.reply, domain.ErrUnsupported)
	assertCoreError(t, backendExecuteCommand(owner, cmd), context.Canceled)
	owner.stopping = true
	assertCoreError(t, backendExecuteCommand(owner, fixtureCommand(t.Context(), func() error { return nil })), domain.ErrClosed)
	owner.stopping = false
	assertCoreError(t, backendExecuteCommand(owner, fixtureCommand(ctx, func() error { return nil })), context.Canceled)

	blocked := fixtureBackend(t)
	blocked.commands = nil
	assertCoreError(t, backendEnqueue(blocked, fixtureCommand(ctx, func() error { return nil })), context.Canceled)
	close(blocked.done)
	assertCoreError(t, backendEnqueue(blocked, fixtureCommand(t.Context(), func() error { return nil })), domain.ErrClosed)
	assertCoreError(t, submitCommand(blocked, fixtureCommand(t.Context(), func() error { return nil })), domain.ErrClosed)

	fixture.err = domain.ErrPermissionDenied
	assertCoreError(t, backendSignalCommand(owner, fixtureCommand(t.Context(), func() error { return nil })), domain.ErrPermissionDenied)
	started := fixtureCommand(t.Context(), func() error { return nil })
	started.state.Store(1)
	nativeOK(t, backendSignalCommand(owner, started))
	close(owner.done)
	assertCoreError(t, backendSignalCommand(owner, fixtureCommand(t.Context(), func() error { return nil })), domain.ErrClosed)
	owner = fixtureBackend(t)
	assertCoreError(t, submitCommand(owner, fixtureCommand(t.Context(), func() error { return nil })), domain.ErrPermissionDenied)
	assertCoreError(t, backendCall(t.Context(), owner, func() error { return nil }), domain.ErrPermissionDenied)
	fixture.err = nil
	replaceNative(t, &winSetEvent, func(foundation.HANDLE) error { backendDrainCommands(owner); return nil })
	nativeOK(t, backendCall(t.Context(), owner, func() error { return nil }))

	replied := fixtureCommand(t.Context(), func() error { return nil })
	replied.reply <- domain.ErrUnsupported
	assertCoreError(t, backendAwaitCommand(owner, replied), domain.ErrUnsupported)
	wait := &commandWait{command: fixtureCommand(ctx, func() error { return nil }), contextDone: ctx.Done()}
	backendWaitCommandStep(owner, wait)
	assertCoreError(t, wait.err, context.Canceled)
	assertCoreEqual(t, wait.complete, true)
	wait = &commandWait{command: started, contextDone: ctx.Done()}
	backendWaitCommandStep(owner, wait)
	assertCoreEqual(t, wait.complete, false)
	assertCoreEqual(t, wait.contextDone == nil, true)
	close(owner.done)
	assertCoreError(t, backendAwaitCommand(owner, started), domain.ErrClosed)
}

func testNativeLifecycle(t *testing.T) {
	t.Log("startup")
	fixture := newNativeFixture(t)
	owner := fixtureBackend(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := newBackend(ctx, owner.native, nil)
	assertCoreError(t, err, context.Canceled)

	fixture.err = domain.ErrUnsupported
	_, err = newBackend(t.Context(), owner.native, nil)
	assertCoreError(t, err, domain.ErrUnsupported)
	fixture.err = nil
	started, err := newBackend(t.Context(), owner.native, nil)
	t.Log("backend created")
	nativeOK(t, err)
	select {
	case <-started.done:
	case <-time.After(time.Second):
		t.Fatal("message loop did not stop")
	}
	assertCoreEqual(t, started.closed, true)

	owner = fixtureBackend(t)
	nativeOK(t, backendInitialize(owner))
	t.Log("window initialized")
	assertCoreEqual(t, owner.hwnd, foundation.HWND(2))
	assertCoreEqual(t, owner.wakeEvent, foundation.HANDLE(1))
	assertCoreEqual(t, fixture.callback(99, 0, 0, 0), foundation.LRESULT(7))
	owner.native.windows.Store(foundation.HWND(99), "wrong type")
	assertCoreEqual(t, fixture.callback(99, 0, 0, 0), foundation.LRESULT(7))
	assertCoreEqual(t, fixture.callback(owner.hwnd, messageWake, 0, 0), foundation.LRESULT(0))
	assertCoreEqual(t, backendHandleWindowMessage(owner, &windowMessage{message: 123}), foundation.LRESULT(7))
	assertCoreEqual(t, backendHandleWindowMessage(owner, &windowMessage{message: messageDeviceChange, wParam: 2}), foundation.LRESULT(0))
	backendHandleDeviceChange(owner, &windowMessage{})
	assertCoreEqual(t, backendHandleInputMessage(owner, &windowMessage{}), foundation.LRESULT(7))
	assertCoreEqual(t, backendHandleWindowMessage(owner, &windowMessage{message: byteMask, wParam: 1}), foundation.LRESULT(0))

	replaceNative(t, &winMsgWaitForMultipleObjectsEx, func([]foundation.HANDLE, uint32, wm.QUEUE_STATUS_FLAGS, wm.MSG_WAIT_FOR_MULTIPLE_OBJECTS_EX_FLAGS) (foundation.WAIT_EVENT, error) {
		return infiniteWait, nil
	})
	if backendMessageLoop(owner) == nil {
		t.Fatal("failed wait accepted")
	}
	replaceNative(t, &winMsgWaitForMultipleObjectsEx, func([]foundation.HANDLE, uint32, wm.QUEUE_STATUS_FLAGS, wm.MSG_WAIT_FOR_MULTIPLE_OBJECTS_EX_FLAGS) (foundation.WAIT_EVENT, error) {
		return 0, nil
	})
	nativeOK(t, backendWaitForMessages(owner, nil))
	assertCoreEqual(t, backendDispatchMessages(owner), true)
	peeked := false
	replaceNative(t, &winPeekMessage, func(msg *wm.MSG, _ foundation.HWND, _, _ uint32, _ wm.PEEK_MESSAGE_REMOVE_TYPE) bool {
		if peeked {
			return false
		}
		peeked = true
		msg.Message = messageWake
		return true
	})
	assertCoreEqual(t, backendDispatchMessages(owner), false)
	owner.stopping = true
	nativeOK(t, backendMessageLoop(owner))
	owner.stopping = false

	fixture.err = domain.ErrPermissionDenied
	assertCoreError(t, backendCreateWindow(owner), domain.ErrPermissionDenied)
	t.Log("window failures")
	replaceNative(t, &winGetModuleHandle, func(*string) (foundation.HMODULE, error) { return 1, nil })
	replaceNative(t, &winRegisterClass, func(*wm.WNDCLASSW) (uint16, error) { return 0, nil })
	if backendCreateWindow(owner) == nil {
		t.Fatal("zero class atom accepted")
	}
	assertCoreEqual(t, owner.className, "")
	replaceNative(t, &winRegisterClass, func(*wm.WNDCLASSW) (uint16, error) { return 1, nil })
	assertCoreError(t, backendCreateWindow(owner), domain.ErrPermissionDenied)
	replaceNative(t, &winFindProcedure, func(*native.Proc) error { return nil })
	assertCoreError(t, backendInitialize(owner), domain.ErrPermissionDenied)
	replaceNative(t, &winCreateEvent, func(*security.SECURITY_ATTRIBUTES, bool, bool, *string) (foundation.HANDLE, error) { return 1, nil })
	assertCoreError(t, backendInitialize(owner), domain.ErrPermissionDenied)
	fixture.err = nil

	owner = fixtureBackend(t)
	nativeOK(t, backendCleanupWindow(owner))
	nativeOK(t, backendCloseWakeEvent(owner))
	owner.hwnd, owner.wakeEvent, owner.className = 2, 1, "class"
	nativeOK(t, backendCleanupWindow(owner))
	nativeOK(t, backendCloseWakeEvent(owner))
	assertCoreEqual(t, owner.wakeEvent, foundation.HANDLE(0))

	owner = fixtureBackend(t)
	owner.ready <- domain.ErrUnsupported
	t.Log("startup completion")
	close(owner.done)
	_, err = finishBackendStartup(t.Context(), owner)
	assertCoreError(t, err, domain.ErrUnsupported)
	owner = fixtureBackend(t)
	owner.closed = true
	close(owner.done)
	owner.closeErr = domain.ErrUnsupported
	_, err = backendCheckStartup(ctx, owner)
	assertCoreError(t, err, context.Canceled)
	assertCoreError(t, err, domain.ErrUnsupported)
	owner.ready <- nil
	_, err = finishBackendStartup(ctx, owner)
	assertCoreError(t, err, context.Canceled)
	owner = fixtureBackend(t)
	owner.ready <- nil
	result, err := finishBackendStartup(t.Context(), owner)
	nativeOK(t, err)
	assertCoreEqual(t, result, owner)

	owner = fixtureBackend(t)
	fixture.err = domain.ErrUnsupported
	t.Log("close")
	assertCoreError(t, backendCloseContext(t.Context(), owner), domain.ErrUnsupported)
	fixture.err = nil
	replaceNative(t, &winSetEvent, func(foundation.HANDLE) error {
		go func() { backendDrainCommands(owner); backendFinish(owner, nil) }()
		return nil
	})
	nativeOK(t, backendCloseContext(t.Context(), owner))
	assertCoreEqual(t, owner.stopping, true)
	assertCoreEqual(t, owner.closed, true)
	if !errors.Is(normalizeError(domain.ErrUnsupported), domain.ErrUnsupported) {
		t.Fatal("error lost")
	}
}
