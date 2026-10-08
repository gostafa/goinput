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
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"
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
)

func newBackend(ctx context.Context, retrier ports.Retrier) (ports.Backend, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	b := makeBackend(retrier)
	go b.run()
	if err := <-b.ready; err != nil {
		<-b.done
		return nil, err
	}
	return b.checkStartup(ctx)
}

func makeBackend(retrier ports.Retrier) *backend {
	return &backend{
		commands: make(chan *command, commandCapacity),
		ready:    make(chan error, singleValue),
		done:     make(chan struct{}),
		retrier:  retrier,
		captures: make(
			map[foundation.HANDLE]map[*capture]struct{},
		),
		registrations: make(map[topLevel]int),
	}
}

func (b *backend) checkStartup(ctx context.Context) (ports.Backend, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, errors.Join(err, b.Close())
	}
	return b, nil
}

func (b *backend) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var err error
	defer func() { b.finish(err) }()
	err = b.initialize()
	b.ready <- err
	if err == nil {
		err = b.messageLoop()
	}
}

func findProcedures(procedures []*native.Proc) error {
	for _, procedure := range procedures {
		if err := procedure.Find(); err != nil {
			return errors.Join(domain.ErrUnsupported, err)
		}
	}
	return nil
}

func (b *backend) initialize() error {
	procedures := []*native.Proc{
		wm.Procs.MsgWaitForMultipleObjectsEx, wm.Procs.CreateWindowEx,
		input.Procs.GetRawInputData, input.Procs.RegisterRawInputDevices,
	}
	if err := findProcedures(procedures); err != nil {
		return err
	}
	callbackOnce.Do(func() {
		callbackAddr = syscall.NewCallback(func(hwnd foundation.HWND, message uint32,
			wParam foundation.WPARAM, lParam foundation.LPARAM,
		) foundation.LRESULT {
			return windowProc(
				windowMessage{hwnd: hwnd, message: message, wParam: wParam, lParam: lParam},
			)
		})
	})
	event, err := threading.CreateEvent(nil, false, false, nil)
	if err != nil {
		return normalizeError(err)
	}
	b.wakeEvent = event
	return b.createWindow()
}

func (b *backend) createWindow() error {
	instance, err := libraryloader.GetModuleHandle(nil)
	if err != nil {
		return normalizeError(err)
	}
	if err := b.registerWindowClass(foundation.HINSTANCE(instance)); err != nil {
		return err
	}
	window, err := wm.CreateWindowEx(
		noValue, &b.className, nil, noValue, noValue, noValue,
		noValue, noValue, wm.HWND_MESSAGE, noValue, foundation.HINSTANCE(instance), nil,
	)
	if err != nil {
		return normalizeError(err)
	}
	b.publishWindow(window)
	return nil
}

func (b *backend) publishWindow(window foundation.HWND) {
	b.mu.Lock()
	b.hwnd = window
	b.mu.Unlock()
	windows.Store(window, b)
}

func (b *backend) registerWindowClass(instance foundation.HINSTANCE) error {
	b.className = fmt.Sprintf("goinput.rawinput.%d", classNumber.Add(singleValue))
	class := wm.WNDCLASSW{
		LpfnWndProc: wm.WNDPROC(callbackAddr), HInstance: instance,
		LpszClassName: native.UTF16Ptr(b.className),
	}
	atom, err := wm.RegisterClass(&class)
	if atom != noValue {
		return nil
	}
	// The atom is the native success sentinel; successful calls may leave stale errors.
	b.className = emptyString
	return normalizeError(nonzeroError(err))
}

func (b *backend) messageLoop() error {
	handles := []foundation.HANDLE{b.wakeEvent}
	for !b.stopping {
		if err := b.waitForMessages(handles); err != nil {
			return err
		}
		if b.dispatchMessages() {
			return nil
		}
	}
	return nil
}

func (b *backend) waitForMessages(handles []foundation.HANDLE) error {
	ret, _, errno := syscall.SyscallN(
		wm.Procs.MsgWaitForMultipleObjectsEx.Addr(), singleValue,
		uintptr(unsafe.Pointer(&handles[noValue])), infiniteWait, queueAllInput, fourthValue,
	)
	if uint32(ret) == infiniteWait {
		return normalizeError(native.LastError(errno))
	}
	if ret == noValue {
		b.drainCommands()
	}
	return nil
}

func (b *backend) dispatchMessages() bool {
	var msg wm.MSG
	for !b.stopping && wm.PeekMessage(&msg, noValue, noValue, noValue, wm.PEEK_MESSAGE_REMOVE_TYPE(singleValue)) {
		if msg.Message == messageQuit {
			return true
		}
		wm.DispatchMessage(&msg)
		b.drainCommands() // High-frequency input must not starve Close/Open.
	}
	return false
}

func windowProc(event windowMessage) foundation.LRESULT {
	value, found := windows.Load(event.hwnd)
	if !found {
		return event.defaultResult()
	}
	owner, valid := value.(*backend)
	if !valid {
		return event.defaultResult()
	}
	return owner.handleWindowMessage(event)
}

func (event windowMessage) defaultResult() foundation.LRESULT {
	return wm.DefWindowProc(event.hwnd, event.message, event.wParam, event.lParam)
}

func (b *backend) handleWindowMessage(event windowMessage) foundation.LRESULT {
	switch event.message {
	case messageWake:
		b.drainCommands()
	case byteMask:
		return b.handleInputMessage(event)
	case messageDeviceChange:
		b.handleDeviceChange(event)
	default:
		return event.defaultResult()
	}
	return noValue
}

func (b *backend) handleInputMessage(event windowMessage) foundation.LRESULT {
	b.readInput(input.HRAWINPUT(event.lParam))
	// Foreground WM_INPUT requires the default procedure for native cleanup.
	if uint32(event.wParam)&byteMask == noValue {
		return event.defaultResult()
	}
	return noValue
}

func (b *backend) handleDeviceChange(event windowMessage) {
	if uint32(event.wParam) == secondValue {
		b.disconnect(foundation.HANDLE(event.lParam))
	}
}

func (b *backend) drainCommands() {
	for {
		select {
		case cmd := <-b.commands:
			cmd.reply <- b.executeCommand(cmd)
		default:
			return
		}
	}
}

func (b *backend) executeCommand(cmd *command) error {
	if !cmd.state.CompareAndSwap(noValue, singleValue) {
		return context.Canceled
	}
	if b.stopping {
		return domain.ErrClosed
	}
	if err := context.Cause(cmd.ctx); err != nil {
		return err
	}
	return cmd.fn()
}

func (b *backend) call(ctx context.Context, fn func() error) error {
	if err := b.checkCall(ctx); err != nil {
		return err
	}
	cmd := &command{ctx: ctx, fn: fn, reply: make(chan error, singleValue)}
	if err := b.enqueue(cmd); err != nil {
		return err
	}
	if err := b.signalCommand(cmd); err != nil {
		return err
	}
	return b.awaitCommand(cmd)
}

func (b *backend) checkCall(ctx context.Context) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if b.isClosed() {
		return domain.ErrClosed
	}
	return nil
}

func (b *backend) isClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

func (b *backend) enqueue(cmd *command) error {
	select {
	case b.commands <- cmd:
		return nil
	case <-cmd.ctx.Done():
		return context.Cause(cmd.ctx)
	case <-b.done:
		return domain.ErrClosed
	}
}

func (b *backend) signalWake() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return domain.ErrClosed
	}
	// Holding mu keeps teardown from closing or reusing the HANDLE during SetEvent.
	return threading.SetEvent(b.wakeEvent)
}

func (b *backend) signalCommand(cmd *command) error {
	err := b.signalWake()
	if err == nil {
		return nil
	}
	// Commands already running must finish so their acquired resources can be released.
	if !cmd.state.CompareAndSwap(noValue, secondValue) {
		return nil
	}
	select {
	case <-b.done:
		return domain.ErrClosed
	default:
		return normalizeError(err)
	}
}

func (b *backend) awaitCommand(cmd *command) error {
	wait := commandWait{command: cmd, contextDone: cmd.ctx.Done()}
	for !wait.complete {
		b.waitCommandStep(&wait)
	}
	return wait.err
}

func (b *backend) waitCommandStep(wait *commandWait) {
	select {
	case wait.err = <-wait.command.reply:
		wait.complete = true
	case <-b.done:
		wait.err, wait.complete = domain.ErrClosed, true
	case <-wait.contextDone:
		wait.err = cancelPendingCommand(wait.command)
		wait.complete = wait.err != nil
		wait.contextDone = nil // Native work has started; wait for its result before releasing resources.
	}
}

func (b *backend) finish(cause error) {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	b.failCaptures(cause)
	cleanupErr := b.cleanupWindow()
	b.mu.Lock()
	cleanupErr = errors.Join(cleanupErr, b.closeWakeEvent())
	b.closeErr = normalizeError(errors.Join(cause, cleanupErr))
	b.mu.Unlock()
	close(b.done)
}

func (b *backend) failCaptures(cause error) {
	if cause == nil {
		cause = domain.ErrClosed
	}
	b.failAll(cause)
}

func (b *backend) cleanupWindow() error {
	var err error
	for usage := range b.registrations {
		err = errors.Join(err, b.removeRegistration(usage))
	}
	return errors.Join(err, b.destroyWindow(), b.unregisterWindowClass())
}

func (b *backend) destroyWindow() error {
	if b.hwnd == noValue {
		return nil
	}
	windows.Delete(b.hwnd)
	return wm.DestroyWindow(b.hwnd)
}

func (b *backend) unregisterWindowClass() error {
	if b.className == emptyString {
		return nil
	}
	instance, err := libraryloader.GetModuleHandle(nil)
	return errors.Join(err, wm.UnregisterClass(b.className, foundation.HINSTANCE(instance)))
}

func (b *backend) closeWakeEvent() error {
	if b.wakeEvent == noValue {
		return nil
	}
	err := foundation.CloseHandle(b.wakeEvent)
	b.wakeEvent = noValue
	return err
}

func (b *backend) Close() error {
	err := b.call(context.Background(), func() error {
		b.mu.Lock()
		b.closed = true
		b.mu.Unlock()
		b.stopping = true
		return nil
	})
	if err != nil && !errors.Is(err, domain.ErrClosed) {
		return err
	}
	<-b.done
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closeErr
}

func (b *backend) Discover(ctx context.Context) ([]domain.DeviceInfo, error) {
	devices, err := b.enumerateDevices(ctx)
	infos := make([]domain.DeviceInfo, noValue, len(devices))
	for _, d := range devices {
		infos = append(infos, domain.CloneInfo(&d.info))
	}
	return infos, err
}

func (b *backend) enumerateDevices(ctx context.Context) ([]nativeDevice, error) {
	if b.isClosed() {
		return nil, domain.ErrClosed
	}
	var list []input.RAWINPUTDEVICELIST
	op := func(ctx context.Context) error {
		var err error
		list, err = enumerateRawDevices(ctx)
		return err
	}
	if err := b.retry(ctx, op); err != nil {
		return nil, normalizeError(err)
	}
	return b.describeDevices(ctx, list)
}

func (b *backend) retry(ctx context.Context, op func(context.Context) error) error {
	if b.retrier == nil {
		return op(ctx)
	}
	return b.retrier.Do(ctx, op, transient)
}

func enumerateRawDevices(ctx context.Context) ([]input.RAWINPUTDEVICELIST, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	return queryRawDevices()
}

func queryRawDevices() ([]input.RAWINPUTDEVICELIST, error) {
	var count uint32
	size := uint32(unsafe.Sizeof(input.RAWINPUTDEVICELIST{}))
	if err := resultError(rawDeviceList(nil, &count, size)); err != nil {
		return nil, err
	}
	if count > maxDevices {
		return nil, domain.ErrInvalidOptions
	}
	if count == noValue {
		return nil, nil
	}
	return readRawDeviceList(count, size)
}

func readRawDeviceList(count, size uint32) ([]input.RAWINPUTDEVICELIST, error) {
	list := make([]input.RAWINPUTDEVICELIST, count)
	n, err := rawDeviceList(&list[noValue], &count, size)
	if err != nil {
		return nil, err
	}
	if uint64(n) > uint64(len(list)) {
		return nil, domain.ErrInvalidOptions
	}
	return list[:n], nil
}

func (b *backend) describeDevices(
	ctx context.Context,
	list []input.RAWINPUTDEVICELIST,
) ([]nativeDevice, error) {
	snapshot := deviceSnapshot{}
	for _, item := range list {
		if err := context.Cause(ctx); err != nil {
			return snapshot.devices, errors.Join(snapshot.diagnostics, err)
		}
		device, err := b.describeRawDevice(ctx, item)
		snapshot.add(device, err)
	}
	slices.SortFunc(
		snapshot.devices,
		func(a, b nativeDevice) int { return cmp.Compare(a.info.ID, b.info.ID) },
	)
	return snapshot.devices, errors.Join(snapshot.diagnostics, context.Cause(ctx))
}

func (snapshot *deviceSnapshot) add(device nativeDevice, err error) {
	if err != nil {
		snapshot.diagnostics = errors.Join(snapshot.diagnostics, normalizeError(err))
		return
	}
	snapshot.devices = append(snapshot.devices, device)
}

func (b *backend) describeRawDevice(
	ctx context.Context,
	item input.RAWINPUTDEVICELIST,
) (nativeDevice, error) {
	var device nativeDevice
	err := b.retry(ctx, func(ctx context.Context) error {
		var err error
		device, err = describeDevice(ctx, item.HDevice, uint32(item.DwType))
		return err
	})
	return device, err
}

func (b *backend) Open(
	ctx context.Context,
	id domain.DeviceID,
	sink ports.EventSink,
) (ports.Capture, error) {
	device, err := b.findDevice(ctx, id)
	if err != nil {
		return nil, err
	}
	c := b.makeCapture(device, sink)
	if err := c.loadCapabilities(ctx); err != nil {
		return nil, normalizeError(err)
	}
	err = b.call(ctx, c.register)
	if err != nil {
		return nil, normalizeError(err)
	}
	return c.checkOpened(ctx)
}

func (b *backend) findDevice(ctx context.Context, id domain.DeviceID) (nativeDevice, error) {
	discovery, cancel := context.WithTimeout(ctx, ports.DiscoveryBudget)
	devices, err := b.enumerateDevices(discovery)
	cancel()
	if err != nil && (len(devices) == noValue || context.Cause(ctx) != nil) {
		return nativeDevice{}, err
	}
	return selectDevice(devices, id, err)
}

func selectDevice(
	devices []nativeDevice,
	id domain.DeviceID,
	diagnostics error,
) (nativeDevice, error) {
	for _, device := range devices {
		if device.info.ID == id {
			return device, nil
		}
	}
	return nativeDevice{}, errors.Join(domain.ErrNotFound, diagnostics)
}

func (b *backend) makeCapture(device nativeDevice, sink ports.EventSink) *capture {
	return &capture{
		backend: b, device: device, info: domain.CloneInfo(&device.info), sink: sink,
		held: make(map[domain.ControlID]bool), values: make(map[domain.ControlID]int64),
		buttons: make(map[byte]map[domain.ControlID]bool),
		native: ext.Info{
			RawInputHandle: uintptr(device.handle), DeviceType: device.kind,
			UsagePage: device.tlc.page, Usage: device.tlc.usage, Version: device.version,
		},
	}
}

func (c *capture) loadCapabilities(ctx context.Context) error {
	switch c.device.kind {
	case singleValue:
		c.keyboardCapabilities()
	case noValue:
		c.mouseCapabilities()
	case secondValue:
		return c.hidCapabilities(ctx)
	default:
		return domain.ErrUnsupported
	}
	return nil
}

func (c *capture) register() error {
	b := c.backend
	if err := b.acquireRegistration(c.device.tlc); err != nil {
		return err
	}
	if b.captures[c.device.handle] == nil {
		b.captures[c.device.handle] = make(map[*capture]struct{})
	}
	b.captures[c.device.handle][c] = struct{}{}
	return nil
}

func (c *capture) checkOpened(ctx context.Context) (ports.Capture, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, errors.Join(err, c.Close())
	}
	return c, nil
}

func (c *capture) Info() domain.DeviceInfo { return domain.CloneInfo(&c.info) }

func (c *capture) Capabilities() domain.Capabilities { return domain.CloneCapabilities(&c.caps) }

func (c *capture) NativeInfo() ext.Info {
	info := c.native
	info.Controls = append([]ext.NativeControl(nil), info.Controls...)
	return info
}

func (c *capture) Extension(target any) bool {
	if out, ok := target.(*ext.Info); ok && out != nil {
		*out = c.NativeInfo()
		return true
	}
	if out, ok := target.(*ext.Metadata); ok && out != nil {
		*out = c
		return true
	}
	return false
}

func (c *capture) Close() error {
	c.closeOnce.Do(c.closeCapture)
	return normalizeError(c.closeErr)
}

func (c *capture) closeCapture() {
	c.closed.Store(true)
	c.closeErr = c.backend.call(context.Background(), c.unregister)
	if errors.Is(c.closeErr, domain.ErrClosed) {
		c.closeErr = nil
	}
}

func (c *capture) unregister() error {
	group := c.backend.captures[c.device.handle]
	if _, exists := group[c]; !exists {
		return nil
	}
	delete(group, c)
	if len(group) == noValue {
		delete(c.backend.captures, c.device.handle)
	}
	return c.backend.releaseRegistration(c.device.tlc)
}

func (b *backend) acquireRegistration(usage topLevel) error {
	if usage.page == noValue || usage.usage == noValue {
		return domain.ErrUnsupported
	}
	list, err := registeredDevices()
	if err != nil {
		return err
	}
	if b.registrationConflict(usage, list) {
		return domain.ErrRegistrationConflict
	}
	return b.retainRegistration(usage)
}

func (b *backend) retainRegistration(usage topLevel) error {
	if b.registrations[usage] != noValue {
		b.registrations[usage]++
		return nil
	}
	if err := b.addRegistration(usage); err != nil {
		return err
	}
	b.registrations[usage] = singleValue
	return nil
}

func (b *backend) registrationConflict(usage topLevel, list []input.RAWINPUTDEVICE) bool {
	for _, registration := range list {
		if matchesRegistration(usage, registration) && registration.HwndTarget != b.hwnd {
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

func (b *backend) addRegistration(usage topLevel) error {
	return input.RegisterRawInputDevices([]input.RAWINPUTDEVICE{{
		UsUsagePage: usage.page, UsUsage: usage.usage,
		DwFlags: input.RAWINPUTDEVICE_FLAGS(registrationFlags), HwndTarget: b.hwnd,
	}}, uint32(unsafe.Sizeof(input.RAWINPUTDEVICE{})))
}

func (b *backend) releaseRegistration(usage topLevel) error {
	if b.registrations[usage] > singleValue {
		b.registrations[usage]--
		return nil
	}
	if b.registrations[usage] == noValue {
		return nil
	}
	b.registrations[usage] = noValue
	err := b.removeRegistration(usage)
	if err == nil {
		delete(b.registrations, usage)
	}
	return err
}

func (b *backend) removeRegistration(usage topLevel) error {
	list, err := registeredDevices()
	if err != nil {
		return err
	}
	for _, registration := range list {
		if registration.UsUsagePage == usage.page && registration.UsUsage == usage.usage {
			return b.removeOwnedRegistration(registration)
		}
	}
	return nil
}

func (b *backend) removeOwnedRegistration(registration input.RAWINPUTDEVICE) error {
	if registration.HwndTarget != b.hwnd {
		return domain.ErrRegistrationConflict
	}
	registration.DwFlags = input.RAWINPUTDEVICE_FLAGS(singleValue)
	registration.HwndTarget = noValue
	return input.RegisterRawInputDevices(
		[]input.RAWINPUTDEVICE{registration},
		uint32(unsafe.Sizeof(registration)),
	)
}

func registeredDevices() ([]input.RAWINPUTDEVICE, error) {
	var count uint32
	size := uint32(unsafe.Sizeof(input.RAWINPUTDEVICE{}))
	n, err := input.GetRegisteredRawInputDevices(nil, &count, size)
	if n == infiniteWait {
		return nil, nonzeroError(err)
	}
	if count == noValue {
		return nil, nil
	}
	if count > maxDevices {
		return nil, domain.ErrInvalidOptions
	}
	return readRegisteredDevices(count, size)
}

func readRegisteredDevices(count, size uint32) ([]input.RAWINPUTDEVICE, error) {
	list := make([]input.RAWINPUTDEVICE, count)
	n, err := input.GetRegisteredRawInputDevices(&list[noValue], &count, size)
	if n == infiniteWait {
		return nil, nonzeroError(err)
	}
	if uint64(n) > uint64(len(list)) {
		return nil, domain.ErrInvalidOptions
	}
	return list[:n], nil // Ignore stale GetLastError on successful UINT results.
}

func (b *backend) disconnect(handle foundation.HANDLE) {
	group := b.captures[handle]
	delete(b.captures, handle)
	for c := range group {
		_ = b.releaseRegistration(c.device.tlc)
		if !c.closed.Swap(true) {
			c.sink.Fail(domain.ErrDisconnected)
		}
	}
}

func rawDeviceList(list *input.RAWINPUTDEVICELIST, count *uint32, size uint32) (uint32, error) {
	n, err := input.GetRawInputDeviceList(list, count, size)
	if n == infiniteWait {
		return n, nonzeroError(err)
	}
	return n, nil
}

func rawDeviceInfo(handle foundation.HANDLE, command input.RAW_INPUT_DEVICE_INFO_COMMAND,
	buffer nativeBuffer,
) (uint32, error) {
	n, err := input.GetRawInputDeviceInfo(handle, command, buffer.data, buffer.size)
	if n == infiniteWait {
		return n, nonzeroError(err)
	}
	return n, nil
}

func describeDevice(
	ctx context.Context,
	handle foundation.HANDLE,
	kind uint32,
) (nativeDevice, error) {
	device := nativeDevice{handle: handle, kind: kind}
	if err := device.loadDeviceInfo(); err != nil {
		return device, err
	}
	if err := device.loadDevicePath(); err != nil {
		return device, err
	}
	if err := context.Cause(ctx); err != nil {
		return device, err
	}
	enrichIdentity(&device)
	return device, nil
}

func (d *nativeDevice) loadDeviceInfo() error {
	var info input.RID_DEVICE_INFO
	info.CbSize = uint32(unsafe.Sizeof(info))
	size := info.CbSize
	err := resultError(
		rawDeviceInfo(d.handle, deviceInfoCommand, nativeBuffer{unsafe.Pointer(&info), &size}),
	)
	if err != nil {
		return err
	}
	return d.applyDeviceInfo(info.Anonymous.Data)
}

func (d *nativeDevice) applyDeviceInfo(words [sixthValue]uint32) error {
	switch d.kind {
	case noValue:
		d.tlc = topLevel{singleValue, secondValue}
		d.buttons, d.hwheel = words[singleValue], words[thirdValue] != noValue
		d.info.Classes = []domain.DeviceClass{domain.ClassMouse}
	case singleValue:
		d.tlc = topLevel{singleValue, sixthValue}
		d.info.Classes = []domain.DeviceClass{domain.ClassKeyboard}
	case secondValue:
		d.applyHIDDeviceInfo(words)
	default:
		return domain.ErrUnsupported
	}
	return nil
}

func (d *nativeDevice) applyHIDDeviceInfo(words [sixthValue]uint32) {
	d.tlc = topLevel{uint16(words[thirdValue]), uint16(words[thirdValue] >> sixteenthValue)}
	d.version = words[secondValue]
	d.info.VendorID = optionalWord(words[noValue])
	d.info.ProductID = optionalWord(words[singleValue])
	d.info.Classes = []domain.DeviceClass{classFor(d.tlc)}
}

func optionalWord(value uint32) *uint16 {
	if value > wordMask {
		return nil
	}
	result := uint16(value)
	return &result
}

func (d *nativeDevice) loadDevicePath() error {
	path, err := readDevicePath(d.handle)
	if err != nil {
		return err
	}
	d.info.Path = path
	d.info.ID = domain.DeviceID("win32:" + strings.ToLower(path))
	return nil
}

func readDevicePath(handle foundation.HANDLE) (string, error) {
	var chars uint32
	if err := resultError(
		rawDeviceInfo(handle, deviceNameCommand, nativeBuffer{nil, &chars}),
	); err != nil {
		return emptyString, err
	}
	if chars == noValue || chars > maxNativeBuffer/secondValue {
		return emptyString, domain.ErrNotFound
	}
	return readDeviceName(handle, chars)
}

func readDeviceName(handle foundation.HANDLE, chars uint32) (string, error) {
	path := make([]uint16, chars+singleValue)
	err := resultError(
		rawDeviceInfo(
			handle,
			deviceNameCommand,
			nativeBuffer{unsafe.Pointer(&path[noValue]), &chars},
		),
	)
	if err != nil {
		return emptyString, err
	}
	result := syscall.UTF16ToString(path)
	if result == emptyString {
		return emptyString, domain.ErrNotFound
	}
	return result, nil
}

func enrichIdentity(d *nativeDevice) {
	procedures := []*native.Proc{
		hid.Procs.HidD_GetAttributes, hid.Procs.HidD_GetProductString,
		hid.Procs.HidD_GetManufacturerString, hid.Procs.HidD_GetSerialNumberString,
	}
	if findProcedures(procedures) != nil {
		return
	}
	// Zero access avoids stealing keyboard/mouse read access from the OS.
	handle, err := filesystem.CreateFile(
		d.info.Path,
		noValue,
		filesystem.FILE_SHARE_MODE(thirdValue),
		nil,
		filesystem.FILE_CREATION_DISPOSITION(thirdValue),
		noValue,
		noValue,
	)
	if err != nil {
		return
	}
	defer foundation.CloseHandle(handle)
	d.loadAttributes(handle)
	d.loadStrings(handle)
}

func (d *nativeDevice) loadAttributes(handle foundation.HANDLE) {
	var attributes hid.HIDD_ATTRIBUTES
	attributes.Size = uint32(unsafe.Sizeof(attributes))
	if hid.HidD_GetAttributes(handle, &attributes) != noValue {
		vendor, product := attributes.VendorID, attributes.ProductID
		d.info.VendorID, d.info.ProductID = &vendor, &product
		d.version = uint32(attributes.VersionNumber)
	}
}

func (d *nativeDevice) loadStrings(handle foundation.HANDLE) {
	if name := readHIDString(handle, hid.HidD_GetProductString); name != emptyString {
		d.info.Name = name
	}
	d.info.Manufacturer = readHIDString(handle, hid.HidD_GetManufacturerString)
	d.info.Serial = readHIDString(handle, hid.HidD_GetSerialNumberString)
}

func readHIDString(
	handle foundation.HANDLE,
	get func(foundation.HANDLE, []byte) foundation.BOOLEAN,
) string {
	data := make([]byte, hidStringBytes)
	if get(handle, data) == noValue {
		return emptyString
	}
	wide := make([]uint16, len(data)/secondValue)
	for i := range wide {
		wide[i] = binary.LittleEndian.Uint16(data[i*secondValue:])
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
		return syscall.EINVAL
	}
	return err
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
	return categorizeError(err, categories)
}

func categorizeError(err error, categories []errorCategory) error {
	for _, category := range categories {
		if errors.Is(err, category.native) {
			return errors.Join(category.domain, err)
		}
	}
	return err
}

func (c *capture) keyboardCapabilities() {
	controls := make(map[domain.ControlID]domain.Control)
	c.addScanControls(controls, scanUsages, seventhValue)
	c.addScanControls(controls, consumerScans, twelfthValue)
	c.addVirtualControls(controls)
	c.addSystemControls(controls)
	for _, control := range controls {
		c.caps.Controls = append(c.caps.Controls, control)
	}
	c.caps.Repeat = domain.SupportSupported
	slices.SortFunc(c.caps.Controls, compareControls)
	slices.SortFunc(c.native.Controls, compareKeyboardControls)
}

func (c *capture) addScanControls(
	controls map[domain.ControlID]domain.Control,
	scans map[uint16]uint16,
	page uint16,
) {
	for scan, usage := range scans {
		control := c.makeKeyControl(scan, domain.HID(page, usage))
		controls[control.ID] = control
	}
}

func (c *capture) addVirtualControls(controls map[domain.ControlID]domain.Control) {
	for key, usage := range consumerVirtualKeys {
		control := c.makeKeyControl(virtualScanPrefix|key, domain.HID(twelfthValue, usage))
		controls[control.ID] = control
	}
}

func (c *capture) addSystemControls(controls map[domain.ControlID]domain.Control) {
	for _, scan := range []uint16{scanPower, scanSleep, scanWake} {
		control := c.makeKeyControl(scan, systemScanUsage(scan))
		controls[control.ID] = control
	}
}

func (c *capture) makeKeyControl(scan uint16, usage domain.Usage) domain.Control {
	id := keyID(usage, scan)
	control := domain.Control{
		Mode:    domain.AxisUnknown,
		ID:      id,
		Name:    fmt.Sprintf("Key %04x:%04x", uint16(usage>>sixteenthValue), uint16(usage)),
		Kind:    domain.ControlKey,
		Usage:   usage,
		Mapping: domain.MappingInferred,
		Range:   &domain.Range{Min: noValue, Max: singleValue},
		Unit:    domain.UnitBoolean,
		Support: domain.SupportUnknown,
	}
	nativeControl := ext.NativeControl{
		ID:       string(id),
		ScanCode: scan,
	}
	if scan&virtualScanPrefix == virtualScanPrefix {
		nativeControl.ScanCode, nativeControl.VirtualKey = noValue, scan&byteMask
	}
	c.native.Controls = append(c.native.Controls, nativeControl)
	return control
}

func compareKeyboardControls(a, b ext.NativeControl) int {
	if a.ID == b.ID {
		return cmp.Compare(a.ScanCode, b.ScanCode)
	}
	return cmp.Compare(a.ID, b.ID)
}

func keyID(usage domain.Usage, scan uint16) domain.ControlID {
	if scan&virtualScanPrefix == virtualScanPrefix {
		return domain.ControlID(fmt.Sprintf("key:virtual:%04x", scan&byteMask))
	}
	if scan != noValue {
		return domain.ControlID(fmt.Sprintf("key:scan:%04x", scan))
	}
	if usage != noValue {
		return domain.ControlID(fmt.Sprintf("key:%08x", uint32(usage)))
	}
	return domain.ControlID(fmt.Sprintf("key:native:%04x", scan))
}

func (c *capture) mouseCapabilities() {
	c.caps.Repeat = domain.SupportUnsupported
	c.addMouseButtons()
	c.addMouseAxes()
	c.addMouseWheels()
	if c.device.hwheel {
		c.caps.Controls[len(c.caps.Controls)-singleValue].Support = domain.SupportSupported
	}
}

func (c *capture) addMouseButtons() {
	for i := uint16(singleValue); i <= fifthValue; i++ {
		support := domain.SupportUnknown
		if uint32(i) <= c.device.buttons {
			support = domain.SupportSupported
		}
		c.caps.Controls = append(c.caps.Controls, domain.Control{
			Mode:    domain.AxisUnknown,
			ID:      domain.ControlID(fmt.Sprintf(buttonIDFormat, i)),
			Name:    fmt.Sprintf("Button %d", i),
			Kind:    domain.ControlButton,
			Usage:   domain.HID(buttonPage, i),
			Mapping: domain.MappingInferred,
			Range:   &domain.Range{Min: noValue, Max: singleValue},
			Unit:    domain.UnitBoolean,
			Support: support,
		})
	}
}

func (c *capture) addMouseAxes() {
	for i, axis := range []string{"x", "y"} {
		c.caps.Controls = append(c.caps.Controls,
			domain.Control{
				ID: domain.ControlID(
					"rel:" + axis,
				),
				Name:    "Relative " + strings.ToUpper(axis),
				Kind:    domain.ControlAxis,
				Usage:   domain.HID(singleValue, uint16(fortyEighthValue+i)),
				Mapping: domain.MappingInferred,
				Mode:    domain.AxisRelative,
				Unit:    domain.UnitCounts,
				Support: domain.SupportUnknown,
			},
			domain.Control{
				ID: domain.ControlID(
					"abs:" + axis,
				),
				Name:    "Absolute " + strings.ToUpper(axis),
				Kind:    domain.ControlAxis,
				Usage:   domain.HID(singleValue, uint16(fortyEighthValue+i)),
				Mapping: domain.MappingInferred,
				Mode:    domain.AxisAbsolute,
				Range:   &domain.Range{Min: noValue, Max: wordMask},
				Unit:    domain.UnitLogical,
				Support: domain.SupportUnknown,
			})
	}
}

func (c *capture) addMouseWheels() {
	c.caps.Controls = append(
		c.caps.Controls,
		domain.Control{
			ID:      wheelControlID,
			Name:    "Wheel",
			Kind:    domain.ControlAxis,
			Usage:   domain.HID(singleValue, wheelUsage),
			Mapping: domain.MappingInferred,
			Mode:    domain.AxisRelative,
			Unit:    domain.UnitDetents,
			Support: domain.SupportUnknown,
		},
		domain.Control{
			ID:      panControlID,
			Name:    "Horizontal wheel",
			Kind:    domain.ControlAxis,
			Usage:   domain.HID(twelfthValue, panUsage),
			Mapping: domain.MappingInferred,
			Mode:    domain.AxisRelative,
			Unit:    domain.UnitDetents,
			Support: domain.SupportUnknown,
		},
	)
}

func (c *capture) hidCapabilities(ctx context.Context) error {
	return c.retryMetadata(ctx, c.loadHIDCapabilities)
}

func (c *capture) loadHIDCapabilities(ctx context.Context) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if err := findProcedures(hidProcedures()); err != nil {
		return err
	}
	data, err := c.readPreparsedData()
	if err != nil {
		return err
	}
	return c.buildHIDCapabilities(data)
}

func (c *capture) buildHIDCapabilities(data []byte) error {
	builder, err := makeHIDBuilder(data)
	if err != nil {
		return err
	}
	if err := builder.loadControls(); err != nil {
		return err
	}
	c.commitHID(builder)
	return nil
}

func hidProcedures() []*native.Proc {
	return []*native.Proc{
		hid.Procs.HidP_GetCaps, hid.Procs.HidP_GetButtonCaps, hid.Procs.HidP_GetValueCaps,
		hid.Procs.HidP_GetData, hid.Procs.HidP_MaxDataListLength,
	}
}

func (c *capture) readPreparsedData() ([]byte, error) {
	var size uint32
	command := input.RAW_INPUT_DEVICE_INFO_COMMAND(preparsedDataCommand)
	if err := resultError(
		rawDeviceInfo(c.device.handle, command, nativeBuffer{nil, &size}),
	); err != nil {
		return nil, err
	}
	if size == noValue || size > maxNativeBuffer {
		return nil, domain.ErrUnsupported
	}
	return c.readPreparsedBuffer(size)
}

func (c *capture) readPreparsedBuffer(size uint32) ([]byte, error) {
	data := make([]byte, size)
	err := resultError(rawDeviceInfo(c.device.handle, preparsedDataCommand,
		nativeBuffer{unsafe.Pointer(&data[noValue]), &size}))
	if err != nil {
		return nil, err
	}
	return trimPreparsedData(data, size)
}

func trimPreparsedData(data []byte, size uint32) ([]byte, error) {
	if size > uint32(len(data)) {
		return nil, domain.ErrEventLoss
	}
	if size == noValue {
		return nil, domain.ErrUnsupported
	}
	return data[:size], nil
}

func makeHIDBuilder(data []byte) (*hidBuilder, error) {
	pp := hid.PHIDP_PREPARSED_DATA(uintptr(unsafe.Pointer(&data[noValue])))
	defer runtime.KeepAlive(data)
	var caps hid.HIDP_CAPS
	if status := hid.HidP_GetCaps(pp, &caps); status != hid.HIDP_STATUS_SUCCESS {
		return nil, hidError("HidP_GetCaps", status)
	}
	desc := &descriptor{
		preparsed: data, reportLen: caps.InputReportByteLength,
		controls: make(map[hidIndex]hidControl), reportIDs: make(map[byte]bool),
		maxData: hid.HidP_MaxDataListLength(hid.HidP_Input, pp),
	}
	if desc.maxData > maxDevices || desc.reportLen == noValue {
		return nil, domain.ErrUnsupported
	}
	return &hidBuilder{descriptor: desc, caps: caps}, nil
}

func (builder *hidBuilder) preparsedPointer() hid.PHIDP_PREPARSED_DATA {
	return hid.PHIDP_PREPARSED_DATA(uintptr(unsafe.Pointer(&builder.descriptor.preparsed[noValue])))
}

func (builder *hidBuilder) loadControls() error {
	defer runtime.KeepAlive(builder.descriptor.preparsed)
	if err := builder.loadButtons(); err != nil {
		return err
	}
	return builder.loadValues()
}

func (builder *hidBuilder) add(control hidControl) {
	key := hidIndex{control.native.ReportID, control.native.DataIndex}
	if _, exists := builder.descriptor.controls[key]; exists {
		return
	}
	builder.descriptor.controls[key] = control
	builder.descriptor.reportIDs[key.report] = true
	builder.controls = append(builder.controls, control.control)
	builder.nativeControls = append(builder.nativeControls, control.native)
}

func (builder *hidBuilder) loadButtons() error {
	buttons, err := builder.readButtonCaps()
	if err != nil {
		return err
	}
	for _, capability := range buttons {
		if err := builder.addButtonCapability(capability); err != nil {
			return err
		}
	}
	return nil
}

func (builder *hidBuilder) readButtonCaps() ([]hid.HIDP_BUTTON_CAPS, error) {
	count := builder.caps.NumberInputButtonCaps
	if count == noValue {
		return nil, nil
	}
	buttons := make([]hid.HIDP_BUTTON_CAPS, count)
	status := hid.HidP_GetButtonCaps(
		hid.HidP_Input,
		&buttons[noValue],
		&count,
		builder.preparsedPointer(),
	)
	if status != hid.HIDP_STATUS_SUCCESS {
		return nil, hidError("HidP_GetButtonCaps", status)
	}
	if int(count) > len(buttons) {
		return nil, domain.ErrEventLoss
	}
	return buttons[:count], nil
}

func (builder *hidBuilder) addButtonCapability(button hid.HIDP_BUTTON_CAPS) error {
	if button.IsAlias != noValue {
		return nil
	}
	span, err := capRange(button.Anonymous.Data, button.IsRange)
	if err != nil {
		return err
	}
	for usage, index := span.firstUsage, span.firstIndex; usage <= span.lastUsage; usage, index =
		usage+singleValue, index+singleValue {
		builder.add(makeHIDButton(button, usage, index))
	}
	return nil
}

func makeHIDButton(button hid.HIDP_BUTTON_CAPS, usage, dataIndex uint32) hidControl {
	id := hidID(button.ReportID, button.LinkCollection, uint16(dataIndex))
	kind := buttonKind(button.UsagePage)
	support := buttonSupport(button.IsAbsolute)
	return hidControl{
		button: true,
		control: domain.Control{
			Mode:    domain.AxisUnknown,
			ID:      id,
			Name:    fmt.Sprintf(hidNameFormat, button.UsagePage, usage),
			Kind:    kind,
			Usage:   domain.HID(button.UsagePage, uint16(usage)),
			Mapping: domain.MappingReported,
			Range:   &domain.Range{Min: noValue, Max: singleValue},
			Unit:    domain.UnitBoolean,
			Support: support,
		},
		native: ext.NativeControl{
			ID:             string(id),
			ReportID:       button.ReportID,
			DataIndex:      uint16(dataIndex),
			LinkCollection: button.LinkCollection,
			ReportCount:    button.ReportCount,
			BitSize:        singleValue,
			Bounds:         ext.DescriptorBounds{LogicalMin: noValue, LogicalMax: singleValue},
			Absolute:       button.IsAbsolute != noValue,
		},
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

func (builder *hidBuilder) loadValues() error {
	values, err := builder.readValueCaps()
	if err != nil {
		return err
	}
	for _, capability := range values {
		if err := builder.addValueCapability(capability); err != nil {
			return err
		}
	}
	return nil
}

func (builder *hidBuilder) readValueCaps() ([]hid.HIDP_VALUE_CAPS, error) {
	count := builder.caps.NumberInputValueCaps
	if count == noValue {
		return nil, nil
	}
	values := make([]hid.HIDP_VALUE_CAPS, count)
	status := hid.HidP_GetValueCaps(
		hid.HidP_Input,
		&values[noValue],
		&count,
		builder.preparsedPointer(),
	)
	if status != hid.HIDP_STATUS_SUCCESS {
		return nil, hidError("HidP_GetValueCaps", status)
	}
	if int(count) > len(values) {
		return nil, domain.ErrEventLoss
	}
	return values[:count], nil
}

func (builder *hidBuilder) addValueCapability(value hid.HIDP_VALUE_CAPS) error {
	if value.IsAlias != noValue {
		return nil
	}
	span, err := capRange(value.Anonymous.Data, value.IsRange)
	if err != nil {
		return err
	}
	for usage, index := span.firstUsage, span.firstIndex; usage <= span.lastUsage; usage, index =
		usage+singleValue, index+singleValue {
		builder.add(makeHIDValue(value, usage, index))
	}
	return nil
}

func makeHIDValue(value hid.HIDP_VALUE_CAPS, usage, dataIndex uint32) hidControl {
	bounds := logicalBounds(value.LogicalMin, value.LogicalMax)
	id := hidID(value.ReportID, value.LinkCollection, uint16(dataIndex))
	ctrl := domain.Control{
		Mode: domain.AxisUnknown,
		ID:   id,
		Name: fmt.Sprintf(hidNameFormat, value.UsagePage, usage),
		Kind: domain.ControlAxis,
		Usage: domain.HID(
			value.UsagePage,
			uint16(usage),
		),
		Mapping: domain.MappingReported,
		Unit:    domain.UnitLogical,
		Support: domain.SupportSupported,
	}
	classifyValue(&ctrl, value, bounds)
	normalHat := isConventionalHat(value, usage, bounds)
	classifyHat(&ctrl, value, usage)

	return hidControl{control: ctrl, hat: normalHat, native: ext.NativeControl{
		ID:             string(id),
		ReportID:       value.ReportID,
		DataIndex:      uint16(dataIndex),
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
		Absolute:      value.IsAbsolute != noValue,
	}}
}

func logicalBounds(minimum, maximum int32) domain.Range {
	bounds := domain.Range{Min: int64(minimum), Max: int64(maximum)}
	if minimum >= noValue && maximum < noValue {
		bounds.Max = int64(uint32(maximum))
	}
	return bounds
}

func (c *capture) commitHID(builder *hidBuilder) {
	slices.SortFunc(builder.controls, compareControls)
	slices.SortFunc(builder.nativeControls, compareNativeControls)
	c.hid = builder.descriptor
	c.caps = domain.Capabilities{
		Controls: builder.controls,
		Complete: true,
		Repeat:   domain.SupportUnsupported,
	}
	c.native.Controls = builder.nativeControls
}

func compareControls(a, b domain.Control) int { return cmp.Compare(a.ID, b.ID) }

func compareNativeControls(a, b ext.NativeControl) int { return cmp.Compare(a.ID, b.ID) }

func (c *capture) retryMetadata(ctx context.Context, op func(context.Context) error) error {
	if c.backend.retrier == nil {
		return op(ctx)
	}
	return c.backend.retrier.Do(ctx, op, transient)
}

func hidID(report byte, collection, index uint16) domain.ControlID {
	return domain.ControlID(fmt.Sprintf("hid:%02x:%04x:%04x", report, collection, index))
}

func capRange(words [eighthValue]uint16, rangeKind foundation.BOOLEAN) (capabilityRange, error) {
	span := capabilityRange{
		firstUsage: uint32(words[noValue]), lastUsage: uint32(words[noValue]),
		firstIndex: uint32(words[sixthValue]), lastIndex: uint32(words[sixthValue]),
	}
	if rangeKind != noValue {
		span.lastUsage, span.lastIndex = uint32(words[singleValue]), uint32(words[seventhValue])
	}
	if !span.valid() {
		return capabilityRange{}, errors.Join(
			domain.ErrUnsupported,
			errors.New("invalid HID capability range"),
		)
	}
	return span, nil
}

func (span capabilityRange) valid() bool {
	return span.lastUsage >= span.firstUsage && span.lastIndex >= span.firstIndex &&
		span.lastUsage-span.firstUsage == span.lastIndex-span.firstIndex
}

func conventionalHat(value hid.HIDP_VALUE_CAPS, minValue, maxValue int64) bool {
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

func conventionalHatUnits(value hid.HIDP_VALUE_CAPS) bool {
	return value.PhysicalMin == noValue &&
		(value.Units == noValue || value.Units == angularUnits) &&
		value.UnitsExp == noValue
}

func conventionalHatExtent(maximum int32, positions int64) bool {
	return maximum == noValue || (positions == fourthValue && maximum == cardinalExtent) ||
		(positions == eighthValue && maximum == compassExtent)
}

func hidError(op string, status foundation.NTSTATUS) error {
	return fmt.Errorf("%s: %w (HID status 0x%08x)", op, domain.ErrUnsupported, uint32(status))
}

func (b *backend) readInput(handle input.HRAWINPUT) {
	data, err := readInputData(handle)
	if err != nil {
		b.failAll(err)
		return
	}
	packet, err := decodeInputPacket(data)
	if err != nil {
		b.failAll(err)
		return
	}
	for c := range b.captures[packet.device] {
		c.handlePacket(packet)
	}
}

func rawInputData(handle input.HRAWINPUT, buffer nativeBuffer) (uint32, error) {
	ret, _, errno := syscall.SyscallN(
		input.Procs.GetRawInputData.Addr(),
		uintptr(handle),
		inputDataCommand,
		uintptr(
			buffer.data,
		),
		uintptr(unsafe.Pointer(buffer.size)),
		uintptr(unsafe.Sizeof(input.RAWINPUTHEADER{})),
	)
	if uint32(ret) == infiniteWait {
		return noValue, errors.Join(domain.ErrEventLoss, native.LastError(errno))
	}
	return uint32(ret), nil
}

func readInputData(handle input.HRAWINPUT) ([]byte, error) {
	var size uint32
	if err := resultError(rawInputData(handle, nativeBuffer{nil, &size})); err != nil {
		return nil, err
	}
	headerSize := uint32(unsafe.Sizeof(input.RAWINPUTHEADER{}))
	if size < headerSize || size > maxNativeBuffer {
		return nil, domain.ErrEventLoss
	}
	return readInputBuffer(handle, size)
}

func readInputBuffer(handle input.HRAWINPUT, size uint32) ([]byte, error) {
	data := make([]byte, size)
	n, err := rawInputData(handle, nativeBuffer{unsafe.Pointer(&data[noValue]), &size})
	if err != nil {
		return nil, err
	}
	headerSize := uint32(unsafe.Sizeof(input.RAWINPUTHEADER{}))
	if n < headerSize || uint64(n) > uint64(len(data)) {
		return nil, domain.ErrEventLoss
	}
	return data[:n], nil
}

func decodeInputPacket(data []byte) (inputPacket, error) {
	headerSize := uint32(unsafe.Sizeof(input.RAWINPUTHEADER{}))
	declared := binary.LittleEndian.Uint32(data[fourthValue:])
	if declared < headerSize || uint64(declared) > uint64(len(data)) {
		return inputPacket{}, domain.ErrEventLoss
	}
	now := time.Now()
	return inputPacket{
		kind:   binary.LittleEndian.Uint32(data),
		device: foundation.HANDLE(binary.LittleEndian.Uint64(data[eighthValue:])),
		body:   data[headerSize:declared],
		stamp:  domain.Timestamp{Time: now.UTC(), ReceivedAt: now, Source: domain.TimestampReceipt},
	}, nil
}

func (c *capture) handlePacket(packet inputPacket) {
	if c.closed.Load() {
		return
	}
	if packet.kind != c.device.kind {
		c.fail(domain.ErrEventLoss)
		return
	}
	c.dispatchInput(packet)
}

func (c *capture) dispatchInput(packet inputPacket) {
	switch packet.kind {
	case singleValue:
		c.keyboard(packet.body, packet.stamp)
	case noValue:
		c.mouse(packet.body, packet.stamp)
	case secondValue:
		c.reports(packet.body, packet.stamp)
	default:
		c.fail(domain.ErrEventLoss)
	}
}

func (b *backend) failAll(err error) {
	for _, group := range b.captures {
		for c := range group {
			c.fail(err)
		}
	}
}

func (c *capture) fail(err error) {
	if !c.closed.Swap(true) {
		c.sink.Fail(err)
	}
}

func (c *capture) emit(event domain.Event) {
	event.DeviceID = c.info.ID
	if !c.closed.Load() && !c.sink.Publish(&event) {
		c.closed.Store(true)
	}
}

func (c *capture) keyboard(data []byte, stamp domain.Timestamp) {
	if len(data) < sixteenthValue {
		c.fail(domain.ErrEventLoss)
		return
	}
	key := keyboardInput{
		makeCode:   binary.LittleEndian.Uint16(data),
		flags:      binary.LittleEndian.Uint16(data[secondValue:]),
		virtualKey: binary.LittleEndian.Uint16(data[sixthValue:]),
	}
	if key.makeCode == byteMask {
		c.fail(domain.ErrEventLoss)
		return
	}
	if key.ignored() {
		return
	}
	c.emitKey(key, stamp)
}

func (key keyboardInput) scanCode() uint16 {
	scan := key.makeCode
	if key.flags&secondValue != noValue {
		scan |= scanPrefixE0
	} else if key.flags&fourthValue != noValue {
		scan |= scanPrefixE1
	}
	return scan
}

func (key keyboardInput) ignored() bool {
	if key.virtualKey >= byteMask {
		return true
	}
	switch key.scanCode() {
	case scanPrintScreenPrefix, scanPrintScreenSuffix, scanPausePrefix:
		return true
	default:
		return false
	}
}

func (key keyboardInput) identity() domain.ControlID {
	scan := key.scanCode()
	if key.makeCode == noValue && !knownScan(scan) {
		scan = virtualScanPrefix | key.virtualKey
	}
	return keyID(key.usage(), scan)
}

func knownScan(scan uint16) bool {
	_, keyboard := scanUsages[scan]
	_, consumer := consumerScans[scan]
	return keyboard || consumer
}

func (key keyboardInput) usage() domain.Usage {
	scan := key.scanCode()
	if code, ok := scanUsages[scan]; ok {
		return domain.HID(seventhValue, code)
	}
	if code, ok := consumerScans[scan]; ok {
		return domain.HID(twelfthValue, code)
	}
	if key.makeCode == noValue {
		return virtualKeyUsage(key.virtualKey)
	}
	return systemScanUsage(scan)
}

func virtualKeyUsage(key uint16) domain.Usage {
	if code, ok := consumerVirtualKeys[key]; ok {
		return domain.HID(twelfthValue, code)
	}
	return noValue
}

func systemScanUsage(scan uint16) domain.Usage {
	switch scan {
	case scanPower:
		return domain.HID(singleValue, systemPowerUsage)
	case scanSleep:
		return domain.HID(singleValue, systemSleepUsage)
	case scanWake:
		return domain.HID(singleValue, systemWakeUsage)
	default:
		return noValue
	}
}

func (c *capture) emitKey(key keyboardInput, stamp domain.Timestamp) {
	id := key.identity()
	if key.flags&singleValue != noValue {
		delete(c.held, id)
		c.emit(
			domain.Event{
				ControlID: id,
				Action:    domain.ActionRelease,
				Value:     noValue,
				Timestamp: stamp,
			},
		)
		return
	}
	action := domain.ActionPress
	if c.held[id] {
		action = domain.ActionRepeat
	}
	c.held[id] = true
	c.emit(domain.Event{ControlID: id, Action: action, Value: singleValue, Timestamp: stamp})
}

func (c *capture) mouse(data []byte, stamp domain.Timestamp) {
	if len(data) < mousePacketBytes {
		c.fail(domain.ErrEventLoss)
		return
	}
	mouse := mouseInput{
		flags:   binary.LittleEndian.Uint16(data),
		buttons: binary.LittleEndian.Uint16(data[fourthValue:]),
		wheel:   int16(binary.LittleEndian.Uint16(data[sixthValue:])),
		x:       int64(int32(binary.LittleEndian.Uint32(data[twelfthValue:]))),
		y:       int64(int32(binary.LittleEndian.Uint32(data[sixteenthValue:]))), stamp: stamp,
	}
	c.mouseButtons(mouse)
	c.mouseAxes(mouse)
	c.mouseWheels(mouse)
}

func (c *capture) mouseButtons(mouse mouseInput) {
	for i := uint16(noValue); i < fifthValue; i++ {
		id := domain.ControlID(fmt.Sprintf(buttonIDFormat, i+singleValue))
		if mouse.buttons&(singleValue<<(i*secondValue)) != noValue {
			c.emit(
				domain.Event{
					ControlID: id,
					Action:    domain.ActionPress,
					Value:     singleValue,
					Timestamp: mouse.stamp,
				},
			)
		}
		if mouse.buttons&(singleValue<<(i*secondValue+singleValue)) != noValue {
			c.emit(
				domain.Event{
					ControlID: id,
					Action:    domain.ActionRelease,
					Value:     noValue,
					Timestamp: mouse.stamp,
				},
			)
		}
	}
}

func (c *capture) mouseAxes(mouse mouseInput) {
	if mouse.flags&singleValue != noValue {
		c.absoluteAxis("abs:x", mouse.x, mouse.stamp)
		c.absoluteAxis("abs:y", mouse.y, mouse.stamp)
		return
	}
	c.relativeAxis("rel:x", mouse.x, mouse.stamp)
	c.relativeAxis("rel:y", mouse.y, mouse.stamp)
}

func (c *capture) absoluteAxis(id domain.ControlID, value int64, stamp domain.Timestamp) {
	if previous, exists := c.values[id]; exists && previous == value {
		return
	}
	c.values[id] = value
	c.emit(
		domain.Event{
			ControlID: id,
			Action:    domain.ActionChange,
			Value:     float64(value),
			Timestamp: stamp,
		},
	)
}

func (c *capture) relativeAxis(id domain.ControlID, value int64, stamp domain.Timestamp) {
	if value != noValue {
		c.emit(
			domain.Event{
				ControlID: id,
				Action:    domain.ActionChange,
				Value:     float64(value),
				Timestamp: stamp,
			},
		)
	}
}

func (c *capture) mouseWheels(mouse mouseInput) {
	if mouse.buttons&mouseVerticalWheel != noValue {
		c.emit(domain.Event{
			ControlID: wheelControlID, Action: domain.ActionChange,
			Value: float64(mouse.wheel) / wheelDelta, Timestamp: mouse.stamp,
		})
	}
	if mouse.buttons&mouseHorizontalWheel != noValue {
		c.emit(domain.Event{
			ControlID: panControlID, Action: domain.ActionChange,
			Value: float64(mouse.wheel) / wheelDelta, Timestamp: mouse.stamp,
		})
	}
}

func (c *capture) reports(body []byte, stamp domain.Timestamp) {
	if c.hid == nil || len(body) < eighthValue {
		c.fail(domain.ErrEventLoss)
		return
	}
	size, count := binary.LittleEndian.Uint32(body), binary.LittleEndian.Uint32(body[fourthValue:])
	if !validReportBatch(body, size, count) {
		c.fail(domain.ErrEventLoss)
		return
	}
	c.reportBatch(body, reportBatch{size: size, count: count, stamp: stamp})
}

func validReportBatch(body []byte, size, count uint32) bool {
	return size != noValue && count <= maxDevices &&
		uint64(size)*uint64(count) <= uint64(len(body)-eighthValue)
}

func (c *capture) reportBatch(body []byte, batch reportBatch) {
	for i := uint32(noValue); i < batch.count && !c.closed.Load(); i++ {
		start := eighthValue + uint64(i)*uint64(batch.size)
		c.report(body[start:start+uint64(batch.size)], batch.stamp)
	}
}

func (c *capture) report(report []byte, stamp domain.Timestamp) {
	if len(report) != int(c.hid.reportLen) || len(report) == noValue {
		c.fail(errors.Join(domain.ErrEventLoss, errors.New("unexpected HID report length")))
		return
	}
	reportID := report[noValue]
	if !c.hid.reportIDs[reportID] || c.hid.maxData == noValue {
		return
	}
	c.applyHIDReport(report, stamp)
}

func (c *capture) applyHIDReport(report []byte, stamp domain.Timestamp) {
	data, err := c.readHIDReport(report)
	if err != nil {
		c.fail(err)
		return
	}
	state := hidReportState{
		reportID: report[noValue],
		stamp:    stamp,
		pressed:  make(map[domain.ControlID]bool),
	}
	c.processHIDData(data, state)
	c.updateButtons(state)
}

func (c *capture) readHIDReport(report []byte) ([]hid.HIDP_DATA, error) {
	data := make([]hid.HIDP_DATA, c.hid.maxData)
	count := uint32(len(data))
	pp := hid.PHIDP_PREPARSED_DATA(uintptr(unsafe.Pointer(&c.hid.preparsed[noValue])))
	status := hid.HidP_GetData(
		hid.HidP_Input,
		&data[noValue],
		&count,
		pp,
		&report[noValue],
		uint32(len(report)),
	)
	runtime.KeepAlive(c.hid.preparsed)
	runtime.KeepAlive(report)
	if status != hid.HIDP_STATUS_SUCCESS || count > uint32(len(data)) {
		return nil, errors.Join(domain.ErrEventLoss, hidError("HidP_GetData", status))
	}
	return data[:count], nil
}

func (c *capture) processHIDData(data []hid.HIDP_DATA, state hidReportState) {
	for _, item := range data {
		ctrl, found := c.hid.controls[hidIndex{state.reportID, item.DataIndex}]
		if !found || ctrl.control.Support != domain.SupportSupported {
			continue
		}
		c.processHIDControl(ctrl, item.Anonymous.Data[noValue], state)
	}
}

func (c *capture) processHIDControl(ctrl hidControl, word uint32, state hidReportState) {
	if ctrl.button {
		if uint8(word) != noValue {
			state.pressed[ctrl.control.ID] = true
		}
		return
	}
	value, valid := ctrl.reportValue(word)
	if valid {
		c.emitHIDValue(ctrl.control, value, state.stamp)
	}
}

func (ctrl hidControl) reportValue(word uint32) (int64, bool) {
	value := logicalValue(word, ctrl.native)
	bounds := logicalBounds(ctrl.native.Bounds.LogicalMin, ctrl.native.Bounds.LogicalMax)
	if ctrl.hat {
		direction, ok := domain.Hat(value, bounds, ctrl.native.HasNull)
		return int64(direction), ok
	}
	return value, !ctrl.native.HasNull || (value >= bounds.Min && value <= bounds.Max)
}

func (c *capture) emitHIDValue(control domain.Control, value int64, stamp domain.Timestamp) {
	previous, exists := c.values[control.ID]
	if control.Mode != domain.AxisRelative && exists && previous == value {
		return
	}
	c.values[control.ID] = value
	c.emit(domain.Event{
		ControlID: control.ID, Action: hidValueAction(control.Kind, value),
		Value: float64(value), Timestamp: stamp,
	})
}

func hidValueAction(kind domain.ControlKind, value int64) domain.EventAction {
	if kind != domain.ControlSwitch {
		return domain.ActionChange
	}
	if value != noValue {
		return domain.ActionPress
	}
	return domain.ActionRelease
}

func (c *capture) updateButtons(state hidReportState) {
	previous := c.buttons[state.reportID]
	c.emitButtonChanges(state.pressed, previous, domain.Event{
		Action: domain.ActionPress,
		Value:  singleValue, Timestamp: state.stamp,
	})
	c.emitButtonChanges(previous, state.pressed, domain.Event{
		Action: domain.ActionRelease,
		Value:  noValue, Timestamp: state.stamp,
	})
	c.buttons[state.reportID] = state.pressed
}

func (c *capture) emitButtonChanges(
	current, previous map[domain.ControlID]bool,
	event domain.Event,
) {
	for id := range current {
		if !previous[id] {
			event.ControlID = id
			c.emit(event)
		}
	}
}

func logicalValue(word uint32, control ext.NativeControl) int64 {
	bits := control.BitSize
	if bits == noValue || bits > thirtySecondValue {
		return noValue
	}
	mask := uint64(singleValue)<<bits - singleValue
	value := uint64(word) & mask
	if control.Bounds.LogicalMin < noValue &&
		value&(uint64(singleValue)<<(bits-singleValue)) != noValue {
		return int64(value) - int64(uint64(singleValue)<<bits)
	}
	return int64(value)
}

// Factory returns the native backend constructor.
func Factory() ports.Factory { return newBackend }

func resultError[Value any](_ Value, err error) error { return errors.Join(err) }

func classifyValue(ctrl *domain.Control, value hid.HIDP_VALUE_CAPS, bounds domain.Range) {
	if value.IsAbsolute != noValue {
		ctrl.Mode, ctrl.Range = domain.AxisAbsolute, &bounds
	} else {
		ctrl.Mode, ctrl.Unit = domain.AxisRelative, domain.UnitCounts
	}
	if unsupportedValue(value) {
		ctrl.Support = domain.SupportUnsupported
	}
}

func unsupportedValue(value hid.HIDP_VALUE_CAPS) bool {
	return value.BitSize == noValue || value.BitSize > thirtySecondValue ||
		(value.IsRange == noValue && value.ReportCount > singleValue)
}

func isConventionalHat(value hid.HIDP_VALUE_CAPS, usage uint32, bounds domain.Range) bool {
	return value.UsagePage == singleValue && usage == hatUsage && value.IsAbsolute != noValue &&
		conventionalHat(value, bounds.Min, bounds.Max)
}

func classifyHat(ctrl *domain.Control, value hid.HIDP_VALUE_CAPS, usage uint32) {
	if isConventionalHat(value, usage, logicalBounds(value.LogicalMin, value.LogicalMax)) {
		ctrl.Kind, ctrl.Unit, ctrl.Range = domain.ControlHat, domain.UnitDirection,
			&domain.Range{Min: int64(domain.HatNeutral), Max: int64(domain.HatNorthWest)}
		return
	}
	if value.IsAbsolute != noValue && ctrl.Range.Min == noValue && ctrl.Range.Max == singleValue {
		ctrl.Kind, ctrl.Unit = domain.ControlSwitch, domain.UnitBoolean
	}
}

func cancelPendingCommand(cmd *command) error {
	if cmd.state.CompareAndSwap(noValue, secondValue) {
		return context.Cause(cmd.ctx)
	}
	return nil
}
