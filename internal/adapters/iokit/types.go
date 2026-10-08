// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build darwin && (amd64 || arm64)

package iokit

import (
	"sync"
	"sync/atomic"
	"time"

	extension "github.com/gostafa/goinput/extensions/iokit"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	backendview "github.com/gostafa/goinput/internal/ports/backend"
	captureview "github.com/gostafa/goinput/internal/ports/capture"
	cf "github.com/tmc/apple/corefoundation"
	native "github.com/tmc/apple/iokit"
)

type (
	openFailure = openOutcome[*capture]

	openOutcome[Value any] struct {
		result    Value
		err       error
		recovered any
	}
	openTarget = openRequest[ports.EventSink, domain.DeviceID]

	openRequest[Sink, ID any] struct {
		sink Sink
		id   ID
	}
	deviceSearch = endpointSearch[domain.DeviceID, native.IOHIDDeviceRef]

	endpointSearch[ID, Device any] struct {
		id      ID
		devices []Device
	}
	captureSetup = subscriptionSetup[*deviceResources, ports.EventSink]

	subscriptionSetup[Resources, Sink any] struct {
		resources Resources
		sink      Sink
	}
	deviceInventory struct {
		devices []native.IOHIDDeviceRef
		set     cf.CFSetRef
	}
	deviceReference struct{ ref native.IOHIDDeviceRef }
)

type (
	nativeCoreAPI = struct {
		runLoop      func() cf.CFRunLoopRef
		stringRef    func(string) cf.CFStringRef
		release      func(uintptr) error
		setCount     func(cf.CFSetRef) int
		setDevices   func(cf.CFSetRef, int) ([]native.IOHIDDeviceRef, error)
		arrayCount   func(cf.CFArrayRef) int
		arrayElement func(cf.CFArrayRef, int) native.IOHIDElementRef
		openLibrary  func(string, int) (uintptr, error)
		bind         func(uintptr, string, any) error
		probeHID     func() (any, error)
	}
)

type (
	classCandidate = usageClass[domain.DeviceClass]

	usageClass[Class any] struct {
		class Class
		usage uint32
	}
)

type (
	backendAwaitResponseArguments = struct {
		backend *backend
		request *request
	}
	backendCallArguments = struct {
		backend *backend
		perform func(*session) (any, error)
	}
	backendDiscardResponseArguments = struct {
		backend *backend
		request *request
	}
	backendOpenArguments = struct {
		backend *backend
		args    *openTarget
	}
	captureAddControlArguments = struct {
		capture     *capture
		multipliers map[multiplierKey]multiplierValue
		element     native.IOHIDElementRef
	}
	captureProcessValueArguments = struct {
		capture *capture
		element native.IOHIDElementRef
		value   native.IOHIDValueRef
	}
	capturePublishValueArguments = struct {
		capture *capture
		item    *elementControl
		value   native.IOHIDValueRef
	}
	deviceRemovalCallbackArguments = struct {
		token uintptr
	}
	finishCaptureOpenArguments = struct {
		session   *session
		resources *deviceResources
		failure   *openFailure
	}
	inputValueCallbackArguments = struct {
		token  uintptr
		status int32
		value  uintptr
	}
	sessionAbortOpenArguments = struct {
		session   *session
		resources *deviceResources
		args      *openFailure
	}
	sessionDeviceInfosArguments = struct {
		session *session
		devices []native.IOHIDDeviceRef
	}
	sessionFindDeviceArguments = struct {
		session *session
		id      domain.DeviceID
	}
	sessionMatchDeviceArguments = struct {
		session *session
		args    *deviceSearch
	}
	sessionNewCaptureArguments = struct {
		sink    ports.EventSink
		session *session
		ref     native.IOHIDDeviceRef
	}
	sessionNumberPropertyArguments = struct {
		session *session
		key     string
		ref     native.IOHIDDeviceRef
	}
	sessionOpenArguments = struct {
		session *session
		args    *openTarget
	}
	sessionOpenDeviceArguments = struct {
		session *session
		args    captureSetup
	}
	sessionPrepareCaptureArguments = struct {
		session   *session
		resources *deviceResources
		sink      ports.EventSink
	}
	sessionScheduleCaptureArguments = struct {
		session   *session
		resources *deviceResources
		capture   *capture
	}
	sessionStringPropertyArguments = struct {
		session *session
		key     string
		ref     native.IOHIDDeviceRef
	}
	sessionUsagePropertyArguments = struct {
		session *session
		key     string
		ref     native.IOHIDDeviceRef
	}
)

type (
	backendOperations = backendview.Operations[domain.DeviceInfo, domain.DeviceID, ports.EventSink, ports.Capture]
	captureOperations = captureview.Operations[domain.DeviceInfo, domain.Capabilities]
)

type (
	backend = backendRecord[sessionJob, ports.Retrier]

	backendRecord[J, R any] struct {
		closeErr  error
		retrier   R
		jobs      chan J
		stop      chan struct{}
		done      chan struct{}
		ready     chan error
		closeOnce sync.Once
	}

	// sessionJob is a command run on the native session's locked thread.
	sessionJob func(*session)

	request = requestRecord[*session, response]

	requestRecord[S, R any] struct {
		cause   func() error
		perform func(S) (any, error)
		result  chan R
		pack    func(any, error) R
	}

	response struct {
		value any
		err   error
	}

	session = sessionRecord[*backend, *capture]

	sessionRecord[B any, C comparable] struct {
		anchor      time.Time
		backend     B
		keys        map[string]cf.CFStringRef
		captures    map[C]struct{}
		manager     native.IOHIDManagerRef
		runLoop     cf.CFRunLoopRef
		mode        cf.CFStringRef
		anchorTicks uint64
	}

	capture = captureRecord[
		ports.EventSink,
		*captureClock,
		domain.ControlID,
		elementControl,
		*backend,
		domain.DeviceInfo,
		domain.Capabilities,
		extension.Metadata,
	]

	captureRecord[S, Clock any, ID comparable, C, B, I, Caps, M any] struct {
		sink      S
		closeErr  error
		clock     Clock
		pressed   map[ID]bool
		controls  map[uint32]C
		backend   B
		info      I
		caps      Caps
		metadata  M
		token     uintptr
		ref       native.IOHIDDeviceRef
		closeOnce sync.Once
		closed    atomic.Bool
	}

	elementControl = elementControlRecord[domain.Control]

	elementControlRecord[C any] struct {
		control    C
		element    native.IOHIDElementRef
		hasNull    bool
		multiplier float64
	}

	multiplierKey [nativeTwo]uint64

	multiplierValue struct {
		value float64
		valid bool
	}

	machTimebase struct {
		// Numer is the Mach tick conversion numerator.
		Numer uint32
		// Denom is the Mach tick conversion denominator.
		Denom uint32
	}

	// nativeAPI preserves the requested binding's native handle types. The two
	// callback registration bridges pass opaque numeric contexts through uintptr.
	// A private loader also handles the upstream lowercase IOKit framework path.
	nativeAPI struct {
		// IOHIDManagerCreate is the native binding for that IOKit operation.
		IOHIDManagerCreate func(cf.CFAllocatorRef, uint32) native.IOHIDManagerRef
		// IOHIDManagerSetDeviceMatching is the native binding for that IOKit operation.
		IOHIDManagerSetDeviceMatching func(native.IOHIDManagerRef, cf.CFDictionaryRef)
		// IOHIDManagerCopyDevices is the native binding for that IOKit operation.
		IOHIDManagerCopyDevices func(native.IOHIDManagerRef) cf.CFSet
		// IOHIDManagerScheduleWithRunLoop is the native binding for that IOKit operation.
		IOHIDManagerScheduleWithRunLoop func(native.IOHIDManagerRef, cf.CFRunLoopRef, cf.CFStringRef)
		// IOHIDManagerUnscheduleFromRunLoop is the native binding for that IOKit operation.
		IOHIDManagerUnscheduleFromRunLoop func(native.IOHIDManagerRef, cf.CFRunLoopRef, cf.CFStringRef)

		// IOHIDDeviceCreate is the native binding for that IOKit operation.
		IOHIDDeviceCreate func(cf.CFAllocatorRef, uint32) native.IOHIDDeviceRef
		// IOHIDDeviceGetService is the native binding for that IOKit operation.
		IOHIDDeviceGetService func(native.IOHIDDeviceRef) uint32
		// IOHIDDeviceGetProperty is the native binding for that IOKit operation.
		IOHIDDeviceGetProperty func(native.IOHIDDeviceRef, cf.CFStringRef) cf.CFTypeRef
		// IOHIDDeviceConformsTo is the native binding for that IOKit operation.
		IOHIDDeviceConformsTo func(native.IOHIDDeviceRef, uint32, uint32) bool
		// IOHIDDeviceCopyMatchingElements is the native binding for that IOKit operation.
		IOHIDDeviceCopyMatchingElements func(native.IOHIDDeviceRef, cf.CFDictionaryRef, uint32) cf.CFArrayRef
		// IOHIDDeviceGetValue is the native binding for that IOKit operation.
		IOHIDDeviceGetValue func(
			native.IOHIDDeviceRef,
			native.IOHIDElementRef,
			*native.IOHIDValueRef,
		) int32
		// IOHIDDeviceOpen is the native binding for that IOKit operation.
		IOHIDDeviceOpen func(native.IOHIDDeviceRef, uint32) int32
		// IOHIDDeviceClose is the native binding for that IOKit operation.
		IOHIDDeviceClose func(native.IOHIDDeviceRef, uint32) int32
		// Callback addresses and numeric contexts use the C pointer ABI without
		// placing fabricated pointers into Go's garbage-collected stack maps.
		// IOHIDDeviceRegisterInputValueCallback is the native binding for that IOKit operation.
		IOHIDDeviceRegisterInputValueCallback func(native.IOHIDDeviceRef, uintptr, uintptr)
		// IOHIDDeviceRegisterRemovalCallback is the native binding for that IOKit operation.
		IOHIDDeviceRegisterRemovalCallback func(native.IOHIDDeviceRef, uintptr, uintptr)
		// IOHIDDeviceScheduleWithRunLoop is the native binding for that IOKit operation.
		IOHIDDeviceScheduleWithRunLoop func(native.IOHIDDeviceRef, cf.CFRunLoopRef, cf.CFStringRef)
		// IOHIDDeviceUnscheduleFromRunLoop is the native binding for that IOKit operation.
		IOHIDDeviceUnscheduleFromRunLoop func(native.IOHIDDeviceRef, cf.CFRunLoopRef, cf.CFStringRef)

		// IOHIDElementGetCookie is the native binding for that IOKit operation.
		IOHIDElementGetCookie func(native.IOHIDElementRef) uint32
		// IOHIDElementGetType is the native binding for that IOKit operation.
		IOHIDElementGetType func(native.IOHIDElementRef) native.IOHIDElementType
		// IOHIDElementGetUsagePage is the native binding for that IOKit operation.
		IOHIDElementGetUsagePage func(native.IOHIDElementRef) uint32
		// IOHIDElementGetUsage is the native binding for that IOKit operation.
		IOHIDElementGetUsage func(native.IOHIDElementRef) uint32
		// IOHIDElementGetName is the native binding for that IOKit operation.
		IOHIDElementGetName func(native.IOHIDElementRef) cf.CFStringRef
		// IOHIDElementGetParent is the native binding for that IOKit operation.
		IOHIDElementGetParent func(native.IOHIDElementRef) native.IOHIDElementRef
		// IOHIDElementGetCollectionType is the native binding for that IOKit operation.
		IOHIDElementGetCollectionType func(native.IOHIDElementRef) native.IOHIDElementCollectionType
		// IOHIDElementGetLogicalMin is the native binding for that IOKit operation.
		IOHIDElementGetLogicalMin func(native.IOHIDElementRef) cf.CFIndex
		// IOHIDElementGetLogicalMax is the native binding for that IOKit operation.
		IOHIDElementGetLogicalMax func(native.IOHIDElementRef) cf.CFIndex
		// IOHIDElementGetPhysicalMin is the native binding for that IOKit operation.
		IOHIDElementGetPhysicalMin func(native.IOHIDElementRef) cf.CFIndex

		// IOHIDElementGetPhysicalMax is the native binding for that IOKit operation.
		IOHIDElementGetPhysicalMax func(native.IOHIDElementRef) cf.CFIndex
		// IOHIDElementGetUnit is the native binding for that IOKit operation.
		IOHIDElementGetUnit func(native.IOHIDElementRef) uint32
		// IOHIDElementGetUnitExponent is the native binding for that IOKit operation.
		IOHIDElementGetUnitExponent func(native.IOHIDElementRef) uint32
		// IOHIDElementHasNullState is the native binding for that IOKit operation.
		IOHIDElementHasNullState func(native.IOHIDElementRef) bool
		// IOHIDElementIsRelative is the native binding for that IOKit operation.
		IOHIDElementIsRelative func(native.IOHIDElementRef) bool
		// IOHIDElementGetReportSize is the native binding for that IOKit operation.
		IOHIDElementGetReportSize func(native.IOHIDElementRef) uint32
		// IOHIDElementGetReportCount is the native binding for that IOKit operation.
		IOHIDElementGetReportCount func(native.IOHIDElementRef) uint32
		// IOHIDElementGetReportID is the native binding for that IOKit operation.
		IOHIDElementGetReportID func(native.IOHIDElementRef) uint32
		// IOHIDElementGetDevice is the native binding for that IOKit operation.
		IOHIDElementGetDevice func(native.IOHIDElementRef) native.IOHIDDeviceRef

		// IOHIDValueGetElement is the native binding for that IOKit operation.
		IOHIDValueGetElement func(native.IOHIDValueRef) native.IOHIDElementRef
		// IOHIDValueGetIntegerValue is the native binding for that IOKit operation.
		IOHIDValueGetIntegerValue func(native.IOHIDValueRef) cf.CFIndex
		// IOHIDValueGetLength is the native binding for that IOKit operation.
		IOHIDValueGetLength func(native.IOHIDValueRef) cf.CFIndex
		// IOHIDValueGetTimeStamp is the native binding for that IOKit operation.
		IOHIDValueGetTimeStamp func(native.IOHIDValueRef) uint64

		// IORegistryEntryGetRegistryEntryID is the native binding for that IOKit operation.
		IORegistryEntryGetRegistryEntryID func(uint32, *uint64) int32
	}

	// captureClock is a non-owning copy of the session's immutable clock anchor.
	captureClock struct {
		anchor      time.Time
		anchorTicks uint64
	}

	deviceResources struct {
		ref       native.IOHIDDeviceRef
		token     uintptr
		opened    bool
		scheduled bool
	}
)

type (
	nativeState = nativeStateRecord[nativeAPI, machTimebase, nativeCoreAPI]

	nativeStateRecord[API, Clock, Core any] struct {
		api                  API
		core                 Core
		timebase             Clock
		symbolErr            error
		machAbsoluteTime     func() uint64
		machTimebaseInfo     func(*Clock) int32
		registryPath         func(uint32, string, *byte) int32
		callbackRegistry     sync.Map
		nativeLibraryHandles []uintptr
		nextCallbackToken    atomic.Uint64
		valueCallback        uintptr
		removalCallback      uintptr
		symbolOnce           sync.Once
	}
)

type (
	metadataView[T any] func() T
)
