//go:build darwin && (amd64 || arm64)

package iokit

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	extension "github.com/gostafa/goinput/extensions/iokit"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	cf "github.com/tmc/apple/corefoundation"
	native "github.com/tmc/apple/iokit"
)

type backend struct {
	jobs      chan request
	stop      chan struct{}
	done      chan struct{}
	ready     chan error
	closeOnce sync.Once
	closeErr  error
	retrier   ports.Retrier
}

type request struct {
	ctx     context.Context
	perform func(*session) (any, error)
	result  chan response
}

type response struct {
	value any
	err   error
}

type session struct {
	backend     *backend
	manager     native.IOHIDManagerRef
	runLoop     cf.CFRunLoopRef
	mode        cf.CFStringRef
	keys        map[string]cf.CFStringRef
	captures    map[*capture]struct{}
	anchor      time.Time
	anchorTicks uint64
}

type capture struct {
	backend   *backend
	clock     *session
	ref       native.IOHIDDeviceRef
	token     uintptr
	info      domain.DeviceInfo
	caps      domain.Capabilities
	metadata  extension.Metadata
	sink      ports.EventSink
	controls  map[uint32]elementControl
	pressed   map[domain.ControlID]bool
	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

type elementControl struct {
	control    domain.Control
	element    native.IOHIDElementRef
	hasNull    bool
	multiplier float64
}

type multiplierKey struct {
	collection native.IOHIDElementRef
	report     uint32
}

type multiplierValue struct {
	value float64
	valid bool
}

type machTimebase struct {
	Numer uint32
	Denom uint32
}

// nativeAPI preserves the requested binding's native handle types. The two
// callback registration bridges pass opaque numeric contexts through uintptr.
// A private loader also handles the upstream lowercase IOKit framework path.
type nativeAPI struct {
	IOHIDManagerCreate                func(cf.CFAllocatorRef, uint32) native.IOHIDManagerRef
	IOHIDManagerSetDeviceMatching     func(native.IOHIDManagerRef, cf.CFDictionaryRef)
	IOHIDManagerCopyDevices           func(native.IOHIDManagerRef) cf.CFSet
	IOHIDManagerScheduleWithRunLoop   func(native.IOHIDManagerRef, cf.CFRunLoopRef, cf.CFStringRef)
	IOHIDManagerUnscheduleFromRunLoop func(native.IOHIDManagerRef, cf.CFRunLoopRef, cf.CFStringRef)
	IOHIDDeviceCreate                 func(cf.CFAllocatorRef, uint32) native.IOHIDDeviceRef
	IOHIDDeviceGetService             func(native.IOHIDDeviceRef) uint32
	IOHIDDeviceGetProperty            func(native.IOHIDDeviceRef, cf.CFStringRef) cf.CFTypeRef
	IOHIDDeviceConformsTo             func(native.IOHIDDeviceRef, uint32, uint32) bool
	IOHIDDeviceCopyMatchingElements   func(native.IOHIDDeviceRef, cf.CFDictionaryRef, uint32) cf.CFArrayRef
	IOHIDDeviceGetValue               func(native.IOHIDDeviceRef, native.IOHIDElementRef, *native.IOHIDValueRef) int32
	IOHIDDeviceOpen                   func(native.IOHIDDeviceRef, uint32) int32
	IOHIDDeviceClose                  func(native.IOHIDDeviceRef, uint32) int32
	// Callback addresses and numeric contexts use the C pointer ABI without
	// placing fabricated pointers into Go's garbage-collected stack maps.
	IOHIDDeviceRegisterInputValueCallback func(native.IOHIDDeviceRef, uintptr, uintptr)
	IOHIDDeviceRegisterRemovalCallback    func(native.IOHIDDeviceRef, uintptr, uintptr)
	IOHIDDeviceScheduleWithRunLoop        func(native.IOHIDDeviceRef, cf.CFRunLoopRef, cf.CFStringRef)
	IOHIDDeviceUnscheduleFromRunLoop      func(native.IOHIDDeviceRef, cf.CFRunLoopRef, cf.CFStringRef)
	IOHIDElementGetCookie                 func(native.IOHIDElementRef) uint32
	IOHIDElementGetType                   func(native.IOHIDElementRef) native.IOHIDElementType
	IOHIDElementGetUsagePage              func(native.IOHIDElementRef) uint32
	IOHIDElementGetUsage                  func(native.IOHIDElementRef) uint32
	IOHIDElementGetName                   func(native.IOHIDElementRef) cf.CFStringRef
	IOHIDElementGetParent                 func(native.IOHIDElementRef) native.IOHIDElementRef
	IOHIDElementGetCollectionType         func(native.IOHIDElementRef) native.IOHIDElementCollectionType
	IOHIDElementGetLogicalMin             func(native.IOHIDElementRef) cf.CFIndex
	IOHIDElementGetLogicalMax             func(native.IOHIDElementRef) cf.CFIndex
	IOHIDElementGetPhysicalMin            func(native.IOHIDElementRef) cf.CFIndex
	IOHIDElementGetPhysicalMax            func(native.IOHIDElementRef) cf.CFIndex
	IOHIDElementGetUnit                   func(native.IOHIDElementRef) uint32
	IOHIDElementGetUnitExponent           func(native.IOHIDElementRef) uint32
	IOHIDElementHasNullState              func(native.IOHIDElementRef) bool
	IOHIDElementIsRelative                func(native.IOHIDElementRef) bool
	IOHIDElementGetReportSize             func(native.IOHIDElementRef) uint32
	IOHIDElementGetReportCount            func(native.IOHIDElementRef) uint32
	IOHIDElementGetReportID               func(native.IOHIDElementRef) uint32
	IOHIDElementGetDevice                 func(native.IOHIDElementRef) native.IOHIDDeviceRef
	IOHIDValueGetElement                  func(native.IOHIDValueRef) native.IOHIDElementRef
	IOHIDValueGetIntegerValue             func(native.IOHIDValueRef) cf.CFIndex
	IOHIDValueGetLength                   func(native.IOHIDValueRef) cf.CFIndex
	IOHIDValueGetTimeStamp                func(native.IOHIDValueRef) uint64
	IORegistryEntryGetRegistryEntryID     func(uint32, *uint64) int32
}
