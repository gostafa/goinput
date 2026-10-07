//go:build darwin && (amd64 || arm64)

package iokit

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	extension "github.com/gostafa/goinput/extensions/iokit"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	cf "github.com/tmc/apple/corefoundation"
	native "github.com/tmc/apple/iokit"
)

type (
	classCandidate struct {
		usage uint32
		class domain.DeviceClass
	}
)

func (environment *nativeState) newBackend(
	ctx context.Context,
	retrier ports.Retrier,
) (ports.Backend, error) {
	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("newBackend: %w", err)
	}

	backend := &backend{
		closeErr: nil, closeOnce: sync.Once{},
		jobs: make(chan sessionJob), stop: make(chan struct{}), done: make(chan struct{}),
		ready: make(chan error, nativeOne), retrier: retrier,
	}

	go environment.backendRun(backend)

	result0, callErr := environment.backendAwaitReady(ctx, backend)
	if callErr != nil {
		return nil, errors.Join(callErr)
	}

	return result0, nil
}

func (environment *nativeState) backendRun(backend *backend) {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()
	defer close(backend.done)

	session := newSession(backend)

	defer func() { environment.sessionFinish(session, recover()) }()

	environment.sessionServe(session)
}

func safeCall(call func() (any, error)) (value any, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%w: IOHID binding: %v", domain.ErrUnsupported, p)
		}
	}()

	result0, callErr := call()
	if callErr != nil {
		return nil, errors.Join(callErr)
	}

	return result0, nil
}

func (environment *nativeState) sessionStart(session *session) error {
	err := environment.loadSymbols()
	if err != nil {
		return fmt.Errorf("load IOHID symbols: %w", err)
	}

	return errors.Join(environment.sessionStartResources(session))
}

func (environment *nativeState) sessionStartResources(session *session) error {
	err := sessionCreateRunLoop(session)
	if err != nil {
		return fmt.Errorf("create IOHID run loop: %w", err)
	}

	err = environment.sessionCreateManager(session)
	if err != nil {
		return fmt.Errorf("create IOHID manager: %w", err)
	}

	environment.sessionAnchorClock(session)

	return nil
}

func (environment *nativeState) loadSymbols() error {
	environment.symbolOnce.Do(func() {
		_, environment.symbolErr = safeCall(
			func() (any, error) { return nil, environment.initializeSymbols() },
		)
	})

	return environment.symbolErr
}

func (environment *nativeState) loadHIDFunctions(library uintptr) error {
	environment.setNativeAPI()

	if library == nativeZero {
		return fmt.Errorf("%w: cannot load IOKit.framework", domain.ErrUnsupported)
	}

	if err := environment.bindCallbacks(library); err != nil {
		return errors.Join(err)
	}

	var err error

	err = resultError(safeCall(func() (any, error) { return native.IOHIDManagerGetTypeID(), nil }))
	if err == nil {
		return nil
	}

	return errors.Join(environment.bindNativeAPI(library))
}

func bind(library uintptr, name string, target any) error {
	address, err := purego.Dlsym(library, name)
	if err != nil {
		return fmt.Errorf("%w: macOS symbol %s: %v", domain.ErrUnsupported, name, err)
	}

	purego.RegisterFunc(target, address)

	return nil
}

func (environment *nativeState) backendCall(ctx context.Context,
	backend *backend,
	perform func(*session) (any, error),
) (any, error) {
	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("call: %w", err)
	}

	request := newRequest(ctx, perform)

	err = backendSendRequest(ctx, backend, &request)
	if err != nil {
		return nil, fmt.Errorf("call: %w", err)
	}

	value, err := environment.backendAwaitResponse(ctx, backend, &request)

	return value, errors.Join(err)
}

func (environment *nativeState) backendDiscover(
	ctx context.Context,
	backend *backend,
) ([]domain.DeviceInfo, error) {
	var devices []domain.DeviceInfo

	operation := func(ctx context.Context) error {
		var err error

		devices, err = environment.backendDiscoverDevices(ctx, backend)

		return errors.Join(err)
	}
	err := backendRunDiscovery(ctx, backend, operation)

	return devices, errors.Join(err)
}

func (environment *nativeState) sessionDevices(session *session) (deviceInventory, error) {
	set := environment.api.IOHIDManagerCopyDevices(session.manager)
	if set == nativeZero {
		return deviceInventory{devices: nil, set: 0}, nil
	}

	count := cf.CFSetGetCount(set)
	if count < nativeZero || count > maxNativeElements {
		cf.CFRelease(pointer(uintptr(set)))

		return deviceInventory{}, fmt.Errorf("IOHID: invalid device count %d", count)
	}

	return deviceInventory{devices: setDevices(set, count), set: set}, nil
}

func (environment *nativeState) sessionDiscover(
	ctx context.Context,
	session *session,
) ([]domain.DeviceInfo, error) {
	inventory, err := environment.sessionDevices(session)
	if err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}
	defer releaseSet(inventory.set)

	result0, callErr := environment.sessionDeviceInfos(ctx, session, inventory.devices)
	if callErr != nil {
		return nil, errors.Join(callErr)
	}

	return result0, nil
}

func (environment *nativeState) backendOpen(ctx context.Context,
	backend *backend,
	args openTarget,
) (ports.Capture, error) {
	if args.sink == nil {
		return nil, domain.ErrInvalidOptions
	}

	result, err := environment.backendCall(
		ctx,
		backend,
		func(s *session) (any, error) { return environment.sessionOpen(ctx, s, args) },
	)
	if err != nil {
		return nil, fmt.Errorf("Open: %w", err)
	}

	opened, err := environment.openedCapture(result)

	return opened, errors.Join(err)
}

func (environment *nativeState) sessionOpen(ctx context.Context,
	session *session,
	args openTarget,
) (*capture, error) {
	ref, err := environment.sessionFindDevice(ctx, session, args.id)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	resources := &deviceResources{token: nativeZero, opened: false, scheduled: false, ref: ref.ref}

	result0, callErr := environment.sessionOpenDevice(
		ctx,
		session,
		captureSetup{resources: resources, sink: args.sink},
	)
	if callErr != nil {
		return nil, errors.Join(callErr)
	}

	return result0, nil
}

func (environment *nativeState) sessionDeviceInfo(session *session,
	ref native.IOHIDDeviceRef,
) (domain.DeviceInfo, extension.Metadata, error) {
	service := environment.api.IOHIDDeviceGetService(ref)

	registryID, err := environment.registryIdentifier(service)
	if err != nil {
		return domain.DeviceInfo{}, extension.Metadata{}, fmt.Errorf("deviceInfo: %w", err)
	}

	info := environment.sessionEndpointInfo(session, ref)

	info.ID = domain.DeviceID(fmt.Sprintf("darwin:%016x", registryID))
	info.Path = environment.devicePath(service)

	metadata := environment.sessionEndpointMetadata(session, ref)

	metadata.RegistryEntryID = registryID

	return info, metadata, nil
}

func (environment *nativeState) registryIdentifier(service uint32) (uint64, error) {
	var registryID uint64

	err := statusError(environment.api.IORegistryEntryGetRegistryEntryID(service, &registryID))

	return registryID, errors.Join(err)
}

func containsClass(classes []domain.DeviceClass, target domain.DeviceClass) bool {
	return slices.Contains(classes, target)
}

func sessionKey(session *session, name string) cf.CFStringRef {
	if key := session.keys[name]; key != nativeZero {
		return key
	}

	key := cf.CFStringCreateWithCString(nativeZero, name, utf8Encoding)
	if key != nativeZero {
		session.keys[name] = key
	}

	return key
}

func (environment *nativeState) sessionStringProperty(
	session *session,
	ref native.IOHIDDeviceRef,
	key string,
) string {
	value := environment.api.IOHIDDeviceGetProperty(ref, sessionKey(session, key))
	if value == nil || cf.CFGetTypeID(value) != cf.CFStringGetTypeID() {
		return ""
	}

	return cfString(cf.CFStringRef(uintptr(value)))
}

func (environment *nativeState) sessionNumberProperty(
	session *session,
	ref native.IOHIDDeviceRef,
	key string,
) (int64, bool) {
	value := environment.api.IOHIDDeviceGetProperty(ref, sessionKey(session, key))
	if value == nil || cf.CFGetTypeID(value) != cf.CFNumberGetTypeID() {
		return nativeZero, false
	}

	var number int64

	ok := cf.CFNumberGetValue(
		cf.CFNumberRef(uintptr(value)),
		cf.KCFNumberSInt64Type,
		unsafe.Pointer(&number),
	)

	return number, ok
}

func cfString(ref cf.CFStringRef) string {
	if ref == nativeZero {
		return ""
	}

	length := cf.CFStringGetMaximumSizeForEncoding(cf.CFStringGetLength(ref), utf8Encoding)
	if length < nativeZero || length >= maxNativeString {
		return ""
	}

	buffer := make([]byte, length+nativeOne)
	if !cf.CFStringGetCString(ref, &buffer[nativeZero], len(buffer), utf8Encoding) {
		return ""
	}

	return nulString(buffer)
}

func nulString(buffer []byte) string {
	for i := range buffer {
		if buffer[i] == nativeZero {
			return string(buffer[:i])
		}
	}

	return string(buffer)
}

func (environment *nativeState) captureLoadCapabilities(capture *capture) error {
	array := environment.api.IOHIDDeviceCopyMatchingElements(capture.ref, nativeZero, nativeZero)
	if array == nativeZero {
		capture.caps = unavailableCapabilities()

		return nil
	}

	defer cf.CFRelease(pointer(uintptr(array)))

	elements, err := matchingElements(array)
	if err != nil {
		return fmt.Errorf("loadCapabilities: %w", err)
	}

	environment.captureBuildControls(capture, elements)

	return nil
}

func (environment *nativeState) wheelCollection(element native.IOHIDElementRef) multiplierKey {
	parent := environment.api.IOHIDElementGetParent(element)
	for depth := nativeZero; parent != nativeZero && depth < maxCollectionDepth; depth++ {
		if environment.api.IOHIDElementGetType(parent) == elementCollection &&
			environment.api.IOHIDElementGetCollectionType(parent) == nativeTwo {
			return multiplierKey{report: nativeZero, collection: parent}
		}

		parent = environment.api.IOHIDElementGetParent(parent)
	}

	return multiplierKey{
		collection: nativeZero,
		report:     environment.api.IOHIDElementGetReportID(element),
	}
}

func unitExponent(value uint32) int32 {
	// HID Unit Exponent is a signed four-bit nibble, though some devices and
	// IOHID representations already provide a sign-extended integer.
	if value <= unitExponentMask {
		if value >= nativeEight {
			return int32(value) - unitExponentModulus
		}

		return int32(value)
	}

	return int32(value)
}

func (environment *nativeState) readMultiplier(
	device native.IOHIDDeviceRef,
	element native.IOHIDElementRef,
) (float64, bool) {
	var value native.IOHIDValueRef

	if environment.api.IOHIDDeviceGetValue(device, element, &value) != nativeZero ||
		!environment.scalarValue(value) {

		return nativeZero, false
	}

	return environment.elementMultiplier(element, value)
}

func (environment *nativeState) inputValueCallback(token uintptr, status int32, value uintptr) {
	capture := environment.registeredCapture(token)

	defer func() { recoverCallback(capture, recover()) }()

	if capture == nil || capture.closed.Load() {
		return
	}

	if status != nativeZero {
		capture.sink.Fail(statusError(status))

		return
	}

	environment.captureReceiveValue(capture, native.IOHIDValueRef(value))
}

// Device removal uses IOHIDCallback, which has three arguments; manager device
// notifications use the distinct four-argument IOHIDDeviceCallback ABI.
func (environment *nativeState) deviceRemovalCallback(token uintptr, _ int32, _ uintptr) {
	capture := environment.registeredCapture(token)
	if capture == nil || capture.closed.Load() {
		return
	}

	capture.sink.Fail(
		&domain.OpError{Op: operationRead, DeviceID: capture.info.ID, Err: domain.ErrDisconnected},
	)
}

func (environment *nativeState) captureProcessValue(
	capture *capture,
	element native.IOHIDElementRef,
	value native.IOHIDValueRef,
) {
	item, ok := capture.controls[environment.api.IOHIDElementGetCookie(element)]
	if !ok || item.control.Support != domain.SupportSupported {
		return
	}

	output, action, valid := captureDecodeValue(
		capture,
		&item,
		int64(environment.api.IOHIDValueGetIntegerValue(value)),
	)
	if !valid {
		return
	}

	environment.capturePublishValue(capture, &item, output, action, value)
}

func (environment *nativeState) tickDuration(ticks, anchor uint64) (time.Duration, bool) {
	delta := ticks - anchor
	if ticks < anchor {
		delta = anchor - ticks
	}

	duration, valid := environment.scaledDuration(delta)

	if ticks < anchor {
		duration = -duration
	}

	return duration, valid
}

func captureInfo(capture *capture) domain.DeviceInfo { return domain.CloneInfo(&capture.info) }

func captureCapabilities(capture *capture) domain.Capabilities {
	return domain.CloneCapabilities(&capture.caps)
}

func captureIOKitMetadata(capture *capture) extension.Metadata {
	metadata := capture.metadata

	metadata.Elements = append([]extension.Element(nil), metadata.Elements...)

	if metadata.LocationID != nil {
		location := *metadata.LocationID

		metadata.LocationID = &location
	}

	return metadata
}

func captureExtension(capture *capture, target any) bool {
	switch value := target.(type) {
	case *extension.MetadataProvider:
		return assignMetadata(value, captureMetadataView(capture))
	case *extension.Metadata:
		return assignMetadata(value, captureIOKitMetadata(capture))
	default:
		return false
	}
}

func (environment *nativeState) captureClose(capture *capture) error {
	capture.closeOnce.Do(func() {
		capture.closed.Store(true)

		var err error

		err = resultError(environment.backendCall(
			context.Background(),
			capture.backend,
			func(s *session) (any, error) { return nil, environment.sessionCloseCapture(s, capture) },
		))
		if !errors.Is(err, domain.ErrClosed) {
			capture.closeErr = err
		}
	})

	return capture.closeErr
}

func (environment *nativeState) sessionCloseCapture(session *session, capture *capture) error {
	if _, exists := session.captures[capture]; !exists {
		return nil
	}

	delete(session.captures, capture)
	capture.closed.Store(true)

	defer environment.callbackRegistry.Delete(capture.token)

	return errors.Join(environment.sessionReleaseDevice(session,
		&deviceResources{ref: capture.ref, scheduled: true, opened: true, token: capture.token},
	))
}

func (environment *nativeState) sessionReleaseDevice(
	session *session,
	resources *deviceResources,
) error {
	steps := environment.deviceResourcesCallbackCleanup(resources)
	if resources.scheduled {
		steps = append(steps, func() error {
			environment.api.IOHIDDeviceUnscheduleFromRunLoop(
				resources.ref,
				session.runLoop,
				session.mode,
			)

			return nil
		})
	}

	steps = append(steps, environment.deviceResourcesCloseSteps(resources)...)

	return errors.Join(cleanupSteps(steps...))
}

func cleanupSteps(steps ...func() error) error {
	var failures []error

	for index := range steps {
		var err error

		err = resultError(safeCall(func() (any, error) { return nil, steps[index]() }))

		failures = append(failures, err)
	}

	return errors.Join(failures...)
}

func (environment *nativeState) sessionClose(session *session) error {
	failures := environment.sessionCloseCaptures(session)

	failures = append(failures, environment.sessionReleaseManager(session))

	for index := range session.keys {
		failures = append(failures, releaseNative(uintptr(session.keys[index])))
	}

	if session.mode != nativeZero {
		failures = append(failures, releaseNative(uintptr(session.mode)))
	}

	return errors.Join(failures...)
}

func backendClose(backend *backend) error {
	backend.closeOnce.Do(func() { close(backend.stop) })
	<-backend.done

	return backend.closeErr
}

func pointer(value uintptr) unsafe.Pointer {
	// IOHID and CoreFoundation bindings represent opaque native pointers as
	// uintptr. Reinterpret that representation without Go-pointer arithmetic.
	return *(*unsafe.Pointer)(unsafe.Pointer(&value))
}

func statusError(status int32) error {
	switch status {
	case nativeZero:
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

func (environment *nativeState) backendAwaitReady(
	ctx context.Context,
	backend *backend,
) (ports.Backend, error) {
	select {
	case err := <-backend.ready:
		if err != nil {
			<-backend.done

			return nil, err
		}

		result0, callErr := environment.backendReadyBackend(ctx, backend)
		if callErr != nil {
			return nil, errors.Join(callErr)
		}

		return result0, nil
	case <-ctx.Done():
		_ = backendClose(backend)

		return nil, errors.Join(ctx.Err())
	}
}

func (environment *nativeState) backendReadyBackend(
	ctx context.Context,
	backend *backend,
) (ports.Backend, error) {
	err := ctx.Err()
	if err != nil {
		_ = backendClose(backend)

		return nil, errors.Join(err)
	}

	return environment.backendView(backend), nil
}

func (environment *nativeState) sessionFinish(session *session, recovered any) {
	if recovered != nil {
		session.backend.closeErr = fmt.Errorf(
			"%w: IOHID session panic: %v",
			domain.ErrEventLoss,
			recovered,
		)
		for capture := range session.captures {
			capture.sink.Fail(session.backend.closeErr)
		}
	}

	var cleanupErr error

	cleanupErr = resultError(
		safeCall(func() (any, error) { return nil, environment.sessionClose(session) }),
	)

	session.backend.closeErr = errors.Join(session.backend.closeErr, cleanupErr)
}

func sessionRunJobs(session *session) {
	for {
		select {
		case <-session.backend.stop:
			return
		case job := <-session.backend.jobs:
			job(session)
		default:
			cf.CFRunLoopRunInMode(cf.CFRunLoopMode(session.mode), runLoopInterval, true)
		}
	}
}

func sessionCreateRunLoop(session *session) error {
	session.runLoop = cf.CFRunLoopGetCurrent()
	session.mode = cf.CFStringCreateWithCString(nativeZero, "goinput.IOHID", utf8Encoding)

	if session.runLoop == nativeZero || session.mode == nativeZero {
		return errors.New("IOHID: unable to create event loop")
	}

	return nil
}

func (environment *nativeState) sessionCreateManager(session *session) error {
	session.manager = environment.api.IOHIDManagerCreate(nativeZero, nativeZero)
	if session.manager == nativeZero {
		return errors.New("IOHID: unable to create manager")
	}

	environment.api.IOHIDManagerSetDeviceMatching(session.manager, nativeZero)
	environment.api.IOHIDManagerScheduleWithRunLoop(session.manager, session.runLoop, session.mode)

	return nil
}

func (environment *nativeState) sessionAnchorClock(session *session) {
	before := time.Now()

	session.anchorTicks = environment.machAbsoluteTime()

	after := time.Now()

	session.anchor = before.Add(after.Sub(before) / nativeTwo)
}

func (environment *nativeState) initializeSymbols() error {
	err := environment.loadClock()
	if err != nil {
		return fmt.Errorf("initializeSymbols: %w", err)
	}

	library := environment.loadFramework()

	err = environment.loadHIDFunctions(library)
	if err != nil {
		return fmt.Errorf("initializeSymbols: %w", err)
	}

	// IOHIDValueCallback's four-argument C ABI forwards the three values we use.
	environment.valueCallback = purego.NewCallback(
		func(token uintptr, status int32, _ uintptr, value uintptr) {
			environment.inputValueCallback(token, status, value)
		},
	)
	environment.removalCallback = purego.NewCallback(environment.deviceRemovalCallback)

	return nil
}

func (environment *nativeState) loadClock() error {
	system, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return fmt.Errorf("%w: load macOS clock: %v", domain.ErrUnsupported, err)
	}

	environment.nativeLibraryHandles = append(environment.nativeLibraryHandles, system)
	if err := bind(system, "mach_absolute_time", &environment.machAbsoluteTime); err != nil {
		return errors.Join(err)
	}

	if err := bind(system, "mach_timebase_info", &environment.machTimebaseInfo); err != nil {
		return errors.Join(err)
	}

	return errors.Join(environment.validateTimebase())
}

func (environment *nativeState) validateTimebase() error {
	if environment.machTimebaseInfo(&environment.timebase) != nativeZero ||
		environment.timebase.Numer == nativeZero ||
		environment.timebase.Denom == nativeZero {
		return fmt.Errorf("%w: invalid macOS clock timebase", domain.ErrUnsupported)
	}

	return nil
}

func (environment *nativeState) loadFramework() uintptr {
	library, err := purego.Dlopen(
		"/System/Library/Frameworks/IOKit.framework/IOKit",
		purego.RTLD_NOW|purego.RTLD_LOCAL,
	)
	if err == nil {
		environment.nativeLibraryHandles = append(environment.nativeLibraryHandles, library)
		// Use a writable output buffer instead of the generated string argument.
		_ = bind(library, "IORegistryEntryGetPath", &environment.registryPath)
	}

	return library
}

func (environment *nativeState) setNativeAPI() {
	environment.api = defaultNativeAPI()
}

func (environment *nativeState) bindCallbacks(library uintptr) error {
	err := bind(
		library,
		"IOHIDDeviceRegisterInputValueCallback",
		&environment.api.IOHIDDeviceRegisterInputValueCallback,
	)
	if err != nil {
		return fmt.Errorf("bindCallbacks: %w", err)
	}

	return errors.Join(bind(
		library,
		"IOHIDDeviceRegisterRemovalCallback",
		&environment.api.IOHIDDeviceRegisterRemovalCallback,
	))
}

func (environment *nativeState) bindNativeAPI(library uintptr) error {
	// The generated bindings use a lowercase framework path; retain their types
	// while binding the symbols from the canonical framework path.
	value := reflect.ValueOf(&environment.api).Elem()
	for index := nativeZero; index < value.NumField(); index++ {
		err := bind(library, value.Type().Field(index).Name, value.Field(index).Addr().Interface())
		if err != nil {
			return fmt.Errorf("bindNativeAPI: %w", err)
		}
	}

	return nil
}

func backendSendRequest(ctx context.Context, backend *backend, request *request) error {
	select {
	case <-backend.done:
		return domain.ErrClosed
	case <-backend.stop:
		return domain.ErrClosed
	case <-ctx.Done():
		return errors.Join(ctx.Err())
	case backend.jobs <- request.Execute:
		return nil
	}
}

func (environment *nativeState) backendAwaitResponse(
	ctx context.Context,
	backend *backend,
	request *request,
) (any, error) {
	select {
	case result := <-request.result:
		if result.err != nil {
			return nil, result.err
		}

		return result.value, nil
	case <-backend.done:
		return nil, domain.ErrClosed
	case <-ctx.Done():
		go environment.backendDiscardResponse(backend, request)

		return nil, errors.Join(ctx.Err())
	}
}

func (environment *nativeState) backendDiscardResponse(backend *backend, request *request) {
	// Close a capture whose caller stopped waiting while native Open executed.
	select {
	case result := <-request.result:
		if capture, ok := result.value.(*capture); ok {
			_ = environment.captureClose(capture)
		}
	case <-backend.done:
	}
}

func backendRunDiscovery(ctx context.Context,
	backend *backend,
	operation func(context.Context) error,
) error {
	if backend.retrier == nil {
		return errors.Join(operation(ctx))
	}

	return errors.Join(backend.retrier.Do(ctx, operation, func(error) bool { return false }))
}

func (environment *nativeState) backendDiscoverDevices(
	ctx context.Context,
	backend *backend,
) ([]domain.DeviceInfo, error) {
	result, err := environment.backendCall(
		ctx,
		backend,
		func(s *session) (any, error) { return environment.sessionDiscover(ctx, s) },
	)
	if err != nil {
		return nil, fmt.Errorf("discoverDevices: %w", err)
	}

	devices, ok := result.([]domain.DeviceInfo)
	if !ok {
		return nil, domain.ErrUnsupported
	}

	return devices, nil
}

func setDevices(set cf.CFSetRef, count int) []native.IOHIDDeviceRef {
	if count == nativeZero {
		return nil
	}

	values := make([]uintptr, count)
	cf.CFSetGetValues(set, unsafe.Pointer(&values[nativeZero]))

	devices := make([]native.IOHIDDeviceRef, count)

	for index := range values {
		devices[index] = native.IOHIDDeviceRef(values[index])
	}

	return devices
}

func releaseSet(set cf.CFSetRef) {
	if set != nativeZero {
		cf.CFRelease(pointer(uintptr(set)))
	}
}

func (environment *nativeState) sessionDeviceInfos(ctx context.Context,
	session *session,
	devices []native.IOHIDDeviceRef,
) ([]domain.DeviceInfo, error) {
	result := make([]domain.DeviceInfo, nativeZero, len(devices))
	for index := range devices {
		if err := ctx.Err(); err != nil {
			return nil, errors.Join(err)
		}

		info, _, err := environment.sessionDeviceInfo(session, devices[index])
		if err == nil {
			result = append(result, info)
		}
	}

	slices.SortFunc(result, func(a, b domain.DeviceInfo) int { return cmp.Compare(a.ID, b.ID) })

	return result, nil
}

func (environment *nativeState) sessionFindDevice(ctx context.Context,
	session *session,
	id domain.DeviceID,
) (deviceReference, error) {
	inventory, err := environment.sessionDevices(session)
	if err != nil {
		return deviceReference{}, fmt.Errorf("findDevice: %w", err)
	}
	defer releaseSet(inventory.set)

	ref, err := environment.sessionMatchDevice(
		ctx,
		session,
		deviceSearch{devices: inventory.devices, id: id},
	)
	if err == nil && ref.ref == nativeZero {
		err = &domain.OpError{Op: operationOpen, DeviceID: id, Err: domain.ErrNotFound}
	}

	if err != nil {
		return deviceReference{}, err
	}

	return ref, nil
}

func (environment *nativeState) sessionMatchDevice(ctx context.Context,
	session *session,
	args deviceSearch,
) (deviceReference, error) {
	for index := range args.devices {
		if err := ctx.Err(); err != nil {
			return deviceReference{}, errors.Join(err)
		}

		info, _, err := environment.sessionDeviceInfo(session, args.devices[index])
		if matchesDevice(&info, args.id, err) {
			service := environment.api.IOHIDDeviceGetService(args.devices[index])

			return deviceReference{ref: environment.api.IOHIDDeviceCreate(nativeZero, service)}, nil
		}
	}

	return deviceReference{ref: 0}, nil
}

func (environment *nativeState) sessionOpenDevice(ctx context.Context,
	session *session,
	args captureSetup,
) (result *capture, err error) {
	defer func() {
		result, err = environment.finishCaptureOpen(
			session,
			args.resources,
			result,
			openFailure{err: err, recovered: recover()},
		)
	}()

	result, err = environment.sessionPrepareCapture(session, args.resources, args.sink)
	if err != nil {
		return nil, fmt.Errorf("prepare IOHID capture: %w", err)
	}

	err = environment.sessionScheduleCapture(session, args.resources, result)
	if err != nil {
		return nil, fmt.Errorf("schedule IOHID capture: %w", err)
	}

	committed, err := sessionCommitCapture(ctx, session, result)

	return committed, errors.Join(err)
}

func (environment *nativeState) finishCaptureOpen(
	session *session,
	resources *deviceResources,
	result *capture,
	failure openFailure,
) (*capture, error) {
	err := environment.sessionAbortOpen(session, resources, failure)

	return successfulCapture(result, err), errors.Join(err)
}

func (environment *nativeState) sessionPrepareCapture(session *session,
	resources *deviceResources,
	sink ports.EventSink,
) (*capture, error) {
	err := environment.deviceResourcesOpen(resources)
	if err != nil {
		return nil, fmt.Errorf("open IOHID device: %w", err)
	}

	result, err := environment.sessionNewCapture(session, resources.ref, sink)
	if err != nil {
		return nil, errors.Join(err)
	}

	return result, nil
}

func (environment *nativeState) deviceResourcesOpen(resources *deviceResources) error {
	err := statusError(environment.api.IOHIDDeviceOpen(resources.ref, nativeZero))
	if err == nil {
		resources.opened = true
	}

	return errors.Join(err)
}

func (environment *nativeState) sessionAbortOpen(
	session *session,
	resources *deviceResources,
	args openFailure,
) error {
	if args.recovered != nil {
		args.err = fmt.Errorf("%w: IOHID open: %v", domain.ErrUnsupported, args.recovered)
	}

	if args.err != nil {
		if capture := environment.registeredCapture(resources.token); capture != nil {
			capture.closed.Store(true)
		}

		args.err = errors.Join(args.err, environment.sessionReleaseDevice(session, resources))
		environment.callbackRegistry.Delete(resources.token)
	}

	return args.err
}

func (environment *nativeState) sessionNewCapture(session *session,
	ref native.IOHIDDeviceRef,
	sink ports.EventSink,
) (*capture, error) {
	info, metadata, err := environment.sessionDeviceInfo(session, ref)
	if err != nil {
		return nil, fmt.Errorf("newCapture: %w", err)
	}

	capture := newCaptureState(session, ref, sink, info, metadata)

	if err := environment.captureLoadCapabilities(capture); err != nil {
		return nil, errors.Join(err)
	}

	return capture, nil
}

func (environment *nativeState) sessionScheduleCapture(
	session *session,
	resources *deviceResources,
	capture *capture,
) error {
	resources.token = uintptr(environment.nextCallbackToken.Add(nativeOne))
	if resources.token == nativeZero {
		return fmt.Errorf("%w: IOHID callback token exhausted", domain.ErrUnsupported)
	}

	capture.token = resources.token
	environment.callbackRegistry.Store(resources.token, capture)
	environment.sessionRegisterCallbacks(session, resources)

	return nil
}

func sessionCommitCapture(
	ctx context.Context,
	session *session,
	capture *capture,
) (*capture, error) {
	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("commitCapture: %w", err)
	}

	session.captures[capture] = struct{}{}

	return capture, nil
}

func (environment *nativeState) registeredCapture(token uintptr) *capture {
	owner, ok := environment.callbackRegistry.Load(token)
	if !ok {
		return nil
	}

	capture, ok := owner.(*capture)
	if !ok {
		return nil
	}

	return capture
}

func (environment *nativeState) sessionEndpointInfo(
	session *session,
	ref native.IOHIDDeviceRef,
) domain.DeviceInfo {
	return domain.DeviceInfo{
		ID:   "",
		Path: "",
		Name: environment.sessionStringProperty(session,
			ref,
			"Product",
		),
		Manufacturer: environment.sessionStringProperty(session, ref, "Manufacturer"),
		Serial:       environment.sessionStringProperty(session, ref, "SerialNumber"),
		VendorID: environment.sessionIdentifier16(session,
			ref,
			"VendorID",
		),
		ProductID: environment.sessionIdentifier16(session, ref, "ProductID"),
		Transport: deviceTransport(environment.sessionStringProperty(session, ref, "Transport")),
		Classes:   environment.deviceClasses(ref),
	}
}

func (environment *nativeState) devicePath(service uint32) string {
	if environment.registryPath == nil {
		return ""
	}

	buffer := make([]byte, registryPathCapacity)
	if environment.registryPath(service, "IOService", &buffer[nativeZero]) != nativeZero {
		return ""
	}

	return nulString(buffer)
}

func (environment *nativeState) sessionIdentifier16(
	session *session,
	ref native.IOHIDDeviceRef,
	key string,
) *uint16 {
	return sessionIdentifier[uint16](environment, session, ref, key)
}

func (environment *nativeState) sessionIdentifier32(
	session *session,
	ref native.IOHIDDeviceRef,
	key string,
) *uint32 {
	return sessionIdentifier[uint32](environment, session, ref, key)
}

func (environment *nativeState) sessionUsageProperty(
	session *session,
	ref native.IOHIDDeviceRef,
	key string,
) uint32 {
	value := environment.sessionIdentifier32(session, ref, key)
	if value == nil {
		return nativeZero
	}

	return *value
}

func (environment *nativeState) sessionEndpointMetadata(
	session *session,
	ref native.IOHIDDeviceRef,
) extension.Metadata {
	return extension.Metadata{
		Elements: nil, RegistryEntryID: nativeZero,
		LocationID:       environment.sessionIdentifier32(session, ref, "LocationID"),
		PrimaryUsagePage: environment.sessionUsageProperty(session, ref, "PrimaryUsagePage"),
		PrimaryUsage:     environment.sessionUsageProperty(session, ref, "PrimaryUsage"),
	}
}

func deviceTransport(name string) domain.Transport {
	return map[string]domain.Transport{
		"usb": domain.TransportUSB, "bluetooth": domain.TransportBluetooth,
		"bluetooth low energy": domain.TransportBluetooth, "i2c": domain.TransportI2C,
		"virtual": domain.TransportVirtual,
	}[strings.ToLower(name)]
}

func (environment *nativeState) deviceClasses(ref native.IOHIDDeviceRef) []domain.DeviceClass {
	var classes []domain.DeviceClass

	candidates := deviceClassCandidates()
	for index := range candidates {
		if environment.matchesClass(ref, candidates[index].usage) &&
			!containsClass(classes, candidates[index].class) {
			classes = append(classes, candidates[index].class)
		}
	}

	if len(classes) == nativeZero {
		return []domain.DeviceClass{domain.ClassOther}
	}

	return classes
}

func matchingElements(array cf.CFArrayRef) ([]native.IOHIDElementRef, error) {
	count := cf.CFArrayGetCount(array)
	if count < nativeZero || count > maxNativeElements {
		return nil, fmt.Errorf("IOHID: invalid element count %d", count)
	}

	elements := make([]native.IOHIDElementRef, count)
	for index := range elements {
		elements[index] = native.IOHIDElementRef(uintptr(cf.CFArrayGetValueAtIndex(array, index)))
	}

	return elements, nil
}

func (environment *nativeState) captureBuildControls(
	capture *capture,
	elements []native.IOHIDElementRef,
) {
	multipliers := environment.captureResolutionMultipliers(capture, elements)

	capture.caps = domain.Capabilities{
		Controls: nil,
		Complete: true,
		Repeat:   domain.SupportUnsupported,
	}

	for index := range elements {
		if environment.inputElement(elements[index]) {
			environment.captureAddControl(capture, elements[index], multipliers)
		}
	}

	sortCaptureControls(capture)
}

func (environment *nativeState) inputElement(element native.IOHIDElementRef) bool {
	kind := environment.api.IOHIDElementGetType(element)

	return kind >= nativeOne && kind <= nativeFour
}

func (environment *nativeState) captureResolutionMultipliers(capture *capture,
	elements []native.IOHIDElementRef,
) map[multiplierKey]multiplierValue {
	multipliers := make(map[multiplierKey]multiplierValue)

	for index := range elements {
		if environment.resolutionElement(elements[index]) {
			key := environment.wheelCollection(elements[index])
			value, valid := environment.readMultiplier(capture.ref, elements[index])

			var duplicate bool

			_, duplicate = multipliers[key]

			multipliers[key] = multiplierValue{value: value, valid: valid && !duplicate}
		}
	}

	return multipliers
}

func (environment *nativeState) resolutionElement(element native.IOHIDElementRef) bool {
	return environment.api.IOHIDElementGetType(element) == elementFeature &&
		environment.api.IOHIDElementGetUsagePage(element) == uint32(domain.PageGenericDesktop) &&
		environment.api.IOHIDElementGetUsage(element) == usageResolutionMultiplier
}

func (environment *nativeState) captureAddControl(capture *capture,
	element native.IOHIDElementRef,
	multipliers map[multiplierKey]multiplierValue,
) {
	item := environment.newElementControl(element)
	environment.elementControlClassify(&item)
	environment.elementControlResolveMultiplier(&item, multipliers)

	metadata := environment.elementMetadata(element, item.control.ID)
	if !exactScalar(&metadata) {
		item.control.Support = domain.SupportUnsupported
	}

	capture.controls[metadata.Cookie] = item
	capture.caps.Controls = append(capture.caps.Controls, item.control)
	capture.metadata.Elements = append(capture.metadata.Elements, metadata)
}

func (environment *nativeState) newElementControl(element native.IOHIDElementRef) elementControl {
	control := environment.elementControlDescriptor(element)
	environment.assignUsage(&control, element)
	environment.assignAxis(&control, element)

	if control.Name == "" {
		control.Name = control.Usage.String()
	}

	return elementControl{
		control:    control,
		element:    element,
		hasNull:    environment.api.IOHIDElementHasNullState(element),
		multiplier: identityMultiplier,
	}
}

func (environment *nativeState) assignUsage(
	control *domain.Control,
	element native.IOHIDElementRef,
) {
	page, usage := environment.api.IOHIDElementGetUsagePage(
		element,
	), environment.api.IOHIDElementGetUsage(
		element,
	)
	if page <= math.MaxUint16 && usage <= math.MaxUint16 {
		control.Usage = domain.HID(uint16(page), uint16(usage))
	} else {
		control.Mapping = domain.MappingUnknown
	}
}

func (environment *nativeState) assignAxis(
	control *domain.Control,
	element native.IOHIDElementRef,
) {
	if environment.api.IOHIDElementIsRelative(element) {
		control.Mode, control.Unit = domain.AxisRelative, domain.UnitCounts
	} else {
		control.Mode = domain.AxisAbsolute
		control.Range = &domain.Range{
			Min: int64(
				environment.api.IOHIDElementGetLogicalMin(element),
			),
			Max: int64(environment.api.IOHIDElementGetLogicalMax(element)),
		}
	}
}

func (environment *nativeState) elementControlClassify(item *elementControl) {
	page, usage := environment.api.IOHIDElementGetUsagePage(
		item.element,
	), environment.api.IOHIDElementGetUsage(
		item.element,
	)
	if elementControlClassifyDigital(item, page, usage) {
		return
	}

	if page == uint32(domain.PageGenericDesktop) && usage == uint32(domain.HatSwitch.ID()) {
		environment.elementControlClassifyHat(item)

		return
	}

	environment.elementControlClassifyAnalog(item, page, usage)
}

func elementControlClassifyDigital(item *elementControl, page, usage uint32) bool {
	switch {
	case keyboardUsage(page, usage):
		elementControlDigital(item, domain.ControlKey)
	case page == uint32(domain.PageButton):
		elementControlDigital(item, domain.ControlButton)
	case directionalButton(page, usage):
		elementControlDigital(item, domain.ControlButton)
	default:
		return false
	}

	return true
}

func directionalButton(page, usage uint32) bool {
	return page == uint32(domain.PageGenericDesktop) &&
		usage >= uint32(domain.DPadUp.ID()) && usage <= uint32(domain.DPadLeft.ID())
}

func elementControlDigital(item *elementControl, kind domain.ControlKind) {
	item.control.Kind, item.control.Unit = kind, domain.UnitBoolean
	item.control.Mode, item.control.Range = domain.AxisUnknown, nil
}

func (environment *nativeState) elementControlClassifyHat(item *elementControl) {
	logical := environment.elementRange(item.element)
	direction, valid := domain.Hat(logical.Min, logical, item.hasNull)

	item.control.Kind = domain.ControlAxis

	if valid && direction == domain.HatNorth {
		item.control.Kind, item.control.Unit = domain.ControlHat, domain.UnitDirection
	}
}

func (environment *nativeState) elementRange(element native.IOHIDElementRef) domain.Range {
	return domain.Range{
		Min: int64(
			environment.api.IOHIDElementGetLogicalMin(element),
		),
		Max: int64(environment.api.IOHIDElementGetLogicalMax(element)),
	}
}

func (environment *nativeState) elementControlClassifyAnalog(
	item *elementControl,
	page, usage uint32,
) {
	kind := environment.api.IOHIDElementGetType(item.element)
	switch {
	case kind == nativeTwo:
		elementControlDigital(item, domain.ControlButton)
	case analogElement(kind, page, usage) || item.control.Usage == domain.AxisPan:
		item.control.Kind = domain.ControlAxis
	default:
		environment.elementControlClassifyConsumer(item, page)
	}
}

func analogElement(kind native.IOHIDElementType, page, usage uint32) bool {
	if kind == elementInputAxis || page == uint32(domain.PageSimulation) {
		return true
	}

	return page == uint32(domain.PageGenericDesktop) &&
		usage >= uint32(domain.AxisX.ID()) && usage <= uint32(domain.AxisWheel.ID())
}

func booleanRange(logical domain.Range) bool {
	return logical.Min == nativeZero && logical.Max == nativeOne
}

func (environment *nativeState) elementControlResolveMultiplier(
	item *elementControl,
	multipliers map[multiplierKey]multiplierValue,
) {
	if !elementControlRelativeWheel(item) {
		return
	}

	resolution, exists := multipliers[environment.wheelCollection(item.element)]
	if !exists {
		item.control.Unit = domain.UnitDetents

		return
	}

	if resolution.valid {
		item.multiplier, item.control.Unit = resolution.value, domain.UnitDetents
	} else {
		item.multiplier = nativeZero
	}
}

func elementControlRelativeWheel(item *elementControl) bool {
	return item.control.Mode == domain.AxisRelative &&
		(item.control.Usage == domain.AxisWheel || item.control.Usage == domain.AxisPan)
}

func (environment *nativeState) elementMetadata(
	element native.IOHIDElementRef,
	id domain.ControlID,
) extension.Element {
	return extension.Element{
		ControlID: string(id),
		Cookie:    environment.api.IOHIDElementGetCookie(element),
		ReportID: environment.api.IOHIDElementGetReportID(
			element,
		),
		ReportSize:      environment.api.IOHIDElementGetReportSize(element),
		ReportCount:     environment.api.IOHIDElementGetReportCount(element),
		PhysicalMinimum: int64(environment.api.IOHIDElementGetPhysicalMin(element)),
		PhysicalMaximum: int64(environment.api.IOHIDElementGetPhysicalMax(element)),
		Unit: environment.api.IOHIDElementGetUnit(
			element,
		),
		UnitExponent: unitExponent(environment.api.IOHIDElementGetUnitExponent(element)),
		HasNullState: environment.api.IOHIDElementHasNullState(element),
	}
}

func exactScalar(metadata *extension.Element) bool {
	// A float64 scalar can represent arbitrary integers through 53 significant bits.
	return metadata.ReportSize <= exactIntegerBits &&
		uint64(metadata.ReportSize)*uint64(metadata.ReportCount) <= exactIntegerBits
}

func (environment *nativeState) scalarValue(value native.IOHIDValueRef) bool {
	return value != nativeZero && environment.api.IOHIDValueGetLength(value) <= nativeEight
}

func (environment *nativeState) elementMultiplier(
	element native.IOHIDElementRef,
	value native.IOHIDValueRef,
) (float64, bool) {
	logicalRange := environment.elementRange(element)
	low, high := float64(logicalRange.Min), float64(logicalRange.Max)
	logical := float64(environment.api.IOHIDValueGetIntegerValue(value))

	if high <= low || logical < low || logical > high {
		return nativeZero, false
	}

	multiplier := environment.physicalMultiplier(element, (logical-low)/(high-low))

	return multiplier, validMultiplier(multiplier)
}

func (environment *nativeState) physicalMultiplier(
	element native.IOHIDElementRef,
	fraction float64,
) float64 {
	low := float64(environment.api.IOHIDElementGetPhysicalMin(element))
	high := float64(environment.api.IOHIDElementGetPhysicalMax(element))
	exponent := unitExponent(environment.api.IOHIDElementGetUnitExponent(element))

	return (fraction*(high-low) + low) * math.Pow10(int(exponent))
}

func validMultiplier(multiplier float64) bool {
	return multiplier != nativeZero && !math.IsNaN(multiplier) &&
		!math.IsInf(multiplier, nativeZero)
}

func recoverCallback(capture *capture, recovered any) {
	if recovered != nil && capture != nil {
		capture.sink.Fail(fmt.Errorf("%w: IOHID callback: %v", domain.ErrEventLoss, recovered))
	}
}

func (environment *nativeState) captureReceiveValue(capture *capture, value native.IOHIDValueRef) {
	if value == nativeZero {
		return
	}

	length := environment.api.IOHIDValueGetLength(value)
	if length <= nativeZero || length > nativeEight {
		return
	}

	environment.captureProcessValue(capture, environment.api.IOHIDValueGetElement(value), value)
}

func captureDecodeValue(capture *capture,
	item *elementControl,
	raw int64,
) (float64, domain.EventAction, bool) {
	switch item.control.Kind {
	case domain.ControlKey, domain.ControlButton, domain.ControlSwitch:
		return captureDecodeDigital(capture, item.control.ID, raw)
	case domain.ControlHat:
		direction, valid := domain.Hat(raw, *item.control.Range, item.hasNull)

		return float64(direction), domain.ActionChange, valid
	default:
		return float64(raw), domain.ActionChange, true
	}
}

func captureDecodeDigital(capture *capture,
	id domain.ControlID,
	raw int64,
) (float64, domain.EventAction, bool) {
	pressed := raw != nativeZero
	if previous, known := capture.pressed[id]; known && previous == pressed {
		return nativeZero, domain.ActionChange, false
	}

	capture.pressed[id] = pressed
	if pressed {
		return nativeOne, domain.ActionPress, true
	}

	return nativeZero, domain.ActionRelease, true
}

func elementControlScaleValue(item *elementControl, value float64) float64 {
	if item.control.Unit == domain.UnitDetents && item.multiplier != nativeZero {
		return value / item.multiplier
	}

	return value
}

func (environment *nativeState) captureEventTimestamp(
	capture *capture,
	value native.IOHIDValueRef,
) domain.Timestamp {
	now := time.Now()
	timestamp := domain.Timestamp{Time: now.UTC(), ReceivedAt: now, Source: domain.TimestampReceipt}
	ticks := environment.api.IOHIDValueGetTimeStamp(value)

	if ticks != nativeZero {
		if delta, valid := environment.tickDuration(ticks, capture.clock.anchorTicks); valid {
			timestamp.Time = capture.clock.anchor.Add(delta).UTC()
			timestamp.Source = domain.TimestampEstimated
		}
	}

	return timestamp
}

func (environment *nativeState) scaledDuration(delta uint64) (time.Duration, bool) {
	hi, lo := bits.Mul64(delta, uint64(environment.timebase.Numer))
	if environment.timebase.Denom == nativeZero || hi >= uint64(environment.timebase.Denom) {
		return nativeZero, false
	}

	nanos := tickQuotient(bits.Div64(hi, lo, uint64(environment.timebase.Denom)))
	if nanos > math.MaxInt64 {
		return nativeZero, false
	}

	return time.Duration(nanos), true
}

func (environment *nativeState) deviceResourcesCallbackCleanup(
	resources *deviceResources,
) []func() error {
	if resources.token == nativeZero {
		return nil
	}

	return []func() error{
		func() error {
			environment.api.IOHIDDeviceRegisterInputValueCallback(
				resources.ref,
				nativeZero,
				resources.token,
			)

			return nil
		},
		func() error {
			environment.api.IOHIDDeviceRegisterRemovalCallback(
				resources.ref,
				nativeZero,
				resources.token,
			)

			return nil
		},
	}
}

func (environment *nativeState) deviceResourcesCloseSteps(
	resources *deviceResources,
) []func() error {
	var steps []func() error

	if resources.opened {
		steps = append(steps, func() error {
			return statusError(environment.api.IOHIDDeviceClose(resources.ref, nativeZero))
		})
	}

	return append(steps, func() error {
		cf.CFRelease(pointer(uintptr(resources.ref)))

		return nil
	})
}

func (environment *nativeState) sessionCloseCaptures(session *session) []error {
	var failures []error

	for capture := range session.captures {
		capture.sink.Fail(domain.ErrClosed)

		var err error

		err = resultError(safeCall(
			func() (any, error) { return nil, environment.sessionCloseCapture(session, capture) },
		))

		failures = append(failures, err)
	}

	return failures
}

func releaseNative(handle uintptr) error {
	return errors.Join(cleanupSteps(func() error {
		cf.CFRelease(pointer(handle))

		return nil
	}))
}

func (environment *nativeState) sessionReleaseManager(session *session) error {
	if session.manager == nativeZero {
		return nil
	}

	return errors.Join(cleanupSteps(func() error {
		return environment.sessionUnscheduleManager(session)
	}, func() error {
		cf.CFRelease(pointer(uintptr(session.manager)))

		return nil
	}))
}

func (environment *nativeState) sessionUnscheduleManager(session *session) error {
	if session.runLoop != nativeZero && session.mode != nativeZero {
		environment.api.IOHIDManagerUnscheduleFromRunLoop(
			session.manager,
			session.runLoop,
			session.mode,
		)
	}

	return nil
}

func matchesDevice(info *domain.DeviceInfo, id domain.DeviceID, err error) bool {
	return err == nil && info.ID == id
}

func keyboardUsage(page, usage uint32) bool {
	return page == uint32(domain.PageKeyboard) && usage != nativeZero
}

func (environment *nativeState) elementControlClassifyConsumer(item *elementControl, page uint32) {
	if page == uint32(domain.PageConsumer) && booleanRange(environment.elementRange(item.element)) {
		elementControlDigital(item, domain.ControlKey)
	}
}

func (environment *nativeState) sessionRegisterCallbacks(
	session *session,
	resources *deviceResources,
) {
	environment.api.IOHIDDeviceRegisterInputValueCallback(
		resources.ref,
		environment.valueCallback,
		resources.token,
	)
	environment.api.IOHIDDeviceRegisterRemovalCallback(
		resources.ref,
		environment.removalCallback,
		resources.token,
	)
	environment.api.IOHIDDeviceScheduleWithRunLoop(resources.ref, session.runLoop, session.mode)

	resources.scheduled = true
}

func successfulCapture(result *capture, err error) *capture {
	if err != nil {
		return nil
	}

	return result
}

func newSession(backend *backend) *session {
	return &session{
		anchor:      time.Time{},
		manager:     nativeZero,
		runLoop:     nativeZero,
		mode:        nativeZero,
		anchorTicks: nativeZero,
		backend:     backend,
		keys:        make(map[string]cf.CFStringRef),
		captures:    make(map[*capture]struct{}),
	}
}

func newRequest(ctx context.Context, perform func(*session) (any, error)) request {
	return request{
		ctx:     ctx,
		perform: perform,
		result:  make(chan response, nativeOne),
		pack:    packResponse,
	}
}

func (environment *nativeState) openedCapture(result any) (ports.Capture, error) {
	capture, ok := result.(*capture)
	if !ok {
		return nil, domain.ErrUnsupported
	}

	return environment.captureView(capture), nil
}

func unavailableCapabilities() domain.Capabilities {
	return domain.Capabilities{Controls: nil, Complete: false, Repeat: domain.SupportUnsupported}
}

func (environment *nativeState) capturePublishValue(
	capture *capture,
	item *elementControl,
	output float64,
	action domain.EventAction,
	value native.IOHIDValueRef,
) {
	capture.sink.Publish(&domain.Event{
		DeviceID:  capture.info.ID,
		ControlID: item.control.ID,
		Action:    action,
		Value: elementControlScaleValue(
			item,
			output,
		),
		Timestamp: environment.captureEventTimestamp(capture, value),
	})
}

func assignMetadata[T any](target *T, value T) bool {
	if target == nil {
		return false
	}

	*target = value

	return true
}

func sessionIdentifier[T ~uint16 | ~uint32](
	environment *nativeState,
	session *session,
	ref native.IOHIDDeviceRef,
	key string,
) *T {
	value, ok := environment.sessionNumberProperty(session, ref, key)
	if !ok || value < nativeZero || uint64(value) > uint64(^T(0)) {
		return nil
	}

	identifier := T(value)

	return &identifier
}

func deviceClassCandidates() []classCandidate {
	return []classCandidate{
		{usageKeyboard, domain.ClassKeyboard},
		{usageKeypad, domain.ClassKeyboard},
		{nativeTwo, domain.ClassMouse},
		{nativeFour, domain.ClassJoystick},
		{usageGamepad, domain.ClassGamepad},
	}
}

func (environment *nativeState) matchesClass(ref native.IOHIDDeviceRef, usage uint32) bool {
	return environment.api.IOHIDDeviceConformsTo(ref, uint32(domain.PageGenericDesktop), usage)
}

func sortCaptureControls(capture *capture) {
	slices.SortFunc(
		capture.caps.Controls,
		func(a, b domain.Control) int { return cmp.Compare(a.ID, b.ID) },
	)
	slices.SortFunc(
		capture.metadata.Elements,
		func(a, b extension.Element) int { return cmp.Compare(a.Cookie, b.Cookie) },
	)
}

func (environment *nativeState) elementControlDescriptor(
	element native.IOHIDElementRef,
) domain.Control {
	return domain.Control{
		Range: nil, Usage: domain.UsageUnknown, Mode: domain.AxisUnknown,
		ID: domain.ControlID(
			fmt.Sprintf("element:%08x", environment.api.IOHIDElementGetCookie(element)),
		),
		Name:    cfString(environment.api.IOHIDElementGetName(element)),
		Kind:    domain.ControlUnknown,
		Unit:    domain.UnitLogical,
		Support: domain.SupportSupported,
		Mapping: domain.MappingReported,
	}
}

func tickQuotient(quotient, _ uint64) uint64 { return quotient }

func (environment *nativeState) sessionServe(session *session) {
	var err error

	err = resultError(
		safeCall(func() (any, error) { return nil, environment.sessionStart(session) }),
	)
	session.backend.ready <- err

	if err == nil {
		sessionRunJobs(session)
	}
}

func newCaptureState(
	session *session,
	ref native.IOHIDDeviceRef,
	sink ports.EventSink,
	info domain.DeviceInfo,
	metadata extension.Metadata,
) *capture {
	capture := new(capture)

	capture.caps.Repeat = domain.SupportUnknown
	capture.backend, capture.ref, capture.sink = session.backend, ref, sink
	capture.clock = &captureClock{anchor: session.anchor, anchorTicks: session.anchorTicks}
	capture.info, capture.metadata = info, metadata
	capture.controls = make(map[uint32]elementControl)
	capture.pressed = make(map[domain.ControlID]bool)

	return capture
}
