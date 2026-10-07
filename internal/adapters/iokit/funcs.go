//go:build darwin && (amd64 || arm64)

package iokit

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	extension "github.com/gostafa/goinput/extensions/iokit"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/platform"
	"github.com/gostafa/goinput/internal/ports"
	cf "github.com/tmc/apple/corefoundation"
	native "github.com/tmc/apple/iokit"
)

func init() { platform.Register(newBackend) }

func newBackend(ctx context.Context, retrier ports.Retrier) (ports.Backend, error) {
	err := ctx.Err()
	if err != nil {
		return nil, err
	}

	b := &backend{
		jobs:    make(chan request),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		ready:   make(chan error, 1),
		retrier: retrier,
	}

	go b.run()

	select {
	case err := <-b.ready:
		if err != nil {
			<-b.done

			return nil, err
		}

		if err := ctx.Err(); err != nil {
			_ = b.Close()

			return nil, err
		}

		return b, nil
	case <-ctx.Done():
		_ = b.Close()

		return nil, ctx.Err()
	}
}

func (b *backend) run() {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()
	defer close(b.done)

	s := &session{
		backend:  b,
		keys:     make(map[string]cf.CFStringRef),
		captures: make(map[*capture]struct{}),
	}

	defer func() {
		if p := recover(); p != nil {
			b.closeErr = fmt.Errorf("%w: IOHID session panic: %v", domain.ErrEventLoss, p)

			for c := range s.captures {
				c.sink.Fail(b.closeErr)
			}
		}

		_, cleanupErr := safeCall(func() (any, error) { return nil, s.close() })

		b.closeErr = errors.Join(b.closeErr, cleanupErr)
	}()

	_, err := safeCall(func() (any, error) { return nil, s.start() })
	b.ready <- err

	if err != nil {
		return
	}

	for {
		select {
		case <-b.stop:
			return
		case req := <-b.jobs:
			var result response

			if result.err = req.ctx.Err(); result.err == nil {
				result.value, result.err = safeCall(func() (any, error) { return req.perform(s) })
			}

			req.result <- result
		default:
			cf.CFRunLoopRunInMode(cf.CFRunLoopMode(s.mode), 0.02, true)
		}
	}
}

func safeCall(call func() (any, error)) (value any, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%w: IOHID binding: %v", domain.ErrUnsupported, p)
		}
	}()

	return call()
}

func (s *session) start() error {
	err := loadSymbols()
	if err != nil {
		return err
	}

	s.runLoop = cf.CFRunLoopGetCurrent()
	s.mode = cf.CFStringCreateWithCString(0, "goinput.IOHID", utf8Encoding)

	if s.runLoop == 0 || s.mode == 0 {
		return errors.New("IOHID: unable to create event loop")
	}

	s.manager = api.IOHIDManagerCreate(0, 0)
	if s.manager == 0 {
		return errors.New("IOHID: unable to create manager")
	}

	// Matching enumerates without opening every keyboard or requesting consent.
	api.IOHIDManagerSetDeviceMatching(s.manager, 0)
	api.IOHIDManagerScheduleWithRunLoop(s.manager, s.runLoop, s.mode)

	before := time.Now()

	s.anchorTicks = machAbsoluteTime()

	after := time.Now()

	s.anchor = before.Add(after.Sub(before) / 2)

	return nil
}

func loadSymbols() error {
	symbolOnce.Do(func() {
		_, symbolErr = safeCall(func() (any, error) {
			system, err := purego.Dlopen(
				"/usr/lib/libSystem.B.dylib",
				purego.RTLD_NOW|purego.RTLD_LOCAL,
			)
			if err != nil {
				return nil, fmt.Errorf("%w: load macOS clock: %v", domain.ErrUnsupported, err)
			}

			nativeLibraryHandles = append(nativeLibraryHandles, system)
			if err := bind(system, "mach_absolute_time", &machAbsoluteTime); err != nil {
				return nil, err
			}

			if err := bind(system, "mach_timebase_info", &machTimebaseInfo); err != nil {
				return nil, err
			}

			if machTimebaseInfo(&timebase) != 0 || timebase.Numer == 0 || timebase.Denom == 0 {
				return nil, fmt.Errorf("%w: invalid macOS clock timebase", domain.ErrUnsupported)
			}

			// The generated path binding takes an immutable string for an output
			// buffer. Bind its writable-buffer ABI privately.
			library, err := purego.Dlopen(
				"/System/Library/Frameworks/IOKit.framework/IOKit",
				purego.RTLD_NOW|purego.RTLD_LOCAL,
			)
			if err == nil {
				nativeLibraryHandles = append(nativeLibraryHandles, library)
				_ = bind(library, "IORegistryEntryGetPath", &registryPath)
			}

			if err := loadHIDFunctions(library); err != nil {
				return nil, err
			}

			valueCallback = purego.NewCallback(inputValueCallback)
			removalCallback = purego.NewCallback(deviceRemovalCallback)

			return nil, nil
		})
	})

	return symbolErr
}

func loadHIDFunctions(library uintptr) error {
	api = nativeAPI{
		IOHIDManagerCreate:                native.IOHIDManagerCreate,
		IOHIDManagerSetDeviceMatching:     native.IOHIDManagerSetDeviceMatching,
		IOHIDManagerCopyDevices:           native.IOHIDManagerCopyDevices,
		IOHIDManagerScheduleWithRunLoop:   native.IOHIDManagerScheduleWithRunLoop,
		IOHIDManagerUnscheduleFromRunLoop: native.IOHIDManagerUnscheduleFromRunLoop,
		IOHIDDeviceCreate:                 native.IOHIDDeviceCreate,
		IOHIDDeviceGetService:             native.IOHIDDeviceGetService,
		IOHIDDeviceGetProperty:            native.IOHIDDeviceGetProperty,
		IOHIDDeviceConformsTo:             native.IOHIDDeviceConformsTo,
		IOHIDDeviceCopyMatchingElements:   native.IOHIDDeviceCopyMatchingElements,
		IOHIDDeviceGetValue:               native.IOHIDDeviceGetValue,
		IOHIDDeviceOpen:                   native.IOHIDDeviceOpen,
		IOHIDDeviceClose:                  native.IOHIDDeviceClose,
		IOHIDDeviceScheduleWithRunLoop:    native.IOHIDDeviceScheduleWithRunLoop,
		IOHIDDeviceUnscheduleFromRunLoop:  native.IOHIDDeviceUnscheduleFromRunLoop,
		IOHIDElementGetCookie:             native.IOHIDElementGetCookie,
		IOHIDElementGetType:               native.IOHIDElementGetType,
		IOHIDElementGetUsagePage:          native.IOHIDElementGetUsagePage,
		IOHIDElementGetUsage:              native.IOHIDElementGetUsage,
		IOHIDElementGetName:               native.IOHIDElementGetName,
		IOHIDElementGetParent:             native.IOHIDElementGetParent,
		IOHIDElementGetCollectionType:     native.IOHIDElementGetCollectionType,
		IOHIDElementGetLogicalMin:         native.IOHIDElementGetLogicalMin,
		IOHIDElementGetLogicalMax:         native.IOHIDElementGetLogicalMax,
		IOHIDElementGetPhysicalMin:        native.IOHIDElementGetPhysicalMin,
		IOHIDElementGetPhysicalMax:        native.IOHIDElementGetPhysicalMax,
		IOHIDElementGetUnit:               native.IOHIDElementGetUnit,
		IOHIDElementGetUnitExponent:       native.IOHIDElementGetUnitExponent,
		IOHIDElementHasNullState:          native.IOHIDElementHasNullState,
		IOHIDElementIsRelative:            native.IOHIDElementIsRelative,
		IOHIDElementGetReportSize:         native.IOHIDElementGetReportSize,
		IOHIDElementGetReportCount:        native.IOHIDElementGetReportCount,
		IOHIDElementGetReportID:           native.IOHIDElementGetReportID,
		IOHIDElementGetDevice:             native.IOHIDElementGetDevice,
		IOHIDValueGetElement:              native.IOHIDValueGetElement,
		IOHIDValueGetIntegerValue:         native.IOHIDValueGetIntegerValue,
		IOHIDValueGetLength:               native.IOHIDValueGetLength,
		IOHIDValueGetTimeStamp:            native.IOHIDValueGetTimeStamp,
		IORegistryEntryGetRegistryEntryID: native.IORegistryEntryGetRegistryEntryID,
	}

	if library == 0 {
		return fmt.Errorf("%w: cannot load IOKit.framework", domain.ErrUnsupported)
	}

	// These two binding wrappers accept unsafe.Pointer contexts. Use their
	// uintptr-equivalent ABI so our opaque tokens are never Go pointers.
	err := bind(
		library,
		"IOHIDDeviceRegisterInputValueCallback",
		&api.IOHIDDeviceRegisterInputValueCallback,
	)
	if err != nil {
		return err
	}

	err := bind(
		library,
		"IOHIDDeviceRegisterRemovalCallback",
		&api.IOHIDDeviceRegisterRemovalCallback,
	)
	if err != nil {
		return err
	}

	if _, err := safeCall(
		func() (any, error) { return native.IOHIDManagerGetTypeID(), nil },
	); err == nil {
		return nil
	}

	// v0.6.18 tries lowercase iokit.framework/iokit. There is no exported
	// injection API. Bind only the public symbols used by this adapter from
	// the canonical framework, retaining the requested binding's exact types.
	value := reflect.ValueOf(&api).Elem()
	for i := 0; i < value.NumField(); i++ {
		err := bind(
			library,
			value.Type().Field(i).Name,
			value.Field(i).Addr().Interface(),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func bind(library uintptr, name string, target any) error {
	address, err := purego.Dlsym(library, name)
	if err != nil {
		return fmt.Errorf("%w: macOS symbol %s: %v", domain.ErrUnsupported, name, err)
	}

	purego.RegisterFunc(target, address)

	return nil
}

func (b *backend) call(ctx context.Context, perform func(*session) (any, error)) (any, error) {
	err := ctx.Err()
	if err != nil {
		return nil, err
	}

	req := request{ctx: ctx, perform: perform, result: make(chan response, 1)}
	select {
	case <-b.done:
		return nil, domain.ErrClosed
	case <-b.stop:
		return nil, domain.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	case b.jobs <- req:
	}

	select {
	case result := <-req.result:
		return result.value, result.err
	case <-b.done:
		return nil, domain.ErrClosed
	case <-ctx.Done():
		// An Open already executing must not leak its native device if its
		// caller stops waiting. The response is buffered and cleanup is joined
		// either here or by the session's shutdown.
		go func() {
			select {
			case result := <-req.result:
				if c, ok := result.value.(*capture); ok {
					_ = c.Close()
				}
			case <-b.done:
			}
		}()

		return nil, ctx.Err()
	}
}

func (b *backend) Discover(ctx context.Context) ([]domain.DeviceInfo, error) {
	var devices []domain.DeviceInfo

	operation := func(ctx context.Context) error {
		result, err := b.call(ctx, func(s *session) (any, error) { return s.discover(ctx) })
		if err == nil {
			devices = result.([]domain.DeviceInfo)
		}

		return err
	}

	var err error

	if b.retrier == nil {
		err = operation(ctx)
	} else {
		err = b.retrier.Do(ctx, operation, func(error) bool { return false })
	}

	return devices, err
}

func (s *session) devices() ([]native.IOHIDDeviceRef, cf.CFSetRef, error) {
	set := api.IOHIDManagerCopyDevices(s.manager)
	if set == 0 {
		return nil, 0, nil
	}

	count := cf.CFSetGetCount(set)
	if count < 0 || count > maxNativeElements {
		cf.CFRelease(pointer(uintptr(set)))

		return nil, 0, fmt.Errorf("IOHID: invalid device count %d", count)
	}

	if count == 0 {
		return nil, set, nil
	}

	values := make([]uintptr, count)
	cf.CFSetGetValues(set, unsafe.Pointer(&values[0]))

	devices := make([]native.IOHIDDeviceRef, count)

	for i, value := range values {
		devices[i] = native.IOHIDDeviceRef(value)
	}

	return devices, set, nil
}

func (s *session) discover(ctx context.Context) ([]domain.DeviceInfo, error) {
	devices, set, err := s.devices()
	if err != nil {
		return nil, err
	}

	if set != 0 {
		defer cf.CFRelease(pointer(uintptr(set)))
	}

	result := make([]domain.DeviceInfo, 0, len(devices))
	for _, device := range devices {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		info, _, err := s.deviceInfo(device)
		if err != nil {
			continue
		} // Removed devices may vanish during a snapshot.

		result = append(result, info)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })

	return result, nil
}

func (b *backend) Open(
	ctx context.Context,
	id domain.DeviceID,
	sink ports.EventSink,
) (ports.Capture, error) {
	if sink == nil {
		return nil, domain.ErrInvalidOptions
	}

	result, err := b.call(ctx, func(s *session) (any, error) { return s.open(ctx, id, sink) })
	if err != nil {
		return nil, err
	}

	return result.(*capture), nil
}

func (s *session) open(
	ctx context.Context,
	id domain.DeviceID,
	sink ports.EventSink,
) (result *capture, err error) {
	devices, set, err := s.devices()
	if err != nil {
		return nil, err
	}

	if set != 0 {
		defer cf.CFRelease(pointer(uintptr(set)))
	}

	var ref native.IOHIDDeviceRef

	for _, device := range devices {
		err := ctx.Err()
		if err != nil {
			return nil, err
		}

		info, _, infoErr := s.deviceInfo(device)
		if infoErr != nil || info.ID != id {
			continue
		}

		ref = api.IOHIDDeviceCreate(0, api.IOHIDDeviceGetService(device))

		break
	}

	if ref == 0 {
		return nil, &domain.OpError{Op: "open", DeviceID: id, Err: domain.ErrNotFound}
	}

	opened := false
	scheduled := false

	var token uintptr

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%w: IOHID open: %v", domain.ErrUnsupported, p)
		}

		if err != nil {
			if owner, ok := callbackRegistry.Load(token); ok {
				owner.(*capture).closed.Store(true)
			}

			err = errors.Join(err, s.releaseDevice(ref, scheduled, opened, token))
			callbackRegistry.Delete(token)
		}
	}()

	err = statusError(int32(api.IOHIDDeviceOpen(ref, 0)))
	if err != nil {
		return nil, &domain.OpError{Op: "open", DeviceID: id, Err: err}
	}

	opened = true

	info, metadata, err := s.deviceInfo(ref)
	if err != nil {
		return nil, err
	}

	c := &capture{
		backend:  s.backend,
		clock:    s,
		ref:      ref,
		info:     info,
		metadata: metadata,
		sink:     sink,
		controls: make(map[uint32]elementControl),
		pressed:  make(map[domain.ControlID]bool),
	}
	err = s.capabilities(c)
	if err != nil {
		return nil, err
	}

	token = uintptr(nextCallbackToken.Add(1))
	if token == 0 {
		return nil, fmt.Errorf("%w: IOHID callback token exhausted", domain.ErrUnsupported)
	}

	c.token = token
	callbackRegistry.Store(token, c)
	api.IOHIDDeviceRegisterInputValueCallback(ref, valueCallback, token)
	api.IOHIDDeviceRegisterRemovalCallback(ref, removalCallback, token)
	api.IOHIDDeviceScheduleWithRunLoop(ref, s.runLoop, s.mode)

	scheduled = true

	err = ctx.Err()
	if err != nil {
		return nil, err
	}

	s.captures[c] = struct{}{}

	return c, nil
}

func (s *session) deviceInfo(
	ref native.IOHIDDeviceRef,
) (domain.DeviceInfo, extension.Metadata, error) {
	service := api.IOHIDDeviceGetService(ref)

	var registryID uint64

	err := statusError(api.IORegistryEntryGetRegistryEntryID(service, &registryID))
	if err != nil {
		return domain.DeviceInfo{}, extension.Metadata{}, err
	}

	info := domain.DeviceInfo{
		ID:           domain.DeviceID(fmt.Sprintf("darwin:%016x", registryID)),
		Name:         s.stringProperty(ref, "Product"),
		Manufacturer: s.stringProperty(ref, "Manufacturer"),
		Serial:       s.stringProperty(ref, "SerialNumber"),
	}
	if value, ok := s.numberProperty(ref, "VendorID"); ok && value >= 0 && value <= math.MaxUint16 {
		n := uint16(value)

		info.VendorID = &n
	}

	if value, ok := s.numberProperty(
		ref,
		"ProductID",
	); ok && value >= 0 &&
		value <= math.MaxUint16 {

		n := uint16(value)

		info.ProductID = &n
	}

	if registryPath != nil {
		buffer := make([]byte, 4096)
		if registryPath(service, "IOService", &buffer[0]) == 0 {
			info.Path = nulString(buffer)
		}
	}

	switch strings.ToLower(s.stringProperty(ref, "Transport")) {
	case "usb":
		info.Transport = domain.TransportUSB
	case "bluetooth", "bluetooth low energy":
		info.Transport = domain.TransportBluetooth
	case "i2c":
		info.Transport = domain.TransportI2C
	case "virtual":
		info.Transport = domain.TransportVirtual
	}

	for _, candidate := range []struct {
		usage uint32
		class domain.DeviceClass
	}{{6, domain.ClassKeyboard}, {7, domain.ClassKeyboard}, {2, domain.ClassMouse}, {4, domain.ClassJoystick}, {5, domain.ClassGamepad}} {
		if api.IOHIDDeviceConformsTo(ref, 1, candidate.usage) &&
			!containsClass(info.Classes, candidate.class) {

			info.Classes = append(info.Classes, candidate.class)
		}
	}

	if len(info.Classes) == 0 {
		info.Classes = []domain.DeviceClass{domain.ClassOther}
	}

	metadata := extension.Metadata{RegistryEntryID: registryID}

	if value, ok := s.numberProperty(
		ref,
		"LocationID",
	); ok && value >= 0 &&
		value <= math.MaxUint32 {

		n := uint32(value)

		metadata.LocationID = &n
	}

	if value, ok := s.numberProperty(
		ref,
		"PrimaryUsagePage",
	); ok && value >= 0 &&
		value <= math.MaxUint32 {

		metadata.PrimaryUsagePage = uint32(value)
	}

	if value, ok := s.numberProperty(
		ref,
		"PrimaryUsage",
	); ok && value >= 0 &&
		value <= math.MaxUint32 {

		metadata.PrimaryUsage = uint32(value)
	}

	return info, metadata, nil
}

func containsClass(classes []domain.DeviceClass, target domain.DeviceClass) bool {
	for _, class := range classes {
		if class == target {
			return true
		}
	}

	return false
}

func (s *session) key(name string) cf.CFStringRef {
	if key := s.keys[name]; key != 0 {
		return key
	}

	key := cf.CFStringCreateWithCString(0, name, utf8Encoding)
	if key != 0 {
		s.keys[name] = key
	}

	return key
}

func (s *session) stringProperty(ref native.IOHIDDeviceRef, key string) string {
	value := api.IOHIDDeviceGetProperty(ref, s.key(key))
	if value == nil || cf.CFGetTypeID(value) != cf.CFStringGetTypeID() {
		return ""
	}

	return cfString(cf.CFStringRef(uintptr(value)))
}

func (s *session) numberProperty(ref native.IOHIDDeviceRef, key string) (int64, bool) {
	value := api.IOHIDDeviceGetProperty(ref, s.key(key))
	if value == nil || cf.CFGetTypeID(value) != cf.CFNumberGetTypeID() {
		return 0, false
	}

	var number int64

	ok := cf.CFNumberGetValue(
		cf.CFNumberRef(uintptr(value)),
		cf.CFNumberType(cf.KCFNumberSInt64Type),
		unsafe.Pointer(&number),
	)

	return number, ok
}

func cfString(ref cf.CFStringRef) string {
	if ref == 0 {
		return ""
	}

	length := cf.CFStringGetMaximumSizeForEncoding(cf.CFStringGetLength(ref), utf8Encoding)
	if length < 0 || length >= maxNativeString {
		return ""
	}

	buffer := make([]byte, length+1)
	if !cf.CFStringGetCString(ref, &buffer[0], len(buffer), utf8Encoding) {
		return ""
	}

	return nulString(buffer)
}

func nulString(buffer []byte) string {
	for i, b := range buffer {
		if b == 0 {
			return string(buffer[:i])
		}
	}

	return string(buffer)
}

func (s *session) capabilities(c *capture) error {
	array := api.IOHIDDeviceCopyMatchingElements(c.ref, 0, 0)
	if array == 0 {
		c.caps = domain.Capabilities{Complete: false, Repeat: domain.SupportUnsupported}

		return nil
	}

	defer cf.CFRelease(pointer(uintptr(array)))

	count := cf.CFArrayGetCount(array)
	if count < 0 || count > maxNativeElements {
		return fmt.Errorf("IOHID: invalid element count %d", count)
	}

	elements := make([]native.IOHIDElementRef, count)
	multipliers := make(map[multiplierKey]multiplierValue)

	for i := range elements {
		element := native.IOHIDElementRef(uintptr(cf.CFArrayGetValueAtIndex(array, i)))

		elements[i] = element

		if api.IOHIDElementGetType(element) == elementFeature &&
			api.IOHIDElementGetUsagePage(element) == 1 &&
			api.IOHIDElementGetUsage(element) == usageResolutionMultiplier {

			key := wheelCollection(element)
			value, valid := readMultiplier(c.ref, element)

			if _, duplicate := multipliers[key]; duplicate {
				valid = false
			}

			multipliers[key] = multiplierValue{value: value, valid: valid}
		}
	}

	c.caps = domain.Capabilities{Complete: true, Repeat: domain.SupportUnsupported}

	for _, element := range elements {
		kind := api.IOHIDElementGetType(element)
		if kind < elementInputFirst || kind > elementInputLast {
			continue
		}

		cookie := uint32(api.IOHIDElementGetCookie(element))
		page, usage := api.IOHIDElementGetUsagePage(element), api.IOHIDElementGetUsage(element)
		control := domain.Control{
			ID:      domain.ControlID(fmt.Sprintf("element:%08x", cookie)),
			Name:    cfString(api.IOHIDElementGetName(element)),
			Kind:    domain.ControlUnknown,
			Unit:    domain.UnitLogical,
			Support: domain.SupportSupported,
			Mapping: domain.MappingReported,
		}

		if page <= math.MaxUint16 && usage <= math.MaxUint16 {
			control.Usage = domain.HID(uint16(page), uint16(usage))
		} else {
			control.Mapping = domain.MappingUnknown
		}

		minimum, maximum := int64(
			api.IOHIDElementGetLogicalMin(element),
		), int64(
			api.IOHIDElementGetLogicalMax(element),
		)
		hasNull := api.IOHIDElementHasNullState(element)
		relative := api.IOHIDElementIsRelative(element)

		if relative {
			control.Mode = domain.AxisRelative
			control.Unit = domain.UnitCounts
		} else {
			control.Mode = domain.AxisAbsolute
			control.Range = &domain.Range{Min: minimum, Max: maximum}
		}

		switch {
		case page == 7 && usage != 0:
			control.Kind = domain.ControlKey
			control.Unit = domain.UnitBoolean
			control.Mode = domain.AxisUnknown
			control.Range = nil
		case page == 9:
			control.Kind = domain.ControlButton
			control.Unit = domain.UnitBoolean
			control.Mode = domain.AxisUnknown
			control.Range = nil
		case page == 1 && usage >= 0x90 && usage <= 0x93:
			control.Kind = domain.ControlButton
			control.Unit = domain.UnitBoolean
			control.Mode = domain.AxisUnknown
			control.Range = nil
		case page == 1 && usage == 0x39:
			if _, ok := domain.Hat(minimum, minimum, maximum, hasNull); ok {
				control.Kind = domain.ControlHat
				control.Unit = domain.UnitDirection
			} else {
				control.Kind = domain.ControlAxis
			}
		case kind == 2:
			control.Kind = domain.ControlButton
			control.Unit = domain.UnitBoolean
			control.Mode = domain.AxisUnknown
			control.Range = nil
		case kind == 3 || page == 1 && usage >= 0x30 && usage <= 0x38 || page == 2 || control.Usage == domain.AxisPan:
			control.Kind = domain.ControlAxis
		case page == 0x0c && maximum == 1 && minimum == 0:
			control.Kind = domain.ControlKey
			control.Unit = domain.UnitBoolean
			control.Mode = domain.AxisUnknown
			control.Range = nil
		}

		multiplier := 1.0

		if relative && (control.Usage == domain.AxisWheel || control.Usage == domain.AxisPan) {
			if resolution, ok := multipliers[wheelCollection(element)]; ok {
				if resolution.valid {
					multiplier = resolution.value
					control.Unit = domain.UnitDetents
				} else {
					multiplier = 0
				}
			} else {
				control.Unit = domain.UnitDetents
			}
		}

		size, count := api.IOHIDElementGetReportSize(
			element,
		), api.IOHIDElementGetReportCount(
			element,
		)
		// Event.Value uses float64, which preserves arbitrary integer values
		// only through 53 significant bits. Retain wider native metadata but
		// never publish a silently rounded scalar or packed value.
		if size > 53 || uint64(size)*uint64(count) > 53 {
			control.Support = domain.SupportUnsupported
		}

		if control.Name == "" {
			control.Name = control.Usage.String()
		}

		c.controls[cookie] = elementControl{
			control:    control,
			element:    element,
			hasNull:    hasNull,
			multiplier: multiplier,
		}
		c.caps.Controls = append(c.caps.Controls, control)
		c.metadata.Elements = append(
			c.metadata.Elements,
			extension.Element{
				ControlID:       string(control.ID),
				Cookie:          cookie,
				ReportID:        api.IOHIDElementGetReportID(element),
				ReportSize:      size,
				ReportCount:     count,
				PhysicalMinimum: int64(api.IOHIDElementGetPhysicalMin(element)),
				PhysicalMaximum: int64(api.IOHIDElementGetPhysicalMax(element)),
				Unit:            api.IOHIDElementGetUnit(element),
				UnitExponent:    unitExponent(api.IOHIDElementGetUnitExponent(element)),
				HasNullState:    hasNull,
			},
		)
	}

	sort.Slice(
		c.caps.Controls,
		func(i, j int) bool { return c.caps.Controls[i].ID < c.caps.Controls[j].ID },
	)
	sort.Slice(
		c.metadata.Elements,
		func(i, j int) bool { return c.metadata.Elements[i].Cookie < c.metadata.Elements[j].Cookie },
	)

	return nil
}

func wheelCollection(element native.IOHIDElementRef) multiplierKey {
	parent := api.IOHIDElementGetParent(element)
	for depth := 0; parent != 0 && depth < 256; depth++ {
		if api.IOHIDElementGetType(parent) == elementCollection &&
			api.IOHIDElementGetCollectionType(parent) == collectionLogical {

			return multiplierKey{collection: parent}
		}

		parent = api.IOHIDElementGetParent(parent)
	}

	return multiplierKey{report: api.IOHIDElementGetReportID(element)}
}

func unitExponent(value uint32) int32 {
	// HID Unit Exponent is a signed four-bit nibble, though some devices and
	// IOHID representations already provide a sign-extended integer.
	if value <= 15 {
		if value >= 8 {
			return int32(value) - 16
		}

		return int32(value)
	}

	return int32(value)
}

func readMultiplier(device native.IOHIDDeviceRef, element native.IOHIDElementRef) (float64, bool) {
	var value native.IOHIDValueRef

	if api.IOHIDDeviceGetValue(device, element, &value) != 0 || value == 0 ||
		api.IOHIDValueGetLength(value) > 8 {

		return 0, false
	}

	low, high := float64(
		api.IOHIDElementGetLogicalMin(element),
	), float64(
		api.IOHIDElementGetLogicalMax(element),
	)
	physicalLow, physicalHigh := float64(
		api.IOHIDElementGetPhysicalMin(element),
	), float64(
		api.IOHIDElementGetPhysicalMax(element),
	)

	if high <= low {
		return 0, false
	}

	logical := float64(api.IOHIDValueGetIntegerValue(value))
	if logical < low || logical > high {
		return 0, false
	}

	exponent := unitExponent(api.IOHIDElementGetUnitExponent(element))
	multiplier := ((logical-low)/(high-low)*(physicalHigh-physicalLow) + physicalLow) * math.Pow10(
		int(exponent),
	)

	return multiplier, multiplier != 0 && !math.IsNaN(multiplier) && !math.IsInf(multiplier, 0)
}

func inputValueCallback(token uintptr, status int32, _ uintptr, value uintptr) {
	var c *capture

	defer func() {
		if p := recover(); p != nil && c != nil {
			c.sink.Fail(fmt.Errorf("%w: IOHID callback: %v", domain.ErrEventLoss, p))
		}
	}()

	if owner, ok := callbackRegistry.Load(token); ok {
		c = owner.(*capture)
	}

	if c == nil || c.closed.Load() {
		return
	}

	if status != ioSuccess {
		c.sink.Fail(statusError(status))

		return
	}

	if value == 0 {
		return
	}

	ref := native.IOHIDValueRef(value)
	if length := api.IOHIDValueGetLength(ref); length <= 0 || length > 8 {
		return
	}

	c.processValue(api.IOHIDValueGetElement(ref), ref)
}

// Device removal uses IOHIDCallback, which has three arguments; manager device
// notifications use the distinct four-argument IOHIDDeviceCallback ABI.
func deviceRemovalCallback(token uintptr, _ int32, _ uintptr) {
	owner, ok := callbackRegistry.Load(token)
	if !ok {
		return
	}

	c := owner.(*capture)
	if c.closed.Load() {
		return
	}

	c.sink.Fail(&domain.OpError{Op: "read", DeviceID: c.info.ID, Err: domain.ErrDisconnected})
}

func (c *capture) processValue(element native.IOHIDElementRef, value native.IOHIDValueRef) {
	item, ok := c.controls[uint32(api.IOHIDElementGetCookie(element))]
	if !ok || item.control.Support != domain.SupportSupported {
		return
	}

	raw := int64(api.IOHIDValueGetIntegerValue(value))
	output := float64(raw)
	action := domain.ActionChange

	switch item.control.Kind {
	case domain.ControlKey, domain.ControlButton, domain.ControlSwitch:
		pressed := raw != 0
		if previous, known := c.pressed[item.control.ID]; known && previous == pressed {
			return
		}

		c.pressed[item.control.ID] = pressed
		if pressed {
			output = 1
			action = domain.ActionPress
		} else {
			output = 0
			action = domain.ActionRelease
		}
	case domain.ControlHat:
		direction, valid := domain.Hat(
			raw,
			item.control.Range.Min,
			item.control.Range.Max,
			item.hasNull,
		)
		if !valid {
			return
		}

		output = float64(direction)
	}

	if item.control.Unit == domain.UnitDetents && item.multiplier != 0 {
		output /= item.multiplier
	}

	now := time.Now()
	timestamp := domain.Timestamp{Time: now.UTC(), ReceivedAt: now, Source: domain.TimestampReceipt}
	// Capture's run-loop owner supplies its session clock through the event's
	// monotonic OS tick offset; absolute wall time remains an estimate.
	if ticks := api.IOHIDValueGetTimeStamp(value); ticks != 0 {
		if delta, valid := tickDuration(ticks, c.clock.anchorTicks); valid {
			timestamp.Time = c.clock.anchor.Add(delta).UTC()
			timestamp.Source = domain.TimestampEstimated
		}
	}

	c.sink.Publish(
		domain.Event{
			DeviceID:  c.info.ID,
			ControlID: item.control.ID,
			Action:    action,
			Value:     output,
			Timestamp: timestamp,
		},
	)
}

func tickDuration(ticks, anchor uint64) (time.Duration, bool) {
	negative := ticks < anchor
	delta := ticks - anchor

	if negative {
		delta = anchor - ticks
	}

	hi, lo := bits.Mul64(delta, uint64(timebase.Numer))
	if timebase.Denom == 0 || hi >= uint64(timebase.Denom) {
		return 0, false
	}

	nanos, _ := bits.Div64(hi, lo, uint64(timebase.Denom))
	if nanos > math.MaxInt64 {
		return 0, false
	}

	duration := time.Duration(nanos)

	if negative {
		duration = -duration
	}

	return duration, true
}

func (c *capture) Info() domain.DeviceInfo           { return domain.CloneInfo(c.info) }
func (c *capture) Capabilities() domain.Capabilities { return domain.CloneCapabilities(c.caps) }

func (c *capture) IOKitMetadata() extension.Metadata {
	metadata := c.metadata

	metadata.Elements = append([]extension.Element(nil), metadata.Elements...)

	if metadata.LocationID != nil {
		location := *metadata.LocationID

		metadata.LocationID = &location
	}

	return metadata
}

func (c *capture) Extension(target any) bool {
	switch value := target.(type) {
	case *extension.MetadataProvider:
		if value == nil {
			return false
		}

		*value = c

		return true
	case *extension.Metadata:
		if value == nil {
			return false
		}

		*value = c.IOKitMetadata()

		return true
	}

	return false
}

func (c *capture) Close() error {
	c.closeOnce.Do(func() {
		c.closed.Store(true)

		_, err := c.backend.call(
			context.Background(),
			func(s *session) (any, error) { return nil, s.closeCapture(c) },
		)
		if !errors.Is(err, domain.ErrClosed) {
			c.closeErr = err
		}
	})

	return c.closeErr
}

func (s *session) closeCapture(c *capture) error {
	if _, exists := s.captures[c]; !exists {
		return nil
	}

	delete(s.captures, c)
	c.closed.Store(true)

	defer callbackRegistry.Delete(c.token)

	return s.releaseDevice(c.ref, true, true, c.token)
}

func (s *session) releaseDevice(
	ref native.IOHIDDeviceRef,
	scheduled, opened bool,
	token uintptr,
) error {
	var steps []func() error

	if token != 0 {
		steps = append(steps,

			func() error {
				api.IOHIDDeviceRegisterInputValueCallback(ref, 0, token)
				return nil
			},

			func() error {
				api.IOHIDDeviceRegisterRemovalCallback(ref, 0, token)
				return nil
			},
		)
	}

	if scheduled {
		steps = append(
			steps,

			func() error {
				api.IOHIDDeviceUnscheduleFromRunLoop(ref, s.runLoop, s.mode)
				return nil
			},
		)
	}

	if opened {
		steps = append(
			steps,
			func() error { return statusError(int32(api.IOHIDDeviceClose(ref, 0))) },
		)
	}

	steps = append(steps, func() error {
		cf.CFRelease(pointer(uintptr(ref)))
		return nil
	})

	return cleanupSteps(steps...)
}

func cleanupSteps(steps ...func() error) error {
	var failures []error

	for _, step := range steps {
		_, err := safeCall(func() (any, error) { return nil, step() })

		failures = append(failures, err)
	}

	return errors.Join(failures...)
}

func (s *session) close() error {
	var failures []error

	for c := range s.captures {
		c.sink.Fail(domain.ErrClosed)

		_, err := safeCall(func() (any, error) { return nil, s.closeCapture(c) })

		failures = append(failures, err)
	}

	if s.manager != 0 {
		failures = append(failures, cleanupSteps(
			func() error {
				if s.runLoop != 0 && s.mode != 0 {
					api.IOHIDManagerUnscheduleFromRunLoop(s.manager, s.runLoop, s.mode)
				}

				return nil
			},

			func() error {
				cf.CFRelease(pointer(uintptr(s.manager)))
				return nil
			},
		))
	}

	for _, key := range s.keys {
		failures = append(
			failures,

			cleanupSteps(func() error {
				cf.CFRelease(pointer(uintptr(key)))
				return nil
			}),
		)
	}

	if s.mode != 0 {
		failures = append(
			failures,

			cleanupSteps(func() error {
				cf.CFRelease(pointer(uintptr(s.mode)))
				return nil
			}),
		)
	}

	return errors.Join(failures...)
}

func (b *backend) Close() error {
	b.closeOnce.Do(func() { close(b.stop) })
	<-b.done

	return b.closeErr
}

func pointer(value uintptr) unsafe.Pointer {
	// IOHID and CoreFoundation bindings represent opaque native pointers as
	// uintptr. Reinterpret that representation without Go-pointer arithmetic.
	return *(*unsafe.Pointer)(unsafe.Pointer(&value))
}

func statusError(status int32) error {
	switch status {
	case ioSuccess:
		return nil
	case ioNotPermitted, ioNotPrivileged:
		return fmt.Errorf(
			"%w: IOReturn 0x%08x (Input Monitoring may be required)",
			domain.ErrPermissionDenied,
			uint32(status),
		)
	case ioNoDevice, ioNotOpen:
		return fmt.Errorf("%w: IOReturn 0x%08x", domain.ErrDisconnected, uint32(status))
	default:
		return fmt.Errorf("IOHID IOReturn 0x%08x", uint32(status))
	}
}
