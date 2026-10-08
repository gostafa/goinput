// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

import (
	"cmp"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"unsafe"

	native "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	hid "github.com/deploymenttheory/go-bindings-win32/bindings/win32/devices/humaninterfacedevice"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/storage/filesystem"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/libraryloader"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/threading"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/input"
	wm "github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/windowsandmessaging"
	ext "github.com/gostafa/goinput/extensions/win32"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	backendview "github.com/gostafa/goinput/internal/ports/backend"
	captureview "github.com/gostafa/goinput/internal/ports/capture"
)

const (
	relativeAxisPrefix = "rel:"
	absoluteAxisPrefix = "abs:"
	axisX              = "x"
	axisY              = "y"
	angularUnits       = 0x14
	buttonIDFormat     = "button:%d"
	buttonPage         = 9
	fortyEighthValue   = 0x30
)

func newBackend(
	ctx context.Context,
	environment *nativeState,
	retrier ports.Retrier,
) (*backend, error) {
	cause := context.Cause(ctx)
	if cause != nil {
		return nil, errors.Join(cause)
	}

	owner := makeBackend(environment, retrier)

	go backendRun(owner)

	result, err := finishBackendStartup(ctx, owner)
	if err != nil {
		return nil, errors.Join(err)
	}

	return result, nil
}

func makeBackend(environment *nativeState, retrier ports.Retrier) *backend {
	owner := new(backend)

	owner.native, owner.retrier = environment, retrier
	owner.commands = make(chan *command, commandCapacity)
	owner.ready = make(chan error, singleValue)
	owner.done = make(chan struct{})
	owner.captures = make(map[foundation.HANDLE]map[*capture]struct{})
	owner.registrations = make(map[topLevel]int)

	return owner
}

func backendCheckStartup(ctx context.Context, owner *backend) (*backend, error) {
	cause := context.Cause(ctx)
	if cause != nil {
		return nil, errors.Join(cause, backendCloseContext(ctx, owner))
	}

	return owner, nil
}

func backendRun(owner *backend) {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	var err error

	defer func() { backendFinish(owner, err) }()

	err = backendInitialize(owner)
	owner.ready <- err

	if err == nil {
		err = backendMessageLoop(owner)
	}
}

func findProcedures(procedures []*native.Proc) error {
	for entryIndex := range procedures {
		procedure := procedures[entryIndex]

		probeErr := procedure.Find()
		if probeErr != nil {
			return errors.Join(domain.ErrUnsupported, probeErr)
		}
	}

	return nil
}

func backendInitialize(owner *backend) error {
	procedures := []*native.Proc{
		wm.Procs.MsgWaitForMultipleObjectsEx, wm.Procs.CreateWindowEx,
		input.Procs.GetRawInputData, input.Procs.RegisterRawInputDevices,
	}

	probeErr := findProcedures(procedures)
	if probeErr != nil {
		return errors.Join(probeErr)
	}

	initializeCallback(owner.native)

	event, err := threading.CreateEvent(nil, false, false, nil)
	if err != nil {
		return errors.Join(normalizeError(err))
	}

	owner.wakeEvent = event

	return errors.Join(backendCreateWindow(owner))
}

func backendCreateWindow(owner *backend) error {
	instance, err := libraryloader.GetModuleHandle(nil)
	if err != nil {
		return errors.Join(normalizeError(err))
	}

	registerErr := backendRegisterWindowClass(owner, foundation.HINSTANCE(instance))
	if registerErr != nil {
		return errors.Join(registerErr)
	}

	return errors.Join(backendCreateMessageWindow(owner, foundation.HINSTANCE(instance)))
}

func backendPublishWindow(owner *backend, window foundation.HWND) {
	owner.mu.Lock()

	owner.hwnd = window
	owner.mu.Unlock()
	owner.native.windows.Store(window, owner)
}

func backendRegisterWindowClass(owner *backend, instance foundation.HINSTANCE) error {
	owner.className = windowClassName(owner.native)

	class := makeWindowClass(owner, instance)
	atom, err := wm.RegisterClass(&class)

	if atom != noValue {
		return nil
	}

	owner.className = emptyString

	return errors.Join(normalizeError(nonzeroError(err)))
}

func backendMessageLoop(owner *backend) error {
	handles := []foundation.HANDLE{owner.wakeEvent}
	for !owner.stopping {
		waitErr := backendWaitForMessages(owner, handles)
		if waitErr != nil {
			return errors.Join(waitErr)
		}

		if backendDispatchMessages(owner) {
			return nil
		}
	}

	return nil
}

func backendWaitForMessages(owner *backend, handles []foundation.HANDLE) error {
	result, err := wm.MsgWaitForMultipleObjectsEx(
		handles,
		infiniteWait,
		wm.QUEUE_STATUS_FLAGS(
			queueAllInput,
		),
		wm.MSG_WAIT_FOR_MULTIPLE_OBJECTS_EX_FLAGS(fourthValue),
	)
	if result == infiniteWait {
		return errors.Join(normalizeError(nonzeroError(err)))
	}

	if result == noValue {
		backendDrainCommands(owner)
	}

	return nil
}

func backendDispatchMessages(owner *backend) bool {
	var msg wm.MSG

	for !owner.stopping && wm.PeekMessage(&msg, noValue, noValue, noValue, wm.PEEK_MESSAGE_REMOVE_TYPE(singleValue)) {
		if msg.Message == messageQuit {
			return true
		}

		wm.DispatchMessage(&msg)
		backendDrainCommands(owner) // High-frequency input must not starve Close/Open.
	}

	return false
}

func nativeStateWindowProc(environment *nativeState, event *windowMessage) foundation.LRESULT {
	value, found := environment.windows.Load(event.hwnd)
	if !found {
		return windowMessageDefaultResult(event)
	}

	owner, valid := value.(*backend)
	if !valid {
		return windowMessageDefaultResult(event)
	}

	return backendHandleWindowMessage(owner, event)
}

func windowMessageDefaultResult(event *windowMessage) foundation.LRESULT {
	return wm.DefWindowProc(event.hwnd, event.message, event.wParam, event.lParam)
}

func backendHandleWindowMessage(owner *backend, event *windowMessage) foundation.LRESULT {
	switch event.message {
	case messageWake:
		backendDrainCommands(owner)
	case byteMask:
		return backendHandleInputMessage(owner, event)
	case messageDeviceChange:
		backendHandleDeviceChange(owner, event)
	default:
		return windowMessageDefaultResult(event)
	}

	return noValue
}

func backendHandleInputMessage(owner *backend, event *windowMessage) foundation.LRESULT {
	backendReadInput(owner, input.HRAWINPUT(event.lParam))

	// Foreground WM_INPUT requires the default procedure for native cleanup.
	if event.wParam&byteMask == noValue {
		return windowMessageDefaultResult(event)
	}

	return noValue
}

func backendHandleDeviceChange(owner *backend, event *windowMessage) {
	if event.wParam == secondValue {
		backendDisconnect(owner, foundation.HANDLE(event.lParam))
	}
}

func backendDrainCommands(owner *backend) {
	for {
		select {
		case cmd := <-owner.commands:
			cmd.reply <- backendExecuteCommand(owner, cmd)
		default:
			return
		}
	}
}

func backendExecuteCommand(owner *backend, cmd *command) error {
	if !cmd.state.CompareAndSwap(noValue, singleValue) {
		return errors.Join(context.Canceled)
	}

	if owner.stopping {
		return domain.ErrClosed
	}

	cause := cmd.cause()
	if cause != nil {
		return errors.Join(cause)
	}

	return errors.Join(cmd.operation())
}

func backendCall(ctx context.Context, owner *backend, operation func() error) error {
	callErr := backendCheckCall(ctx, owner)
	if callErr != nil {
		return errors.Join(callErr)
	}

	cmd := &command{
		commandState: makeCommand(ctx, operation),
		reply:        make(chan error, singleValue),
	}

	submitErr := submitCommand(owner, cmd)
	if submitErr != nil {
		return errors.Join(submitErr)
	}

	return errors.Join(backendAwaitCommand(owner, cmd))
}

func backendCheckCall(ctx context.Context, owner *backend) error {
	cause := context.Cause(ctx)
	if cause != nil {
		return errors.Join(cause)
	}

	if backendIsClosed(owner) {
		return domain.ErrClosed
	}

	return nil
}

func backendIsClosed(owner *backend) bool {
	owner.mu.Lock()
	defer owner.mu.Unlock()

	return owner.closed
}

func backendEnqueue(owner *backend, cmd *command) error {
	select {
	case owner.commands <- cmd:
		return nil
	case <-cmd.done:
		return errors.Join(cmd.cause())
	case <-owner.done:
		return domain.ErrClosed
	}
}

func backendSignalWake(owner *backend) error {
	owner.mu.Lock()
	defer owner.mu.Unlock()

	if owner.closed {
		return domain.ErrClosed
	}

	// Holding mu keeps teardown from closing or reusing the HANDLE during SetEvent.
	return errors.Join(threading.SetEvent(owner.wakeEvent))
}

func backendSignalCommand(owner *backend, cmd *command) error {
	err := backendSignalWake(owner)
	if err == nil {
		return nil
	}

	// Commands already running must finish so their acquired resources can be released.
	if !cmd.state.CompareAndSwap(noValue, secondValue) {
		return nil
	}

	select {
	case <-owner.done:
		return domain.ErrClosed
	default:
		return errors.Join(normalizeError(err))
	}
}

func backendAwaitCommand(owner *backend, cmd *command) error {
	wait := commandWait{command: cmd, contextDone: cmd.done, err: nil, complete: false}
	for !wait.complete {
		backendWaitCommandStep(owner, &wait)
	}

	return errors.Join(wait.err)
}

func backendWaitCommandStep(owner *backend, wait *commandWait) {
	select {
	case wait.err = <-wait.command.reply:
		wait.complete = true
	case <-owner.done:
		wait.err, wait.complete = domain.ErrClosed, true
	case <-wait.contextDone:
		wait.err = cancelPendingCommand(wait.command.commandState)
		wait.complete = wait.err != nil
		wait.contextDone = nil // Native work has started; wait for its result before releasing resources.
	}
}

func backendFinish(owner *backend, cause error) {
	owner.mu.Lock()

	owner.closed = true
	owner.mu.Unlock()
	backendFailCaptures(owner, cause)

	cleanupErr := backendCleanupWindow(owner)
	owner.mu.Lock()

	cleanupErr = errors.Join(cleanupErr, backendCloseWakeEvent(owner))
	owner.closeErr = normalizeError(errors.Join(cause, cleanupErr))
	owner.mu.Unlock()
	close(owner.done)
}

func backendFailCaptures(owner *backend, cause error) {
	if cause == nil {
		cause = domain.ErrClosed
	}

	backendFailAll(owner, cause)
}

func backendCleanupWindow(owner *backend) error {
	var err error

	for usage := range owner.registrations {
		err = errors.Join(err, backendRemoveRegistration(owner, usage))
	}

	return errors.Join(err, backendDestroyWindow(owner), backendUnregisterWindowClass(owner))
}

func backendDestroyWindow(owner *backend) error {
	if owner.hwnd == noValue {
		return nil
	}

	owner.native.windows.Delete(owner.hwnd)

	return errors.Join(wm.DestroyWindow(owner.hwnd))
}

func backendUnregisterWindowClass(owner *backend) error {
	if owner.className == emptyString {
		return nil
	}

	instance, err := libraryloader.GetModuleHandle(nil)

	return errors.Join(err, wm.UnregisterClass(owner.className, foundation.HINSTANCE(instance)))
}

func backendCloseWakeEvent(owner *backend) error {
	if owner.wakeEvent == noValue {
		return nil
	}

	err := foundation.CloseHandle(owner.wakeEvent)

	owner.wakeEvent = noValue

	return errors.Join(err)
}

func backendCloseContext(ctx context.Context, owner *backend) error {
	err := backendCall(context.WithoutCancel(ctx), owner, func() error {
		owner.mu.Lock()

		owner.closed = true
		owner.mu.Unlock()

		owner.stopping = true

		return nil
	})
	if err != nil && !errors.Is(err, domain.ErrClosed) {
		return errors.Join(err)
	}

	<-owner.done
	owner.mu.Lock()
	defer owner.mu.Unlock()

	return errors.Join(owner.closeErr)
}

func backendDiscover(ctx context.Context, owner *backend) ([]domain.DeviceInfo, error) {
	devices, err := backendEnumerateDevices(ctx, owner)
	infos := make([]domain.DeviceInfo, noValue, len(devices))

	for entryIndex := range devices {
		device := devices[entryIndex]

		infos = append(infos, domain.CloneInfo(&device.info))
	}

	return infos, errors.Join(err)
}

func backendEnumerateDevices(ctx context.Context, owner *backend) ([]nativeDevice, error) {
	if backendIsClosed(owner) {
		return nil, domain.ErrClosed
	}

	list, err := backendRawDevices(ctx, owner)
	if err != nil {
		return nil, errors.Join(err)
	}

	result, err := backendDescribeDevices(ctx, owner, list)

	return result, errors.Join(err)
}

func backendRetry(
	ctx context.Context, owner *backend,

	operation func(context.Context) error,
) error {
	if owner.retrier == nil {
		return errors.Join(operation(ctx))
	}

	return errors.Join(owner.retrier.Do(ctx, operation, transient))
}

func enumerateRawDevices(ctx context.Context) ([]input.RAWINPUTDEVICELIST, error) {
	cause := context.Cause(ctx)
	if cause != nil {
		return nil, errors.Join(cause)
	}

	result, err := queryRawDevices()

	return result, errors.Join(err)
}

func queryRawDevices() ([]input.RAWINPUTDEVICELIST, error) {
	var count uint32

	size := nativeSize[input.RAWINPUTDEVICELIST]()

	queryErr := resultError(rawDeviceList(nil, &count, size))
	if queryErr != nil {
		return nil, errors.Join(queryErr)
	}

	values, readErr := readDeviceInventory(count, size, readRawDeviceList)

	return values, errors.Join(readErr)
}

func readRawDeviceList(count, size uint32) ([]input.RAWINPUTDEVICELIST, error) {
	list := allocateBuffer[input.RAWINPUTDEVICELIST](int(count))

	written, err := rawDeviceList(&list[noValue], &count, size)
	if err != nil {
		return nil, errors.Join(err)
	}

	if uint64(written) > uint64(len(list)) {
		return nil, domain.ErrInvalidOptions
	}

	return list[:written], nil
}

func backendDescribeDevices(
	ctx context.Context, owner *backend,
	list []input.RAWINPUTDEVICELIST,
) ([]nativeDevice, error) {
	var snapshot deviceSnapshot

	for entryIndex := range list {
		cause := context.Cause(ctx)
		if cause != nil {
			return snapshot.devices, errors.Join(snapshot.diagnostics, cause)
		}

		entry := inventoryEntryArguments{snapshot: &snapshot, item: list[entryIndex]}
		describeInventoryEntry(ctx, owner, &entry)
	}

	values, err := finishDeviceSnapshot(ctx, &snapshot)

	return values, errors.Join(err)
}

func deviceSnapshotAdd(snapshot *deviceSnapshot, device *nativeDevice, err error) {
	if err != nil {
		snapshot.diagnostics = errors.Join(snapshot.diagnostics, normalizeError(err))

		return
	}

	snapshot.devices = append(snapshot.devices, *device)
}

func backendDescribeRawDevice(
	ctx context.Context, owner *backend,

	item input.RAWINPUTDEVICELIST,
) (nativeDevice, error) {
	var device nativeDevice

	err := backendRetry(ctx, owner, func(ctx context.Context) error {
		var err error

		device, err = describeDevice(ctx, item.HDevice, uint32(item.DwType))

		return errors.Join(err)
	})

	return device, errors.Join(err)
}

func backendOpen(
	ctx context.Context, owner *backend, args *backendOpenArguments,
) (*capture, error) {
	id, sink := args.id, args.sink

	device, err := backendFindDevice(ctx, owner, id)
	if err != nil {
		return nil, errors.Join(err)
	}

	subscription := backendMakeCapture(owner, &device, sink)

	result, openErr := completeCaptureOpen(ctx, owner, subscription)
	if openErr != nil {
		return nil, errors.Join(openErr)
	}

	return result, nil
}

func backendFindDevice(
	ctx context.Context, owner *backend,

	id domain.DeviceID,
) (nativeDevice, error) {
	discovery, cancel := context.WithTimeout(ctx, ports.DiscoveryBudget)
	devices, err := backendEnumerateDevices(discovery, owner)

	cancel()

	if err != nil && (len(devices) == noValue || context.Cause(ctx) != nil) {
		return nativeDevice{}, errors.Join(err)
	}

	result, err := selectDevice(devices, id, err)

	return result, errors.Join(err)
}

func selectDevice(
	devices []nativeDevice,
	id domain.DeviceID,
	diagnostics error,
) (nativeDevice, error) {
	for entryIndex := range devices {
		device := devices[entryIndex]
		if device.info.ID == id {
			return device, nil
		}
	}

	return nativeDevice{}, errors.Join(domain.ErrNotFound, diagnostics)
}

func backendMakeCapture(owner *backend, device *nativeDevice, sink ports.EventSink) *capture {
	subscription := new(capture)

	subscription.device, subscription.sink = *device, sink
	subscription.info = domain.CloneInfo(&device.info)
	subscription.native = makeNativeInfo(device)
	subscription.held = make(map[domain.ControlID]bool)
	subscription.values = make(map[domain.ControlID]int64)
	subscription.buttons = make(map[byte]map[domain.ControlID]bool)
	subscription.backend = makeCaptureHost(owner, subscription)

	return subscription
}

func captureLoadCapabilities(ctx context.Context, subscription *capture) error {
	switch subscription.device.kind {
	case singleValue:
		captureKeyboardCapabilities(subscription)
	case noValue:
		captureMouseCapabilities(subscription)
	case secondValue:
		return errors.Join(captureHidCapabilities(ctx, subscription))
	default:
		return domain.ErrUnsupported
	}

	return nil
}

func captureRegister(owner *backend, subscription *capture) error {
	registerErr := backendAcquireRegistration(owner, subscription.device.tlc)
	if registerErr != nil {
		return errors.Join(registerErr)
	}

	if owner.captures[subscription.device.handle] == nil {
		owner.captures[subscription.device.handle] = make(map[*capture]struct{})
	}

	owner.captures[subscription.device.handle][subscription] = struct{}{}

	return nil
}

func captureCheckOpened(ctx context.Context, subscription *capture) (*capture, error) {
	cause := context.Cause(ctx)
	if cause != nil {
		return nil, errors.Join(cause, captureClose(ctx, subscription))
	}

	return subscription, nil
}

func captureInfo(subscription *capture) domain.DeviceInfo {
	return domain.CloneInfo(&subscription.info)
}

func captureCapabilities(subscription *capture) domain.Capabilities {
	return domain.CloneCapabilities(&subscription.caps)
}

func captureNativeInfo(subscription *capture) ext.Info {
	info := subscription.native

	info.Controls = append([]ext.NativeControl(nil), info.Controls...)

	return info
}

func captureExtension(subscription *capture, target any) bool {
	if out, ok := target.(*ext.Info); ok && out != nil {
		*out = captureNativeInfo(subscription)

		return true
	}

	if out, ok := target.(*ext.Metadata); ok && out != nil {
		*out = metadataView[ext.Info](func() ext.Info { return captureNativeInfo(subscription) })

		return true
	}

	return false
}

func captureClose(ctx context.Context, subscription *capture) error {
	subscription.closeOnce.Do(func() { captureCloseCapture(ctx, subscription) })

	return errors.Join(normalizeError(subscription.closeErr))
}

func captureCloseCapture(ctx context.Context, subscription *capture) {
	subscription.closed.Store(true)

	subscription.closeErr = subscription.backend.call(
		context.WithoutCancel(ctx),
		subscription.backend.unregister,
	)
	if errors.Is(subscription.closeErr, domain.ErrClosed) {
		subscription.closeErr = nil
	}
}

func captureUnregister(owner *backend, subscription *capture) error {
	group := owner.captures[subscription.device.handle]
	if _, exists := group[subscription]; !exists {
		return nil
	}

	delete(group, subscription)

	if len(group) == noValue {
		delete(owner.captures, subscription.device.handle)
	}

	return errors.Join(backendReleaseRegistration(owner, subscription.device.tlc))
}

func backendAcquireRegistration(owner *backend, usage topLevel) error {
	if usage.page == noValue || usage.usage == noValue {
		return domain.ErrUnsupported
	}

	list, err := registeredDevices()
	if err != nil {
		return errors.Join(err)
	}

	if backendRegistrationConflict(owner, usage, list) {
		return domain.ErrRegistrationConflict
	}

	return errors.Join(backendRetainRegistration(owner, usage))
}

func backendRetainRegistration(owner *backend, usage topLevel) error {
	if owner.registrations[usage] != noValue {
		owner.registrations[usage]++

		return nil
	}

	registerErr := backendAddRegistration(owner, usage)
	if registerErr != nil {
		return errors.Join(registerErr)
	}

	owner.registrations[usage] = singleValue

	return nil
}

func backendRegistrationConflict(owner *backend, usage topLevel, list []input.RAWINPUTDEVICE) bool {
	for entryIndex := range list {
		registration := list[entryIndex]
		if matchesRegistration(usage, registration) && registration.HwndTarget != owner.hwnd {
			return true
		}
	}

	return false
}

func matchesRegistration(usage topLevel, registration input.RAWINPUTDEVICE) bool {
	pageOnly := uint32(registration.DwFlags)&fortyEighthValue == thirtySecondValue

	return registration.UsUsagePage == usage.page &&
		(registration.UsUsage == usage.usage || pageOnly)
}

func backendAddRegistration(owner *backend, usage topLevel) error {
	return errors.Join(input.RegisterRawInputDevices([]input.RAWINPUTDEVICE{{
		UsUsagePage: usage.page, UsUsage: usage.usage,
		DwFlags: input.RAWINPUTDEVICE_FLAGS(registrationFlags), HwndTarget: owner.hwnd,
	}}, nativeSize[input.RAWINPUTDEVICE]()))
}

func backendReleaseRegistration(owner *backend, usage topLevel) error {
	if owner.registrations[usage] > singleValue {
		owner.registrations[usage]--

		return nil
	}

	if owner.registrations[usage] == noValue {
		return nil
	}

	owner.registrations[usage] = noValue

	err := backendRemoveRegistration(owner, usage)
	if err == nil {
		delete(owner.registrations, usage)
	}

	return errors.Join(err)
}

func backendRemoveRegistration(owner *backend, usage topLevel) error {
	list, err := registeredDevices()
	if err != nil {
		return errors.Join(err)
	}

	for entryIndex := range list {
		registration := list[entryIndex]
		if registration.UsUsagePage == usage.page && registration.UsUsage == usage.usage {
			return errors.Join(backendRemoveOwnedRegistration(owner, registration))
		}
	}

	return nil
}

func backendRemoveOwnedRegistration(owner *backend, registration input.RAWINPUTDEVICE) error {
	if registration.HwndTarget != owner.hwnd {
		return domain.ErrRegistrationConflict
	}

	registration.DwFlags = input.RAWINPUTDEVICE_FLAGS(singleValue)
	registration.HwndTarget = noValue

	return errors.Join(
		input.RegisterRawInputDevices(
			[]input.RAWINPUTDEVICE{registration},
			uint32(unsafe.Sizeof(registration)),
		),
	)
}

func registeredDevices() ([]input.RAWINPUTDEVICE, error) {
	var count uint32

	size := nativeSize[input.RAWINPUTDEVICE]()
	written, err := input.GetRegisteredRawInputDevices(nil, &count, size)

	if written == infiniteWait {
		return nil, errors.Join(nonzeroError(err))
	}

	values, readErr := readDeviceInventory(count, size, readRegisteredDevices)

	return values, errors.Join(readErr)
}

func readRegisteredDevices(count, size uint32) ([]input.RAWINPUTDEVICE, error) {
	list := allocateBuffer[input.RAWINPUTDEVICE](int(count))
	written, err := input.GetRegisteredRawInputDevices(&list[noValue], &count, size)

	if written == infiniteWait {
		return nil, errors.Join(nonzeroError(err))
	}

	if uint64(written) > uint64(len(list)) {
		return nil, domain.ErrInvalidOptions
	}

	return list[:written], nil // Ignore stale GetLastError on successful UINT results.
}

func backendDisconnect(owner *backend, handle foundation.HANDLE) {
	group := owner.captures[handle]
	delete(owner.captures, handle)

	for subscription := range group {
		releaseErr := backendReleaseRegistration(owner, subscription.device.tlc)
		if !subscription.closed.Swap(true) {
			subscription.sink.Fail(errors.Join(domain.ErrDisconnected, releaseErr))
		}
	}
}

func rawDeviceList(list *input.RAWINPUTDEVICELIST, count *uint32, size uint32) (uint32, error) {
	written, err := input.GetRawInputDeviceList(list, count, size)
	if written == infiniteWait {
		return written, errors.Join(nonzeroError(err))
	}

	return written, nil
}

func rawDeviceInfo(handle foundation.HANDLE, command input.RAW_INPUT_DEVICE_INFO_COMMAND,
	buffer nativeBuffer,
) (uint32, error) {
	written, err := input.GetRawInputDeviceInfo(handle, command, buffer.data, buffer.size)
	if written == infiniteWait {
		return written, errors.Join(nonzeroError(err))
	}

	return written, nil
}

func describeDevice(
	ctx context.Context,
	handle foundation.HANDLE,
	kind uint32,
) (nativeDevice, error) {
	var device nativeDevice

	device.handle, device.kind = handle, kind

	err := loadDeviceMetadata(&device)
	if err != nil {
		return device, errors.Join(err)
	}

	identityErr := enrichDeviceIdentity(ctx, &device)

	return device, errors.Join(identityErr)
}

func nativeDeviceLoadDeviceInfo(device *nativeDevice) error {
	var info input.RID_DEVICE_INFO

	info.CbSize = uint32(unsafe.Sizeof(info))

	size := info.CbSize

	err := resultError(
		rawDeviceInfo(device.handle, deviceInfoCommand, nativeBuffer{nativePointer(&info), &size}),
	)
	if err != nil {
		return errors.Join(err)
	}

	return errors.Join(nativeDeviceApplyDeviceInfo(device, info.Anonymous.Data))
}

func nativeDeviceApplyDeviceInfo(device *nativeDevice, words [sixthValue]uint32) error {
	switch device.kind {
	case noValue:
		applyMouseDeviceInfo(device, words)
	case singleValue:
		applyKeyboardDeviceInfo(device)
	case secondValue:
		nativeDeviceApplyHIDDeviceInfo(device, words)
	default:
		return domain.ErrUnsupported
	}

	return nil
}

func nativeDeviceApplyHIDDeviceInfo(device *nativeDevice, words [sixthValue]uint32) {
	device.tlc = topLevel{
		uint16(words[thirdValue] & wordMask),
		uint16(words[thirdValue] >> sixteenthValue),
	}
	device.version = words[secondValue]
	device.info.VendorID = optionalWord(words[noValue])
	device.info.ProductID = optionalWord(words[singleValue])
	device.info.Classes = []domain.DeviceClass{classFor(device.tlc)}
}

func optionalWord(value uint32) *uint16 {
	if value > wordMask {
		return nil
	}

	result := uint16(value)

	return &result
}

func nativeDeviceLoadDevicePath(device *nativeDevice) error {
	path, err := readDevicePath(device.handle)
	if err != nil {
		return errors.Join(err)
	}

	device.info.Path = path
	device.info.ID = domain.DeviceID("win32:" + strings.ToLower(path))

	return nil
}

func readDevicePath(handle foundation.HANDLE) (string, error) {
	var chars uint32

	queryErr := resultError(
		rawDeviceInfo(handle, deviceNameCommand, nativeBuffer{nil, &chars}),
	)
	if queryErr != nil {
		return emptyString, errors.Join(queryErr)
	}

	if chars == noValue || chars > maxNativeBuffer/secondValue {
		return emptyString, domain.ErrNotFound
	}

	result, err := readDeviceName(handle, chars)

	return result, errors.Join(err)
}

func readDeviceName(handle foundation.HANDLE, chars uint32) (string, error) {
	path := allocateBuffer[uint16](int(chars + singleValue))

	err := resultError(
		rawDeviceInfo(
			handle,
			deviceNameCommand,
			nativeBuffer{nativePointer(&path[noValue]), &chars},
		),
	)
	if err != nil {
		return emptyString, errors.Join(err)
	}

	result := syscall.UTF16ToString(path)
	if result == emptyString {
		return emptyString, domain.ErrNotFound
	}

	return result, nil
}

func enrichIdentity(device *nativeDevice) (err error) {
	if !identityProceduresAvailable() {
		return nil
	}

	handle, openErr := openIdentityHandle(device.info.Path)
	if openErr != nil {
		return nil
	}

	defer func() { err = errors.Join(err, foundation.CloseHandle(handle.value)) }()

	nativeDeviceLoadAttributes(device, handle.value)
	nativeDeviceLoadStrings(device, handle.value)

	return nil
}

func nativeDeviceLoadAttributes(device *nativeDevice, handle foundation.HANDLE) {
	var attributes hid.HIDD_ATTRIBUTES

	attributes.Size = uint32(unsafe.Sizeof(attributes))

	if hid.HidD_GetAttributes(handle, &attributes) != noValue {
		vendor, product := attributes.VendorID, attributes.ProductID

		device.info.VendorID, device.info.ProductID = &vendor, &product
		device.version = uint32(attributes.VersionNumber)
	}
}

func nativeDeviceLoadStrings(device *nativeDevice, handle foundation.HANDLE) {
	if name := readHIDString(handle, hid.HidD_GetProductString); name != emptyString {
		device.info.Name = name
	}

	device.info.Manufacturer = readHIDString(handle, hid.HidD_GetManufacturerString)
	device.info.Serial = readHIDString(handle, hid.HidD_GetSerialNumberString)
}

func readHIDString(
	handle foundation.HANDLE,
	get func(foundation.HANDLE, []byte) foundation.BOOLEAN,
) string {
	data := allocateBuffer[byte](hidStringBytes)
	if get(handle, data) == noValue {
		return emptyString
	}

	wide := allocateBuffer[uint16](len(data) / secondValue)
	for index := range wide {
		wide[index] = binary.LittleEndian.Uint16(data[index*secondValue:])
	}

	return syscall.UTF16ToString(wide)
}

func classFor(tlc topLevel) domain.DeviceClass {
	if tlc.page != singleValue {
		return domain.ClassOther
	}

	return desktopClass(tlc.usage)
}

func desktopClass(usage uint16) domain.DeviceClass {
	classes := map[uint16]domain.DeviceClass{
		secondValue:  domain.ClassMouse,
		fourthValue:  domain.ClassJoystick,
		fifthValue:   domain.ClassGamepad,
		sixthValue:   domain.ClassKeyboard,
		seventhValue: domain.ClassKeyboard,
	}
	if class, found := classes[usage]; found {
		return class
	}

	return domain.ClassOther
}

func nonzeroError(err error) error {
	if err == nil {
		return errors.Join(syscall.EINVAL)
	}

	return errors.Join(err)
}

func transient(err error) bool {
	return errors.Is(err, syscall.Errno(errorInsufficientBuffer)) ||
		errors.Is(err, syscall.Errno(errorMoreData)) ||
		errors.Is(err, syscall.Errno(errorNotReady))
}

func normalizeError(err error) error {
	if err == nil {
		return nil
	}

	categories := []errorCategory{
		{syscall.ERROR_ACCESS_DENIED, domain.ErrPermissionDenied},
		{syscall.Errno(thirtySecondValue), domain.ErrPermissionDenied},
		{syscall.ERROR_FILE_NOT_FOUND, domain.ErrNotFound},
		{syscall.ERROR_PATH_NOT_FOUND, domain.ErrNotFound},
		{syscall.Errno(sixthValue), domain.ErrNotFound},
		{syscall.Errno(errorDeviceDisconnected), domain.ErrDisconnected},
		{syscall.Errno(errorInvalidParameter), domain.ErrInvalidOptions},
	}

	return errors.Join(categorizeError(err, categories))
}

func categorizeError(err error, categories []errorCategory) error {
	for entryIndex := range categories {
		category := categories[entryIndex]
		if errors.Is(err, category.native) {
			return errors.Join(category.domain, err)
		}
	}

	return errors.Join(err)
}

func captureKeyboardCapabilities(subscription *capture) {
	controls := make(map[domain.ControlID]domain.Control)
	addPhysicalKeyControls(subscription, controls)
	captureAddVirtualControls(subscription, controls)
	captureAddSystemControls(subscription, controls)

	appendKeyboardControls(subscription, controls)

	subscription.caps.Repeat = domain.SupportSupported
	sortKeyboardControls(subscription)
}

func captureAddScanControls(subscription *capture,
	controls map[domain.ControlID]domain.Control, args *captureAddScanControlsArguments,
) {
	scans, page := args.scans, args.page
	for scan := range scans {
		usage := scans[scan]
		control := captureMakeKeyControl(subscription, scan, domain.HID(page, usage))

		controls[control.ID] = control
	}
}

func captureAddVirtualControls(
	subscription *capture,
	controls map[domain.ControlID]domain.Control,
) {
	for key := range subscription.backend.tables.virtual {
		usage := subscription.backend.tables.virtual[key]
		control := captureMakeKeyControl(subscription,
			virtualScanPrefix|key,
			domain.HID(twelfthValue, usage),
		)

		controls[control.ID] = control
	}
}

func captureAddSystemControls(subscription *capture, controls map[domain.ControlID]domain.Control) {
	for entryIndex := range []uint16{scanPower, scanSleep, scanWake} {
		scan := []uint16{scanPower, scanSleep, scanWake}[entryIndex]
		control := captureMakeKeyControl(subscription, scan, systemScanUsage(scan))

		controls[control.ID] = control
	}
}

func captureMakeKeyControl(subscription *capture, scan uint16, usage domain.Usage) domain.Control {
	control := makeKeyDescriptor(scan, usage)
	nativeControl := makeKeyNative(control.ID, scan)

	subscription.native.Controls = append(subscription.native.Controls, nativeControl)

	return control
}

func captureMouseCapabilities(subscription *capture) {
	subscription.caps.Repeat = domain.SupportUnsupported
	captureAddMouseButtons(subscription)
	captureAddMouseAxes(subscription)
	captureAddMouseWheels(subscription)

	if subscription.device.hwheel {
		subscription.caps.Controls[len(subscription.caps.Controls)-singleValue].Support = domain.SupportSupported
	}
}

func captureAddMouseButtons(subscription *capture) {
	for index := uint16(singleValue); index <= fifthValue; index++ {
		support := domain.SupportUnknown

		if uint32(index) <= subscription.device.buttons {
			support = domain.SupportSupported
		}

		subscription.caps.Controls = append(subscription.caps.Controls, domain.Control{
			Mode:    domain.AxisUnknown,
			ID:      domain.ControlID(fmt.Sprintf(buttonIDFormat, index)),
			Name:    fmt.Sprintf("Button %d", index),
			Kind:    domain.ControlButton,
			Usage:   domain.HID(buttonPage, index),
			Mapping: domain.MappingInferred,
			Range:   &domain.Range{Min: noValue, Max: singleValue},
			Unit:    domain.UnitBoolean,
			Support: support,
		})
	}
}

func captureAddMouseAxes(subscription *capture) {
	axes := []string{axisX, axisY}
	for index := range axes {
		usage := domain.HID(singleValue, uint16(fortyEighthValue+index))
		relative := makeMouseAxis(axes[index], usage)
		absolute := makeAbsoluteMouseAxis(axes[index], usage)

		subscription.caps.Controls = append(subscription.caps.Controls, relative, absolute)
	}
}

func captureAddMouseWheels(subscription *capture) {
	wheel := makeMouseWheel(wheelControlID, domain.HID(singleValue, wheelUsage), "Wheel")
	pan := makeMouseWheel(panControlID, domain.HID(twelfthValue, panUsage), "Horizontal wheel")

	subscription.caps.Controls = append(subscription.caps.Controls, wheel, pan)
}

func captureHidCapabilities(ctx context.Context, subscription *capture) error {
	return errors.Join(
		captureRetryMetadata(
			ctx,
			subscription,
			func(ctx context.Context) error { return errors.Join(captureLoadHIDCapabilities(ctx, subscription)) },
		),
	)
}

func captureLoadHIDCapabilities(ctx context.Context, subscription *capture) error {
	cause := context.Cause(ctx)
	if cause != nil {
		return errors.Join(cause)
	}

	probeErr := findProcedures(hidProcedures())
	if probeErr != nil {
		return errors.Join(probeErr)
	}

	data, err := captureReadPreparsedData(subscription)
	if err != nil {
		return errors.Join(err)
	}

	return errors.Join(captureBuildHIDCapabilities(subscription, data))
}

func captureBuildHIDCapabilities(subscription *capture, data []byte) error {
	builder, err := makeHIDBuilder(data)
	if err != nil {
		return errors.Join(err)
	}

	controlsErr := hidBuilderLoadControls(builder)
	if controlsErr != nil {
		return errors.Join(controlsErr)
	}

	captureCommitHID(subscription, builder)

	return nil
}

func hidProcedures() []*native.Proc {
	return []*native.Proc{
		hid.Procs.HidP_GetCaps, hid.Procs.HidP_GetButtonCaps, hid.Procs.HidP_GetValueCaps,
		hid.Procs.HidP_GetData, hid.Procs.HidP_MaxDataListLength,
	}
}

func captureReadPreparsedData(subscription *capture) ([]byte, error) {
	var size uint32

	command := input.RAW_INPUT_DEVICE_INFO_COMMAND(preparsedDataCommand)

	queryErr := resultError(
		rawDeviceInfo(subscription.device.handle, command, nativeBuffer{nil, &size}),
	)
	if queryErr != nil {
		return nil, errors.Join(queryErr)
	}

	if size == noValue || size > maxNativeBuffer {
		return nil, domain.ErrUnsupported
	}

	result, err := captureReadPreparsedBuffer(subscription, size)

	return result, errors.Join(err)
}

func captureReadPreparsedBuffer(subscription *capture, size uint32) ([]byte, error) {
	data := allocateBuffer[byte](int(size))

	err := resultError(rawDeviceInfo(subscription.device.handle, preparsedDataCommand,
		nativeBuffer{nativePointer(&data[noValue]), &size}))
	if err != nil {
		return nil, errors.Join(err)
	}

	result, err := trimPreparsedData(data, size)

	return result, errors.Join(err)
}

func trimPreparsedData(data []byte, size uint32) ([]byte, error) {
	if uint64(size) > uint64(len(data)) {
		return nil, domain.ErrEventLoss
	}

	if size == noValue {
		return nil, domain.ErrUnsupported
	}

	return data[:size], nil
}

func makeHIDBuilder(data []byte) (*hidBuilder, error) {
	preparsedAddress := hid.PHIDP_PREPARSED_DATA(uintptr(nativePointer(&data[noValue])))
	defer runtime.KeepAlive(data)

	var caps hid.HIDP_CAPS

	if status := hid.HidP_GetCaps(preparsedAddress, &caps); status != hid.HIDP_STATUS_SUCCESS {
		return nil, errors.Join(hidError("HidP_GetCaps", status))
	}

	desc := &descriptor{
		preparsed: data, reportLen: caps.InputReportByteLength,
		controls: make(map[hidIndex]hidControl), reportIDs: make(map[byte]bool),
		maxData: hid.HidP_MaxDataListLength(hid.HidP_Input, preparsedAddress),
	}
	if desc.maxData > maxDevices || desc.reportLen == noValue {
		return nil, domain.ErrUnsupported
	}

	return &hidBuilder{descriptor: desc, caps: caps, controls: nil, nativeControls: nil}, nil
}

func hidBuilderPreparsedPointer(builder *hidBuilder) hid.PHIDP_PREPARSED_DATA {
	return hid.PHIDP_PREPARSED_DATA(uintptr(nativePointer(&builder.descriptor.preparsed[noValue])))
}

func hidBuilderLoadControls(builder *hidBuilder) error {
	defer runtime.KeepAlive(builder.descriptor.preparsed)

	buttonsErr := hidBuilderLoadButtons(builder)
	if buttonsErr != nil {
		return errors.Join(buttonsErr)
	}

	return errors.Join(hidBuilderLoadValues(builder))
}

func hidBuilderAdd(builder *hidBuilder, control *hidControl) {
	key := hidDataIndex(control.native.ReportID, control.native.DataIndex)
	if _, exists := builder.descriptor.controls[key]; exists {
		return
	}

	builder.descriptor.controls[key] = *control
	builder.descriptor.reportIDs[control.native.ReportID] = true
	builder.controls = append(builder.controls, control.control)
	builder.nativeControls = append(builder.nativeControls, control.native)
}

func hidBuilderLoadButtons(builder *hidBuilder) error {
	args := makeCapabilityAdapter(hid.HidP_GetButtonCaps, projectButtonCapability, makeHIDButton)

	return errors.Join(loadNativeCapabilitySet(builder, builder.caps.NumberInputButtonCaps, args))
}

func projectButtonCapability(value *hid.HIDP_BUTTON_CAPS) *nativeCapability {
	return makeNativeCapability(value.Anonymous.Data, value.IsRange, value.IsAlias)
}

func makeHIDButton(button *hid.HIDP_BUTTON_CAPS, usage, dataIndex uint32) hidControl {
	id := hidID(button.ReportID, button.LinkCollection, uint16(dataIndex&wordMask))

	return hidControl{
		button:  true,
		control: makeHIDButtonDescriptor(button, usage, id),
		native:  makeHIDButtonNative(button, dataIndex, id), hat: false,
	}
}

func buttonKind(page uint16) domain.ControlKind {
	if page == seventhValue || page == twelfthValue {
		return domain.ControlKey
	}

	return domain.ControlButton
}

func buttonSupport(absolute foundation.BOOLEAN) domain.Support {
	if absolute == noValue {
		return domain.SupportUnsupported
	}

	return domain.SupportSupported
}

func hidBuilderLoadValues(builder *hidBuilder) error {
	args := makeCapabilityAdapter(hid.HidP_GetValueCaps, projectValueCapability, makeHIDValue)

	return errors.Join(loadNativeCapabilitySet(builder, builder.caps.NumberInputValueCaps, args))
}

func projectValueCapability(value *hid.HIDP_VALUE_CAPS) *nativeCapability {
	return makeNativeCapability(value.Anonymous.Data, value.IsRange, value.IsAlias)
}

func makeHIDValue(value *hid.HIDP_VALUE_CAPS, usage, dataIndex uint32) hidControl {
	bounds := logicalBounds(value.LogicalMin, value.LogicalMax)
	id := hidID(value.ReportID, value.LinkCollection, uint16(dataIndex&wordMask))
	ctrl := makeHIDValueDescriptor(value, usage, id)
	classifyValue(&ctrl, value, bounds)

	normalHat := isConventionalHat(value, usage, bounds)
	classifyHat(&ctrl, value, usage)

	return hidControl{
		control: ctrl,
		hat:     normalHat,
		native:  makeHIDValueNative(value, dataIndex, id),
		button:  false,
	}
}

func captureCommitHID(subscription *capture, builder *hidBuilder) {
	slices.SortFunc(builder.controls,

		func(left, owner domain.Control) int { return cmp.Compare(left.ID, owner.ID) })
	slices.SortFunc(builder.nativeControls,

		func(
			left, owner ext.NativeControl,
		) int {
			return cmp.Compare(left.ID, owner.ID)
		})

	subscription.hid = builder.descriptor
	subscription.caps = domain.Capabilities{
		Controls: builder.controls,
		Complete: true,
		Repeat:   domain.SupportUnsupported,
	}
	subscription.native.Controls = builder.nativeControls
}

func captureRetryMetadata(
	ctx context.Context, subscription *capture,

	operation func(context.Context) error,
) error {
	return errors.Join(subscription.backend.retry(ctx, operation))
}

func conventionalHat(value *hid.HIDP_VALUE_CAPS, minValue, maxValue int64) bool {
	direction, ok := domain.Hat(
		minValue,
		domain.Range{Min: minValue, Max: maxValue},
		value.HasNull != noValue,
	)
	if !ok || direction != domain.HatNorth {
		return false
	}

	return conventionalHatUnits(value) &&
		conventionalHatExtent(value.PhysicalMax, maxValue-minValue+singleValue)
}

func conventionalHatUnits(value *hid.HIDP_VALUE_CAPS) bool {
	return value.PhysicalMin == noValue &&
		(value.Units == noValue || value.Units == angularUnits) &&
		value.UnitsExp == noValue
}

func conventionalHatExtent(maximum int32, positions int64) bool {
	return maximum == noValue || (positions == fourthValue && maximum == cardinalExtent) ||
		(positions == eighthValue && maximum == compassExtent)
}

func hidError(operation string, status foundation.NTSTATUS) error {
	return fmt.Errorf(
		"%s: %w (HID status 0x%08x)",
		operation,
		domain.ErrUnsupported,
		unsignedWord(int32(status)),
	)
}

func backendReadInput(owner *backend, handle input.HRAWINPUT) {
	data, err := readInputData(handle)
	if err != nil {
		backendFailAll(owner, err)

		return
	}

	packet, err := decodeInputPacket(data)
	if err != nil {
		backendFailAll(owner, err)

		return
	}

	for subscription := range owner.captures[foundation.HANDLE(packet.device)] {
		captureHandlePacket(subscription, &packet)
	}
}

func rawInputData(handle input.HRAWINPUT, buffer nativeBuffer) (uint32, error) {
	result := input.GetRawInputData(handle, input.RAW_INPUT_DATA_COMMAND_FLAGS(inputDataCommand),
		buffer.data, buffer.size, nativeSize[input.RAWINPUTHEADER]())
	if result == infiniteWait {
		return noValue, errors.Join(domain.ErrEventLoss, syscall.GetLastError())
	}

	return result, nil
}

func readInputData(handle input.HRAWINPUT) ([]byte, error) {
	var size uint32

	queryErr := resultError(rawInputData(handle, nativeBuffer{nil, &size}))
	if queryErr != nil {
		return nil, errors.Join(queryErr)
	}

	headerSize := nativeSize[input.RAWINPUTHEADER]()
	if size < headerSize || size > maxNativeBuffer {
		return nil, domain.ErrEventLoss
	}

	result, err := readInputBuffer(handle, size)

	return result, errors.Join(err)
}

func readInputBuffer(handle input.HRAWINPUT, size uint32) ([]byte, error) {
	data := allocateBuffer[byte](int(size))

	written, err := rawInputData(handle, nativeBuffer{nativePointer(&data[noValue]), &size})
	if err != nil {
		return nil, errors.Join(err)
	}

	headerSize := nativeSize[input.RAWINPUTHEADER]()
	if written < headerSize || uint64(written) > uint64(len(data)) {
		return nil, domain.ErrEventLoss
	}

	return data[:written], nil
}

func captureHandlePacket(subscription *capture, packet *inputPacket) {
	if subscription.closed.Load() {
		return
	}

	if packet.kind != subscription.device.kind {
		captureFail(subscription, domain.ErrEventLoss)

		return
	}

	captureDispatchInput(subscription, packet)
}

func captureDispatchInput(subscription *capture, packet *inputPacket) {
	switch packet.kind {
	case singleValue:
		captureKeyboard(subscription, packet.body, &packet.stamp)
	case noValue:
		captureMouse(subscription, packet.body, &packet.stamp)
	case secondValue:
		captureReports(subscription, packet.body, &packet.stamp)
	default:
		captureFail(subscription, domain.ErrEventLoss)
	}
}

func backendFailAll(owner *backend, err error) {
	for entryIndex := range owner.captures {
		group := owner.captures[entryIndex]
		for subscription := range group {
			captureFail(subscription, err)
		}
	}
}

func captureFail(subscription *capture, err error) {
	if !subscription.closed.Swap(true) {
		subscription.sink.Fail(err)
	}
}

func captureEmit(subscription *capture, event *domain.Event) {
	event.DeviceID = subscription.info.ID
	if !subscription.closed.Load() && !subscription.sink.Publish(event) {
		subscription.closed.Store(true)
	}
}

func captureKeyboard(subscription *capture, data []byte, stamp *domain.Timestamp) {
	if len(data) < sixteenthValue {
		captureFail(subscription, domain.ErrEventLoss)

		return
	}

	key := decodeKeyboard(data)
	if key.makeCode == byteMask {
		captureFail(subscription, domain.ErrEventLoss)

		return
	}

	if keyboardInputIgnored(key) {
		return
	}

	captureEmitKey(subscription, key, stamp)
}

func captureEmitKey(subscription *capture, key keyboardInput, stamp *domain.Timestamp) {
	id := keyboardInputIdentity(key, subscription.backend.tables)
	action, value := updateKeyState(subscription, key, id)
	event := domain.Event{
		DeviceID:  emptyString,
		ControlID: id,
		Action:    action,
		Value:     value,
		Timestamp: *stamp,
	}
	captureEmit(subscription, &event)
}

func captureMouse(subscription *capture, data []byte, stamp *domain.Timestamp) {
	if len(data) < mousePacketBytes {
		captureFail(subscription, domain.ErrEventLoss)

		return
	}

	mouse := mouseInput{
		flags:   binary.LittleEndian.Uint16(data),
		buttons: binary.LittleEndian.Uint16(data[fourthValue:]),
		wheel:   signedHalf(binary.LittleEndian.Uint16(data[sixthValue:])),
		x:       signedWord(binary.LittleEndian.Uint32(data[twelfthValue:])),
		y:       signedWord(binary.LittleEndian.Uint32(data[sixteenthValue:])), stamp: *stamp,
	}
	captureMouseButtons(subscription, &mouse)
	captureMouseAxes(subscription, &mouse)
	captureMouseWheels(subscription, &mouse)
}

func captureMouseButtons(subscription *capture, mouse *mouseInput) {
	for index := range uint16(fifthValue) {
		id := domain.ControlID(fmt.Sprintf(buttonIDFormat, index+singleValue))
		flags := mouse.buttons >> (index * secondValue)
		emitMouseButton(subscription, mouse, &mouseButtonArguments{id: id, flags: flags})
	}
}

func captureMouseAxes(subscription *capture, mouse *mouseInput) {
	emit := captureRelativeAxis
	prefix := relativeAxisPrefix

	if mouse.flags&singleValue != noValue {
		emit = emitAbsoluteCoordinate
		prefix = absoluteAxisPrefix
	}

	emit(
		subscription,
		domain.ControlID(prefix+axisX),
		&captureRelativeAxisArguments{value: mouse.x, stamp: &mouse.stamp},
	)
	emit(
		subscription,
		domain.ControlID(prefix+axisY),
		&captureRelativeAxisArguments{value: mouse.y, stamp: &mouse.stamp},
	)
}

func captureAbsoluteAxis(subscription *capture,
	id domain.ControlID, args *captureAbsoluteAxisArguments,
) {
	value, stamp := args.value, args.stamp
	if previous, exists := subscription.values[id]; exists && previous == value {
		return
	}

	subscription.values[id] = value
	captureEmit(subscription, &domain.Event{
		ControlID: id,
		Action:    domain.ActionChange,
		Value:     float64(value),
		Timestamp: *stamp, DeviceID: emptyString,
	},
	)
}

func captureRelativeAxis(subscription *capture,
	id domain.ControlID, args *captureRelativeAxisArguments,
) {
	value, stamp := args.value, args.stamp
	if value != noValue {
		captureEmit(subscription, &domain.Event{
			ControlID: id,
			Action:    domain.ActionChange,
			Value:     float64(value),
			Timestamp: *stamp, DeviceID: emptyString,
		},
		)
	}
}

func captureMouseWheels(subscription *capture, mouse *mouseInput) {
	if mouse.buttons&mouseVerticalWheel != noValue {
		captureEmit(subscription, &domain.Event{
			ControlID: wheelControlID, Action: domain.ActionChange,
			Value: float64(mouse.wheel) / wheelDelta, Timestamp: mouse.stamp, DeviceID: emptyString,
		})
	}

	if mouse.buttons&mouseHorizontalWheel != noValue {
		captureEmit(subscription, &domain.Event{
			ControlID: panControlID, Action: domain.ActionChange,
			Value: float64(mouse.wheel) / wheelDelta, Timestamp: mouse.stamp, DeviceID: emptyString,
		})
	}
}

func captureReports(subscription *capture, body []byte, stamp *domain.Timestamp) {
	if subscription.hid == nil || len(body) < eighthValue {
		captureFail(subscription, domain.ErrEventLoss)

		return
	}

	size, count := binary.LittleEndian.Uint32(body), binary.LittleEndian.Uint32(body[fourthValue:])
	if !validReportBatch(body, size, count) {
		captureFail(subscription, domain.ErrEventLoss)

		return
	}

	captureReportBatch(subscription, body, &reportBatch{size: size, count: count, stamp: *stamp})
}

func captureReportBatch(subscription *capture, body []byte, batch *reportBatch) {
	for index := uint32(noValue); index < batch.count && !subscription.closed.Load(); index++ {
		start := eighthValue + uint64(index)*uint64(batch.size)
		captureReport(subscription, body[start:start+uint64(batch.size)], &batch.stamp)
	}
}

func captureReport(subscription *capture, report []byte, stamp *domain.Timestamp) {
	if len(report) != int(subscription.hid.reportLen) || len(report) == noValue {
		captureFail(
			subscription,
			fmt.Errorf("unexpected HID report length: %w", domain.ErrEventLoss),
		)

		return
	}

	reportID := report[noValue]
	if !subscription.hid.reportIDs[reportID] || subscription.hid.maxData == noValue {
		return
	}

	captureApplyHIDReport(subscription, report, stamp)
}

func captureApplyHIDReport(subscription *capture, report []byte, stamp *domain.Timestamp) {
	data, err := captureReadHIDReport(subscription, report)
	if err != nil {
		captureFail(subscription, err)

		return
	}

	state := hidReportState{
		reportID: report[noValue],
		stamp:    *stamp,
		pressed:  make(map[domain.ControlID]bool),
	}
	captureProcessHIDData(subscription, data, &state)
	captureUpdateButtons(subscription, &state)
}

func captureReadHIDReport(subscription *capture, report []byte) ([]hid.HIDP_DATA, error) {
	data := allocateBuffer[hid.HIDP_DATA](int(subscription.hid.maxData))
	count := subscription.hid.maxData
	preparsedAddress := descriptorPointer(subscription.hid)
	status := hid.HidP_GetData(
		hid.HidP_Input,
		&data[noValue],
		&count,
		preparsedAddress,
		&report[noValue],
		uint32(subscription.hid.reportLen),
	)
	runtime.KeepAlive(subscription.hid.preparsed)
	runtime.KeepAlive(report)

	if status != hid.HIDP_STATUS_SUCCESS || count > subscription.hid.maxData {
		return nil, errors.Join(domain.ErrEventLoss, hidError("HidP_GetData", status))
	}

	return data[:count], nil
}

func captureProcessHIDData(subscription *capture, data []hid.HIDP_DATA, state *hidReportState) {
	for entryIndex := range data {
		item := data[entryIndex]
		ctrl, found := subscription.hid.controls[hidDataIndex(state.reportID, item.DataIndex)]

		if !found || ctrl.control.Support != domain.SupportSupported {
			continue
		}

		captureProcessHIDControl(
			subscription,
			&ctrl,
			&captureProcessHIDControlArguments{word: item.Anonymous.Data[noValue], state: state},
		)
	}
}

func captureProcessHIDControl(
	subscription *capture,
	ctrl *hidControl, args *captureProcessHIDControlArguments,
) {
	word, state := args.word, args.state
	if ctrl.button && word&byteMask != noValue {
		state.pressed[ctrl.control.ID] = true
	}

	if ctrl.button {
		return
	}

	value, valid := hidControlReportValue(ctrl, word)
	if valid {
		captureEmitHIDValue(
			subscription,
			&ctrl.control,
			&captureEmitHIDValueArguments{value: value, stamp: &state.stamp},
		)
	}
}

func hidControlReportValue(ctrl *hidControl, word uint32) (int64, bool) {
	value := logicalValue(word, &ctrl.native)
	bounds := logicalBounds(ctrl.native.Bounds.LogicalMin, ctrl.native.Bounds.LogicalMax)

	if ctrl.hat {
		direction, ok := domain.Hat(value, bounds, ctrl.native.HasNull)

		return int64(direction), ok
	}

	return value, !ctrl.native.HasNull || (value >= bounds.Min && value <= bounds.Max)
}

func captureEmitHIDValue(subscription *capture,
	control *domain.Control, args *captureEmitHIDValueArguments,
) {
	value, stamp := args.value, args.stamp
	previous, exists := subscription.values[control.ID]

	if control.Mode != domain.AxisRelative && exists && previous == value {
		return
	}

	subscription.values[control.ID] = value
	captureEmit(subscription, &domain.Event{
		ControlID: control.ID, Action: hidValueAction(control.Kind, value),
		Value: float64(value), Timestamp: *stamp, DeviceID: emptyString,
	})
}

func captureUpdateButtons(subscription *capture, state *hidReportState) {
	previous := subscription.buttons[state.reportID]
	event := makeButtonEvent(state)
	captureEmitButtonChanges(
		subscription,
		state.pressed,
		&buttonChangesArguments{previous: previous, event: &event},
	)

	event.Action, event.Value = domain.ActionRelease, noValue
	captureEmitButtonChanges(
		subscription,
		previous,
		&buttonChangesArguments{previous: state.pressed, event: &event},
	)

	subscription.buttons[state.reportID] = state.pressed
}

func captureEmitButtonChanges(subscription *capture, current map[domain.ControlID]bool,
	args *buttonChangesArguments,
) {
	previous, event := args.previous, args.event

	for id := range current {
		if !previous[id] {
			event.ControlID = id
			captureEmit(subscription, event)
		}
	}
}

// Factory returns the native backend constructor.
func Factory() ports.Factory {
	environment := new(nativeState)

	tables, tableErr := makeKeyTables(
		&[thirdValue]string{scanUsagesData, consumerScansData, consumerVirtualKeysData},
	)

	environment.tables = tables

	return func(ctx context.Context, retrier ports.Retrier) (ports.Backend, error) {
		if tableErr != nil {
			return nil, errors.Join(tableErr)
		}

		owner, err := newBackend(ctx, environment, retrier)
		if err != nil {
			return nil, errors.Join(err)
		}

		return makeBackendView(ctx, owner), nil
	}
}

func resultError[Value any](_ Value, err error) error { return errors.Join(err) }

func classifyValue(ctrl *domain.Control, value *hid.HIDP_VALUE_CAPS, bounds domain.Range) {
	if unsupportedValue(value) {
		ctrl.Support = domain.SupportUnsupported
	}

	if value.IsAbsolute != noValue {
		ctrl.Mode, ctrl.Range = domain.AxisAbsolute, &bounds

		return
	}

	ctrl.Mode, ctrl.Unit = domain.AxisRelative, domain.UnitCounts
}

func unsupportedValue(value *hid.HIDP_VALUE_CAPS) bool {
	return value.BitSize == noValue || value.BitSize > thirtySecondValue ||
		(value.IsRange == noValue && value.ReportCount > singleValue)
}

func isConventionalHat(value *hid.HIDP_VALUE_CAPS, usage uint32, bounds domain.Range) bool {
	return value.UsagePage == singleValue && usage == hatUsage && value.IsAbsolute != noValue &&
		conventionalHat(value, bounds.Min, bounds.Max)
}

func classifyHat(ctrl *domain.Control, value *hid.HIDP_VALUE_CAPS, usage uint32) {
	if isConventionalHat(value, usage, logicalBounds(value.LogicalMin, value.LogicalMax)) {
		ctrl.Kind, ctrl.Unit, ctrl.Range = domain.ControlHat, domain.UnitDirection,
			&domain.Range{Min: int64(domain.HatNeutral), Max: int64(domain.HatNorthWest)}

		return
	}

	if value.IsAbsolute != noValue && ctrl.Range.Min == noValue && ctrl.Range.Max == singleValue {
		ctrl.Kind, ctrl.Unit = domain.ControlSwitch, domain.UnitBoolean
	}
}

func makeBackendView(
	ctx context.Context,
	owner *backend,
) *backendview.Operations[domain.DeviceInfo, domain.DeviceID, ports.EventSink, ports.Capture] {
	view := new(
		backendview.Operations[domain.DeviceInfo, domain.DeviceID, ports.EventSink, ports.Capture],
	)

	view.Operations.Discover = func(ctx context.Context) ([]domain.DeviceInfo, error) {
		infos, err := backendDiscover(ctx, owner)

		return infos, errors.Join(err)
	}
	view.Operations.Close = func() error { return errors.Join(backendCloseContext(ctx, owner)) }
	configureBackendOpen(view, owner)

	return view
}

func nativePointer[Value any](value *Value) unsafe.Pointer {
	return reflect.ValueOf(value).UnsafePointer()
}

// NativeInfo copies platform metadata from the bound capture.
func (view metadataView[Value]) NativeInfo() Value { return view() }

func makeCaptureView(
	ctx context.Context,
	subscription *capture,
) *captureview.Operations[domain.DeviceInfo, domain.Capabilities] {
	view := new(captureview.Operations[domain.DeviceInfo, domain.Capabilities])

	view.Operations.Info = func() domain.DeviceInfo { return captureInfo(subscription) }
	view.Operations.Capabilities = func() domain.Capabilities { return captureCapabilities(subscription) }
	view.Operations.Extension = func(target any) bool { return captureExtension(subscription, target) }
	view.Operations.Close = func() error { return errors.Join(captureClose(ctx, subscription)) }

	return view
}

func makeCaptureHost(owner *backend, subscription *capture) *captureHost {
	host := new(captureHost)

	host.tables = owner.native.tables
	host.call = func(ctx context.Context, operation func() error) error {
		return errors.Join(backendCall(ctx, owner, operation))
	}
	host.retry = func(ctx context.Context, operation func(context.Context) error) error {
		return errors.Join(backendRetry(ctx, owner, operation))
	}
	configureCaptureRegistration(host, owner, subscription)

	return host
}

func nativeSize[Value any]() uint32 {
	var value Value

	return uint32(unsafe.Sizeof(value) & math.MaxUint32)
}

func makeNativeInfo(device *nativeDevice) ext.Info {
	return ext.Info{
		RawInputHandle: uintptr(device.handle), DeviceType: device.kind,
		UsagePage: device.tlc.page, Usage: device.tlc.usage, Version: device.version, Controls: nil,
	}
}

func readCapabilities[Value any](
	count uint16,
	read func(*Value, *uint16) foundation.NTSTATUS,
) ([]Value, error) {
	if count == noValue {
		return make([]Value, noValue), nil
	}

	caps := allocateBuffer[Value](int(count))
	status := read(&caps[noValue], &count)

	result, err := checkedCapabilities(caps, count, status)

	return result, errors.Join(err)
}

func checkedCapabilities[Value any](
	caps []Value,
	count uint16,
	status foundation.NTSTATUS,
) ([]Value, error) {
	if status != hid.HIDP_STATUS_SUCCESS {
		return nil, errors.Join(hidError("read HID capabilities", status))
	}

	if int(count) > len(caps) {
		return nil, domain.ErrEventLoss
	}

	return caps[:count], nil
}

func addCapabilityRange(
	builder *hidBuilder,
	span *capabilityRange,
	makeControl func(uint32, uint32) hidControl,
) {
	if span.lastUsage < span.firstUsage {
		return
	}

	for offset := uint32(noValue); offset <= span.lastUsage-span.firstUsage; offset++ {
		control := makeControl(span.firstUsage+offset, span.firstIndex+offset)
		hidBuilderAdd(builder, &control)
	}
}

func awaitBackendStartup(owner *backend) error {
	err := <-owner.ready
	if err != nil {
		<-owner.done
	}

	return errors.Join(err)
}

func initializeCallback(environment *nativeState) {
	environment.callbackOnce.Do(func() {
		environment.callbackAddr = syscall.NewCallback(func(hwnd foundation.HWND, message uint32,
			wParam foundation.WPARAM, lParam foundation.LPARAM,
		) foundation.LRESULT {
			return nativeStateWindowProc(environment,
				&windowMessage{hwnd: hwnd, message: message, wParam: wParam, lParam: lParam},
			)
		})
	})
}

func backendCreateMessageWindow(owner *backend, instance foundation.HINSTANCE) error {
	window, err := wm.CreateWindowEx(
		noValue, &owner.className, nil, noValue, noValue, noValue,
		noValue, noValue, wm.HWND_MESSAGE, noValue, instance, nil,
	)
	if err != nil {
		return errors.Join(normalizeError(err))
	}

	backendPublishWindow(owner, window)

	return nil
}

func windowClassName(environment *nativeState) string {
	return fmt.Sprintf(
		"goinput.rawinput.%x.%d",
		environment.callbackAddr,
		environment.classNumber.Add(singleValue),
	)
}

func makeWindowClass(owner *backend, instance foundation.HINSTANCE) wm.WNDCLASSW {
	var class wm.WNDCLASSW

	class.LpfnWndProc, class.HInstance = wm.WNDPROC(owner.native.callbackAddr), instance
	class.LpszClassName = native.UTF16Ptr(owner.className)

	return class
}

func submitCommand(owner *backend, cmd *command) error {
	enqueueErr := backendEnqueue(owner, cmd)
	if enqueueErr != nil {
		return errors.Join(enqueueErr)
	}

	signalErr := backendSignalCommand(owner, cmd)
	if signalErr != nil {
		return errors.Join(signalErr)
	}

	return nil
}

func backendRawDevices(ctx context.Context, owner *backend) ([]input.RAWINPUTDEVICELIST, error) {
	var list []input.RAWINPUTDEVICELIST

	operation := func(ctx context.Context) error {
		var err error

		list, err = enumerateRawDevices(ctx)

		return errors.Join(err)
	}

	retryErr := backendRetry(ctx, owner, operation)
	if retryErr != nil {
		return nil, errors.Join(normalizeError(retryErr))
	}

	return list, nil
}

func prepareCapture(ctx context.Context, owner *backend, subscription *capture) error {
	metadataErr := captureLoadCapabilities(ctx, subscription)
	if metadataErr != nil {
		return errors.Join(normalizeError(metadataErr))
	}

	err := backendCall(ctx, owner, subscription.backend.register)
	if err != nil {
		return errors.Join(normalizeError(err))
	}

	return nil
}

func loadDeviceMetadata(device *nativeDevice) error {
	err := nativeDeviceLoadDeviceInfo(device)
	if err != nil {
		return errors.Join(err)
	}

	return errors.Join(nativeDeviceLoadDevicePath(device))
}

func applyMouseDeviceInfo(device *nativeDevice, words [sixthValue]uint32) {
	device.tlc = topLevel{singleValue, secondValue}
	device.buttons, device.hwheel = words[singleValue], words[thirdValue] != noValue
	device.info.Classes = []domain.DeviceClass{domain.ClassMouse}
}

func identityProceduresAvailable() bool {
	procedures := []*native.Proc{
		hid.Procs.HidD_GetAttributes, hid.Procs.HidD_GetProductString,
		hid.Procs.HidD_GetManufacturerString, hid.Procs.HidD_GetSerialNumberString,
	}

	return findProcedures(procedures) == nil
}

func appendKeyboardControls(subscription *capture, controls map[domain.ControlID]domain.Control) {
	for entryIndex := range controls {
		control := controls[entryIndex]

		subscription.caps.Controls = append(subscription.caps.Controls, control)
	}
}

func sortKeyboardControls(subscription *capture) {
	slices.SortFunc(subscription.caps.Controls,

		func(left, owner domain.Control) int { return cmp.Compare(left.ID, owner.ID) })
	slices.SortFunc(subscription.native.Controls,

		func(left, owner ext.NativeControl) int {
			if left.ID == owner.ID {
				return cmp.Compare(left.ScanCode, owner.ScanCode)
			}

			return cmp.Compare(left.ID, owner.ID)
		})
}

func makeKeyDescriptor(scan uint16, usage domain.Usage) domain.Control {
	return domain.Control{
		Mode:    domain.AxisUnknown,
		ID:      keyID(usage, scan),
		Name:    keyName(usage),
		Kind:    domain.ControlKey,
		Usage:   usage,
		Mapping: domain.MappingInferred,
		Range:   &domain.Range{Min: noValue, Max: singleValue},
		Unit:    domain.UnitBoolean,
		Support: domain.SupportUnknown,
	}
}

func keyName(usage domain.Usage) string {
	return fmt.Sprintf("Key %04x:%04x", uint16(usage>>sixteenthValue), uint16(usage&wordMask))
}

func makeKeyNative(id domain.ControlID, scan uint16) ext.NativeControl {
	var control ext.NativeControl

	control.ID, control.ScanCode = string(id), scan

	if scan&virtualScanPrefix == virtualScanPrefix {
		control.ScanCode, control.VirtualKey = noValue, scan&byteMask
	}

	return control
}

func makeMouseAxis(axis string, usage domain.Usage) domain.Control {
	return domain.Control{
		ID: domain.ControlID(
			"rel:" + axis,
		),
		Name:    "Relative " + strings.ToUpper(axis),
		Kind:    domain.ControlAxis,
		Usage:   usage,
		Mapping: domain.MappingInferred,
		Mode:    domain.AxisRelative,
		Range:   nil,
		Unit:    domain.UnitCounts,
		Support: domain.SupportUnknown,
	}
}

func makeAbsoluteMouseAxis(axis string, usage domain.Usage) domain.Control {
	control := makeMouseAxis(axis, usage)

	control.ID, control.Name = domain.ControlID("abs:"+axis), "Absolute "+strings.ToUpper(axis)
	control.Mode, control.Unit = domain.AxisAbsolute, domain.UnitLogical
	control.Range = &domain.Range{Min: noValue, Max: wordMask}

	return control
}

func makeMouseWheel(id domain.ControlID, usage domain.Usage, name string) domain.Control {
	return domain.Control{
		ID:      id,
		Name:    name,
		Kind:    domain.ControlAxis,
		Usage:   usage,
		Mapping: domain.MappingInferred,
		Mode:    domain.AxisRelative,
		Range:   nil,
		Unit:    domain.UnitDetents,
		Support: domain.SupportUnknown,
	}
}

func makeHIDButtonDescriptor(
	button *hid.HIDP_BUTTON_CAPS,
	usage uint32,
	id domain.ControlID,
) domain.Control {
	return domain.Control{
		Mode:    domain.AxisUnknown,
		ID:      id,
		Name:    fmt.Sprintf(hidNameFormat, button.UsagePage, usage),
		Kind:    buttonKind(button.UsagePage),
		Usage:   domain.HID(button.UsagePage, uint16(usage&wordMask)),
		Mapping: domain.MappingReported,
		Range:   &domain.Range{Min: noValue, Max: singleValue},
		Unit:    domain.UnitBoolean,
		Support: buttonSupport(button.IsAbsolute),
	}
}

func makeHIDButtonNative(
	button *hid.HIDP_BUTTON_CAPS,
	dataIndex uint32,
	id domain.ControlID,
) ext.NativeControl {
	var control ext.NativeControl

	control.ID, control.ReportID = string(id), button.ReportID
	control.DataIndex, control.LinkCollection = uint16(dataIndex&wordMask), button.LinkCollection
	control.ReportCount, control.BitSize = button.ReportCount, singleValue
	control.Bounds.LogicalMax = singleValue
	control.Absolute = button.IsAbsolute != noValue

	return control
}

func makeHIDValueNative(
	value *hid.HIDP_VALUE_CAPS,
	dataIndex uint32,
	id domain.ControlID,
) ext.NativeControl {
	return ext.NativeControl{
		ID:             string(id),
		ReportID:       value.ReportID,
		DataIndex:      uint16(dataIndex & wordMask),
		LinkCollection: value.LinkCollection,
		BitSize:        value.BitSize,
		ReportCount:    value.ReportCount,
		Bounds: ext.DescriptorBounds{
			LogicalMin: value.LogicalMin, LogicalMax: value.LogicalMax,
			PhysicalMin: value.PhysicalMin, PhysicalMax: value.PhysicalMax,
		},
		Units:         value.Units,
		UnitsExponent: value.UnitsExp,
		HasNull:       value.HasNull != noValue,
		Absolute:      value.IsAbsolute != noValue, VirtualKey: noValue, ScanCode: noValue,
	}
}

func addCapabilities[Value any](values []Value, add func(*Value) error) error {
	for index := range values {
		err := add(&values[index])
		if err != nil {
			return errors.Join(err)
		}
	}

	return nil
}

func finishBackendStartup(ctx context.Context, owner *backend) (*backend, error) {
	err := awaitBackendStartup(owner)
	if err != nil {
		return nil, errors.Join(err)
	}

	result, startupErr := backendCheckStartup(ctx, owner)
	if startupErr != nil {
		return nil, errors.Join(startupErr)
	}

	return result, nil
}

func completeCaptureOpen(
	ctx context.Context,
	owner *backend,
	subscription *capture,
) (*capture, error) {
	err := prepareCapture(ctx, owner, subscription)
	if err != nil {
		return nil, errors.Join(err)
	}

	result, openErr := captureCheckOpened(ctx, subscription)
	if openErr != nil {
		return nil, errors.Join(openErr)
	}

	return result, nil
}

func readDeviceInventory[Value any](
	count, size uint32,
	read func(uint32, uint32) ([]Value, error),
) ([]Value, error) {
	if count > maxDevices {
		return nil, domain.ErrInvalidOptions
	}

	if count == noValue {
		return make([]Value, noValue), nil
	}

	values, err := read(count, size)

	return values, errors.Join(err)
}

func describeInventoryEntry(ctx context.Context, owner *backend, args *inventoryEntryArguments) {
	device, err := backendDescribeRawDevice(ctx, owner, args.item)
	deviceSnapshotAdd(args.snapshot, &device, err)
}

func applyKeyboardDeviceInfo(device *nativeDevice) {
	device.tlc = topLevel{singleValue, sixthValue}
	device.info.Classes = []domain.DeviceClass{domain.ClassKeyboard}
}

func openIdentityHandle(path string) (*identityHandle, error) {
	handle, err := filesystem.CreateFile(path, noValue, filesystem.FILE_SHARE_MODE(thirdValue), nil,
		filesystem.FILE_CREATION_DISPOSITION(thirdValue), noValue, noValue)
	if err != nil {
		return nil, fmt.Errorf("open identity handle: %w", err)
	}

	return &identityHandle{value: handle}, nil
}

func makeHIDValueDescriptor(
	value *hid.HIDP_VALUE_CAPS,
	usage uint32,
	id domain.ControlID,
) domain.Control {
	return domain.Control{
		Mode: domain.AxisUnknown,
		ID:   id,
		Name: fmt.Sprintf(hidNameFormat, value.UsagePage, usage),
		Kind: domain.ControlAxis,
		Usage: domain.HID(
			value.UsagePage,
			uint16(usage&wordMask),
		),
		Mapping: domain.MappingReported,
		Unit:    domain.UnitLogical,
		Support: domain.SupportSupported, Range: nil,
	}
}

func decodeKeyboard(data []byte) keyboardInput {
	return keyboardInput{
		makeCode:   binary.LittleEndian.Uint16(data),
		flags:      binary.LittleEndian.Uint16(data[secondValue:]),
		virtualKey: binary.LittleEndian.Uint16(data[sixthValue:]),
	}
}

func updateKeyState(
	subscription *capture,
	key keyboardInput,
	id domain.ControlID,
) (action domain.EventAction, value float64) {
	if key.flags&singleValue != noValue {
		delete(subscription.held, id)

		return domain.ActionRelease, noValue
	}

	action = domain.ActionPress

	if subscription.held[id] {
		action = domain.ActionRepeat
	}

	subscription.held[id] = true

	return action, singleValue
}

func emitMouseButton(subscription *capture, mouse *mouseInput, args *mouseButtonArguments) {
	event := domain.Event{
		DeviceID:  emptyString,
		ControlID: args.id,
		Timestamp: mouse.stamp,
		Action:    domain.ActionPress,
		Value:     singleValue,
	}
	if args.flags&singleValue != noValue {
		captureEmit(subscription, &event)
	}

	event.Action, event.Value = domain.ActionRelease, noValue
	if args.flags&secondValue != noValue {
		captureEmit(subscription, &event)
	}
}

func configureBackendOpen(
	view *backendview.Operations[domain.DeviceInfo, domain.DeviceID, ports.EventSink, ports.Capture],
	owner *backend,
) {
	view.Operations.Open = func(ctx context.Context, id domain.DeviceID, sink ports.EventSink) (ports.Capture, error) {
		subscription, err := backendOpen(ctx, owner, &backendOpenArguments{id: id, sink: sink})
		if err != nil {
			return nil, errors.Join(err)
		}

		return makeCaptureView(ctx, subscription), nil
	}
}

func configureCaptureRegistration(host *captureHost, owner *backend, subscription *capture) {
	host.register = func() error { return errors.Join(captureRegister(owner, subscription)) }
	host.unregister = func() error { return errors.Join(captureUnregister(owner, subscription)) }
}

func hidDataIndex(report byte, index uint16) hidIndex {
	return uint32(report)<<sixteenthValue | uint32(index)
}

func finishDeviceSnapshot(ctx context.Context, snapshot *deviceSnapshot) ([]nativeDevice, error) {
	slices.SortFunc(
		snapshot.devices,
		func(left, owner nativeDevice) int { return cmp.Compare(left.info.ID, owner.info.ID) },
	)

	return snapshot.devices, errors.Join(snapshot.diagnostics, context.Cause(ctx))
}

func enrichDeviceIdentity(ctx context.Context, device *nativeDevice) error {
	cause := context.Cause(ctx)
	if cause != nil {
		return errors.Join(cause)
	}

	return errors.Join(enrichIdentity(device))
}

func addPhysicalKeyControls(subscription *capture, controls map[domain.ControlID]domain.Control) {
	captureAddScanControls(
		subscription,
		controls,
		&captureAddScanControlsArguments{
			scans: subscription.backend.tables.keyboard,
			page:  seventhValue,
		},
	)
	captureAddScanControls(
		subscription,
		controls,
		&captureAddScanControlsArguments{
			scans: subscription.backend.tables.consumer,
			page:  twelfthValue,
		},
	)
}

func descriptorPointer(desc *descriptor) hid.PHIDP_PREPARSED_DATA {
	return hid.PHIDP_PREPARSED_DATA(uintptr(nativePointer(&desc.preparsed[noValue])))
}

func readNativeCapabilities[Value any](count uint16, preparsed hid.PHIDP_PREPARSED_DATA,
	read func(hid.HIDP_REPORT_TYPE, *Value, *uint16, hid.PHIDP_PREPARSED_DATA) foundation.NTSTATUS,
) ([]Value, error) {
	values, err := readCapabilities(count, func(first *Value, length *uint16) foundation.NTSTATUS {
		return read(hid.HidP_Input, first, length, preparsed)
	})

	return values, errors.Join(err)
}

func emitAbsoluteCoordinate(
	subscription *capture,
	id domain.ControlID,
	args *captureRelativeAxisArguments,
) {
	captureAbsoluteAxis(
		subscription,
		id,
		&captureAbsoluteAxisArguments{value: args.value, stamp: args.stamp},
	)
}

func addNativeCapability(builder *hidBuilder, args *nativeCapability) error {
	if args.alias != noValue {
		return nil
	}

	span, err := capRange(args.words, uint8(args.rangeKind))
	if err != nil {
		return errors.Join(err)
	}

	addCapabilityRange(builder, &span, args.makeControl)

	return nil
}

func makeNativeCapability(
	words [eighthValue]uint16,
	rangeKind, alias foundation.BOOLEAN,
) *nativeCapability {
	return &nativeCapability{words: words, rangeKind: rangeKind, alias: alias, makeControl: nil}
}

func bindCapability[Value any](
	value *Value,
	makeControl func(*Value, uint32, uint32) hidControl,
) func(uint32, uint32) hidControl {
	return func(usage, index uint32) hidControl { return makeControl(value, usage, index) }
}

func makeButtonEvent(state *hidReportState) domain.Event {
	return domain.Event{
		DeviceID:  emptyString,
		ControlID: emptyString,
		Action:    domain.ActionPress,
		Value:     singleValue,
		Timestamp: state.stamp,
	}
}

func makeCapabilityAdder[Value any](
	builder *hidBuilder,
	args *capabilityAdapter[Value],
) func(*Value) error {
	return func(value *Value) error {
		spec := args.project(value)

		spec.makeControl = bindCapability(value, args.makeControl)

		return errors.Join(addNativeCapability(builder, spec))
	}
}

func makeCapabilityAdapter[Value any](
	read func(hid.HIDP_REPORT_TYPE, *Value, *uint16, hid.PHIDP_PREPARSED_DATA) foundation.NTSTATUS,
	project func(*Value) *nativeCapability,
	makeControl func(*Value, uint32, uint32) hidControl,
) *capabilityAdapter[Value] {
	return &capabilityAdapter[Value]{read: read, project: project, makeControl: makeControl}
}

func loadNativeCapabilitySet[Value any](
	builder *hidBuilder,
	count uint16,
	args *capabilityAdapter[Value],
) error {
	values, err := readNativeCapabilities(count, hidBuilderPreparsedPointer(builder), args.read)
	if err != nil {
		return errors.Join(err)
	}

	return errors.Join(addCapabilities(values, makeCapabilityAdder(builder, args)))
}
