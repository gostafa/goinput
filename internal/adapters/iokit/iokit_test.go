// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build darwin && (amd64 || arm64)

package iokit

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/ebitengine/purego"
	extension "github.com/gostafa/goinput/extensions/iokit"
	"github.com/gostafa/goinput/internal/domain"
	sinkview "github.com/gostafa/goinput/internal/ports/sink"
	cf "github.com/tmc/apple/corefoundation"
	native "github.com/tmc/apple/iokit"
)

type (
	callbackSink struct {
		failure error
	}

	tickExample struct {
		ticks    uint64
		anchor   uint64
		duration time.Duration
		valid    bool
	}

	collectionSymbols = struct{ create, insert string }

	coverageRetrier struct{}

	contextValueLookup interface{ Value(key any) any }

	cancelOnCheckContext struct {
		contextValueLookup

		deadline func() (time.Time, bool)
		done     func() <-chan struct{}
	}
)

const (
	testValueHandle = 1
	testEarlierTick = 95
	testAnchorTick  = 100
	testLaterTick   = 105
	testTickDelta   = 5

	testPropertyNumber = 42
	testNumberProperty = "number"
	testMissingDevice  = "missing"
	testNativeResource = "test-native-resource"
	testWrongType      = "wrong type"
	testPlainString    = "abc"
)

var errUnexpectedExecution = errors.New("unexpected execution")

func TestCoreFoundationReleaseUsesOpaqueHandle(t *testing.T) {
	t.Parallel()

	ref := cf.CFStringCreateWithCString(nativeZero, "goinput.native-reference-test", utf8Encoding)
	if ref == nativeZero {
		t.Fatal("CoreFoundation test string was not created")
	}

	err := releaseNative(uintptr(ref))
	if err != nil {
		t.Fatal(err)
	}
}

func TestMissingCoreFoundationSymbolReportsUnsupported(t *testing.T) {
	t.Parallel()

	err := coreFoundationCall("goinputMissingCoreFoundationSymbol", func(func()) error {
		t.Error("missing native symbol was invoked")

		return nil
	})
	if !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("missing symbol = %v", err)
	}
}

func TestZeroSetDoesNotReleaseReference(t *testing.T) {
	t.Parallel()

	err := releaseSet(fakeNativeState(), nativeZero)
	if err != nil {
		t.Fatal(err)
	}

	devices, err := setDevices(nativeZero, nativeZero)
	if devices != nil || err != nil {
		t.Fatalf("empty set = (%v, %v)", devices, err)
	}
}

func descriptorState(usage domain.Usage) *nativeState {
	environment := fakeNativeState()

	environment.api.IOHIDElementGetUsagePage = func(native.IOHIDElementRef) uint32 { return uint32(usage.Page()) }
	environment.api.IOHIDElementGetUsage = func(native.IOHIDElementRef) uint32 { return uint32(usage.ID()) }
	configureDescriptorProperties(environment)

	return environment
}

func TestAxisDescriptorProducesControlAndNativeMetadata(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.AxisX)
	device := newCaptureState(
		newSession(newBackendState(nil)),
		nativeZero,
		&callbackSink{failure: nil},
	)
	captureBuildControls(environment, device, []native.IOHIDElementRef{nativeTwo, nativeOne})
	nativeEqual(t, len(device.caps.Controls), nativeTwo)
	nativeEqual(t, device.caps.Controls[nativeZero].Kind, domain.ControlAxis)
	nativeEqual(t, device.caps.Controls[nativeZero].Range.Max, int64(testAnchorTick))
	nativeEqual(t, device.metadata.Elements[nativeZero].Cookie, uint32(nativeOne))
	nativeEqual(t, device.caps.Complete, true)
}

func TestDescriptorWithOversizedUsageHasUnknownMapping(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.AxisX)

	environment.api.IOHIDElementGetUsagePage = func(native.IOHIDElementRef) uint32 { return math.MaxUint32 }

	item := newElementControl(environment, nativeOne)
	nativeEqual(t, item.control.Mapping, domain.MappingUnknown)
}

func TestLargeReportIsMarkedUnsupported(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.AxisX)

	environment.api.IOHIDElementGetReportSize = func(native.IOHIDElementRef) uint32 { return math.MaxUint32 }

	device := newCaptureState(
		newSession(newBackendState(nil)),
		nativeZero,
		&callbackSink{failure: nil},
	)
	captureBuildControls(environment, device, []native.IOHIDElementRef{nativeOne})
	nativeEqual(t, device.caps.Controls[nativeZero].Support, domain.SupportUnsupported)
}

func TestDigitalDescriptorClassification(t *testing.T) {
	t.Parallel()
	checkDescriptorKind(t, domain.HID(domain.PageKeyboard, nativeOne), domain.ControlKey)
	checkDescriptorKind(t, domain.HID(domain.PageButton, nativeOne), domain.ControlButton)
	checkDescriptorKind(t, domain.DPadUp, domain.ControlButton)
}

func checkDescriptorKind(t *testing.T, usage domain.Usage, expected domain.ControlKind) {
	t.Helper()

	environment := descriptorState(usage)
	item := newElementControl(environment, nativeOne)
	elementControlClassify(environment, &item)
	nativeEqual(t, item.control.Kind, expected)
}

func TestInputButtonClassificationIgnoresAnalogUsage(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.AxisX)

	environment.api.IOHIDElementGetType = func(native.IOHIDElementRef) native.IOHIDElementType { return nativeTwo }

	item := newElementControl(environment, nativeOne)
	elementControlClassify(environment, &item)
	nativeEqual(t, item.control.Kind, domain.ControlButton)
}

func TestConsumerBooleanDescriptorBecomesKey(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.HID(domain.PageConsumer, nativeOne))

	environment.api.IOHIDElementGetType = func(native.IOHIDElementRef) native.IOHIDElementType { return nativeOne }
	environment.api.IOHIDElementGetLogicalMax = func(native.IOHIDElementRef) cf.CFIndex { return nativeOne }

	item := newElementControl(environment, nativeOne)
	elementControlClassify(environment, &item)
	nativeEqual(t, item.control.Kind, domain.ControlKey)
}

func TestHatDescriptorRequiresSupportedRange(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.HatSwitch)
	item := newElementControl(environment, nativeOne)
	elementControlClassify(environment, &item)
	nativeEqual(t, item.control.Kind, domain.ControlAxis)

	environment.api.IOHIDElementGetLogicalMax = func(native.IOHIDElementRef) cf.CFIndex {
		return nativeEight - nativeOne
	}
	item = newElementControl(environment, nativeOne)
	elementControlClassify(environment, &item)
	nativeEqual(t, item.control.Kind, domain.ControlHat)
}

func TestRelativeWheelDefaultsToDetents(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.AxisWheel)

	environment.api.IOHIDElementIsRelative = func(native.IOHIDElementRef) bool { return true }

	item := newElementControl(environment, nativeOne)
	elementControlResolveMultiplier(environment, &item, nil)
	nativeEqual(t, item.control.Mode, domain.AxisRelative)
	nativeEqual(t, item.control.Unit, domain.UnitDetents)
	checkWheelMultiplier(t, environment, &item)
}

func checkWheelMultiplier(t *testing.T, environment *nativeState, item *elementControl) {
	t.Helper()

	key := wheelCollection(environment, item.element)
	multipliers := map[multiplierKey]multiplierValue{key: {value: nativeTwo, valid: true}}
	elementControlResolveMultiplier(environment, item, multipliers)
	nativeEqual(t, elementControlScaleValue(item, nativeFour), float64(nativeTwo))

	multipliers[key] = multiplierValue{value: nativeZero, valid: false}
	elementControlResolveMultiplier(environment, item, multipliers)
	nativeEqual(t, item.multiplier, float64(nativeZero))
	nativeEqual(t, elementControlScaleValue(item, nativeFour), float64(nativeFour))
}

func TestWheelCollectionScopeUsesNearestLogicalCollection(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.AxisWheel)

	configureWheelCollection(environment)
	nativeEqual(t, wheelCollection(environment, nativeOne), multiplierKey{nativeTwo, nativeZero})

	environment.api.IOHIDElementGetCollectionType = func(native.IOHIDElementRef) uint32 { return nativeOne }
	nativeEqual(t, wheelCollection(environment, nativeOne), multiplierKey{nativeZero, nativeZero})
}

func TestResolutionMultiplierInterpolatesPhysicalRange(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.AxisWheel)
	configureMultiplierValue(environment)

	value, valid := readMultiplier(environment, nativeOne, nativeOne)
	nativeEqual(t, valid, true)
	nativeEqual(t, value, float64(nativeTwo))
	checkDuplicateMultiplier(t, environment)
}

func configureMultiplierValue(environment *nativeState) {
	environment.api.IOHIDDeviceGetValue = func(
		_ native.IOHIDDeviceRef,
		_ native.IOHIDElementRef,
		value *native.IOHIDValueRef,
	) int32 {
		*value = nativeOne

		return nativeZero
	}
	environment.api.IOHIDValueGetLength = func(native.IOHIDValueRef) cf.CFIndex { return nativeOne }
	environment.api.IOHIDElementGetPhysicalMin = func(native.IOHIDElementRef) cf.CFIndex { return nativeTwo }
	environment.api.IOHIDElementGetPhysicalMax = func(native.IOHIDElementRef) cf.CFIndex { return nativeFour }
}

func checkDuplicateMultiplier(t *testing.T, environment *nativeState) {
	t.Helper()

	environment.api.IOHIDElementGetType = func(native.IOHIDElementRef) native.IOHIDElementType { return elementFeature }
	environment.api.IOHIDElementGetUsage = func(native.IOHIDElementRef) uint32 { return usageResolutionMultiplier }

	device := newCaptureState(
		newSession(newBackendState(nil)),
		nativeOne,
		&callbackSink{failure: nil},
	)
	multipliers := captureResolutionMultipliers(
		environment,
		device,
		[]native.IOHIDElementRef{nativeOne, nativeTwo},
	)
	nativeEqual(t, len(multipliers), nativeOne)
	nativeEqual(t, multipliers[wheelCollection(environment, nativeOne)].valid, false)
}

func TestInvalidResolutionValuesAreRejected(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.AxisWheel)
	value, valid := readMultiplier(environment, nativeOne, nativeOne)
	nativeEqual(t, valid, false)
	nativeEqual(t, value, float64(nativeZero))

	environment.api.IOHIDElementGetLogicalMax = func(native.IOHIDElementRef) cf.CFIndex { return nativeZero }
	value, valid = elementMultiplier(environment, nativeOne, nativeOne)
	nativeEqual(t, valid, false)
	nativeEqual(t, value, float64(nativeZero))
}

func TestUnitExponentDecodesSignedEncodings(t *testing.T) {
	t.Parallel()
	nativeEqual(t, unitExponent(nativeOne), int32(nativeOne))
	nativeEqual(t, unitExponent(unitExponentMask), int32(-nativeOne))
	nativeEqual(t, unitExponent(math.MaxUint32), int32(-nativeOne))
	nativeEqual(t, signedNativeValue(nativeOne), int32(nativeOne))
	nativeEqual(t, validMultiplier(math.NaN()), false)
	nativeEqual(t, validMultiplier(math.Inf(nativeOne)), false)
}

func configureDescriptorProperties(environment *nativeState) {
	environment.api.IOHIDElementGetType = func(native.IOHIDElementRef) native.IOHIDElementType {
		return elementInputAxis
	}
	environment.api.IOHIDElementGetLogicalMax = func(native.IOHIDElementRef) cf.CFIndex { return testAnchorTick }
	environment.api.IOHIDElementGetReportSize = func(native.IOHIDElementRef) uint32 { return nativeEight }
	environment.api.IOHIDElementGetReportCount = func(native.IOHIDElementRef) uint32 { return nativeOne }
	environment.api.IOHIDElementGetCookie = func(element native.IOHIDElementRef) uint32 {
		return uint32(element & math.MaxUint32)
	}
}

func configureWheelCollection(environment *nativeState) {
	environment.api.IOHIDElementGetParent = func(native.IOHIDElementRef) native.IOHIDElementRef { return nativeTwo }
	environment.api.IOHIDElementGetType = func(native.IOHIDElementRef) native.IOHIDElementType {
		return elementCollection
	}
	environment.api.IOHIDElementGetCollectionType = func(native.IOHIDElementRef) uint32 { return nativeTwo }
}

func TestNativeValuePublishesCopiedEvent(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.AxisX)
	device := newCaptureState(
		newSession(newBackendState(nil)),
		nativeZero,
		new(sinkview.Operations[domain.Event]),
	)

	configureEventValues(environment)
	captureBuildControls(environment, device, []native.IOHIDElementRef{nativeOne})

	events := collectNativeEvents(device)
	captureReceiveValue(environment, device, nativeOne)
	nativeEqual(t, len(*events), nativeOne)
	nativeEqual(t, (*events)[nativeZero].Value, float64(testTickDelta))
	nativeEqual(t, (*events)[nativeZero].Timestamp.Source, domain.TimestampReceipt)
}

func configureEventValues(environment *nativeState) {
	environment.api.IOHIDValueGetLength = func(native.IOHIDValueRef) cf.CFIndex { return nativeOne }
	environment.api.IOHIDValueGetElement = func(native.IOHIDValueRef) native.IOHIDElementRef { return nativeOne }
	environment.api.IOHIDValueGetIntegerValue = func(native.IOHIDValueRef) cf.CFIndex { return testTickDelta }
}

func collectNativeEvents(device *capture) *[]domain.Event {
	events := new([]domain.Event)
	sink := new(sinkview.Operations[domain.Event])

	sink.Operations.Publish = func(event *domain.Event) bool {
		*events = append(*events, *event)

		return true
	}
	sink.Operations.Fail = func(error) {}
	device.sink = sink

	return events
}

func TestUnsupportedAndMissingNativeValuesAreIgnored(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.AxisX)
	device := newCaptureState(
		newSession(newBackendState(nil)),
		nativeZero,
		&callbackSink{failure: nil},
	)
	events := collectNativeEvents(device)
	captureReceiveValue(environment, device, nativeZero)
	captureReceiveValue(environment, device, nativeOne)
	configureEventValues(environment)
	captureReceiveValue(environment, device, nativeOne)
	nativeEqual(t, len(*events), nativeZero)
}

func TestDigitalValuesSuppressRepeatedState(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.HID(domain.PageButton, nativeOne))
	device := newCaptureState(
		newSession(newBackendState(nil)),
		nativeZero,
		&callbackSink{failure: nil},
	)
	item := newElementControl(environment, nativeOne)
	elementControlClassify(environment, &item)
	checkDigitalEvents(t, device, &item)
}

func checkDigitalEvents(t *testing.T, device *capture, item *elementControl) {
	t.Helper()

	event := captureScalarEvent(device, item, nativeOne)
	nativeEqual(t, event.Action, domain.ActionPress)
	nativeEqual(t, captureScalarEvent(device, item, nativeOne) == nil, true)

	event = captureScalarEvent(device, item, nativeZero)
	nativeEqual(t, event.Action, domain.ActionRelease)
	nativeEqual(t, event.Value, float64(nativeZero))
}

func TestHatValuesDecodeDirection(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.HatSwitch)

	environment.api.IOHIDElementGetLogicalMax = func(native.IOHIDElementRef) cf.CFIndex {
		return nativeEight - nativeOne
	}

	device := newCaptureState(
		newSession(newBackendState(nil)),
		nativeZero,
		&callbackSink{failure: nil},
	)
	item := newElementControl(environment, nativeOne)
	elementControlClassify(environment, &item)

	checkHatEvents(t, device, &item)
}

func TestRepeatedDigitalValueDoesNotPublish(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.HID(domain.PageButton, nativeOne))
	device := newCaptureState(
		newSession(newBackendState(nil)),
		nativeZero,
		&callbackSink{failure: nil},
	)

	configureEventValues(environment)
	captureBuildControls(environment, device, []native.IOHIDElementRef{nativeOne})

	events := collectNativeEvents(device)
	captureReceiveValue(environment, device, nativeOne)
	captureReceiveValue(environment, device, nativeOne)
	nativeEqual(t, len(*events), nativeOne)
}

func TestNativeTimestampUsesSessionAnchor(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()
	device := newCaptureState(
		newSession(newBackendState(nil)),
		nativeZero,
		&callbackSink{failure: nil},
	)

	environment.api.IOHIDValueGetTimeStamp = func(native.IOHIDValueRef) uint64 { return testLaterTick }

	timestamp := captureEventTimestamp(environment, device, nativeOne)
	nativeEqual(t, timestamp.Source, domain.TimestampEstimated)
	nativeEqual(t, timestamp.Time.IsZero(), false)
}

func checkHatEvents(t *testing.T, device *capture, item *elementControl) {
	t.Helper()

	event := captureScalarEvent(device, item, nativeOne)
	nativeEqual(t, event.Value, float64(domain.HatNorthEast))

	item.control.Kind = domain.ControlKind(domain.HatSwitch.ID() & math.MaxUint8)
	nativeEqual(t, captureScalarEvent(device, item, nativeOne) == nil, true)
}

func (sink *callbackSink) Fail(err error) { sink.failure = err }

func (*callbackSink) Publish(*domain.Event) bool { return true }

func TestInputCallbackRecoversBindingPanic(t *testing.T) {
	t.Parallel()

	environment := new(nativeState)

	environment.api.IOHIDValueGetLength = nil

	sink := &callbackSink{failure: nil}
	token := registerTestCapture(t, environment, sink)
	inputValueCallback(
		environment,
		newInputValueCallbackArguments(token, nativeZero, testValueHandle),
	)

	if !errors.Is(sink.failure, domain.ErrEventLoss) {
		t.Fatalf("callback panic did not terminate the stream: %v", sink.failure)
	}
}

func TestInputCallbackReportsNativeFailure(t *testing.T) {
	t.Parallel()

	environment := new(nativeState)

	sink := &callbackSink{failure: nil}
	token := registerTestCapture(t, environment, sink)
	inputValueCallback(
		environment,
		newInputValueCallbackArguments(token, ioNoDevice, nativeZero),
	)

	if !errors.Is(sink.failure, domain.ErrDisconnected) {
		t.Fatalf("native callback failure lost its cause: %v", sink.failure)
	}
}

func registerTestCapture(t *testing.T, environment *nativeState, sink *callbackSink) uintptr {
	t.Helper()

	token := uintptr(environment.nextCallbackToken.Add(nativeOne))
	capture := new(capture)

	capture.sink, capture.token = sink, token
	capture.info.Transport = domain.TransportUnknown
	capture.caps.Repeat = domain.SupportUnknown
	environment.callbackRegistry.Store(token, capture)
	t.Cleanup(func() { environment.callbackRegistry.Delete(token) })

	return token
}

func TestTickDurationSignsAndOverflow(t *testing.T) {
	t.Parallel()

	environment := new(nativeState)

	environment.timebase = machTimebase{Numer: testValueHandle, Denom: testValueHandle}

	examples := []tickExample{
		{testEarlierTick, testAnchorTick, -testTickDelta * time.Nanosecond, true},
		{testLaterTick, testAnchorTick, testTickDelta * time.Nanosecond, true},
		{testAnchorTick, testAnchorTick, nativeZero, true},
		{math.MaxUint64, nativeZero, nativeZero, false},
	}
	for index := range examples {
		checkTickDuration(t, environment, &examples[index])
	}
}

func TestTickDurationRejectsInvalidTimebase(t *testing.T) {
	t.Parallel()

	environment := new(nativeState)

	environment.timebase = machTimebase{Denom: nativeZero, Numer: testValueHandle}

	checkTickDuration(
		t,
		environment,
		&tickExample{testLaterTick, testAnchorTick, nativeZero, false},
	)

	environment.timebase = machTimebase{Numer: math.MaxUint32, Denom: testValueHandle}

	checkTickDuration(t, environment, &tickExample{math.MaxUint64, nativeZero, nativeZero, false})
}

func TestCallbackRegistriesAreIsolated(t *testing.T) {
	t.Parallel()

	first, second := new(nativeState), new(nativeState)
	firstSink, secondSink := &callbackSink{failure: nil}, &callbackSink{failure: nil}
	firstToken := registerTestCapture(t, first, firstSink)
	secondToken := registerTestCapture(t, second, secondSink)

	if firstToken != secondToken {
		t.Fatal("independent registries should start with the same numeric token")
	}

	inputValueCallback(
		first,
		newInputValueCallbackArguments(firstToken, ioNoDevice, nativeZero),
	)
	checkIsolatedCallbackFailure(t, firstSink, secondSink)
}

func checkIsolatedCallbackFailure(t *testing.T, first, second *callbackSink) {
	t.Helper()

	if !errors.Is(first.failure, domain.ErrDisconnected) || second.failure != nil {
		t.Fatalf(
			"callback crossed registry ownership: first=%v second=%v",
			first.failure,
			second.failure,
		)
	}
}

func checkTickDuration(t *testing.T, environment *nativeState, example *tickExample) {
	t.Helper()

	duration, valid := tickDuration(environment, example.ticks, example.anchor)
	if duration != example.duration || valid != example.valid {
		t.Fatalf(
			"tickDuration = (%v, %v), want (%v, %v)",
			duration,
			valid,
			example.duration,
			example.valid,
		)
	}
}

func nativeCollection(t *testing.T, symbols *collectionSymbols) uintptr {
	t.Helper()

	var ref uintptr

	err := coreFoundationCall(
		symbols.create,
		func(create func(uintptr, int, uintptr) uintptr) error {
			ref = create(nativeZero, nativeZero, nativeZero)

			return nil
		},
	)
	nativeCause(t, err, nil)
	t.Cleanup(func() { nativeCause(t, releaseNative(ref), nil) })
	insertNativeValues(t, ref, symbols.insert)

	return ref
}

func insertNativeValues(t *testing.T, ref uintptr, symbol string) {
	t.Helper()

	err := coreFoundationCall(symbol, func(insert func(uintptr, uintptr)) error {
		insert(ref, nativeTwo)
		insert(ref, nativeOne)

		return nil
	})
	nativeCause(t, err, nil)
}

func TestCoreFoundationSetCopiesDeviceReferences(t *testing.T) {
	t.Parallel()

	ref := nativeCollection(
		t,
		&collectionSymbols{create: "CFSetCreateMutable", insert: "CFSetAddValue"},
	)
	devices, err := setDevices(cf.CFSetRef(ref), nativeTwo)
	nativeCause(t, err, nil)
	nativeEqual(t, len(devices), nativeTwo)
	nativeEqual(
		t,
		devices[nativeZero]+devices[nativeOne],
		native.IOHIDDeviceRef(nativeOne+nativeTwo),
	)
}

func TestCoreFoundationArrayCopiesDescriptorReferences(t *testing.T) {
	t.Parallel()

	ref := nativeCollection(
		t,
		&collectionSymbols{create: "CFArrayCreateMutable", insert: "CFArrayAppendValue"},
	)
	elements, err := matchingElements(fakeNativeState(), cf.CFArrayRef(ref))
	nativeCause(t, err, nil)
	nativeEqual(t, len(elements), nativeTwo)
	nativeEqual(t, elements[nativeZero], native.IOHIDElementRef(nativeTwo))
}

func inventoryState() *nativeState {
	environment := fakeNativeState()

	configureInventoryCollections(environment)
	configureInventoryIdentifiers(environment)

	return environment
}

func configureInventoryIdentifiers(environment *nativeState) {
	environment.api.IOHIDDeviceGetService = func(ref native.IOHIDDeviceRef) uint32 { return uint32(ref & nativeTwo) }
	environment.api.IORegistryEntryGetRegistryEntryID = func(service uint32, ref *uint64) int32 {
		*ref = uint64(service)

		return nativeZero
	}
}

func TestDiscoverySortsAccessibleEndpoints(t *testing.T) {
	t.Parallel()

	environment := inventoryState()
	session := testNativeSession(t)
	infos, err := sessionDiscover(t.Context(), environment, session)
	nativeCause(t, err, nil)
	nativeEqual(t, len(infos), nativeTwo)
	nativeEqual(t, infos[nativeZero].ID < infos[nativeOne].ID, true)
	checkMissingAndCanceledDiscovery(t, environment, session)
}

func checkMissingAndCanceledDiscovery(t *testing.T, environment *nativeState, session *session) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	nativeCause(t, resultError(sessionDiscover(ctx, environment, session)), context.Canceled)

	search := newSessionFindDeviceArguments(session, testMissingDevice)
	nativeCause(
		t,
		resultError(sessionFindDevice(t.Context(), environment, search)),
		domain.ErrNotFound,
	)
	nativeCause(t, resultError(sessionFindDevice(ctx, environment, search)), context.Canceled)
}

func TestDeviceSearchRetainsMatchingReference(t *testing.T) {
	t.Parallel()

	environment := inventoryState()
	session := testNativeSession(t)
	infos, err := sessionDiscover(t.Context(), environment, session)
	nativeCause(t, err, nil)

	configureOwnedDeviceCreation(t, environment)

	ref, err := sessionFindDevice(
		t.Context(),
		environment,
		newSessionFindDeviceArguments(session, infos[nativeZero].ID),
	)
	nativeCause(t, err, nil)
	nativeEqual(t, ref.ref != nativeZero, true)
	nativeCause(t, releaseNative(uintptr(ref.ref)), nil)
}

func TestInvalidInventoryCountReleasesSet(t *testing.T) {
	t.Parallel()

	environment := inventoryState()

	environment.core.setCount = func(cf.CFSetRef) int { return maxNativeElements + nativeOne }

	session := testNativeSession(t)
	nativeCause(
		t,
		resultError(sessionDiscover(t.Context(), environment, session)),
		domain.ErrUnsupported,
	)

	request := newSessionFindDeviceArguments(session, testMissingDevice)
	nativeCause(
		t,
		resultError(sessionFindDevice(t.Context(), environment, request)),
		domain.ErrUnsupported,
	)
}

func TestInventoryReadFailurePreservesDiagnostic(t *testing.T) {
	t.Parallel()

	environment := inventoryState()

	environment.core.setDevices = func(cf.CFSetRef, int) ([]native.IOHIDDeviceRef, error) {
		return nil, domain.ErrDisconnected
	}
	nativeCause(
		t,
		resultError(sessionDiscover(t.Context(), environment, testNativeSession(t))),
		domain.ErrDisconnected,
	)
}

func TestCapabilitiesCopyAndReleaseMatchingArray(t *testing.T) {
	t.Parallel()

	environment := descriptorState(domain.AxisX)
	configureMatchingArray(environment)

	device := newCaptureState(
		newSession(newBackendState(nil)),
		nativeOne,
		&callbackSink{failure: nil},
	)
	nativeCause(t, captureLoadCapabilities(environment, device), nil)
	nativeEqual(t, len(device.caps.Controls), nativeTwo)
	checkInvalidMatchingArray(t, environment, device)
}

func checkInvalidMatchingArray(t *testing.T, environment *nativeState, device *capture) {
	t.Helper()

	environment.core.arrayCount = func(cf.CFArrayRef) int { return maxNativeElements + nativeOne }
	nativeCause(t, captureLoadCapabilities(environment, device), domain.ErrUnsupported)
}

func TestNewCaptureRejectsInvalidCapabilities(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()

	configureInvalidCapabilities(environment)

	session := testNativeSession(t)
	request := newSessionNewCaptureArguments(&callbackSink{failure: nil}, session, nativeOne)
	nativeCause(t, resultError(sessionNewCapture(environment, request)), domain.ErrUnsupported)
}

func configureInventoryCollections(environment *nativeState) {
	environment.api.IOHIDManagerCopyDevices = func(native.IOHIDManagerRef) cf.CFSet { return nativeOne }
	environment.core.release = func(ref uintptr) error {
		if ref == nativeOne {
			return nil
		}

		return releaseNative(ref)
	}
	environment.core.setCount = func(cf.CFSetRef) int { return nativeTwo }
	environment.core.setDevices = func(cf.CFSetRef, int) ([]native.IOHIDDeviceRef, error) {
		return []native.IOHIDDeviceRef{nativeTwo, nativeOne}, nil
	}
}

func configureOwnedDeviceCreation(t *testing.T, environment *nativeState) {
	t.Helper()

	environment.api.IOHIDDeviceCreate = func(cf.CFAllocatorRef, uint32) native.IOHIDDeviceRef {
		return native.IOHIDDeviceRef(ownedNativeString(t))
	}
}

func configureMatchingArray(environment *nativeState) {
	ref := cf.CFArrayRef(nativeOne)

	environment.core.arrayCount = func(cf.CFArrayRef) int { return nativeTwo }
	environment.core.arrayElement = func(_ cf.CFArrayRef, index int) native.IOHIDElementRef {
		return native.IOHIDElementRef(index + nativeOne)
	}
	environment.core.release = func(uintptr) error { return nil }
	environment.api.IOHIDDeviceCopyMatchingElements = func(
		native.IOHIDDeviceRef, cf.CFDictionaryRef, uint32,
	) cf.CFArrayRef {
		return ref
	}
}

func configureInvalidCapabilities(environment *nativeState) {
	environment.core.arrayCount = func(cf.CFArrayRef) int { return maxNativeElements + nativeOne }
	environment.core.release = func(ref uintptr) error {
		if ref == nativeOne {
			return nil
		}

		return releaseNative(ref)
	}
	environment.api.IOHIDDeviceCopyMatchingElements = func(
		native.IOHIDDeviceRef, cf.CFDictionaryRef, uint32,
	) cf.CFArrayRef {
		return nativeOne
	}
}

func nativeCause(t *testing.T, actual, expected error) {
	t.Helper()

	if !errors.Is(actual, expected) {
		t.Fatalf("error = %v, want cause %v", actual, expected)
	}
}

func nativeEqual[Value comparable](t *testing.T, actual, expected Value) {
	t.Helper()

	if actual != expected {
		t.Fatalf("value = %v, want %v", actual, expected)
	}
}

func fakeNativeState() *nativeState {
	environment := new(nativeState)

	environment.core = defaultCoreAPI()

	configureZeroNativeAPI(environment)
	environment.symbolOnce.Do(func() {})

	configureNativeClock(environment)

	return environment
}

func zeroNativeResults(signature reflect.Type) func([]reflect.Value) []reflect.Value {
	return func([]reflect.Value) []reflect.Value {
		results := make([]reflect.Value, nativeZero, signature.NumOut())
		for out := range signature.Outs() {
			results = append(results, reflect.Zero(out))
		}

		return results
	}
}

func ownedNativeString(t *testing.T) cf.CFStringRef {
	t.Helper()

	ref := cf.CFStringCreateWithCString(nativeZero, testNativeResource, utf8Encoding)
	if ref == nativeZero {
		t.Fatal("CoreFoundation did not allocate a test resource")
	}

	return ref
}

func testNativeSession(t *testing.T) *session {
	t.Helper()

	session := newSession(newBackendState(nil))

	t.Cleanup(func() { nativeCause(t, sessionClose(fakeNativeState(), session), nil) })

	return session
}

func TestBackendOwnsRunLoopAndManager(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()

	environment.api.IOHIDManagerCreate = func(cf.CFAllocatorRef, uint32) native.IOHIDManagerRef {
		return native.IOHIDManagerRef(ownedNativeString(t))
	}

	backend, err := newBackend(t.Context(), environment, nil)
	nativeCause(t, err, nil)
	checkBackendDiscovery(t, backend)
	checkBackendMissingDevice(t, backend)
	nativeCause(t, backend.Close(), nil)
	nativeCause(t, backend.Close(), nil)
}

func checkBackendDiscovery(t *testing.T, backend *backendOperations) {
	t.Helper()

	devices, err := backend.Discover(t.Context())
	nativeCause(t, err, nil)
	nativeEqual(t, len(devices), nativeZero)
}

func checkBackendMissingDevice(t *testing.T, backend *backendOperations) {
	t.Helper()
	nativeCause(
		t,
		resultError(backend.Open(t.Context(), testMissingDevice, nil)),
		domain.ErrInvalidOptions,
	)

	sink := &callbackSink{failure: nil}
	device, err := backend.Open(t.Context(), testMissingDevice, sink)
	nativeCause(t, err, domain.ErrNotFound)
	nativeEqual(t, device == nil, true)
}

func TestBackendStartupFailureClosesThread(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()
	backend, err := newBackend(t.Context(), environment, nil)
	nativeCause(t, err, domain.ErrUnsupported)
	nativeEqual(t, backend == nil, true)
}

func TestBackendCreationRejectsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	nativeCause(t, resultError(newBackend(ctx, fakeNativeState(), nil)), context.Canceled)
	nativeCause(t, resultError(Factory()(ctx, nil)), context.Canceled)
}

func TestCaptureOpenCommitsAndClosesOwnedDevice(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()
	session := testNativeSession(t)
	resources := newDeviceResources(native.IOHIDDeviceRef(ownedNativeString(t)))
	request := newSessionOpenDeviceArguments(
		session,
		newCaptureSetup(resources, &callbackSink{failure: nil}),
	)
	device, err := sessionOpenDevice(t.Context(), environment, request)
	nativeCause(t, err, nil)
	checkCaptureSnapshots(t, environment, device)
	checkCaptureClosed(t, func() error { return sessionCloseCapture(environment, session, device) })
	nativeEqual(t, registeredCapture(environment, resources.token) == nil, true)
}

func checkCaptureSnapshots(t *testing.T, environment *nativeState, device *capture) {
	t.Helper()

	view := captureView(t.Context(), environment, device)
	nativeEqual(t, view.Info().ID != "", true)
	nativeEqual(t, view.Capabilities().Complete, false)

	var metadata extension.Metadata

	nativeEqual(t, view.Extension(&metadata), true)

	var provider extension.MetadataProvider

	nativeEqual(t, view.Extension(&provider), true)
	nativeEqual(t, provider.NativeInfo().RegistryEntryID, uint64(testAnchorTick))
	nativeEqual(t, view.Extension(new(bool)), false)
}

func TestCaptureOpenFailureReleasesDevice(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()

	environment.api.IOHIDDeviceOpen = func(native.IOHIDDeviceRef, uint32) int32 { return ioNotPermitted }

	session := testNativeSession(t)
	request := ownedCaptureRequest(t, session)
	device, err := sessionOpenDevice(t.Context(), environment, request)
	nativeCause(t, err, domain.ErrPermissionDenied)
	nativeEqual(t, device == nil, true)
	nativeEqual(t, len(session.captures), nativeZero)
}

func TestCaptureMetadataFailureReleasesDevice(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()

	environment.api.IORegistryEntryGetRegistryEntryID = func(uint32, *uint64) int32 { return ioNoDevice }

	session := testNativeSession(t)
	resources := newDeviceResources(native.IOHIDDeviceRef(ownedNativeString(t)))
	request := newSessionOpenDeviceArguments(
		session,
		newCaptureSetup(resources, &callbackSink{failure: nil}),
	)
	nativeCause(
		t,
		resultError(sessionOpenDevice(t.Context(), environment, request)),
		domain.ErrDisconnected,
	)
}

func TestCaptureTokenExhaustionReleasesDevice(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()
	environment.nextCallbackToken.Store(math.MaxUint64)

	session := testNativeSession(t)
	resources := newDeviceResources(native.IOHIDDeviceRef(ownedNativeString(t)))
	request := newSessionOpenDeviceArguments(
		session,
		newCaptureSetup(resources, &callbackSink{failure: nil}),
	)
	nativeCause(
		t,
		resultError(sessionOpenDevice(t.Context(), environment, request)),
		domain.ErrUnsupported,
	)
	nativeEqual(t, len(session.captures), nativeZero)
}

func TestCanceledCaptureCommitRollsBackRegistration(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()
	session := testNativeSession(t)
	resources := newDeviceResources(native.IOHIDDeviceRef(ownedNativeString(t)))
	request := newSessionOpenDeviceArguments(
		session,
		newCaptureSetup(resources, &callbackSink{failure: nil}),
	)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	nativeCause(t, resultError(sessionOpenDevice(ctx, environment, request)), context.Canceled)
	nativeEqual(t, registeredCapture(environment, resources.token) == nil, true)
}

func TestCaptureBindingPanicRollsBackOpen(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()

	environment.api.IOHIDDeviceOpen = nil

	session := testNativeSession(t)
	resources := newDeviceResources(native.IOHIDDeviceRef(ownedNativeString(t)))
	request := newSessionOpenDeviceArguments(
		session,
		newCaptureSetup(resources, &callbackSink{failure: nil}),
	)
	nativeCause(
		t,
		resultError(sessionOpenDevice(t.Context(), environment, request)),
		domain.ErrUnsupported,
	)
}

func TestMetadataPointersAreCopied(t *testing.T) {
	t.Parallel()

	device := new(capture)
	location := uint32(testAnchorTick)

	device.metadata.LocationID = &location

	metadata := captureIOKitMetadata(device)

	*metadata.LocationID = testLaterTick

	nativeEqual(t, location, uint32(testAnchorTick))

	var target *extension.Metadata

	nativeEqual(t, captureExtension(device, target), false)
}

func TestRemovalCallbackTerminatesOwnedCapture(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()
	sink := &callbackSink{failure: nil}
	token := registerTestCapture(t, environment, sink)
	deviceRemovalCallback(environment, newDeviceRemovalCallbackArguments(token))
	nativeCause(t, sink.failure, domain.ErrDisconnected)
	deviceRemovalCallback(environment, newDeviceRemovalCallbackArguments(nativeZero))
}

func TestInvalidCallbackRegistryEntryIsIgnored(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()
	environment.callbackRegistry.Store(uintptr(testValueHandle), testWrongType)
	nativeEqual(t, registeredCapture(environment, testValueHandle) == nil, true)
	inputValueCallback(
		environment,
		newInputValueCallbackArguments(nativeZero, nativeZero, nativeZero),
	)
}

func TestSessionPanicFailsRemainingCaptures(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()
	session := newSession(newBackendState(nil))
	sink := &callbackSink{failure: nil}
	device := newCaptureState(session, native.IOHIDDeviceRef(ownedNativeString(t)), sink)

	session.captures[device] = struct{}{}
	sessionFinish(environment, session, "test binding panic")
	nativeCause(t, sink.failure, domain.ErrClosed)
	nativeCause(t, session.backend.closeErr, domain.ErrEventLoss)
}

func configureZeroNativeAPI(environment *nativeState) {
	functions := reflect.ValueOf(&environment.api).Elem()

	for index := range functions.Type().NumField() {
		field := functions.Field(index)
		field.Set(reflect.MakeFunc(field.Type(), zeroNativeResults(field.Type())))
	}
}

func checkCaptureClosed(t *testing.T, closeCapture func() error) {
	t.Helper()
	nativeCause(t, closeCapture(), nil)
	nativeCause(t, closeCapture(), nil)
}

func ownedCaptureRequest(t *testing.T, session *session) *sessionOpenDeviceArguments {
	t.Helper()

	resources := newDeviceResources(native.IOHIDDeviceRef(ownedNativeString(t)))

	return newSessionOpenDeviceArguments(
		session,
		newCaptureSetup(resources, &callbackSink{failure: nil}),
	)
}

func configureNativeClock(environment *nativeState) {
	environment.timebase = machTimebase{Numer: nativeOne, Denom: nativeOne}
	environment.machAbsoluteTime = func() uint64 { return testAnchorTick }
	environment.api.IORegistryEntryGetRegistryEntryID = func(_ uint32, value *uint64) int32 {
		*value = testAnchorTick

		return nativeZero
	}
}

func TestNativeSymbolInitialization(t *testing.T) {
	t.Parallel()

	environment := newNativeTestState()
	nativeCause(t, loadSymbols(environment), nil)
	nativeCause(t, loadSymbols(environment), nil)
	nativeEqual(t, environment.valueCallback != nativeZero, true)
	purego.SyscallN(environment.valueCallback, nativeZero, nativeZero, nativeZero, nativeZero)
	purego.SyscallN(environment.removalCallback, nativeZero, nativeZero, nativeZero)
	nativeCause(t, bindNativeAPI(environment, environment.nativeLibraryHandles[nativeOne]), nil)

	checkNativeHIDProbes(t, environment)

	closeNativeLibraries(t, environment)
}

func checkNativeHIDProbes(t *testing.T, environment *nativeState) {
	t.Helper()

	environment.core.probeHID = func() (any, error) { return uintptr(nativeOne), nil }
	nativeCause(t, loadHIDFunctions(environment, environment.nativeLibraryHandles[nativeOne]), nil)

	environment.core.probeHID = func() (any, error) { return nil, domain.ErrUnsupported }
	nativeCause(t, loadHIDFunctions(environment, environment.nativeLibraryHandles[nativeOne]), nil)
}

func closeNativeLibraries(t *testing.T, environment *nativeState) {
	t.Helper()

	for index := range environment.nativeLibraryHandles {
		nativeCause(t, purego.Dlclose(environment.nativeLibraryHandles[index]), nil)
	}
}

func TestNativeLoadingFailures(t *testing.T) {
	t.Parallel()

	environment := newNativeTestState()

	checkNativeBindingFailures(t, environment)

	checkNativeFallbackBindingFailure(t, environment)

	checkNativeFrameworkAndClockFailures(t, environment)

	checkNativeTimebaseFailures(t, environment)

	checkNativeInitializationFailure(t, environment)

	checkNativeFrameworkLoadingFailure(t, environment)

	checkNativeLibrarySymbolFailures(t)
}

func checkNativeBindingFailures(t *testing.T, environment *nativeState) {
	t.Helper()

	environment.core.openLibrary = unavailableNativeLibrary[uintptr]
	nativeCause(t, initializeSymbols(environment), domain.ErrUnsupported)
	nativeEqual(t, loadFramework(environment), uintptr(nativeZero))
	nativeCause(t, loadHIDFunctions(environment, nativeZero), domain.ErrUnsupported)

	checkNativeAPIBindingFailures(t, environment)
}

func checkNativeAPIBindingFailures(t *testing.T, environment *nativeState) {
	t.Helper()

	environment.core.bind = func(uintptr, string, any) error { return domain.ErrUnsupported }
	nativeCause(t, bindClock(environment, nativeOne), domain.ErrUnsupported)
	nativeCause(t, bindCallbacks(environment, nativeOne), domain.ErrUnsupported)
	nativeCause(t, bindNativeAPI(environment, nativeOne), domain.ErrUnsupported)
	nativeCause(t, loadHIDFunctions(environment, nativeOne), domain.ErrUnsupported)
}

func checkNativeFallbackBindingFailure(t *testing.T, environment *nativeState) {
	t.Helper()

	environment.core.probeHID = func() (any, error) { return nil, domain.ErrUnsupported }
	// Callback symbols bind successfully before the full fallback binding fails.
	calls := nativeZero

	environment.core.bind = func(uintptr, string, any) error {
		calls++
		if calls > nativeTwo {
			return domain.ErrUnsupported
		}

		return nil
	}
	nativeCause(t, loadHIDFunctions(environment, nativeOne), domain.ErrUnsupported)
}

func checkNativeFrameworkAndClockFailures(t *testing.T, environment *nativeState) {
	t.Helper()

	environment.core.bind = func(uintptr, string, any) error { return domain.ErrUnsupported }
	configureFrameworkPath(environment, nativeOne)
	nativeEqual(t, environment.registryPath == nil, true)

	environment.core.openLibrary = func(string, int) (uintptr, error) { return nativeOne, nil }
	nativeCause(t, loadClock(environment), domain.ErrUnsupported)
}

func checkNativeTimebaseFailures(t *testing.T, environment *nativeState) {
	t.Helper()

	environment.core.bind = func(_ uintptr, name string, _ any) error {
		if name == "mach_timebase_info" {
			return domain.ErrUnsupported
		}

		return nil
	}
	nativeCause(t, bindClock(environment, nativeOne), domain.ErrUnsupported)

	environment.machTimebaseInfo = func(*machTimebase) int32 { return nativeOne }
	nativeCause(t, validateTimebase(environment), domain.ErrUnsupported)
}

func checkNativeInitializationFailure(t *testing.T, environment *nativeState) {
	t.Helper()

	environment.machTimebaseInfo = func(base *machTimebase) int32 {
		base.Numer = nativeOne
		base.Denom = nativeOne

		return nativeZero
	}
	nativeCause(t, initializeSymbols(environment), domain.ErrUnsupported)
}

func checkNativeFrameworkLoadingFailure(t *testing.T, environment *nativeState) {
	t.Helper()

	environment.core.openLibrary = func(path string, flags int) (uintptr, error) {
		if path == "/usr/lib/libSystem.B.dylib" {
			return purego.Dlopen(path, flags)
		}

		return unavailableNativeLibrary[uintptr](path, flags)
	}
	environment.core.bind = bind
	nativeCause(t, initializeSymbols(environment), domain.ErrUnsupported)

	libraries := environment.nativeLibraryHandles[nativeOne+nativeTwo:]
	for index := range libraries {
		nativeCause(t, purego.Dlclose(libraries[index]), nil)
	}
}

func checkNativeLibrarySymbolFailures(t *testing.T) {
	t.Helper()

	nativeCause(
		t,
		nativeLibraryCall("/missing/goinput", testMissingDevice, func(func()) error { return nil }),
		domain.ErrUnsupported,
	)

	library, err := purego.Dlopen(coreFoundationLibrary, purego.RTLD_NOW|purego.RTLD_LOCAL)
	nativeCause(t, err, nil)

	t.Cleanup(func() { nativeCause(t, purego.Dlclose(library), nil) })

	nativeCause(t, bind(library, "missing_goinput_symbol", new(func())), domain.ErrUnsupported)
}

func TestNativeProperties(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()
	session := testNativeSession(t)
	checkNativeStringProperty(t, environment, session)

	checkNativeNumberProperties(t, environment, session)

	checkNativeClassesAndStrings(t, environment)

	checkNativeDevicePath(t, environment)

	checkNativeIdentifiersAndStatus(t)
}

func checkNativeStringProperty(t *testing.T, environment *nativeState, session *session) {
	t.Helper()

	text := ownedStringProperty(t)

	environment.api.IOHIDDeviceGetProperty = func(native.IOHIDDeviceRef, cf.CFStringRef) cf.CFTypeRef {
		return text
	}
	nativeEqual(
		t,
		sessionStringProperty(
			environment,
			newSessionStringPropertyArguments("text", session, nativeZero),
		),
		testNativeResource,
	)
}

func checkNativeNumberProperties(t *testing.T, environment *nativeState, session *session) {
	t.Helper()

	number := ownedNumberProperty(t)

	environment.api.IOHIDDeviceGetProperty = func(native.IOHIDDeviceRef, cf.CFStringRef) cf.CFTypeRef {
		return number
	}

	checkNativeNumberValues(t, environment, session)
}

func checkNativeNumberValues(t *testing.T, environment *nativeState, session *session) {
	t.Helper()

	value := int64(testPropertyNumber)

	actual, ok := sessionNumberProperty(
		environment,
		newSessionNumberPropertyArguments(testNumberProperty, session, nativeZero),
	)
	nativeEqual(t, actual, value)
	nativeEqual(t, ok, true)
	nativeEqual(
		t,
		sessionUsageProperty(
			environment,
			newSessionUsagePropertyArguments(testNumberProperty, session, nativeZero),
		),
		uint32(value),
	)
	checkNumberPropertyAsString(t, environment, session)
}

func checkNumberPropertyAsString(t *testing.T, environment *nativeState, session *session) {
	t.Helper()

	nativeEqual(
		t,
		sessionStringProperty(
			environment,
			newSessionStringPropertyArguments(testNumberProperty, session, nativeZero),
		),
		"",
	)
}

func checkNativeClassesAndStrings(t *testing.T, environment *nativeState) {
	t.Helper()

	environment.api.IOHIDDeviceConformsTo = func(native.IOHIDDeviceRef, uint32, uint32) bool { return true }
	nativeEqual(t, len(deviceClasses(environment, nativeZero)) > nativeZero, true)
	nativeEqual(
		t,
		containsClass(deviceClasses(environment, nativeZero), domain.ClassKeyboard),
		true,
	)
	nativeEqual(t, nulString([]byte{'a', nativeZero, 'b'}), "a")
	nativeEqual(t, nulString([]byte(testPlainString)), testPlainString)
}

func checkNativeDevicePath(t *testing.T, environment *nativeState) {
	t.Helper()

	environment.registryPath = func(_ uint32, plane string, output *byte) int32 {
		nativeEqual(t, plane, "IOService")

		*output = 'x'

		return nativeZero
	}
	nativeEqual(t, devicePath(environment, nativeOne), "x")

	environment.registryPath = func(uint32, string, *byte) int32 { return nativeOne }
	nativeEqual(t, devicePath(environment, nativeOne), "")
}

func checkNativeIdentifiersAndStatus(t *testing.T) {
	t.Helper()

	values := [...]int64{-nativeOne, maxNativeElements, testPropertyNumber}
	for index := range values {
		value := values[index]
		id := sessionIdentifier[uint16](func(string) (int64, bool) { return value, true }, "key")

		checkNativeIdentifier(t, id, value)
	}

	nativeCause(t, statusError(nativeOne), domain.ErrUnsupported)

	devices, err := setDevices(nativeZero, nativeZero)
	nativeCause(t, err, nil)
	nativeEqual(t, devices == nil, true)
}

func TestNativeReaderFailures(t *testing.T) {
	t.Parallel()

	checkInvalidNativeStringLengths(t)

	nativeEqual(t, nativeString(nativeOne, func([]byte) bool { return false }), "")

	devices, err := readSetDevices(
		nativeOne,
		func([]uintptr) error { return domain.ErrUnsupported },
	)
	nativeCause(t, err, domain.ErrUnsupported)
	nativeEqual(t, devices == nil, true)
}

func (coverageRetrier) Do(
	ctx context.Context,
	operation func(context.Context) error,
	transient func(error) bool,
) error {
	if transient(domain.ErrUnsupported) {
		return errUnexpectedExecution
	}

	return errors.Join(operation(ctx))
}

func TestDiscoveryRejectsMalformedReply(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()
	backend := newBackendState(coverageRetrier{})
	checkDiscoveryDoesNotRetry(t, backend)

	go serveNativeRequest(newSession(backend))

	// A fake native inventory panic reaches the job error path.
	environment.api.IOHIDManagerCopyDevices = nil
	nativeCause(
		t,
		resultError(backendDiscoverDevices(t.Context(), environment, backend)),
		domain.ErrUnsupported,
	)
	nativeCause(t, resultError(discoveredDevices("bad inventory")), domain.ErrUnsupported)
}

func checkDiscoveryDoesNotRetry(t *testing.T, backend *backend) {
	t.Helper()

	err := backendRunDiscovery(t.Context(), backend, func(context.Context) error {
		return domain.ErrUnsupported
	})
	nativeCause(t, err, domain.ErrUnsupported)
}

func ownedStringProperty(t *testing.T) cf.CFTypeRef {
	t.Helper()

	var text cf.CFTypeRef

	err := coreFoundationCall("CFStringCreateWithCString",
		func(create func(uintptr, string, uint32) cf.CFTypeRef) error {
			text = create(nativeZero, testNativeResource, utf8Encoding)

			return nil
		})
	nativeCause(t, err, nil)
	cleanupNativeProperty(t, text)

	return text
}

func ownedNumberProperty(t *testing.T) cf.CFTypeRef {
	t.Helper()

	var number cf.CFTypeRef

	value := int64(testPropertyNumber)
	err := coreFoundationCall("CFNumberCreate",
		func(create func(uintptr, int64, *int64) cf.CFTypeRef) error {
			number = create(nativeZero, int64(cf.KCFNumberSInt64Type), &value)

			return nil
		})
	nativeCause(t, err, nil)
	cleanupNativeProperty(t, number)

	return number
}

func cleanupNativeProperty(t *testing.T, property cf.CFTypeRef) {
	t.Helper()

	if property == nil {
		t.Fatal("CoreFoundation did not allocate a property")
	}

	t.Cleanup(func() {
		err := coreFoundationCall("CFRelease", func(release func(cf.CFTypeRef)) error {
			release(property)

			return nil
		})
		nativeCause(t, err, nil)
	})
}

func TestReadyCancellationAfterNotification(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()
	backend := newBackendState(nil)
	close(backend.done)

	backend.ready <- nil

	ctx := &cancelOnCheckContext{
		contextValueLookup: t.Context(), deadline: t.Context().Deadline, done: t.Context().Done,
	}
	nativeCause(t, resultError(backendAwaitReady(ctx, environment, backend)), context.Canceled)
}

func (ctx *cancelOnCheckContext) Deadline() (time.Time, bool) { return ctx.deadline() }

func (ctx *cancelOnCheckContext) Done() <-chan struct{} { return ctx.done() }

func (*cancelOnCheckContext) Err() error { return context.Canceled }

func TestRequestReadyCancellation(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	canceledBackend := newBackendState(nil)
	close(canceledBackend.done)
	nativeCause(
		t,
		resultError(backendAwaitReady(ctx, environment, canceledBackend)),
		context.Canceled,
	)
	nativeCause(
		t,
		resultError(backendReadyBackend(ctx, environment, canceledBackend)),
		context.Canceled,
	)
}

func TestRequestSendCancellationAndStop(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	perform := func(*session) (any, error) { return struct{}{}, nil }
	backend := newBackendState(nil)
	job := newRequest(t.Context(), perform)
	nativeCause(t, backendSendRequest(ctx, backend, &job), context.Canceled)
	nativeCause(
		t,
		resultError(backendCall(ctx, environment, newBackendCallArguments(backend, perform))),
		context.Canceled,
	)
}

func TestRequestStoppedBackend(t *testing.T) {
	t.Parallel()

	perform := func(*session) (any, error) { return struct{}{}, nil }
	backend := newBackendState(nil)
	job := newRequest(t.Context(), perform)

	close(backend.stop)
	nativeCause(t, backendSendRequest(t.Context(), backend, &job), domain.ErrClosed)
	nativeCause(t, resultError(submitRequest(t.Context(), backend, perform)), domain.ErrClosed)
}

func TestRequestCompletedBackend(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()

	job := newRequest(t.Context(), func(*session) (any, error) { return struct{}{}, nil })

	backend := newBackendState(nil)
	args := newBackendAwaitResponseArguments(backend, &job)
	close(backend.done)
	nativeCause(t, backendSendRequest(t.Context(), backend, &job), domain.ErrClosed)
	nativeCause(
		t,
		resultError(backendAwaitResponse(t.Context(), environment, args)),
		domain.ErrClosed,
	)

	discard := newBackendDiscardResponseArguments(backend, &job)
	backendDiscardResponse(t.Context(), environment, discard)
}

func TestRequestCanceledReadyNotification(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	backend := newBackendState(nil)
	close(backend.done)

	backend.ready <- nil

	nativeCause(t, resultError(backendAwaitReady(ctx, environment, backend)), context.Canceled)
}

func TestRequestCanceledResponse(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	job := newRequest(t.Context(), func(*session) (any, error) { return struct{}{}, nil })

	backend := newBackendState(nil)
	args := newBackendAwaitResponseArguments(backend, &job)
	nativeCause(
		t,
		resultError(backendAwaitResponse(ctx, environment, args)),
		context.Canceled,
	)
	close(backend.done)
}

func TestRequestDiscardMalformedResponse(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()

	job := newRequest(t.Context(), func(*session) (any, error) { return struct{}{}, nil })

	// The discarded response carries no capture and therefore needs no native cleanup.
	backend := newBackendState(nil)

	job.result <- response{value: testWrongType, err: nil}

	discard := newBackendDiscardResponseArguments(backend, &job)
	backendDiscardResponse(t.Context(), environment, discard)
	nativeCause(
		t,
		resultError(openedCapture(t.Context(), environment, testWrongType)),
		domain.ErrUnsupported,
	)
}

func TestCaptureCloseAndDiscard(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()
	s := newSession(newBackendState(nil))
	device := newCaptureState(s, nativeZero, &callbackSink{failure: nil})

	go serveNativeRequest(s)

	view, err := openedCapture(t.Context(), environment, device)
	nativeCause(t, err, nil)
	nativeCause(t, view.Close(), nil)
	nativeCause(t, view.Close(), nil)
	nativeEqual(t, device.closed.Load(), true)
}

func TestCaptureCloseAfterShutdown(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()

	backend := newBackendState(nil)
	close(backend.done)

	device := newCaptureState(newSession(backend), nativeZero, &callbackSink{failure: nil})
	nativeCause(t, captureClose(t.Context(), environment, device), nil)
}

func TestDiscardCaptureCleanupFailure(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()

	backend := newBackendState(nil)
	s := newSession(backend)

	sink := &callbackSink{failure: nil}

	device := newCaptureState(s, nativeZero, sink)
	// Inject a job failure to verify a discarded capture reports cleanup loss.
	device.closeOnce.Do(func() {})

	device.closeErr = domain.ErrUnsupported
	discardCapture(t.Context(), environment, device)
	nativeCause(t, sink.failure, domain.ErrUnsupported)
}

func TestOpenUsesOwnedDeviceAndReturnsView(t *testing.T) {
	t.Parallel()

	environment := inventoryState()
	session := testNativeSession(t)
	args := nativeOpenTarget(t, environment, session)

	go serveNativeOpenAndClose(session)

	checkOwnedDeviceView(t, backendView(environment, session.backend), args)

	environment.api.IOHIDDeviceOpen = nil
	nativeCause(
		t,
		resultError(sessionOpen(t.Context(), environment, newSessionOpenArguments(session, args))),
		domain.ErrUnsupported,
	)
}

func checkOwnedDeviceView(t *testing.T, backend *backendOperations, args *openTarget) {
	t.Helper()

	view, err := backend.Open(t.Context(), args.id, args.sink)
	nativeCause(t, err, nil)
	nativeEqual(t, view.Info().ID, args.id)
	nativeCause(t, view.Close(), nil)
}

func TestSessionResourceFailures(t *testing.T) {
	t.Parallel()

	environment := fakeNativeState()

	environment.symbolErr = domain.ErrUnsupported
	nativeCause(
		t,
		sessionStart(environment, newSession(newBackendState(nil))),
		domain.ErrUnsupported,
	)

	environment.core.runLoop = func() cf.CFRunLoopRef { return nativeZero }

	s := newSession(newBackendState(nil))
	nativeCause(t, sessionStartResources(environment, s), domain.ErrUnsupported)
	nativeCause(t, sessionClose(environment, s), nil)
}

func serveNativeRequest(session *session) {
	job := <-session.backend.jobs
	job(session)
}

func nativeOpenTarget(t *testing.T, environment *nativeState, session *session) *openTarget {
	t.Helper()

	infos, err := sessionDiscover(t.Context(), environment, session)
	nativeCause(t, err, nil)
	configureOwnedDeviceCreation(t, environment)

	return &openTarget{id: infos[nativeZero].ID, sink: &callbackSink{failure: nil}}
}

func serveNativeOpenAndClose(session *session) {
	serveNativeRequest(session)
	serveNativeRequest(session)
}

func TestQueuedCommandUsesSessionAndReturnsResponse(t *testing.T) {
	t.Parallel()

	result := make(chan response, nativeOne)
	command := requestRecord[int, response]{
		cause: t.Context().Err, result: result, pack: packResponse,
		perform: func(state int) (any, error) { return state, nil },
	}
	command.Execute(testAnchorTick)

	reply := <-result
	if reply.err != nil || reply.value != testAnchorTick {
		t.Fatalf("reply = (%v, %v), want (%d, nil)", reply.value, reply.err, testAnchorTick)
	}
}

func TestCanceledQueuedCommandDoesNotExecute(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result := make(chan response, nativeOne)
	command := requestRecord[int, response]{
		cause: ctx.Err, result: result, pack: packResponse,
		perform: func(int) (any, error) {
			t.Error("canceled queued command executed")

			return nil, errUnexpectedExecution
		},
	}
	command.Execute(testAnchorTick)

	if reply := <-result; !errors.Is(reply.err, context.Canceled) || reply.value != nil {
		t.Fatalf("canceled reply = (%v, %v)", reply.value, reply.err)
	}
}

func newNativeTestState() *nativeState {
	environment := new(nativeState)

	environment.core = defaultCoreAPI()

	return environment
}

func checkNativeIdentifier(t *testing.T, id *uint16, value int64) {
	t.Helper()

	if value == testPropertyNumber {
		nativeEqual(t, *id, uint16(testPropertyNumber))

		return
	}

	nativeEqual(t, id == nil, true)
}

func unavailableNativeLibrary[Handle ~uintptr](string, int) (Handle, error) {
	return Handle(nativeZero), domain.ErrUnsupported
}

func checkInvalidNativeStringLengths(t *testing.T) {
	t.Helper()

	lengths := [...]int{-nativeOne, maxNativeString}
	for index := range lengths {
		length := lengths[index]
		nativeEqual(
			t,

			nativeString(length, func([]byte) bool {
				t.Fatal("invalid length read")

				return true
			}),
			"",
		)
	}
}
