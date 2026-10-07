//go:build windows && (amd64 || arm64)

package win32

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"sort"
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
	"github.com/gostafa/goinput/internal/platform"
	"github.com/gostafa/goinput/internal/ports"
)

func init() { platform.Register(newBackend) }

func newBackend(ctx context.Context, retrier ports.Retrier) (ports.Backend, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	b := &backend{
		commands: make(chan *command, 64), ready: make(chan error, 1), done: make(chan struct{}),
		retrier: retrier, captures: make(map[foundation.HANDLE]map[*capture]struct{}),
		registrations: make(map[topLevel]int),
	}
	go b.run()
	if err := <-b.ready; err != nil {
		<-b.done
		return nil, err
	}
	if err := context.Cause(ctx); err != nil {
		_ = b.Close()
		return nil, err
	}
	return b, nil
}

func (b *backend) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var runErr error
	defer func() { b.finish(runErr) }()
	for _, p := range []*native.Proc{wm.Procs.MsgWaitForMultipleObjectsEx, wm.Procs.CreateWindowEx, input.Procs.GetRawInputData, input.Procs.RegisterRawInputDevices} {
		if err := p.Find(); err != nil {
			runErr = errors.Join(domain.ErrUnsupported, err)
			b.ready <- runErr
			return
		}
	}
	callbackOnce.Do(func() { callbackAddr = syscall.NewCallback(windowProc) })
	event, err := threading.CreateEvent(nil, false, false, nil)
	if err != nil {
		runErr = normalizeError(err)
		b.ready <- runErr
		return
	}
	b.wakeEvent = event
	instance, err := libraryloader.GetModuleHandle(nil)
	if err != nil {
		runErr = normalizeError(err)
		b.ready <- runErr
		return
	}
	b.className = fmt.Sprintf("goinput.rawinput.%d", classNumber.Add(1))
	class := wm.WNDCLASSW{
		LpfnWndProc:   wm.WNDPROC(callbackAddr),
		HInstance:     foundation.HINSTANCE(instance),
		LpszClassName: native.UTF16Ptr(b.className),
	}
	atom, err := wm.RegisterClass(&class)
	if atom == 0 {
		// The generated wrapper checks GetLastError even on success; the atom
		// is the native success sentinel and stale errors must be ignored.
		runErr = normalizeError(nonzeroError(err))
		b.className = ""
		b.ready <- runErr
		return
	}
	window, err := wm.CreateWindowEx(
		0,
		&b.className,
		nil,
		0,
		0,
		0,
		0,
		0,
		wm.HWND_MESSAGE,
		0,
		foundation.HINSTANCE(instance),
		nil,
	)
	if err != nil {
		runErr = normalizeError(err)
		b.ready <- runErr
		return
	}
	b.mu.Lock()
	b.hwnd = window
	b.mu.Unlock()
	windows.Store(window, b)
	b.ready <- nil
	handles := []foundation.HANDLE{event}
	for !b.stopping {
		// A kernel event wakes Close even when message posting fails. PeekMessage
		// also avoids the generated GetMessage wrapper's incorrect tri-state API.
		ret, _, errno := syscall.SyscallN(
			wm.Procs.MsgWaitForMultipleObjectsEx.Addr(),
			1,
			uintptr(unsafe.Pointer(&handles[0])),
			0xffffffff,
			0x04ff,
			0x04,
		)
		if uint32(ret) == maxUint32 {
			runErr = normalizeError(native.LastError(errno))
			return
		}
		if ret == 0 {
			b.drainCommands()
		}
		var msg wm.MSG
		for !b.stopping && wm.PeekMessage(&msg, 0, 0, 0, wm.PEEK_MESSAGE_REMOVE_TYPE(1)) {
			if msg.Message == 0x0012 { // WM_QUIT
				return
			}
			wm.DispatchMessage(&msg)
			b.drainCommands() // high-frequency input must not starve Close/Open
		}
	}
}

func windowProc(
	hwnd foundation.HWND,
	message uint32,
	wParam foundation.WPARAM,
	lParam foundation.LPARAM,
) foundation.LRESULT {
	if value, ok := windows.Load(hwnd); ok {
		b := value.(*backend)
		switch message {
		case messageWake:
			b.drainCommands()
			return 0
		case messageInput:
			b.readInput(input.HRAWINPUT(lParam))
			// Foreground WM_INPUT requires the default procedure for native cleanup.
			if uint32(wParam)&0xff == 0 {
				return wm.DefWindowProc(hwnd, message, wParam, lParam)
			}
			return 0
		case messageDeviceChange:
			if uint32(wParam) == deviceRemoved {
				b.disconnect(foundation.HANDLE(lParam))
			}
			return 0
		}
	}
	return wm.DefWindowProc(hwnd, message, wParam, lParam)
}

func (b *backend) drainCommands() {
	for {
		select {
		case cmd := <-b.commands:
			var err error
			if !cmd.state.CompareAndSwap(0, 1) {
				err = context.Canceled
			} else if b.stopping {
				err = domain.ErrClosed
			} else if err = context.Cause(cmd.ctx); err == nil {
				err = cmd.fn()
			}
			cmd.reply <- err
		default:
			return
		}
	}
}

func (b *backend) call(ctx context.Context, fn func() error) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	b.mu.Lock()
	closed := b.closed
	b.mu.Unlock()
	if closed {
		return domain.ErrClosed
	}
	cmd := &command{ctx: ctx, fn: fn, reply: make(chan error, 1)}
	select {
	case b.commands <- cmd:
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-b.done:
		return domain.ErrClosed
	}
	b.mu.Lock()
	var signalErr error
	if b.closed {
		signalErr = domain.ErrClosed
	} else {
		// Holding mu prevents teardown from closing/reusing this HANDLE before
		// SetEvent returns.
		signalErr = threading.SetEvent(b.wakeEvent)
	}
	b.mu.Unlock()
	if signalErr != nil {
		// Do not abandon resources acquired by a command already running.
		if cmd.state.CompareAndSwap(0, 2) {
			select {
			case <-b.done:
				return domain.ErrClosed
			default:
				return normalizeError(signalErr)
			}
		}
	}
	ctxDone := ctx.Done()
	for {
		select {
		case err := <-cmd.reply:
			return err
		case <-b.done:
			return domain.ErrClosed
		case <-ctxDone:
			if cmd.state.CompareAndSwap(0, 2) {
				return context.Cause(ctx)
			}
			// Native work has started. Wait for its result so Open can release
			// resources before reporting cancellation to its caller.
			ctxDone = nil
		}
	}
}

func (b *backend) finish(cause error) {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	runErr := cause
	if cause == nil {
		cause = domain.ErrClosed
	}
	for _, group := range b.captures {
		for c := range group {
			if !c.closed.Swap(true) {
				c.sink.Fail(cause)
			}
		}
	}
	var cleanupErr error
	for usage := range b.registrations {
		cleanupErr = errors.Join(cleanupErr, b.removeRegistration(usage))
	}
	if b.hwnd != 0 {
		windows.Delete(b.hwnd)
		cleanupErr = errors.Join(cleanupErr, wm.DestroyWindow(b.hwnd))
	}
	if b.className != "" {
		instance, _ := libraryloader.GetModuleHandle(nil)
		cleanupErr = errors.Join(
			cleanupErr,
			wm.UnregisterClass(b.className, foundation.HINSTANCE(instance)),
		)
	}
	b.mu.Lock()
	if b.wakeEvent != 0 {
		cleanupErr = errors.Join(cleanupErr, foundation.CloseHandle(b.wakeEvent))
		b.wakeEvent = 0
	}
	b.closeErr = normalizeError(errors.Join(runErr, cleanupErr))
	b.mu.Unlock()
	close(b.done)
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
	devices, err := b.discover(ctx)
	infos := make([]domain.DeviceInfo, 0, len(devices))
	for _, d := range devices {
		infos = append(infos, domain.CloneInfo(d.info))
	}
	return infos, err
}

func (b *backend) discover(ctx context.Context) ([]nativeDevice, error) {
	b.mu.Lock()
	closed := b.closed
	b.mu.Unlock()
	if closed {
		return nil, domain.ErrClosed
	}
	var list []input.RAWINPUTDEVICELIST
	op := func(ctx context.Context) error {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		var count uint32
		size := uint32(unsafe.Sizeof(input.RAWINPUTDEVICELIST{}))
		if _, err := rawDeviceList(nil, &count, size); err != nil {
			return err
		}
		if count > maxDevices {
			return domain.ErrInvalidOptions
		}
		if count == 0 {
			list = nil
			return nil
		}
		list = make([]input.RAWINPUTDEVICELIST, count)
		n, err := rawDeviceList(&list[0], &count, size)
		if err != nil {
			return err
		}
		if uint64(n) > uint64(len(list)) {
			return domain.ErrInvalidOptions
		}
		list = list[:n]
		return nil
	}
	var err error
	if b.retrier == nil {
		err = op(ctx)
	} else {
		err = b.retrier.Do(ctx, op, transient)
	}
	if err != nil {
		return nil, normalizeError(err)
	}
	// Retry enumeration and each metadata snapshot independently. Wrapping both
	// in one retry would restart successful queries and multiply attempt limits.
	devices := make([]nativeDevice, 0, len(list))
	var diagnostics error
	for _, item := range list {
		if err := context.Cause(ctx); err != nil {
			return devices, errors.Join(diagnostics, err)
		}
		var d nativeDevice
		describe := func(ctx context.Context) error {
			var err error
			d, err = describeDevice(ctx, item.HDevice, uint32(item.DwType))
			return err
		}
		if b.retrier == nil {
			err = describe(ctx)
		} else {
			err = b.retrier.Do(ctx, describe, transient)
		}
		if err != nil {
			diagnostics = errors.Join(diagnostics, normalizeError(err))
			continue
		}
		devices = append(devices, d)
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].info.ID < devices[j].info.ID })
	return devices, errors.Join(diagnostics, context.Cause(ctx))
}

func (b *backend) Open(
	ctx context.Context,
	id domain.DeviceID,
	sink ports.EventSink,
) (ports.Capture, error) {
	discovery, cancel := context.WithTimeout(ctx, ports.DiscoveryBudget)
	devices, err := b.discover(discovery)
	cancel()
	if err != nil && (len(devices) == 0 || context.Cause(ctx) != nil) {
		return nil, err
	}
	var d nativeDevice
	found := false
	for _, item := range devices {
		if item.info.ID == id {
			d, found = item, true
			break
		}
	}
	if !found {
		return nil, errors.Join(domain.ErrNotFound, err)
	}
	c := &capture{
		backend: b,
		device:  d,
		info:    domain.CloneInfo(d.info),
		sink:    sink,
		held: make(
			map[domain.ControlID]bool,
		),
		values:  make(map[domain.ControlID]int64),
		buttons: make(map[byte]map[domain.ControlID]bool),
		native: ext.Info{
			RawInputHandle: uintptr(d.handle),
			DeviceType:     d.kind,
			UsagePage:      d.tlc.page,
			Usage:          d.tlc.usage,
			Version:        d.version,
		},
	}
	switch d.kind {
	case deviceKeyboard:
		c.keyboardCapabilities()
	case deviceMouse:
		c.mouseCapabilities()
	case deviceHID:
		if err := c.hidCapabilities(ctx); err != nil {
			return nil, normalizeError(err)
		}
	default:
		return nil, domain.ErrUnsupported
	}
	if err := b.call(ctx, func() error {
		if err := b.acquireRegistration(d.tlc); err != nil {
			return err
		}
		if b.captures[d.handle] == nil {
			b.captures[d.handle] = make(map[*capture]struct{})
		}
		b.captures[d.handle][c] = struct{}{}
		return nil
	}); err != nil {
		return nil, normalizeError(err)
	}
	if err := context.Cause(ctx); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

func (c *capture) Info() domain.DeviceInfo           { return domain.CloneInfo(c.info) }
func (c *capture) Capabilities() domain.Capabilities { return domain.CloneCapabilities(c.caps) }

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
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		c.closeErr = c.backend.call(context.Background(), func() error {
			group := c.backend.captures[c.device.handle]
			if _, exists := group[c]; !exists {
				return nil
			}
			delete(group, c)
			if len(group) == 0 {
				delete(c.backend.captures, c.device.handle)
			}
			return c.backend.releaseRegistration(c.device.tlc)
		})
		if errors.Is(c.closeErr, domain.ErrClosed) {
			c.closeErr = nil
		}
	})
	return normalizeError(c.closeErr)
}

func (b *backend) acquireRegistration(usage topLevel) error {
	if usage.page == 0 || usage.usage == 0 {
		return domain.ErrUnsupported
	}
	list, err := registeredDevices()
	if err != nil {
		return err
	}
	for _, r := range list {
		pageOnly := uint32(r.DwFlags)&0x30 == 0x20
		if r.UsUsagePage == usage.page && (r.UsUsage == usage.usage || pageOnly) &&
			r.HwndTarget != b.hwnd {
			return domain.ErrRegistrationConflict
		}
	}
	if b.registrations[usage] != 0 {
		b.registrations[usage]++
		return nil
	}
	err = input.RegisterRawInputDevices(
		[]input.RAWINPUTDEVICE{
			{
				UsUsagePage: usage.page,
				UsUsage:     usage.usage,
				DwFlags:     input.RAWINPUTDEVICE_FLAGS(registrationFlags),
				HwndTarget:  b.hwnd,
			},
		},
		uint32(unsafe.Sizeof(input.RAWINPUTDEVICE{})),
	)
	if err != nil {
		return err
	}
	b.registrations[usage] = 1
	return nil
}

func (b *backend) releaseRegistration(usage topLevel) error {
	if b.registrations[usage] > 1 {
		b.registrations[usage]--
		return nil
	}
	if b.registrations[usage] == 0 {
		return nil
	}
	b.registrations[usage] = 0
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
	for _, r := range list {
		if r.UsUsagePage == usage.page && r.UsUsage == usage.usage {
			if r.HwndTarget != b.hwnd {
				return domain.ErrRegistrationConflict // never remove another window's registration
			}
			return input.RegisterRawInputDevices(
				[]input.RAWINPUTDEVICE{
					{
						UsUsagePage: usage.page,
						UsUsage:     usage.usage,
						DwFlags:     input.RAWINPUTDEVICE_FLAGS(registrationRemove),
					},
				},
				uint32(unsafe.Sizeof(input.RAWINPUTDEVICE{})),
			)
		}
	}
	return nil
}

func registeredDevices() ([]input.RAWINPUTDEVICE, error) {
	var count uint32
	size := uint32(unsafe.Sizeof(input.RAWINPUTDEVICE{}))
	n, err := input.GetRegisteredRawInputDevices(nil, &count, size)
	if n == maxUint32 {
		return nil, nonzeroError(err)
	}
	if count == 0 {
		return nil, nil
	}
	if count > maxDevices {
		return nil, domain.ErrInvalidOptions
	}
	list := make([]input.RAWINPUTDEVICE, count)
	n, err = input.GetRegisteredRawInputDevices(&list[0], &count, size)
	if n == maxUint32 {
		return nil, nonzeroError(err)
	}
	if uint64(n) > uint64(len(list)) {
		return nil, domain.ErrInvalidOptions
	}
	return list[:n], nil // ignore stale GetLastError on successful UINT results
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
	if n == maxUint32 {
		return n, nonzeroError(err)
	}
	return n, nil
}

func rawDeviceInfo(
	handle foundation.HANDLE,
	command input.RAW_INPUT_DEVICE_INFO_COMMAND,
	data unsafe.Pointer,
	size *uint32,
) (uint32, error) {
	n, err := input.GetRawInputDeviceInfo(handle, command, data, size)
	if n == maxUint32 {
		return n, nonzeroError(err)
	}
	return n, nil
}

func describeDevice(
	ctx context.Context,
	handle foundation.HANDLE,
	kind uint32,
) (nativeDevice, error) {
	d := nativeDevice{handle: handle, kind: kind}
	var info input.RID_DEVICE_INFO
	info.CbSize = uint32(unsafe.Sizeof(info))
	size := info.CbSize
	if _, err := rawDeviceInfo(
		handle,
		input.RAW_INPUT_DEVICE_INFO_COMMAND(0x2000000b),
		unsafe.Pointer(&info),
		&size,
	); err != nil {
		return d, err
	}
	words := info.Anonymous.Data
	switch kind {
	case deviceMouse:
		d.tlc = topLevel{1, 2}
		d.buttons, d.hwheel = words[1], words[3] != 0
		d.info.Classes = []domain.DeviceClass{domain.ClassMouse}
	case deviceKeyboard:
		d.tlc = topLevel{1, 6}
		d.info.Classes = []domain.DeviceClass{domain.ClassKeyboard}
	case deviceHID:
		d.tlc = topLevel{uint16(words[3]), uint16(words[3] >> 16)}
		d.version = words[2]
		if words[0] <= 0xffff {
			id := uint16(words[0])
			d.info.VendorID = &id
		}
		if words[1] <= 0xffff {
			id := uint16(words[1])
			d.info.ProductID = &id
		}
		d.info.Classes = []domain.DeviceClass{classFor(d.tlc)}
	default:
		return d, domain.ErrUnsupported
	}
	var chars uint32
	if _, err := rawDeviceInfo(
		handle,
		input.RAW_INPUT_DEVICE_INFO_COMMAND(0x20000007),
		nil,
		&chars,
	); err != nil {
		return d, err
	}
	if chars == 0 || chars > maxNativeBuffer/2 {
		return d, domain.ErrNotFound
	}
	path := make([]uint16, chars+1)
	if _, err := rawDeviceInfo(
		handle,
		input.RAW_INPUT_DEVICE_INFO_COMMAND(0x20000007),
		unsafe.Pointer(&path[0]),
		&chars,
	); err != nil {
		return d, err
	}
	d.info.Path = syscall.UTF16ToString(path)
	if d.info.Path == "" {
		return d, domain.ErrNotFound
	}
	d.info.ID = domain.DeviceID("win32:" + strings.ToLower(d.info.Path))
	if err := context.Cause(ctx); err != nil {
		return d, err
	}
	enrichIdentity(&d)
	return d, nil
}

func enrichIdentity(d *nativeDevice) {
	for _, proc := range []*native.Proc{hid.Procs.HidD_GetAttributes, hid.Procs.HidD_GetProductString, hid.Procs.HidD_GetManufacturerString, hid.Procs.HidD_GetSerialNumberString} {
		if proc.Find() != nil {
			return // optional HID metadata can be unavailable
		}
	}
	// DesiredAccess=0 avoids stealing keyboard/mouse read access from the OS.
	handle, err := filesystem.CreateFile(
		d.info.Path,
		0,
		filesystem.FILE_SHARE_MODE(3),
		nil,
		filesystem.FILE_CREATION_DISPOSITION(3),
		0,
		0,
	)
	if err != nil {
		return // optional product/manufacturer/serial metadata
	}
	defer foundation.CloseHandle(handle)
	var attributes hid.HIDD_ATTRIBUTES
	attributes.Size = uint32(unsafe.Sizeof(attributes))
	if hid.HidD_GetAttributes(handle, &attributes) != 0 {
		vendor, product := attributes.VendorID, attributes.ProductID
		d.info.VendorID, d.info.ProductID = &vendor, &product
		d.version = uint32(attributes.VersionNumber)
	}
	readString := func(get func(foundation.HANDLE, []byte) foundation.BOOLEAN) string {
		data := make([]byte, 512)
		if get(handle, data) == 0 {
			return ""
		}
		wide := make([]uint16, len(data)/2)
		for i := range wide {
			wide[i] = binary.LittleEndian.Uint16(data[i*2:])
		}
		return syscall.UTF16ToString(wide)
	}
	if name := readString(hid.HidD_GetProductString); name != "" {
		d.info.Name = name
	}
	d.info.Manufacturer = readString(hid.HidD_GetManufacturerString)
	d.info.Serial = readString(hid.HidD_GetSerialNumberString)
}

func classFor(tlc topLevel) domain.DeviceClass {
	if tlc.page == 1 {
		switch tlc.usage {
		case 2:
			return domain.ClassMouse
		case 4:
			return domain.ClassJoystick
		case 5:
			return domain.ClassGamepad
		case 6, 7:
			return domain.ClassKeyboard
		}
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
	return errors.Is(err, syscall.Errno(122)) || errors.Is(err, syscall.Errno(234)) ||
		errors.Is(err, syscall.Errno(21))
}

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	var category error
	switch {
	case errors.Is(err, syscall.Errno(5)), errors.Is(err, syscall.Errno(32)):
		category = domain.ErrPermissionDenied
	case errors.Is(err, syscall.Errno(2)),
		errors.Is(err, syscall.Errno(3)),
		errors.Is(err, syscall.Errno(6)):
		category = domain.ErrNotFound
	case errors.Is(err, syscall.Errno(1167)):
		category = domain.ErrDisconnected
	case errors.Is(err, syscall.Errno(87)):
		category = domain.ErrInvalidOptions
	}
	if category != nil {
		return errors.Join(category, err)
	}
	return err
}

func (c *capture) keyboardCapabilities() {
	controls := make(map[domain.ControlID]domain.Control)
	add := func(scan uint16, usage domain.Usage) {
		id := keyID(usage, scan)
		controls[id] = domain.Control{
			ID:      id,
			Name:    fmt.Sprintf("Key %04x:%04x", uint16(usage>>16), uint16(usage)),
			Kind:    domain.ControlKey,
			Usage:   usage,
			Mapping: domain.MappingInferred,
			Range:   &domain.Range{Min: 0, Max: 1},
			Unit:    domain.UnitBoolean,
			Support: domain.SupportUnknown,
		}
		control := ext.NativeControl{ID: string(id), ScanCode: scan}
		if scan&0xff00 == 0xff00 {
			control.ScanCode, control.VirtualKey = 0, scan&0xff
		}
		c.native.Controls = append(c.native.Controls, control)
	}
	for scan, usage := range scanUsages {
		add(scan, domain.HID(7, usage))
	}
	for scan, usage := range consumerScans {
		add(scan, domain.HID(0x0c, usage))
	}
	for vkey, usage := range consumerVirtualKeys {
		add(0xff00|vkey, domain.HID(0x0c, usage))
	}
	add(0xe05e, domain.HID(1, 0x81))
	add(0xe05f, domain.HID(1, 0x82))
	add(0xe063, domain.HID(1, 0x83))
	for _, control := range controls {
		c.caps.Controls = append(c.caps.Controls, control)
	}
	c.caps.Repeat = domain.SupportSupported
	sort.Slice(
		c.caps.Controls,
		func(i, j int) bool { return c.caps.Controls[i].ID < c.caps.Controls[j].ID },
	)
	sort.Slice(c.native.Controls, func(i, j int) bool {
		if c.native.Controls[i].ID == c.native.Controls[j].ID {
			return c.native.Controls[i].ScanCode < c.native.Controls[j].ScanCode
		}
		return c.native.Controls[i].ID < c.native.Controls[j].ID
	})
}

func keyID(usage domain.Usage, scan uint16) domain.ControlID {
	if scan&0xff00 == 0xff00 {
		return domain.ControlID(fmt.Sprintf("key:virtual:%04x", scan&0xff))
	}
	if scan != 0 {
		return domain.ControlID(fmt.Sprintf("key:scan:%04x", scan))
	}
	if usage != 0 {
		return domain.ControlID(fmt.Sprintf("key:%08x", uint32(usage)))
	}
	return domain.ControlID(fmt.Sprintf("key:native:%04x", scan))
}

func (c *capture) mouseCapabilities() {
	c.caps.Repeat = domain.SupportUnsupported
	for i := uint16(1); i <= 5; i++ {
		support := domain.SupportUnknown
		if uint32(i) <= c.device.buttons {
			support = domain.SupportSupported
		}
		c.caps.Controls = append(c.caps.Controls, domain.Control{
			ID:      domain.ControlID(fmt.Sprintf("button:%d", i)),
			Name:    fmt.Sprintf("Button %d", i),
			Kind:    domain.ControlButton,
			Usage:   domain.HID(9, i),
			Mapping: domain.MappingInferred,
			Range:   &domain.Range{Min: 0, Max: 1},
			Unit:    domain.UnitBoolean,
			Support: support,
		})
	}
	for i, axis := range []string{"x", "y"} {
		c.caps.Controls = append(c.caps.Controls,
			domain.Control{
				ID: domain.ControlID(
					"rel:" + axis,
				),
				Name:    "Relative " + strings.ToUpper(axis),
				Kind:    domain.ControlAxis,
				Usage:   domain.HID(1, uint16(0x30+i)),
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
				Usage:   domain.HID(1, uint16(0x30+i)),
				Mapping: domain.MappingInferred,
				Mode:    domain.AxisAbsolute,
				Range:   &domain.Range{Min: 0, Max: 65535},
				Unit:    domain.UnitLogical,
				Support: domain.SupportUnknown,
			})
	}
	c.caps.Controls = append(
		c.caps.Controls,
		domain.Control{
			ID:      "wheel",
			Name:    "Wheel",
			Kind:    domain.ControlAxis,
			Usage:   domain.HID(1, 0x38),
			Mapping: domain.MappingInferred,
			Mode:    domain.AxisRelative,
			Unit:    domain.UnitDetents,
			Support: domain.SupportUnknown,
		},
		domain.Control{
			ID:      "pan",
			Name:    "Horizontal wheel",
			Kind:    domain.ControlAxis,
			Usage:   domain.HID(0x0c, 0x238),
			Mapping: domain.MappingInferred,
			Mode:    domain.AxisRelative,
			Unit:    domain.UnitDetents,
			Support: domain.SupportUnknown,
		},
	)
	if c.device.hwheel {
		c.caps.Controls[len(c.caps.Controls)-1].Support = domain.SupportSupported
	}
}

func (c *capture) hidCapabilities(ctx context.Context) error {
	return c.retryMetadata(ctx, func(ctx context.Context) error {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		for _, proc := range []*native.Proc{hid.Procs.HidP_GetCaps, hid.Procs.HidP_GetButtonCaps, hid.Procs.HidP_GetValueCaps, hid.Procs.HidP_GetData, hid.Procs.HidP_MaxDataListLength} {
			if err := proc.Find(); err != nil {
				return errors.Join(domain.ErrUnsupported, err)
			}
		}
		var size uint32
		command := input.RAW_INPUT_DEVICE_INFO_COMMAND(0x20000005)
		if _, err := rawDeviceInfo(c.device.handle, command, nil, &size); err != nil {
			return err
		}
		if size == 0 || size > maxNativeBuffer {
			return domain.ErrUnsupported
		}
		data := make([]byte, size)
		if _, err := rawDeviceInfo(
			c.device.handle,
			command,
			unsafe.Pointer(&data[0]),
			&size,
		); err != nil {
			return err
		}
		if size > uint32(len(data)) {
			return domain.ErrEventLoss
		}
		data = data[:size]
		if len(data) == 0 {
			return domain.ErrUnsupported
		}
		pp := hid.PHIDP_PREPARSED_DATA(uintptr(unsafe.Pointer(&data[0])))
		defer runtime.KeepAlive(data)
		var caps hid.HIDP_CAPS
		if status := hid.HidP_GetCaps(pp, &caps); status != hid.HIDP_STATUS_SUCCESS {
			return hidError("HidP_GetCaps", status)
		}
		desc := &descriptor{
			preparsed: data,
			reportLen: caps.InputReportByteLength,
			controls:  make(map[hidIndex]hidControl),
			reportIDs: make(map[byte]bool),
		}
		desc.maxData = hid.HidP_MaxDataListLength(hid.HidP_Input, pp)
		if desc.maxData > maxDevices || desc.reportLen == 0 {
			return domain.ErrUnsupported
		}
		controls := make([]domain.Control, 0)
		nativeControls := make([]ext.NativeControl, 0)
		add := func(control hidControl) {
			key := hidIndex{control.native.ReportID, control.native.DataIndex}
			if _, exists := desc.controls[key]; exists {
				return
			}
			desc.controls[key] = control
			desc.reportIDs[key.report] = true
			controls = append(controls, control.control)
			nativeControls = append(nativeControls, control.native)
		}
		if caps.NumberInputButtonCaps != 0 {
			count := caps.NumberInputButtonCaps
			buttons := make([]hid.HIDP_BUTTON_CAPS, count)
			if status := hid.HidP_GetButtonCaps(
				hid.HidP_Input,
				&buttons[0],
				&count,
				pp,
			); status != hid.HIDP_STATUS_SUCCESS {
				return hidError("HidP_GetButtonCaps", status)
			}
			if int(count) > len(buttons) {
				return domain.ErrEventLoss
			}
			for _, button := range buttons[:count] {
				if button.IsAlias != 0 {
					continue
				}
				lo, hi, index, end, err := capRange(button.Anonymous.Data, button.IsRange != 0)
				if err != nil {
					return err
				}
				for usage, dataIndex := lo, index; usage <= hi && dataIndex <= end; usage, dataIndex = usage+1, dataIndex+1 {
					id := hidID(button.ReportID, button.LinkCollection, uint16(dataIndex))
					kind := domain.ControlButton
					if button.UsagePage == 7 || button.UsagePage == 0x0c {
						kind = domain.ControlKey
					}
					support := domain.SupportSupported
					if button.IsAbsolute == 0 {
						// Relative boolean deltas are not absolute button state.
						support = domain.SupportUnsupported
					}
					add(hidControl{
						button: true,
						control: domain.Control{
							ID:      id,
							Name:    fmt.Sprintf("HID %04x:%04x", button.UsagePage, usage),
							Kind:    kind,
							Usage:   domain.HID(button.UsagePage, uint16(usage)),
							Mapping: domain.MappingReported,
							Range:   &domain.Range{Min: 0, Max: 1},
							Unit:    domain.UnitBoolean,
							Support: support,
						},
						native: ext.NativeControl{
							ID:             string(id),
							ReportID:       button.ReportID,
							DataIndex:      uint16(dataIndex),
							LinkCollection: button.LinkCollection,
							ReportCount:    button.ReportCount,
							BitSize:        1,
							LogicalMin:     0,
							LogicalMax:     1,
							Absolute:       button.IsAbsolute != 0,
						},
					})
				}
			}
		}
		if caps.NumberInputValueCaps != 0 {
			count := caps.NumberInputValueCaps
			values := make([]hid.HIDP_VALUE_CAPS, count)
			if status := hid.HidP_GetValueCaps(
				hid.HidP_Input,
				&values[0],
				&count,
				pp,
			); status != hid.HIDP_STATUS_SUCCESS {
				return hidError("HidP_GetValueCaps", status)
			}
			if int(count) > len(values) {
				return domain.ErrEventLoss
			}
			for _, value := range values[:count] {
				if value.IsAlias != 0 {
					continue
				}
				lo, hi, index, end, err := capRange(value.Anonymous.Data, value.IsRange != 0)
				if err != nil {
					return err
				}
				minValue, maxValue := int64(value.LogicalMin), int64(value.LogicalMax)
				if value.LogicalMin >= 0 && value.LogicalMax < 0 {
					maxValue = int64(uint32(value.LogicalMax))
				}
				for usage, dataIndex := lo, index; usage <= hi && dataIndex <= end; usage, dataIndex = usage+1, dataIndex+1 {
					id := hidID(value.ReportID, value.LinkCollection, uint16(dataIndex))
					ctrl := domain.Control{
						ID:   id,
						Name: fmt.Sprintf("HID %04x:%04x", value.UsagePage, usage),
						Kind: domain.ControlAxis,
						Usage: domain.HID(
							value.UsagePage,
							uint16(usage),
						),
						Mapping: domain.MappingReported,
						Unit:    domain.UnitLogical,
						Support: domain.SupportSupported,
					}
					if value.IsAbsolute != 0 {
						ctrl.Mode, ctrl.Range = domain.AxisAbsolute, &domain.Range{
							Min: minValue,
							Max: maxValue,
						}
					} else {
						ctrl.Mode, ctrl.Unit = domain.AxisRelative, domain.UnitCounts
					}
					// A range with several distinct usages is not a usage-value array.
					if value.BitSize == 0 || value.BitSize > 32 ||
						(value.IsRange == 0 && value.ReportCount > 1) {
						ctrl.Support = domain.SupportUnsupported
					}
					normalHat := value.UsagePage == 1 && usage == 0x39 && value.IsAbsolute != 0 &&
						conventionalHat(value, minValue, maxValue)
					if normalHat {
						ctrl.Kind, ctrl.Unit, ctrl.Range = domain.ControlHat, domain.UnitDirection, &domain.Range{
							Min: -1,
							Max: 7,
						}
					} else if value.IsAbsolute != 0 && minValue == 0 && maxValue == 1 {
						ctrl.Kind, ctrl.Unit = domain.ControlSwitch, domain.UnitBoolean
					}
					add(hidControl{control: ctrl, hat: normalHat, native: ext.NativeControl{
						ID:             string(id),
						ReportID:       value.ReportID,
						DataIndex:      uint16(dataIndex),
						LinkCollection: value.LinkCollection,
						BitSize:        value.BitSize,
						ReportCount:    value.ReportCount,
						LogicalMin:     value.LogicalMin,
						LogicalMax:     value.LogicalMax,
						PhysicalMin:    value.PhysicalMin,
						PhysicalMax:    value.PhysicalMax,
						Units:          value.Units,
						UnitsExponent:  value.UnitsExp,
						HasNull:        value.HasNull != 0,
						Absolute:       value.IsAbsolute != 0,
					}})
				}
			}
		}
		sort.Slice(controls, func(i, j int) bool { return controls[i].ID < controls[j].ID })
		sort.Slice(
			nativeControls,
			func(i, j int) bool { return nativeControls[i].ID < nativeControls[j].ID },
		)
		c.hid, c.caps = desc, domain.Capabilities{
			Controls: controls,
			Complete: true,
			Repeat:   domain.SupportUnsupported,
		}
		c.native.Controls = nativeControls
		return nil
	})
}

func (c *capture) retryMetadata(ctx context.Context, op func(context.Context) error) error {
	if c.backend.retrier == nil {
		return op(ctx)
	}
	return c.backend.retrier.Do(ctx, op, transient)
}

func hidID(report byte, collection, index uint16) domain.ControlID {
	return domain.ControlID(fmt.Sprintf("hid:%02x:%04x:%04x", report, collection, index))
}

func capRange(words [8]uint16, isRange bool) (uint32, uint32, uint32, uint32, error) {
	lo, hi, index, end := uint32(words[0]), uint32(words[0]), uint32(words[6]), uint32(words[6])
	if isRange {
		hi, end = uint32(words[1]), uint32(words[7])
	}
	if hi < lo || end < index || hi-lo != end-index {
		return 0, 0, 0, 0, errors.Join(
			domain.ErrUnsupported,
			errors.New("invalid HID capability range"),
		)
	}
	return lo, hi, index, end, nil
}

func conventionalHat(value hid.HIDP_VALUE_CAPS, minValue, maxValue int64) bool {
	if _, ok := domain.Hat(minValue, minValue, maxValue, value.HasNull != 0); !ok {
		return false
	}
	if value.PhysicalMin != 0 || (value.Units != 0 && value.Units != 0x14) || value.UnitsExp != 0 {
		return false
	}
	positions := maxValue - minValue + 1
	return value.PhysicalMax == 0 || (positions == 4 && value.PhysicalMax == 270) ||
		(positions == 8 && value.PhysicalMax == 315)
}

func hidError(op string, status foundation.NTSTATUS) error {
	return fmt.Errorf("%s: %w (HID status 0x%08x)", op, domain.ErrUnsupported, uint32(status))
}

func (b *backend) readInput(handle input.HRAWINPUT) {
	var size uint32
	headerSize := uint32(unsafe.Sizeof(input.RAWINPUTHEADER{}))
	ret, _, errno := syscall.SyscallN(
		input.Procs.GetRawInputData.Addr(),
		uintptr(handle),
		0x10000003,
		0,
		uintptr(unsafe.Pointer(&size)),
		uintptr(headerSize),
	)
	if uint32(ret) == maxUint32 {
		b.failAll(errors.Join(domain.ErrEventLoss, native.LastError(errno)))
		return
	}
	if size < headerSize || size > maxNativeBuffer {
		b.failAll(domain.ErrEventLoss)
		return
	}
	data := make([]byte, size)
	ret, _, errno = syscall.SyscallN(
		input.Procs.GetRawInputData.Addr(),
		uintptr(handle),
		0x10000003,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(unsafe.Pointer(&size)),
		uintptr(headerSize),
	)
	if uint32(ret) == maxUint32 {
		b.failAll(errors.Join(domain.ErrEventLoss, native.LastError(errno)))
		return
	}
	if uint32(ret) < headerSize || uint64(ret) > uint64(len(data)) {
		b.failAll(domain.ErrEventLoss)
		return
	}
	data = data[:uint32(ret)]
	kind := binary.LittleEndian.Uint32(data)
	declared := binary.LittleEndian.Uint32(data[4:])
	if declared < headerSize || uint64(declared) > uint64(len(data)) {
		b.failAll(domain.ErrEventLoss)
		return
	}
	device := foundation.HANDLE(binary.LittleEndian.Uint64(data[8:]))
	group := b.captures[device]
	if len(group) == 0 {
		return
	}
	body := data[headerSize:declared]
	now := time.Now()
	stamp := domain.Timestamp{Time: now.UTC(), ReceivedAt: now, Source: domain.TimestampReceipt}
	for c := range group {
		if c.closed.Load() {
			continue
		}
		if kind != c.device.kind {
			c.fail(domain.ErrEventLoss)
			continue
		}
		switch kind {
		case deviceKeyboard:
			c.keyboard(body, stamp)
		case deviceMouse:
			c.mouse(body, stamp)
		case deviceHID:
			c.reports(body, stamp)
		default:
			c.fail(domain.ErrEventLoss)
		}
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

func (c *capture) emit(
	id domain.ControlID,
	action domain.EventAction,
	value float64,
	stamp domain.Timestamp,
) {
	if !c.closed.Load() &&
		!c.sink.Publish(
			domain.Event{
				DeviceID:  c.info.ID,
				ControlID: id,
				Action:    action,
				Value:     value,
				Timestamp: stamp,
			},
		) {
		c.closed.Store(true)
	}
}

func (c *capture) keyboard(data []byte, stamp domain.Timestamp) {
	if len(data) < 16 {
		c.fail(domain.ErrEventLoss)
		return
	}
	makeCode, flags, vkey := binary.LittleEndian.Uint16(
		data,
	), binary.LittleEndian.Uint16(
		data[2:],
	), binary.LittleEndian.Uint16(
		data[6:],
	)
	if makeCode == 0xff {
		c.fail(domain.ErrEventLoss)
		return
	}
	if vkey >= 0xff {
		return
	}
	scan := makeCode
	if flags&2 != 0 {
		scan |= 0xe000
	} else if flags&4 != 0 {
		scan |= 0xe100
	}
	if scan == 0xe02a || scan == 0xe036 || scan == 0xe11d {
		return // synthetic pieces of PrintScreen/Pause, not separate keys
	}
	var usage domain.Usage
	if code, ok := scanUsages[scan]; ok {
		usage = domain.HID(7, code)
	} else if code, ok := consumerScans[scan]; ok {
		usage = domain.HID(0x0c, code)
	} else if makeCode == 0 {
		scan = 0xff00 | vkey // keep virtual-key identity separate from usages
		if code, ok := consumerVirtualKeys[vkey]; ok {
			usage = domain.HID(0x0c, code)
		}
	} else {
		switch scan {
		case 0xe05e:
			usage = domain.HID(1, 0x81)
		case 0xe05f:
			usage = domain.HID(1, 0x82)
		case 0xe063:
			usage = domain.HID(1, 0x83)
		}
	}
	id := keyID(usage, scan)
	if flags&1 != 0 {
		delete(c.held, id)
		c.emit(id, domain.ActionRelease, 0, stamp)
	} else {
		action := domain.ActionPress
		if c.held[id] {
			action = domain.ActionRepeat
		}
		c.held[id] = true
		c.emit(id, action, 1, stamp)
	}
}

func (c *capture) mouse(data []byte, stamp domain.Timestamp) {
	if len(data) < 24 {
		c.fail(domain.ErrEventLoss)
		return
	}
	flags, buttons, wheel := binary.LittleEndian.Uint16(
		data,
	), binary.LittleEndian.Uint16(
		data[4:],
	), int16(
		binary.LittleEndian.Uint16(data[6:]),
	)
	for i := uint16(0); i < 5; i++ {
		id := domain.ControlID(fmt.Sprintf("button:%d", i+1))
		if buttons&(1<<(i*2)) != 0 {
			c.emit(id, domain.ActionPress, 1, stamp)
		}
		if buttons&(1<<(i*2+1)) != 0 {
			c.emit(id, domain.ActionRelease, 0, stamp)
		}
	}
	x, y := int64(
		int32(binary.LittleEndian.Uint32(data[12:])),
	), int64(
		int32(binary.LittleEndian.Uint32(data[16:])),
	)
	if flags&1 != 0 {
		for _, axis := range []struct {
			id    domain.ControlID
			value int64
		}{{"abs:x", x}, {"abs:y", y}} {
			if previous, exists := c.values[axis.id]; !exists || previous != axis.value {
				c.values[axis.id] = axis.value
				c.emit(axis.id, domain.ActionChange, float64(axis.value), stamp)
			}
		}
	} else {
		if x != 0 {
			c.emit("rel:x", domain.ActionChange, float64(x), stamp)
		}
		if y != 0 {
			c.emit("rel:y", domain.ActionChange, float64(y), stamp)
		}
	}
	if buttons&0x400 != 0 {
		c.emit("wheel", domain.ActionChange, float64(wheel)/120, stamp)
	}
	if buttons&0x800 != 0 {
		c.emit("pan", domain.ActionChange, float64(wheel)/120, stamp)
	}
}

func (c *capture) reports(body []byte, stamp domain.Timestamp) {
	if c.hid == nil || len(body) < 8 {
		c.fail(domain.ErrEventLoss)
		return
	}
	size, count := binary.LittleEndian.Uint32(body), binary.LittleEndian.Uint32(body[4:])
	if size == 0 || count > maxDevices || uint64(size)*uint64(count) > uint64(len(body)-8) {
		c.fail(domain.ErrEventLoss)
		return
	}
	for i := uint32(0); i < count && !c.closed.Load(); i++ {
		start := 8 + uint64(i)*uint64(size)
		c.report(body[start:start+uint64(size)], stamp)
	}
}

func (c *capture) report(report []byte, stamp domain.Timestamp) {
	if len(report) != int(c.hid.reportLen) || len(report) == 0 {
		c.fail(errors.Join(domain.ErrEventLoss, errors.New("unexpected HID report length")))
		return
	}
	reportID := report[0]
	if !c.hid.reportIDs[reportID] {
		return // report consists entirely of unsupported or aliased controls
	}
	data := make([]hid.HIDP_DATA, c.hid.maxData)
	if len(data) == 0 {
		return
	}
	count := uint32(len(data))
	pp := hid.PHIDP_PREPARSED_DATA(uintptr(unsafe.Pointer(&c.hid.preparsed[0])))
	status := hid.HidP_GetData(
		hid.HidP_Input,
		&data[0],
		&count,
		pp,
		&report[0],
		uint32(len(report)),
	)
	runtime.KeepAlive(c.hid.preparsed)
	runtime.KeepAlive(report)
	if status != hid.HIDP_STATUS_SUCCESS || count > uint32(len(data)) {
		c.fail(errors.Join(domain.ErrEventLoss, hidError("HidP_GetData", status)))
		return
	}
	pressed := make(map[domain.ControlID]bool)
	for _, item := range data[:count] {
		ctrl, found := c.hid.controls[hidIndex{reportID, item.DataIndex}]
		if !found || ctrl.control.Support != domain.SupportSupported {
			continue
		}
		word := item.Anonymous.Data[0]
		if ctrl.button {
			if uint8(word) != 0 {
				pressed[ctrl.control.ID] = true
			}
			continue
		}
		value := logicalValue(word, ctrl.native.BitSize, ctrl.native.LogicalMin < 0)
		minValue, maxValue := int64(ctrl.native.LogicalMin), int64(ctrl.native.LogicalMax)
		if ctrl.native.LogicalMin >= 0 && ctrl.native.LogicalMax < 0 {
			maxValue = int64(uint32(ctrl.native.LogicalMax))
		}
		if ctrl.hat {
			direction, ok := domain.Hat(value, minValue, maxValue, ctrl.native.HasNull)
			if !ok {
				continue
			}
			value = int64(direction)
		} else if ctrl.native.HasNull && (value < minValue || value > maxValue) {
			continue // null scalar values have no universal numeric position
		}
		previous, exists := c.values[ctrl.control.ID]
		if ctrl.control.Mode == domain.AxisRelative || !exists || previous != value {
			c.values[ctrl.control.ID] = value
			action := domain.ActionChange
			if ctrl.control.Kind == domain.ControlSwitch {
				action = domain.ActionRelease
				if value != 0 {
					action = domain.ActionPress
				}
			}
			c.emit(ctrl.control.ID, action, float64(value), stamp)
		}
	}
	previous := c.buttons[reportID]
	for id := range pressed {
		if !previous[id] {
			c.emit(id, domain.ActionPress, 1, stamp)
		}
	}
	for id := range previous {
		if !pressed[id] {
			c.emit(id, domain.ActionRelease, 0, stamp)
		}
	}
	c.buttons[reportID] = pressed
}

func logicalValue(word uint32, bits uint16, signed bool) int64 {
	if bits == 0 || bits > 32 {
		return 0
	}
	mask := uint64(1)<<bits - 1
	value := uint64(word) & mask
	if signed && value&(uint64(1)<<(bits-1)) != 0 {
		return int64(value) - int64(uint64(1)<<bits)
	}
	return int64(value)
}
