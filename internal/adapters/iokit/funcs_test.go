//go:build darwin && (amd64 || arm64)

// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package iokit

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/gostafa/goinput/internal/domain"
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
)

const (
	testValueHandle = 1
	testEarlierTick = 95
	testAnchorTick  = 100
	testLaterTick   = 105
	testTickDelta   = 5
)

func (sink *callbackSink) Fail(err error) { sink.failure = err }

func (*callbackSink) Publish(*domain.Event) bool { return true }

func TestInputCallbackRecoversBindingPanic(t *testing.T) {
	t.Parallel()

	environment := new(nativeState)

	environment.api.IOHIDValueGetLength = func(native.IOHIDValueRef) int { panic("binding failed") }

	sink := &callbackSink{failure: nil}
	token := registerTestCapture(environment, t, sink)
	environment.inputValueCallback(token, nativeZero, testValueHandle)

	if !errors.Is(sink.failure, domain.ErrEventLoss) {
		t.Fatalf("callback panic did not terminate the stream: %v", sink.failure)
	}
}

func TestInputCallbackReportsNativeFailure(t *testing.T) {
	t.Parallel()

	environment := new(nativeState)

	sink := &callbackSink{failure: nil}
	token := registerTestCapture(environment, t, sink)
	environment.inputValueCallback(token, ioNoDevice, nativeZero)

	if !errors.Is(sink.failure, domain.ErrDisconnected) {
		t.Fatalf("native callback failure lost its cause: %v", sink.failure)
	}
}

func registerTestCapture(environment *nativeState, t *testing.T, sink *callbackSink) uintptr {
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
		checkTickDuration(environment, t, &examples[index])
	}
}

func TestTickDurationRejectsInvalidTimebase(t *testing.T) {
	t.Parallel()

	environment := new(nativeState)

	environment.timebase = machTimebase{Denom: nativeZero, Numer: testValueHandle}

	checkTickDuration(
		environment,
		t,
		&tickExample{testLaterTick, testAnchorTick, nativeZero, false},
	)

	environment.timebase = machTimebase{Numer: math.MaxUint32, Denom: testValueHandle}

	checkTickDuration(environment, t, &tickExample{math.MaxUint64, nativeZero, nativeZero, false})
}

func TestCallbackRegistriesAreIsolated(t *testing.T) {
	t.Parallel()

	first, second := new(nativeState), new(nativeState)
	firstSink, secondSink := &callbackSink{failure: nil}, &callbackSink{failure: nil}
	firstToken := registerTestCapture(first, t, firstSink)
	secondToken := registerTestCapture(second, t, secondSink)

	if firstToken != secondToken {
		t.Fatal("independent registries should start with the same numeric token")
	}

	first.inputValueCallback(firstToken, ioNoDevice, nativeZero)
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

func checkTickDuration(environment *nativeState, t *testing.T, example *tickExample) {
	t.Helper()

	duration, valid := environment.tickDuration(example.ticks, example.anchor)
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
