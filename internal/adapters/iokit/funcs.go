// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

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

	"github.com/ebitengine/purego"
	extension "github.com/gostafa/goinput/extensions/iokit"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	cf "github.com/tmc/apple/corefoundation"
	native "github.com/tmc/apple/iokit"
)

func defaultCoreAPI() nativeCoreAPI {
	api := new(nativeCoreAPI)

	configureCoreCollections(api)
	configureCoreLoader(api)

	return *api
}

func configureCoreLoader(api *nativeCoreAPI) {
	api.openLibrary = purego.Dlopen
	api.bind = bind
	api.probeHID = func() (any, error) { return native.IOHIDManagerGetTypeID(), nil }
}

func configureCoreCollections(api *nativeCoreAPI) {
	api.runLoop = cf.CFRunLoopGetCurrent
	api.stringRef = func(value string) cf.CFStringRef {
		return cf.CFStringCreateWithCString(nativeZero, value, utf8Encoding)
	}
	api.release = releaseNative
	api.setCount = cf.CFSetGetCount
	api.setDevices = setDevices
	api.arrayCount = cf.CFArrayGetCount
	api.arrayElement = func(array cf.CFArrayRef, index int) native.IOHIDElementRef {
		return native.IOHIDElementRef(uintptr(cf.CFArrayGetValueAtIndex(array, index)))
	}
}

func coreFoundationCall[Function any](symbol string, invoke func(Function) error) (err error) {
	return errors.Join(nativeLibraryCall(coreFoundationLibrary, symbol, invoke))
}

func nativeLibraryCall[Function any](path, symbol string, invoke func(Function) error) (err error) {
	library, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return fmt.Errorf("%w: load CoreFoundation: %v", domain.ErrUnsupported, err)
	}

	defer func() { err = errors.Join(err, purego.Dlclose(library)) }()

	return errors.Join(invokeNative(library, symbol, invoke))
}

func invokeNative[Function any](library uintptr, symbol string, invoke func(Function) error) error {
	address, err := purego.Dlsym(library, symbol)
	if err != nil {
		return fmt.Errorf("%w: CoreFoundation symbol %s: %v", domain.ErrUnsupported, symbol, err)
	}

	var function Function

	purego.RegisterFunc(&function, address)

	return errors.Join(invoke(function))
}

func newBackend(
	ctx context.Context, environment *nativeState,
	retrier ports.Retrier,
) (*backendOperations, error) {
	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("newBackend: %w", err)
	}

	backend := newBackendState(retrier)

	go backendRun(environment, backend)

	result0, callErr := backendAwaitReady(ctx, environment, backend)
	if callErr != nil {
		return nil, errors.Join(callErr)
	}

	return result0, nil
}

func backendRun(environment *nativeState, backend *backend) {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()
	defer close(backend.done)

	session := newSession(backend)

	defer func() { sessionFinish(environment, session, recover()) }()

	sessionServe(environment, session)
}

func safeCall[Value any](call func() (Value, error)) (value Value, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%w: IOHID binding: %v", domain.ErrUnsupported, p)
		}
	}()

	result0, callErr := call()
	if callErr != nil {
		return value, errors.Join(callErr)
	}

	return result0, nil
}

func sessionStart(environment *nativeState, session *session) error {
	err := loadSymbols(environment)
	if err != nil {
		return fmt.Errorf("load IOHID symbols: %w", err)
	}

	return errors.Join(sessionStartResources(environment, session))
}

func sessionStartResources(environment *nativeState, session *session) error {
	err := sessionCreateRunLoop(environment, session)
	if err != nil {
		return fmt.Errorf("create IOHID run loop: %w", err)
	}

	err = sessionCreateManager(environment, session)
	if err != nil {
		return fmt.Errorf("create IOHID manager: %w", err)
	}

	sessionAnchorClock(environment, session)

	return nil
}

func loadSymbols(environment *nativeState) error {
	environment.symbolOnce.Do(func() {
		_, environment.symbolErr = safeCall(
			func() (any, error) { return nil, initializeSymbols(environment) },
		)
	})

	return environment.symbolErr
}

func loadHIDFunctions(environment *nativeState, library uintptr) error {
	setNativeAPI(environment)

	if library == nativeZero {
		return fmt.Errorf("%w: cannot load IOKit.framework", domain.ErrUnsupported)
	}

	bindingErr := bindCallbacks(environment, library)
	if bindingErr != nil {
		return errors.Join(bindingErr)
	}

	err := resultError(safeCall(environment.core.probeHID))
	if err == nil {
		return nil
	}

	return errors.Join(bindNativeAPI(environment, library))
}

func bind(library uintptr, name string, target any) error {
	address, err := purego.Dlsym(library, name)
	if err != nil {
		return fmt.Errorf("%w: macOS symbol %s: %v", domain.ErrUnsupported, name, err)
	}

	purego.RegisterFunc(target, address)

	return nil
}

func backendCall(
	ctx context.Context,
	environment *nativeState,
	args *backendCallArguments,
) (response, error) {
	request, err := submitRequest(ctx, args.backend, args.perform)
	if err != nil {
		return response{}, errors.Join(err)
	}

	value, err := backendAwaitResponse(
		ctx,
		environment,
		newBackendAwaitResponseArguments(args.backend, request),
	)

	return value, errors.Join(err)
}

func backendDiscover(
	ctx context.Context, environment *nativeState,
	backend *backend,
) ([]domain.DeviceInfo, error) {
	var devices []domain.DeviceInfo

	operation := func(ctx context.Context) error {
		var err error

		devices, err = backendDiscoverDevices(ctx, environment, backend)

		return errors.Join(err)
	}
	err := backendRunDiscovery(ctx, backend, operation)

	return devices, errors.Join(err)
}

func sessionDevices(environment *nativeState, session *session) (deviceInventory, error) {
	set := environment.api.IOHIDManagerCopyDevices(session.manager)
	if set == nativeZero {
		return deviceInventory{devices: nil, set: nativeZero}, nil
	}

	inventory, failure := inventoryFromSet(environment, set)

	return inventory, errors.Join(failure)
}

func inventoryFromSet(environment *nativeState, set cf.CFSetRef) (deviceInventory, error) {
	count := environment.core.setCount(set)
	if count < nativeZero || count > maxNativeElements {
		err := fmt.Errorf(
			"%w: invalid device count %d",
			domain.ErrUnsupported,
			count,
		)

		return deviceInventory{}, errors.Join(err, environment.core.release(uintptr(set)))
	}

	devices, err := environment.core.setDevices(set, count)
	if err != nil {
		return deviceInventory{}, errors.Join(err, environment.core.release(uintptr(set)))
	}

	return deviceInventory{devices: devices, set: set}, nil
}

func sessionDiscover(
	ctx context.Context, environment *nativeState,
	session *session,
) (infos []domain.DeviceInfo, failure error) {
	inventory, err := sessionDevices(environment, session)
	if err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}

	defer func() { failure = errors.Join(failure, releaseSet(environment, inventory.set)) }()

	result0, callErr := sessionDeviceInfos(
		ctx,
		environment,
		newSessionDeviceInfosArguments(session, inventory.devices),
	)
	if callErr != nil {
		return nil, errors.Join(callErr)
	}

	return result0, nil
}

func backendOpen(
	ctx context.Context,
	environment *nativeState,
	args *backendOpenArguments,
) (opened captureOperations, failure error) {
	if args.args.sink == nil {
		return opened, domain.ErrInvalidOptions
	}

	result, err := requestCapture(ctx, environment, args)
	if err != nil {
		return opened, fmt.Errorf(openErrorFormat, err)
	}

	opened, failure = openedCapture(ctx, environment, result.value)

	return opened, errors.Join(failure)
}

func sessionOpen(
	ctx context.Context,
	environment *nativeState,
	args *sessionOpenArguments,
) (*capture, error) {
	search := newSessionFindDeviceArguments(args.session, args.args.id)

	ref, err := sessionFindDevice(ctx, environment, search)
	if err != nil {
		return nil, fmt.Errorf(openErrorFormat, err)
	}

	request := deviceOpenRequest(args, ref.ref)

	result0, callErr := sessionOpenDevice(ctx, environment, request)
	if callErr != nil {
		return nil, errors.Join(callErr)
	}

	return result0, nil
}

func sessionDeviceInfo(environment *nativeState, session *session,
	ref native.IOHIDDeviceRef,
) (info domain.DeviceInfo, metadata extension.Metadata, failure error) {
	service := environment.api.IOHIDDeviceGetService(ref)

	registryID, err := registryIdentifier(environment, service)
	if err != nil {
		return domain.DeviceInfo{}, extension.Metadata{}, fmt.Errorf("deviceInfo: %w", err)
	}

	info = sessionEndpointInfo(environment, session, ref)

	info.ID = domain.DeviceID(fmt.Sprintf("darwin:%016x", registryID))
	info.Path = devicePath(environment, service)

	metadata = sessionEndpointMetadata(environment, session, ref)

	metadata.RegistryEntryID = registryID

	return info, metadata, nil
}

func registryIdentifier(environment *nativeState, service uint32) (uint64, error) {
	var registryID uint64

	err := statusError(environment.api.IORegistryEntryGetRegistryEntryID(service, &registryID))

	return registryID, errors.Join(err)
}

func containsClass(classes []domain.DeviceClass, target domain.DeviceClass) bool {
	return slices.Contains(classes, target)
}

func sessionKey(environment *nativeState, session *session, name string) cf.CFStringRef {
	if key := session.keys[name]; key != nativeZero {
		return key
	}

	key := environment.core.stringRef(name)
	if key != nativeZero {
		session.keys[name] = key
	}

	return key
}

func sessionStringProperty(environment *nativeState, args *sessionStringPropertyArguments) string {
	value := environment.api.IOHIDDeviceGetProperty(
		args.ref,
		sessionKey(environment, args.session, args.key),
	)
	if value == nil || cf.CFGetTypeID(value) != cf.CFStringGetTypeID() {
		return ""
	}

	return cfString(cf.CFStringRef(uintptr(value)))
}

func sessionNumberProperty(
	environment *nativeState,
	args *sessionNumberPropertyArguments,
) (int64, bool) {
	value := environment.api.IOHIDDeviceGetProperty(
		args.ref,
		sessionKey(environment, args.session, args.key),
	)
	if value == nil || cf.CFGetTypeID(value) != cf.CFNumberGetTypeID() {
		return nativeZero, false
	}

	return cfNumberValue(cf.CFNumberRef(uintptr(value)))
}

func cfString(ref cf.CFStringRef) string {
	if ref == nativeZero {
		return ""
	}

	length := cf.CFStringGetMaximumSizeForEncoding(cf.CFStringGetLength(ref), utf8Encoding)

	return nativeString(length, func(buffer []byte) bool {
		return cf.CFStringGetCString(ref, &buffer[nativeZero], len(buffer), utf8Encoding)
	})
}

func nativeString(length int, read func([]byte) bool) string {
	if length < nativeZero || length >= maxNativeString {
		return ""
	}

	buffer := make([]byte, nativeZero, length+nativeOne)

	buffer = buffer[:cap(buffer)]

	if !read(buffer) {
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

func captureLoadCapabilities(environment *nativeState, capture *capture) (failure error) {
	array := environment.api.IOHIDDeviceCopyMatchingElements(capture.ref, nativeZero, nativeZero)
	if array == nativeZero {
		capture.caps = unavailableCapabilities()

		return nil
	}

	return errors.Join(captureEnumerateControls(environment, capture, array))
}

func captureEnumerateControls(environment *nativeState,
	capture *capture,
	array cf.CFArrayRef,
) (failure error) {
	defer func() { failure = errors.Join(failure, environment.core.release(uintptr(array))) }()

	elements, err := matchingElements(environment, array)
	if err != nil {
		return fmt.Errorf("loadCapabilities: %w", err)
	}

	captureBuildControls(environment, capture, elements)

	return nil
}

func wheelCollection(environment *nativeState, element native.IOHIDElementRef) multiplierKey {
	parent := environment.api.IOHIDElementGetParent(element)
	for depth := nativeZero; parent != nativeZero && depth < maxCollectionDepth; depth++ {
		condition1 := environment.api.IOHIDElementGetType(parent) == elementCollection &&
			environment.api.IOHIDElementGetCollectionType(parent) == nativeTwo
		if condition1 {
			return multiplierKey{uint64(parent), nativeZero}
		}

		parent = environment.api.IOHIDElementGetParent(parent)
	}

	return multiplierKey{nativeZero, uint64(environment.api.IOHIDElementGetReportID(element))}
}

func unitExponent(value uint32) int32 {
	if value > unitExponentMask {
		return signedNativeValue(value)
	}

	if value >= nativeEight {
		return int32(value&unitExponentMask) - unitExponentModulus
	}

	return int32(value & unitExponentMask)
}

func signedNativeValue(value uint32) int32 {
	if value <= math.MaxInt32 {
		return int32(value)
	}

	magnitude := math.MaxUint32 - value

	return -int32(magnitude&math.MaxInt32) - nativeOne
}

func readMultiplier(environment *nativeState,
	device native.IOHIDDeviceRef,
	element native.IOHIDElementRef,
) (float64, bool) {
	var value native.IOHIDValueRef

	condition2 := environment.api.IOHIDDeviceGetValue(device, element, &value) != nativeZero ||
		!scalarValue(environment, value)

	if condition2 {
		return nativeZero, false
	}

	return elementMultiplier(environment, element, value)
}

func inputValueCallback(environment *nativeState, args *inputValueCallbackArguments) {
	capture := registeredCapture(environment, args.token)

	defer func() { recoverCallback(capture, recover()) }()

	if capture == nil || capture.closed.Load() {
		return
	}

	if args.status != nativeZero {
		capture.sink.Fail(statusError(args.status))

		return
	}

	captureReceiveValue(environment, capture, native.IOHIDValueRef(args.value))
}

// Device removal uses IOHIDCallback, which has three arguments; manager device
// notifications use the distinct four-argument IOHIDDeviceCallback ABI.
func deviceRemovalCallback(environment *nativeState, args *deviceRemovalCallbackArguments) {
	capture := registeredCapture(environment, args.token)
	if capture == nil || capture.closed.Load() {
		return
	}

	capture.sink.Fail(
		&domain.OpError{Op: operationRead, DeviceID: capture.info.ID, Err: domain.ErrDisconnected},
	)
}

func captureProcessValue(environment *nativeState, args *captureProcessValueArguments) {
	item, ok := args.capture.controls[environment.api.IOHIDElementGetCookie(args.element)]
	if !ok || item.control.Support != domain.SupportSupported {
		return
	}

	capturePublishValue(
		environment,
		newCapturePublishValueArguments(args.capture, &item, args.value),
	)
}

func tickDuration(environment *nativeState, ticks, anchor uint64) (time.Duration, bool) {
	delta := ticks - anchor
	if ticks < anchor {
		delta = anchor - ticks
	}

	duration, valid := scaledDuration(environment, delta)

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
		return assignMetadata[extension.MetadataProvider](value, captureMetadataView(capture))
	case *extension.Metadata:
		return assignMetadata(value, captureIOKitMetadata(capture))
	default:
		return false
	}
}

func captureClose(ctx context.Context, environment *nativeState, capture *capture) error {
	capture.closeOnce.Do(func() {
		capture.closed.Store(true)

		err := resultError(
			backendCall(
				context.WithoutCancel(ctx),
				environment,
				newBackendCallArguments(
					capture.backend,
					func(s *session) (any, error) { return nil, sessionCloseCapture(environment, s, capture) },
				),
			),
		)
		if !errors.Is(err, domain.ErrClosed) {
			capture.closeErr = err
		}
	})

	return capture.closeErr
}

func sessionCloseCapture(environment *nativeState, session *session, capture *capture) error {
	if _, exists := session.captures[capture]; !exists {
		return nil
	}

	delete(session.captures, capture)
	capture.closed.Store(true)

	defer environment.callbackRegistry.Delete(capture.token)

	return errors.Join(
		sessionReleaseDevice(
			environment,
			session,
			&deviceResources{ref: capture.ref, scheduled: true, opened: true, token: capture.token},
		),
	)
}

func sessionReleaseDevice(environment *nativeState,
	session *session,
	resources *deviceResources,
) error {
	steps := deviceResourcesCallbackCleanup(environment, resources)
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

	steps = append(steps, deviceResourcesCloseSteps(environment, resources)...)

	return errors.Join(cleanupSteps(steps...))
}

func cleanupSteps(steps ...func() error) error {
	failures := make([]error, nativeZero, len(steps))

	for index := range steps {
		err := resultError(safeCall(func() (any, error) { return nil, steps[index]() }))

		failures = append(failures, err)
	}

	return errors.Join(failures...)
}

func sessionClose(environment *nativeState, session *session) error {
	failures := sessionCloseCaptures(environment, session)

	failures = append(failures, sessionReleaseManager(environment, session))

	for index := range session.keys {
		failures = append(failures, environment.core.release(uintptr(session.keys[index])))
	}

	if session.mode != nativeZero {
		failures = append(failures, environment.core.release(uintptr(session.mode)))
	}

	return errors.Join(failures...)
}

func backendClose(backend *backend) error {
	backend.closeOnce.Do(func() { close(backend.stop) })
	<-backend.done

	return backend.closeErr
}

func statusError(status int32) error {
	switch status {
	case nativeZero:
		return nil
	case ioNotPermitted, ioNotPrivileged:
		return fmt.Errorf(
			"%w: IOReturn 0x%08x (Input Monitoring may be required)",
			domain.ErrPermissionDenied,
			statusBits(status),
		)
	case ioNoDevice, ioNotOpen:
		return fmt.Errorf(statusErrorFormat, domain.ErrDisconnected, statusBits(status))
	default:
		return fmt.Errorf(statusErrorFormat, domain.ErrUnsupported, statusBits(status))
	}
}

func backendAwaitReady(
	ctx context.Context, environment *nativeState,
	backend *backend,
) (*backendOperations, error) {
	select {
	case err := <-backend.ready:
		if err != nil {
			<-backend.done

			return nil, err
		}

		result0, callErr := backendReadyBackend(ctx, environment, backend)
		if callErr != nil {
			return nil, errors.Join(callErr)
		}

		return result0, nil
	case <-ctx.Done():
		return nil, errors.Join(ctx.Err(), backendClose(backend))
	}
}

func backendReadyBackend(
	ctx context.Context, environment *nativeState,
	backend *backend,
) (*backendOperations, error) {
	err := ctx.Err()
	if err != nil {
		return nil, errors.Join(err, backendClose(backend))
	}

	return backendView(environment, backend), nil
}

func sessionFinish(environment *nativeState, session *session, recovered any) {
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

	cleanupErr := resultError(
		safeCall(func() (any, error) { return nil, sessionClose(environment, session) }),
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

func sessionCreateRunLoop(environment *nativeState, session *session) error {
	session.runLoop = environment.core.runLoop()
	session.mode = environment.core.stringRef("goinput.IOHID")

	if session.runLoop == nativeZero || session.mode == nativeZero {
		return fmt.Errorf("%w: unable to create event loop", domain.ErrUnsupported)
	}

	return nil
}

func sessionCreateManager(environment *nativeState, session *session) error {
	session.manager = environment.api.IOHIDManagerCreate(nativeZero, nativeZero)
	if session.manager == nativeZero {
		return fmt.Errorf("%w: unable to create manager", domain.ErrUnsupported)
	}

	environment.api.IOHIDManagerSetDeviceMatching(session.manager, nativeZero)
	environment.api.IOHIDManagerScheduleWithRunLoop(session.manager, session.runLoop, session.mode)

	return nil
}

func sessionAnchorClock(environment *nativeState, session *session) {
	before := time.Now()

	session.anchorTicks = environment.machAbsoluteTime()

	after := time.Now()

	session.anchor = before.Add(after.Sub(before) / nativeTwo)
}

func initializeSymbols(environment *nativeState) error {
	err := loadClock(environment)
	if err != nil {
		return fmt.Errorf(symbolsErrorFormat, err)
	}

	library := loadFramework(environment)

	err = loadHIDFunctions(environment, library)
	if err != nil {
		return fmt.Errorf(symbolsErrorFormat, err)
	}

	registerNativeCallbacks(environment)

	return nil
}

func loadClock(environment *nativeState) error {
	system, err := environment.core.openLibrary(
		"/usr/lib/libSystem.B.dylib",
		purego.RTLD_NOW|purego.RTLD_LOCAL,
	)
	if err != nil {
		return fmt.Errorf("%w: load macOS clock: %v", domain.ErrUnsupported, err)
	}

	environment.nativeLibraryHandles = append(environment.nativeLibraryHandles, system)

	return errors.Join(bindClock(environment, system))
}

func bindClock(environment *nativeState, system uintptr) error {
	err := environment.core.bind(system, "mach_absolute_time", &environment.machAbsoluteTime)
	if err != nil {
		return errors.Join(err)
	}

	err = environment.core.bind(system, "mach_timebase_info", &environment.machTimebaseInfo)
	if err != nil {
		return errors.Join(err)
	}

	return errors.Join(validateTimebase(environment))
}

func validateTimebase(environment *nativeState) error {
	condition3 := environment.machTimebaseInfo(&environment.timebase) != nativeZero ||
		environment.timebase.Numer == nativeZero ||
		environment.timebase.Denom == nativeZero
	if condition3 {
		return fmt.Errorf("%w: invalid macOS clock timebase", domain.ErrUnsupported)
	}

	return nil
}

func loadFramework(environment *nativeState) uintptr {
	library, err := environment.core.openLibrary(
		"/System/Library/Frameworks/IOKit.framework/IOKit",
		purego.RTLD_NOW|purego.RTLD_LOCAL,
	)
	if err != nil {
		return nativeZero
	}

	configureFrameworkPath(environment, library)

	return library
}

func setNativeAPI(environment *nativeState) {
	environment.api = defaultNativeAPI()
}

func bindCallbacks(environment *nativeState, library uintptr) error {
	err := environment.core.bind(
		library,
		"IOHIDDeviceRegisterInputValueCallback",
		&environment.api.IOHIDDeviceRegisterInputValueCallback,
	)
	if err != nil {
		return fmt.Errorf("bindCallbacks: %w", err)
	}

	return errors.Join(environment.core.bind(
		library,
		"IOHIDDeviceRegisterRemovalCallback",
		&environment.api.IOHIDDeviceRegisterRemovalCallback,
	))
}

func bindNativeAPI(environment *nativeState, library uintptr) error {
	// The generated bindings use a lowercase framework path; retain their types
	// while binding the symbols from the canonical framework path.
	value := reflect.ValueOf(&environment.api).Elem()
	for index := nativeZero; index < value.NumField(); index++ {
		err := environment.core.bind(
			library,
			value.Type().Field(index).Name,
			value.Field(index).Addr().Interface(),
		)
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

func backendAwaitResponse(
	ctx context.Context,
	environment *nativeState,
	args *backendAwaitResponseArguments,
) (response, error) {
	select {
	case result := <-args.request.result:
		if result.err != nil {
			return response{}, result.err
		}

		return result, nil
	case <-args.backend.done:
		return response{}, domain.ErrClosed
	case <-ctx.Done():
		discardCanceledRequest(ctx, environment, args)

		return response{}, errors.Join(ctx.Err())
	}
}

func backendDiscardResponse(
	ctx context.Context,
	environment *nativeState,
	args *backendDiscardResponseArguments,
) {
	// Close a capture whose caller stopped waiting while native Open executed.
	select {
	case result := <-args.request.result:
		discardCapture(ctx, environment, result.value)
	case <-args.backend.done:
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

func backendDiscoverDevices(
	ctx context.Context, environment *nativeState,
	backend *backend,
) ([]domain.DeviceInfo, error) {
	request := discoveryRequest(ctx, environment, backend)

	result, err := backendCall(ctx, environment, request)
	if err != nil {
		return nil, fmt.Errorf("discoverDevices: %w", err)
	}

	devices, err := discoveredDevices(result.value)

	return devices, errors.Join(err)
}

func discoveredDevices(value any) ([]domain.DeviceInfo, error) {
	devices, ok := value.([]domain.DeviceInfo)
	if !ok {
		return nil, domain.ErrUnsupported
	}

	return devices, nil
}

func setDevices(set cf.CFSetRef, count int) ([]native.IOHIDDeviceRef, error) {
	devices, err := readSetDevices(count, func(values []uintptr) error {
		return coreFoundationCall("CFSetGetValues", func(read func(cf.CFSetRef, *uintptr)) error {
			read(set, &values[nativeZero])

			return nil
		})
	})

	return devices, errors.Join(err)
}

func readSetDevices(count int, read func([]uintptr) error) ([]native.IOHIDDeviceRef, error) {
	if count == nativeZero {
		return nil, nil
	}

	values := make([]uintptr, nativeZero, count)

	values = values[:cap(values)]

	err := read(values)
	if err != nil {
		return nil, errors.Join(err)
	}

	return deviceReferences(values), nil
}

func deviceReferences(values []uintptr) []native.IOHIDDeviceRef {
	devices := make([]native.IOHIDDeviceRef, nativeZero, len(values))
	for index := range values {
		devices = append(devices, native.IOHIDDeviceRef(values[index]))
	}

	return devices
}

func releaseSet(environment *nativeState, set cf.CFSetRef) error {
	if set == nativeZero {
		return nil
	}

	return errors.Join(environment.core.release(uintptr(set)))
}

func sessionDeviceInfos(
	ctx context.Context,
	environment *nativeState,
	args *sessionDeviceInfosArguments,
) ([]domain.DeviceInfo, error) {
	result := make([]domain.DeviceInfo, nativeZero, len(args.devices))
	for index := range args.devices {
		contextErr := ctx.Err()
		if contextErr != nil {
			return nil, errors.Join(contextErr)
		}

		info, _, err := sessionDeviceInfo(environment, args.session, args.devices[index])
		if err == nil {
			result = append(result, info)
		}
	}

	slices.SortFunc(result, func(a, b domain.DeviceInfo) int { return cmp.Compare(a.ID, b.ID) })

	return result, nil
}

func sessionFindDevice(
	ctx context.Context,
	environment *nativeState,
	args *sessionFindDeviceArguments,
) (result deviceReference, failure error) {
	inventory, err := sessionDevices(environment, args.session)
	if err != nil {
		return deviceReference{}, fmt.Errorf("findDevice: %w", err)
	}

	defer func() { failure = errors.Join(failure, releaseSet(environment, inventory.set)) }()

	search := &deviceSearch{devices: inventory.devices, id: args.id}
	request := newSessionMatchDeviceArguments(args.session, search)

	result, failure = findRequestedDevice(ctx, environment, request)

	return result, errors.Join(failure)
}

func sessionMatchDevice(
	ctx context.Context,
	environment *nativeState,
	args *sessionMatchDeviceArguments,
) (deviceReference, error) {
	for index := range args.args.devices {
		contextErr := ctx.Err()
		if contextErr != nil {
			return deviceReference{}, errors.Join(contextErr)
		}

		info, _, err := sessionDeviceInfo(environment, args.session, args.args.devices[index])
		if matchesDevice(&info, args.args.id, err) {
			service := environment.api.IOHIDDeviceGetService(args.args.devices[index])

			return deviceReference{ref: environment.api.IOHIDDeviceCreate(nativeZero, service)}, nil
		}
	}

	return deviceReference{ref: nativeZero}, nil
}

func sessionOpenDevice(
	ctx context.Context,
	environment *nativeState, args *sessionOpenDeviceArguments,
) (result *capture, err error) {
	defer func() {
		failure := captureOpenFailure(result, err, recover())

		result, err = finishDeviceOpen(environment, args, failure)
	}()

	result, err = prepareScheduledCapture(environment, args)
	if err != nil {
		return nil, errors.Join(err)
	}

	result, err = sessionCommitCapture(ctx, args.session, result)
	if err != nil {
		return nil, errors.Join(err)
	}

	return result, nil
}

func finishCaptureOpen(
	environment *nativeState,
	args *finishCaptureOpenArguments,
) (*capture, error) {
	err := sessionAbortOpen(
		environment,
		newSessionAbortOpenArguments(args.session, args.resources, args.failure),
	)
	if err != nil {
		return nil, errors.Join(err)
	}

	return args.failure.result, nil
}

func sessionPrepareCapture(
	environment *nativeState,
	args *sessionPrepareCaptureArguments,
) (*capture, error) {
	err := deviceResourcesOpen(environment, args.resources)
	if err != nil {
		return nil, fmt.Errorf("open IOHID device: %w", err)
	}

	result, err := sessionNewCapture(
		environment,
		newSessionNewCaptureArguments(args.sink, args.session, args.resources.ref),
	)
	if err != nil {
		return nil, errors.Join(err)
	}

	return result, nil
}

func deviceResourcesOpen(environment *nativeState, resources *deviceResources) error {
	err := statusError(environment.api.IOHIDDeviceOpen(resources.ref, nativeZero))
	if err == nil {
		resources.opened = true
	}

	return errors.Join(err)
}

func sessionAbortOpen(environment *nativeState, args *sessionAbortOpenArguments) error {
	if args.args.recovered != nil {
		args.args.err = fmt.Errorf("%w: IOHID open: %v", domain.ErrUnsupported, args.args.recovered)
	}

	if args.args.err == nil {
		return nil
	}

	capture := registeredCapture(environment, args.resources.token)
	if capture != nil {
		capture.closed.Store(true)
	}

	err := errors.Join(
		args.args.err,
		sessionReleaseDevice(environment, args.session, args.resources),
	)
	environment.callbackRegistry.Delete(args.resources.token)

	return err
}

func sessionNewCapture(
	environment *nativeState,
	args *sessionNewCaptureArguments,
) (*capture, error) {
	info, metadata, err := sessionDeviceInfo(environment, args.session, args.ref)
	if err != nil {
		return nil, fmt.Errorf("newCapture: %w", err)
	}

	capture := newCaptureState(args.session, args.ref, args.sink)

	capture.info, capture.metadata = info, metadata

	err = captureLoadCapabilities(environment, capture)
	if err != nil {
		return nil, errors.Join(err)
	}

	return capture, nil
}

func sessionScheduleCapture(environment *nativeState, args *sessionScheduleCaptureArguments) error {
	args.resources.token = uintptr(environment.nextCallbackToken.Add(nativeOne))
	if args.resources.token == nativeZero {
		return fmt.Errorf("%w: IOHID callback token exhausted", domain.ErrUnsupported)
	}

	args.capture.token = args.resources.token
	environment.callbackRegistry.Store(args.resources.token, args.capture)
	sessionRegisterCallbacks(environment, args.session, args.resources)

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

func registeredCapture(environment *nativeState, token uintptr) *capture {
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

func sessionEndpointInfo(
	environment *nativeState,
	session *session,
	ref native.IOHIDDeviceRef,
) domain.DeviceInfo {
	text := textProperties(environment, session, ref)
	identifier := integerProperties[uint16](environment, session, ref)

	return domain.DeviceInfo{
		ID:           "",
		Path:         "",
		Name:         text("Product"),
		Manufacturer: text("Manufacturer"),
		Serial:       text("SerialNumber"),
		VendorID:     identifier("VendorID"),
		ProductID:    identifier("ProductID"),
		Transport:    deviceTransport(text("Transport")),
		Classes:      deviceClasses(environment, ref),
	}
}

func textProperties(
	environment *nativeState,
	session *session,
	ref native.IOHIDDeviceRef,
) func(string) string {
	return func(key string) string {
		return sessionStringProperty(
			environment,
			newSessionStringPropertyArguments(key, session, ref),
		)
	}
}

func integerProperties[Identifier ~uint16 | ~uint32](
	environment *nativeState,
	session *session,
	ref native.IOHIDDeviceRef,
) func(string) *Identifier {
	return func(key string) *Identifier {
		return nativeIdentifier[Identifier](
			environment,
			newSessionNumberPropertyArguments(key, session, ref),
		)
	}
}

func devicePath(environment *nativeState, service uint32) string {
	if environment.registryPath == nil {
		return ""
	}

	buffer := make([]byte, nativeZero, registryPathCapacity)

	buffer = buffer[:cap(buffer)]

	if environment.registryPath(service, "IOService", &buffer[nativeZero]) != nativeZero {
		return ""
	}

	return nulString(buffer)
}

func nativeIdentifier[Identifier ~uint16 | ~uint32](
	environment *nativeState,
	args *sessionNumberPropertyArguments,
) *Identifier {
	return sessionIdentifier[Identifier](func(name string) (int64, bool) {
		return sessionNumberProperty(
			environment,
			newSessionNumberPropertyArguments(name, args.session, args.ref),
		)
	}, args.key)
}

func sessionUsageProperty(environment *nativeState, args *sessionUsagePropertyArguments) uint32 {
	value := nativeIdentifier[uint32](
		environment,
		newSessionNumberPropertyArguments(args.key, args.session, args.ref),
	)
	if value == nil {
		return nativeZero
	}

	return *value
}

func sessionEndpointMetadata(environment *nativeState,
	session *session,
	ref native.IOHIDDeviceRef,
) extension.Metadata {
	return extension.Metadata{
		Elements:        nil,
		RegistryEntryID: nativeZero,
		LocationID: nativeIdentifier[uint32](
			environment,
			newSessionNumberPropertyArguments("LocationID", session, ref),
		),
		PrimaryUsagePage: sessionUsageProperty(
			environment,
			newSessionUsagePropertyArguments("PrimaryUsagePage", session, ref),
		),
		PrimaryUsage: sessionUsageProperty(
			environment,
			newSessionUsagePropertyArguments("PrimaryUsage", session, ref),
		),
	}
}

func deviceTransport(name string) domain.Transport {
	return map[string]domain.Transport{
		"usb": domain.TransportUSB, "bluetooth": domain.TransportBluetooth,
		"bluetooth low energy": domain.TransportBluetooth, "i2c": domain.TransportI2C,
		"virtual": domain.TransportVirtual,
	}[strings.ToLower(name)]
}

func deviceClasses(environment *nativeState, ref native.IOHIDDeviceRef) []domain.DeviceClass {
	var classes []domain.DeviceClass

	candidates := deviceClassCandidates()
	for index := range candidates {
		condition4 := matchesClass(environment, ref, candidates[index].usage) &&
			!containsClass(classes, candidates[index].class)
		if condition4 {
			classes = append(classes, candidates[index].class)
		}
	}

	if len(classes) == nativeZero {
		return []domain.DeviceClass{domain.ClassOther}
	}

	return classes
}

func matchingElements(
	environment *nativeState,
	array cf.CFArrayRef,
) ([]native.IOHIDElementRef, error) {
	count := environment.core.arrayCount(array)
	if count < nativeZero || count > maxNativeElements {
		return nil, fmt.Errorf("%w: invalid element count %d", domain.ErrUnsupported, count)
	}

	elements := make([]native.IOHIDElementRef, nativeZero, count)

	elements = elements[:cap(elements)]

	for index := range elements {
		elements[index] = environment.core.arrayElement(array, index)
	}

	return elements, nil
}

func captureBuildControls(environment *nativeState,
	capture *capture,
	elements []native.IOHIDElementRef,
) {
	multipliers := captureResolutionMultipliers(environment, capture, elements)

	capture.caps = completeCapabilities()

	for index := range elements {
		if inputElement(environment, elements[index]) {
			captureAddControl(
				environment,
				newCaptureAddControlArguments(capture, multipliers, elements[index]),
			)
		}
	}

	sortCaptureControls(capture)
}

func inputElement(environment *nativeState, element native.IOHIDElementRef) bool {
	kind := environment.api.IOHIDElementGetType(element)

	return kind >= nativeOne && kind <= nativeFour
}

func captureResolutionMultipliers(environment *nativeState, capture *capture,
	elements []native.IOHIDElementRef,
) map[multiplierKey]multiplierValue {
	multipliers := make(map[multiplierKey]multiplierValue)

	for index := range elements {
		if !resolutionElement(environment, elements[index]) {
			continue
		}

		request := newCaptureAddControlArguments(capture, multipliers, elements[index])
		storeResolutionMultiplier(environment, request)
	}

	return multipliers
}

func resolutionElement(environment *nativeState, element native.IOHIDElementRef) bool {
	return environment.api.IOHIDElementGetType(element) == elementFeature &&
		environment.api.IOHIDElementGetUsagePage(element) == uint32(domain.PageGenericDesktop) &&
		environment.api.IOHIDElementGetUsage(element) == usageResolutionMultiplier
}

func captureAddControl(environment *nativeState, args *captureAddControlArguments) {
	item := newElementControl(environment, args.element)
	elementControlClassify(environment, &item)
	elementControlResolveMultiplier(environment, &item, args.multipliers)

	metadata := elementMetadata(environment, args.element, item.control.ID)
	if !exactScalar(&metadata) {
		item.control.Support = domain.SupportUnsupported
	}

	args.capture.controls[metadata.Cookie] = item
	args.capture.caps.Controls = append(args.capture.caps.Controls, item.control)
	args.capture.metadata.Elements = append(args.capture.metadata.Elements, metadata)
}

func newElementControl(environment *nativeState, element native.IOHIDElementRef) elementControl {
	control := elementControlDescriptor(environment, element)
	assignUsage(environment, &control, element)
	assignAxis(environment, &control, element)

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

func assignUsage(environment *nativeState,
	control *domain.Control,
	element native.IOHIDElementRef,
) {
	page, usage := environment.api.IOHIDElementGetUsagePage(
		element,
	), environment.api.IOHIDElementGetUsage(
		element,
	)
	if page > math.MaxUint16 || usage > math.MaxUint16 {
		control.Mapping = domain.MappingUnknown

		return
	}

	control.Usage = domain.HID(uint16(page), uint16(usage))
}

func assignAxis(environment *nativeState,
	control *domain.Control,
	element native.IOHIDElementRef,
) {
	if environment.api.IOHIDElementIsRelative(element) {
		control.Mode, control.Unit = domain.AxisRelative, domain.UnitCounts

		return
	}

	control.Mode = domain.AxisAbsolute

	logical := elementRange(environment, element)

	control.Range = &logical
}

func elementControlClassify(environment *nativeState, item *elementControl) {
	page, usage := environment.api.IOHIDElementGetUsagePage(
		item.element,
	), environment.api.IOHIDElementGetUsage(
		item.element,
	)
	if elementControlClassifyDigital(item, page, usage) {
		return
	}

	if page == uint32(domain.PageGenericDesktop) && usage == uint32(domain.HatSwitch.ID()) {
		elementControlClassifyHat(environment, item)

		return
	}

	elementControlClassifyAnalog(environment, item, [nativeTwo]uint32{page, usage})
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

func elementControlClassifyHat(environment *nativeState, item *elementControl) {
	logical := elementRange(environment, item.element)
	direction, valid := domain.Hat(logical.Min, logical, item.hasNull)

	item.control.Kind = domain.ControlAxis

	if valid && direction == domain.HatNorth {
		item.control.Kind, item.control.Unit = domain.ControlHat, domain.UnitDirection
	}
}

func elementRange(environment *nativeState, element native.IOHIDElementRef) domain.Range {
	return domain.Range{
		Min: int64(
			environment.api.IOHIDElementGetLogicalMin(element),
		),
		Max: int64(environment.api.IOHIDElementGetLogicalMax(element)),
	}
}

func elementControlClassifyAnalog(environment *nativeState,
	item *elementControl,
	usage [nativeTwo]uint32,
) {
	kind := environment.api.IOHIDElementGetType(item.element)
	switch {
	case kind == nativeTwo:
		elementControlDigital(item, domain.ControlButton)
	case analogElement(kind, usage[nativeZero], usage[nativeOne]) || item.control.Usage == domain.AxisPan:
		item.control.Kind = domain.ControlAxis
	default:
		elementControlClassifyConsumer(environment, item, usage[nativeZero])
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

func elementControlResolveMultiplier(environment *nativeState,
	item *elementControl,
	multipliers map[multiplierKey]multiplierValue,
) {
	if !elementControlRelativeWheel(item) {
		return
	}

	resolution, exists := multipliers[wheelCollection(environment, item.element)]
	if !exists {
		item.control.Unit = domain.UnitDetents

		return
	}

	applyResolutionMultiplier(item, resolution)
}

func elementControlRelativeWheel(item *elementControl) bool {
	return item.control.Mode == domain.AxisRelative &&
		(item.control.Usage == domain.AxisWheel || item.control.Usage == domain.AxisPan)
}

func elementMetadata(environment *nativeState,
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

func scalarValue(environment *nativeState, value native.IOHIDValueRef) bool {
	return value != nativeZero && environment.api.IOHIDValueGetLength(value) <= nativeEight
}

func elementMultiplier(environment *nativeState,
	element native.IOHIDElementRef,
	value native.IOHIDValueRef,
) (float64, bool) {
	logicalRange := elementRange(environment, element)
	low, high := float64(logicalRange.Min), float64(logicalRange.Max)
	logical := float64(environment.api.IOHIDValueGetIntegerValue(value))

	if high <= low || logical < low || logical > high {
		return nativeZero, false
	}

	multiplier := physicalMultiplier(environment, element, (logical-low)/(high-low))

	return multiplier, validMultiplier(multiplier)
}

func physicalMultiplier(environment *nativeState,
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

func captureReceiveValue(environment *nativeState, capture *capture, value native.IOHIDValueRef) {
	if value == nativeZero {
		return
	}

	length := environment.api.IOHIDValueGetLength(value)
	if length <= nativeZero || length > nativeEight {
		return
	}

	captureProcessValue(
		environment,
		newCaptureProcessValueArguments(
			capture,
			environment.api.IOHIDValueGetElement(value),
			value,
		),
	)
}

func captureDecodeValue(capture *capture,
	item *elementControl,
	raw int64,
) (output float64, action domain.EventAction, accepted bool) {
	kind := item.control.Kind
	if digitalControl(kind) {
		return captureDecodeDigital(capture, item.control.ID, raw)
	}

	if kind == domain.ControlHat {
		return decodeHat(item, raw)
	}

	accepted = kind == domain.ControlUnknown || kind == domain.ControlAxis
	if !accepted {
		return nativeZero, domain.ActionUnknown, false
	}

	return float64(raw), domain.ActionChange, true
}

func captureDecodeDigital(capture *capture,
	id domain.ControlID,
	raw int64,
) (output float64, action domain.EventAction, accepted bool) {
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

func captureEventTimestamp(environment *nativeState,
	capture *capture,
	value native.IOHIDValueRef,
) domain.Timestamp {
	now := time.Now()
	timestamp := domain.Timestamp{Time: now.UTC(), ReceivedAt: now, Source: domain.TimestampReceipt}
	ticks := environment.api.IOHIDValueGetTimeStamp(value)

	if ticks == nativeZero {
		return timestamp
	}

	delta, valid := tickDuration(environment, ticks, capture.clock.anchorTicks)
	if valid {
		timestamp.Time = capture.clock.anchor.Add(delta).UTC()
		timestamp.Source = domain.TimestampEstimated
	}

	return timestamp
}

func scaledDuration(environment *nativeState, delta uint64) (time.Duration, bool) {
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

func deviceResourcesCallbackCleanup(environment *nativeState,
	resources *deviceResources,
) []func() error {
	if resources.token == nativeZero {
		return nil
	}

	return []func() error{
		unregisterValueCallback(environment, resources),
		unregisterRemovalCallback(environment, resources),
	}
}

func deviceResourcesCloseSteps(environment *nativeState,
	resources *deviceResources,
) []func() error {
	var steps []func() error

	if resources.opened {
		steps = append(steps, func() error {
			return statusError(environment.api.IOHIDDeviceClose(resources.ref, nativeZero))
		})
	}

	return append(steps, func() error {
		return errors.Join(environment.core.release(uintptr(resources.ref)))
	})
}

func sessionCloseCaptures(environment *nativeState, session *session) []error {
	failures := make([]error, nativeZero, len(session.captures))

	for capture := range session.captures {
		capture.sink.Fail(domain.ErrClosed)

		err := resultError(safeCall(
			func() (any, error) { return nil, sessionCloseCapture(environment, session, capture) },
		))

		failures = append(failures, err)
	}

	return failures
}

func releaseNative(handle uintptr) error {
	return errors.Join(coreFoundationCall("CFRelease", func(release func(uintptr)) error {
		release(handle)

		return nil
	}))
}

func sessionReleaseManager(environment *nativeState, session *session) error {
	if session.manager == nativeZero {
		return nil
	}

	return errors.Join(cleanupSteps(func() error {
		return sessionUnscheduleManager(environment, session)
	}, func() error {
		return errors.Join(environment.core.release(uintptr(session.manager)))
	}))
}

func sessionUnscheduleManager(environment *nativeState, session *session) error {
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

func elementControlClassifyConsumer(environment *nativeState, item *elementControl, page uint32) {
	condition5 := page == uint32(domain.PageConsumer) &&
		booleanRange(elementRange(environment, item.element))
	if condition5 {
		elementControlDigital(item, domain.ControlKey)
	}
}

func sessionRegisterCallbacks(environment *nativeState,
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
		cause:   ctx.Err,
		perform: perform,
		result:  make(chan response, nativeOne),
		pack:    packResponse,
	}
}

func openedCapture(
	ctx context.Context, environment *nativeState,
	result any,
) (view captureOperations, failure error) {
	capture, ok := result.(*capture)
	if !ok {
		return view, domain.ErrUnsupported
	}

	return *captureView(ctx, environment, capture), nil
}

func unavailableCapabilities() domain.Capabilities {
	return domain.Capabilities{Controls: nil, Complete: false, Repeat: domain.SupportUnsupported}
}

func capturePublishValue(environment *nativeState, args *capturePublishValueArguments) {
	event := captureScalarEvent(
		args.capture,
		args.item,
		int64(environment.api.IOHIDValueGetIntegerValue(args.value)),
	)
	if event == nil {
		return
	}

	event.Timestamp = captureEventTimestamp(environment, args.capture, args.value)
	args.capture.sink.Publish(event)
}

func captureScalarEvent(capture *capture, item *elementControl, raw int64) *domain.Event {
	output, action, valid := captureDecodeValue(capture, item, raw)
	if !valid {
		return nil
	}

	event := new(domain.Event)

	event.DeviceID, event.ControlID, event.Action = capture.info.ID, item.control.ID, action
	event.Value = elementControlScaleValue(item, output)

	return event
}

func assignMetadata[T any](target *T, value T) bool {
	if target == nil {
		return false
	}

	*target = value

	return true
}

func sessionIdentifier[Identifier ~uint16 | ~uint32](
	property func(string) (int64, bool),
	key string,
) *Identifier {
	value, ok := property(key)
	if !ok || value < nativeZero || uint64(value) > uint64(^Identifier(0)) {
		return nil
	}

	identifier := Identifier(value)

	return &identifier
}

func deviceClassCandidates() []classCandidate {
	return []classCandidate{
		{usage: usageKeyboard, class: domain.ClassKeyboard},
		{usage: usageKeypad, class: domain.ClassKeyboard},
		{usage: nativeTwo, class: domain.ClassMouse},
		{usage: nativeFour, class: domain.ClassJoystick},
		{usage: usageGamepad, class: domain.ClassGamepad},
	}
}

func matchesClass(environment *nativeState, ref native.IOHIDDeviceRef, usage uint32) bool {
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

func elementControlDescriptor(environment *nativeState,
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

func sessionServe(environment *nativeState, session *session) {
	err := resultError(
		safeCall(func() (any, error) { return nil, sessionStart(environment, session) }),
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
) *capture {
	capture := new(capture)

	capture.caps.Repeat = domain.SupportUnknown
	capture.backend, capture.ref, capture.sink = session.backend, ref, sink
	capture.clock = &captureClock{anchor: session.anchor, anchorTicks: session.anchorTicks}
	capture.controls = make(map[uint32]elementControl)
	capture.pressed = make(map[domain.ControlID]bool)

	return capture
}

func digitalControl(kind domain.ControlKind) bool {
	return kind == domain.ControlKey || kind == domain.ControlButton || kind == domain.ControlSwitch
}

func decodeHat(
	item *elementControl,
	raw int64,
) (value float64, action domain.EventAction, accepted bool) {
	direction, valid := domain.Hat(raw, *item.control.Range, item.hasNull)

	return float64(direction), domain.ActionChange, valid
}

func discardCapture(ctx context.Context, environment *nativeState, value any) {
	capture, ok := value.(*capture)
	if !ok {
		return
	}

	err := captureClose(ctx, environment, capture)
	if err != nil {
		capture.sink.Fail(err)
	}
}

func newBackendState(retrier ports.Retrier) *backend {
	return &backend{
		closeErr: nil, closeOnce: sync.Once{},
		jobs: make(chan sessionJob), stop: make(chan struct{}), done: make(chan struct{}),
		ready: make(chan error, nativeOne), retrier: retrier,
	}
}

func submitRequest(
	ctx context.Context,
	backend *backend,
	perform func(*session) (any, error),
) (*request, error) {
	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf(callErrorFormat, err)
	}

	request := newRequest(ctx, perform)

	err = backendSendRequest(ctx, backend, &request)
	if err != nil {
		return nil, fmt.Errorf(callErrorFormat, err)
	}

	return &request, nil
}

func openRequestOperation(
	ctx context.Context,
	environment *nativeState,
	target *openTarget,
) func(*session) (any, error) {
	return func(session *session) (any, error) {
		result, err := sessionOpen(ctx, environment, newSessionOpenArguments(session, target))
		if err != nil {
			return nil, errors.Join(err)
		}

		return result, nil
	}
}

func newDeviceResources(ref native.IOHIDDeviceRef) *deviceResources {
	return &deviceResources{token: nativeZero, opened: false, scheduled: false, ref: ref}
}

func newCaptureSetup(resources *deviceResources, sink ports.EventSink) captureSetup {
	return captureSetup{resources: resources, sink: sink}
}

func cfNumberValue(value cf.CFNumberRef) (int64, bool) {
	var number int64

	var ok bool

	err := coreFoundationCall(
		"CFNumberGetValue",
		func(read func(cf.CFNumberRef, int64, *int64) bool) error {
			ok = read(value, int64(cf.KCFNumberSInt64Type), &number)

			return nil
		},
	)

	return number, ok && err == nil
}

func registerNativeCallbacks(environment *nativeState) {
	// IOHIDValueCallback's four-argument C ABI forwards the three values we use.
	environment.valueCallback = purego.NewCallback(
		func(token uintptr, status int32, _ uintptr, value uintptr) {
			inputValueCallback(
				environment,
				newInputValueCallbackArguments(token, status, value),
			)
		},
	)
	environment.removalCallback = purego.NewCallback(
		func(token uintptr, _ int32, _ uintptr) {
			deviceRemovalCallback(
				environment,
				newDeviceRemovalCallbackArguments(token),
			)
		},
	)
}

func requestCapture(
	ctx context.Context,
	environment *nativeState,
	args *backendOpenArguments,
) (response, error) {
	request := newBackendCallArguments(
		args.backend,
		openRequestOperation(ctx, environment, args.args),
	)
	result, err := backendCall(ctx, environment, request)

	return result, errors.Join(err)
}

func deviceOpenRequest(
	args *sessionOpenArguments,
	ref native.IOHIDDeviceRef,
) *sessionOpenDeviceArguments {
	resources := newDeviceResources(ref)
	setup := newCaptureSetup(resources, args.args.sink)

	return newSessionOpenDeviceArguments(args.session, setup)
}

func discardCanceledRequest(
	ctx context.Context,
	environment *nativeState,
	args *backendAwaitResponseArguments,
) {
	request := newBackendDiscardResponseArguments(args.backend, args.request)
	go backendDiscardResponse(context.WithoutCancel(ctx), environment, request)
}

func discoveryRequest(
	ctx context.Context,
	environment *nativeState,
	backend *backend,
) *backendCallArguments {
	operation := func(session *session) (any, error) { return sessionDiscover(ctx, environment, session) }

	return newBackendCallArguments(backend, operation)
}

func findRequestedDevice(
	ctx context.Context,
	environment *nativeState,
	args *sessionMatchDeviceArguments,
) (deviceReference, error) {
	ref, err := sessionMatchDevice(ctx, environment, args)
	if err == nil && ref.ref == nativeZero {
		err = &domain.OpError{Op: operationOpen, DeviceID: args.args.id, Err: domain.ErrNotFound}
	}

	if err != nil {
		return deviceReference{}, errors.Join(err)
	}

	return ref, nil
}

func finishDeviceOpen(
	environment *nativeState,
	args *sessionOpenDeviceArguments,
	failure *openFailure,
) (*capture, error) {
	request := newFinishCaptureOpenArguments(args.session, args.args.resources, failure)

	result, err := finishCaptureOpen(environment, request)
	if err != nil {
		return nil, errors.Join(err)
	}

	return result, nil
}

func prepareScheduledCapture(
	environment *nativeState,
	args *sessionOpenDeviceArguments,
) (*capture, error) {
	request := newSessionPrepareCaptureArguments(args.session, args.args.resources, args.args.sink)

	result, err := sessionPrepareCapture(environment, request)
	if err != nil {
		return nil, fmt.Errorf("prepare IOHID capture: %w", err)
	}

	schedule := newSessionScheduleCaptureArguments(args.session, args.args.resources, result)

	err = sessionScheduleCapture(environment, schedule)
	if err != nil {
		return nil, fmt.Errorf("schedule IOHID capture: %w", err)
	}

	return result, nil
}

func completeCapabilities() domain.Capabilities {
	return domain.Capabilities{Controls: nil, Complete: true, Repeat: domain.SupportUnsupported}
}

func storeResolutionMultiplier(environment *nativeState, args *captureAddControlArguments) {
	key := wheelCollection(environment, args.element)
	value, valid := readMultiplier(environment, args.capture.ref, args.element)
	previous, duplicate := args.multipliers[key]

	if duplicate {
		previous.valid = false
		args.multipliers[key] = previous

		return
	}

	args.multipliers[key] = multiplierValue{value: value, valid: valid}
}

func captureOpenFailure(result *capture, err error, recovered any) *openFailure {
	return &openFailure{result: result, err: err, recovered: recovered}
}

func applyResolutionMultiplier(item *elementControl, resolution multiplierValue) {
	if !resolution.valid {
		item.multiplier = nativeZero

		return
	}

	item.multiplier, item.control.Unit = resolution.value, domain.UnitDetents
}

func unregisterValueCallback(environment *nativeState, resources *deviceResources) func() error {
	return func() error {
		environment.api.IOHIDDeviceRegisterInputValueCallback(
			resources.ref,
			nativeZero,
			resources.token,
		)

		return nil
	}
}

func unregisterRemovalCallback(environment *nativeState, resources *deviceResources) func() error {
	return func() error {
		environment.api.IOHIDDeviceRegisterRemovalCallback(
			resources.ref,
			nativeZero,
			resources.token,
		)

		return nil
	}
}

func configureFrameworkPath(environment *nativeState, library uintptr) {
	{
		environment.nativeLibraryHandles = append(environment.nativeLibraryHandles, library)
		// Use a writable output buffer instead of the generated string argument.
		pathErr := environment.core.bind(
			library,
			"IORegistryEntryGetPath",
			&environment.registryPath,
		)
		if pathErr != nil {
			environment.registryPath = nil
		}
	}
}

func defaultNativeAPI() nativeAPI {
	api := new(nativeAPI)
	configureManagerAPI(api)
	configureDeviceAPI(api)
	configureDeviceLifecycleAPI(api)
	configureElementIdentityAPI(api)
	configureElementHierarchyAPI(api)
	configureElementValuesAPI(api)
	configureValueAPI(api)
	configureRegistryAPI(api)

	return *api
}

func configureManagerAPI(api *nativeAPI) {
	api.IOHIDManagerCreate = native.IOHIDManagerCreate
	api.IOHIDManagerSetDeviceMatching = native.IOHIDManagerSetDeviceMatching
	api.IOHIDManagerCopyDevices = native.IOHIDManagerCopyDevices
	api.IOHIDManagerScheduleWithRunLoop = native.IOHIDManagerScheduleWithRunLoop
	api.IOHIDManagerUnscheduleFromRunLoop = native.IOHIDManagerUnscheduleFromRunLoop
}

func configureDeviceAPI(api *nativeAPI) {
	api.IOHIDDeviceCreate = native.IOHIDDeviceCreate
	api.IOHIDDeviceGetService = native.IOHIDDeviceGetService
	api.IOHIDDeviceGetProperty = native.IOHIDDeviceGetProperty
	api.IOHIDDeviceConformsTo = native.IOHIDDeviceConformsTo
	api.IOHIDDeviceCopyMatchingElements = native.IOHIDDeviceCopyMatchingElements
}

func configureDeviceLifecycleAPI(api *nativeAPI) {
	api.IOHIDDeviceGetValue = native.IOHIDDeviceGetValue
	api.IOHIDDeviceOpen = native.IOHIDDeviceOpen
	api.IOHIDDeviceClose = native.IOHIDDeviceClose
	api.IOHIDDeviceScheduleWithRunLoop = native.IOHIDDeviceScheduleWithRunLoop
	api.IOHIDDeviceUnscheduleFromRunLoop = native.IOHIDDeviceUnscheduleFromRunLoop
}

func configureElementIdentityAPI(api *nativeAPI) {
	api.IOHIDElementGetCookie = native.IOHIDElementGetCookie
	api.IOHIDElementGetType = native.IOHIDElementGetType
	api.IOHIDElementGetUsagePage = native.IOHIDElementGetUsagePage
	api.IOHIDElementGetUsage = native.IOHIDElementGetUsage
	api.IOHIDElementGetName = native.IOHIDElementGetName
}

func configureElementHierarchyAPI(api *nativeAPI) {
	api.IOHIDElementGetParent = native.IOHIDElementGetParent
	api.IOHIDElementGetCollectionType = native.IOHIDElementGetCollectionType
	api.IOHIDElementGetLogicalMin = native.IOHIDElementGetLogicalMin
	api.IOHIDElementGetLogicalMax = native.IOHIDElementGetLogicalMax
	api.IOHIDElementGetPhysicalMin = native.IOHIDElementGetPhysicalMin
}

func configureElementValuesAPI(api *nativeAPI) {
	api.IOHIDElementGetPhysicalMax = native.IOHIDElementGetPhysicalMax
	api.IOHIDElementGetUnit = native.IOHIDElementGetUnit
	api.IOHIDElementGetUnitExponent = native.IOHIDElementGetUnitExponent
	api.IOHIDElementHasNullState = native.IOHIDElementHasNullState
	api.IOHIDElementIsRelative = native.IOHIDElementIsRelative
	api.IOHIDElementGetReportSize = native.IOHIDElementGetReportSize
	api.IOHIDElementGetReportCount = native.IOHIDElementGetReportCount
	api.IOHIDElementGetReportID = native.IOHIDElementGetReportID
	api.IOHIDElementGetDevice = native.IOHIDElementGetDevice
}

func configureValueAPI(api *nativeAPI) {
	api.IOHIDValueGetElement = native.IOHIDValueGetElement
	api.IOHIDValueGetIntegerValue = native.IOHIDValueGetIntegerValue
	api.IOHIDValueGetLength = native.IOHIDValueGetLength
	api.IOHIDValueGetTimeStamp = native.IOHIDValueGetTimeStamp
}

func configureRegistryAPI(api *nativeAPI) {
	api.IORegistryEntryGetRegistryEntryID = native.IORegistryEntryGetRegistryEntryID
}

func newBackendAwaitResponseArguments(
	backend *backend,
	request *request,
) *backendAwaitResponseArguments {
	return &backendAwaitResponseArguments{backend: backend, request: request}
}

func newBackendCallArguments(
	backend *backend,
	perform func(*session) (any, error),
) *backendCallArguments {
	return &backendCallArguments{backend: backend, perform: perform}
}

func newBackendDiscardResponseArguments(
	backend *backend,
	request *request,
) *backendDiscardResponseArguments {
	return &backendDiscardResponseArguments{backend: backend, request: request}
}

func newBackendOpenArguments(backend *backend, args *openTarget) *backendOpenArguments {
	return &backendOpenArguments{backend: backend, args: args}
}

func newCaptureAddControlArguments(
	capture *capture,
	multipliers map[multiplierKey]multiplierValue,
	element native.IOHIDElementRef,
) *captureAddControlArguments {
	return &captureAddControlArguments{capture: capture, multipliers: multipliers, element: element}
}

func newCaptureProcessValueArguments(
	capture *capture,
	element native.IOHIDElementRef,
	value native.IOHIDValueRef,
) *captureProcessValueArguments {
	return &captureProcessValueArguments{capture: capture, element: element, value: value}
}

func newCapturePublishValueArguments(
	capture *capture,
	item *elementControl,
	value native.IOHIDValueRef,
) *capturePublishValueArguments {
	return &capturePublishValueArguments{capture: capture, item: item, value: value}
}

func newDeviceRemovalCallbackArguments(token uintptr) *deviceRemovalCallbackArguments {
	return &deviceRemovalCallbackArguments{token: token}
}

func newFinishCaptureOpenArguments(
	session *session,
	resources *deviceResources,
	failure *openFailure,
) *finishCaptureOpenArguments {
	return &finishCaptureOpenArguments{session: session, resources: resources, failure: failure}
}

func newInputValueCallbackArguments(
	token uintptr,
	status int32,
	value uintptr,
) *inputValueCallbackArguments {
	return &inputValueCallbackArguments{token: token, status: status, value: value}
}

func newSessionAbortOpenArguments(
	session *session,
	resources *deviceResources,
	args *openFailure,
) *sessionAbortOpenArguments {
	return &sessionAbortOpenArguments{session: session, resources: resources, args: args}
}

func newSessionDeviceInfosArguments(
	session *session,
	devices []native.IOHIDDeviceRef,
) *sessionDeviceInfosArguments {
	return &sessionDeviceInfosArguments{session: session, devices: devices}
}

func newSessionFindDeviceArguments(
	session *session,
	id domain.DeviceID,
) *sessionFindDeviceArguments {
	return &sessionFindDeviceArguments{session: session, id: id}
}

func newSessionMatchDeviceArguments(
	session *session,
	args *deviceSearch,
) *sessionMatchDeviceArguments {
	return &sessionMatchDeviceArguments{session: session, args: args}
}

func newSessionNewCaptureArguments(
	sink ports.EventSink,
	session *session,
	ref native.IOHIDDeviceRef,
) *sessionNewCaptureArguments {
	return &sessionNewCaptureArguments{sink: sink, session: session, ref: ref}
}

func newSessionNumberPropertyArguments(
	key string,
	session *session,
	ref native.IOHIDDeviceRef,
) *sessionNumberPropertyArguments {
	return &sessionNumberPropertyArguments{key: key, session: session, ref: ref}
}

func newSessionOpenArguments(session *session, args *openTarget) *sessionOpenArguments {
	return &sessionOpenArguments{session: session, args: args}
}

func newSessionOpenDeviceArguments(
	session *session,
	args captureSetup,
) *sessionOpenDeviceArguments {
	return &sessionOpenDeviceArguments{session: session, args: args}
}

func newSessionPrepareCaptureArguments(
	session *session,
	resources *deviceResources,
	sink ports.EventSink,
) *sessionPrepareCaptureArguments {
	return &sessionPrepareCaptureArguments{session: session, resources: resources, sink: sink}
}

func newSessionScheduleCaptureArguments(
	session *session,
	resources *deviceResources,
	capture *capture,
) *sessionScheduleCaptureArguments {
	return &sessionScheduleCaptureArguments{
		session:   session,
		resources: resources,
		capture:   capture,
	}
}

func newSessionStringPropertyArguments(
	key string,
	session *session,
	ref native.IOHIDDeviceRef,
) *sessionStringPropertyArguments {
	return &sessionStringPropertyArguments{key: key, session: session, ref: ref}
}

func newSessionUsagePropertyArguments(
	key string,
	session *session,
	ref native.IOHIDDeviceRef,
) *sessionUsagePropertyArguments {
	return &sessionUsagePropertyArguments{key: key, session: session, ref: ref}
}

// resultError retains the error when an operation's value is irrelevant.
func resultError[T any](_ T, err error) error { return errors.Join(err) }

// statusBits retains the unsigned representation of a signed IOReturn.
func statusBits(status int32) uint32 {
	if status >= nativeZero {
		return uint32(status)
	}

	magnitude := -(int64(status) + nativeOne)

	return math.MaxUint32 - uint32(magnitude&math.MaxInt32)
}

// Factory owns native bindings and callback registries for a shared session.
func Factory() ports.Factory {
	environment := new(nativeState)

	environment.core = defaultCoreAPI()

	return func(ctx context.Context, retrier ports.Retrier) (ports.Backend, error) {
		return newBackend(ctx, environment, retrier)
	}
}

// NativeInfo returns a fresh platform metadata snapshot.
func (view metadataView[T]) NativeInfo() T { return view() }

func backendView(environment *nativeState, state *backend) *backendOperations {
	view := new(backendOperations)

	view.Operations.Discover = func(ctx context.Context) ([]domain.DeviceInfo, error) {
		return backendDiscover(ctx, environment, state)
	}
	configureBackendOpen(view, environment, state)

	view.Operations.Close = func() error { return backendClose(state) }

	return view
}

func captureView(
	ctx context.Context, environment *nativeState,
	state *capture,
) *captureOperations {
	view := new(captureOperations)

	view.Operations.Info = func() domain.DeviceInfo { return captureInfo(state) }
	view.Operations.Capabilities = func() domain.Capabilities { return captureCapabilities(state) }
	view.Operations.Extension = func(target any) bool { return captureExtension(state, target) }
	view.Operations.Close = func() error { return captureClose(context.WithoutCancel(ctx), environment, state) }

	return view
}

func captureMetadataView(state *capture) metadataView[extension.Metadata] {
	return metadataView[extension.Metadata](
		func() extension.Metadata { return captureIOKitMetadata(state) },
	)
}

func packResponse(value any, err error) response { return response{value: value, err: err} }

// Execute evaluates a request on the session thread and delivers one response.
func (request *requestRecord[S, R]) Execute(session S) {
	var value any

	err := request.cause()
	if err == nil {
		value, err = safeCall(func() (any, error) { return request.perform(session) })
	}

	request.result <- request.pack(value, err)
}

func configureBackendOpen(view *backendOperations, environment *nativeState, state *backend) {
	view.Operations.Open = func(ctx context.Context, id domain.DeviceID, sink ports.EventSink) (ports.Capture, error) {
		capture, err := backendOpen(
			ctx, environment, newBackendOpenArguments(state, &openTarget{id: id, sink: sink}),
		)
		if err != nil {
			return nil, errors.Join(err)
		}

		return &capture, nil
	}
}
