// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
)

type (
	testBackend struct {
		closes int
	}
)

const (
	publishFailureMessage = "publish failed"
	testDeviceID          = "test-device"
	testQueueCapacity     = 2
	firstTestValue        = 1
	thirdTestValue        = 3
	fourthTestValue       = 4
	testEmptyBuffer       = 0
)

func (backend *testBackend) Close() error {
	backend.closes++

	return nil
}

func (*testBackend) Discover(context.Context) ([]domain.DeviceInfo, error) {
	return nil, nil
}

func (*testBackend) Open(context.Context, domain.DeviceID, ports.EventSink) (ports.Capture, error) {
	return nil, domain.ErrUnsupported
}

func TestCoordinatorSharesAndReleasesSession(t *testing.T) {
	t.Parallel()

	backend := &testBackend{closes: testEmptyBuffer}
	coordinator := NewCoordinator(func(context.Context, ports.Retrier) (ports.Backend, error) {
		return backend, nil
	}, nil)
	first := acquireTestLease(t, coordinator)
	second := acquireTestLease(t, coordinator)
	releaseTestLease(t, first)

	if backend.closes != emptyQueueSize {
		t.Fatal("session closed while a lease was still active")
	}

	checkFinalRelease(t, second, backend)
}

func acquireTestLease(t *testing.T, coordinator *Coordinator) *lease {
	t.Helper()

	lease, err := coordinatorAcquire(t.Context(), coordinator)
	if err != nil {
		t.Fatal(err)
	}

	return lease
}

func releaseTestLease(t *testing.T, lease *lease) {
	t.Helper()

	err := leaseRelease(lease)
	if err != nil {
		t.Fatal(err)
	}
}

func checkFinalRelease(t *testing.T, lease *lease, backend *testBackend) {
	t.Helper()
	releaseTestLease(t, lease)
	releaseTestLease(t, lease)

	if backend.closes != unitStep {
		t.Fatalf("backend closed %d times, want once", backend.closes)
	}
}

func TestStreamQueueWraparound(t *testing.T) {
	t.Parallel()

	stream := testStream(testQueueCapacity)

	expectedValues := []float64{firstTestValue, testQueueCapacity, thirdTestValue, fourthTestValue}
	for index := range expectedValues {
		checkRoundTrip(t, stream, expectedValues[index])
	}
}

func TestStreamOverflowDiscardsQueue(t *testing.T) {
	t.Parallel()

	stream := testStream(1)
	if !streamPublish(stream,
		testEvent(1),
	) || streamPublish(stream,
		testEvent(2),
	) {
		t.Fatal("overflow did not terminate the stream")
	}

	var err error = resultError(streamRead(t.Context(), stream))
	if !errors.Is(err, domain.ErrEventLoss) || stream.size != emptyQueueSize {
		t.Fatalf("overflow retained events or lost its cause: %v", err)
	}
}

func TestStreamCancellationPreservesQueue(t *testing.T) {
	t.Parallel()

	stream := testStream(1)
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(domain.ErrDisconnected)

	if !streamPublish(stream,
		testEvent(1),
	) {
		t.Fatal(publishFailureMessage)
	}

	var err error = resultError(streamRead(ctx, stream))
	if !errors.Is(err, domain.ErrDisconnected) || stream.size != unitStep {
		t.Fatalf("canceled read consumed an event or lost its cause: %v", err)
	}
}

func checkRoundTrip(t *testing.T, stream *stream, expected float64) {
	t.Helper()

	input := testEvent(expected)
	if !streamPublish(stream, input) {
		t.Fatal(publishFailureMessage)
	}

	checkUnchangedInput(t, input)

	input.Value = expected + unitStep

	event, err := streamRead(t.Context(), stream)
	if err != nil || event.Value != expected || event.DeviceID != stream.id {
		t.Fatalf("Read = (%+v, %v), want value %v", event, err, expected)
	}
}

func checkUnchangedInput(t *testing.T, input *domain.Event) {
	t.Helper()

	if input.DeviceID != "" || !input.Timestamp.ReceivedAt.IsZero() {
		t.Fatal("Publish mutated the caller's event")
	}
}

func testStream(capacity int) *stream {
	manager := new(Manager)

	manager.options.BufferSize = capacity

	return managerNewStream(manager, testDeviceID)
}

func testEvent(value float64) *domain.Event {
	event := emptyStreamEvent()

	event.Value = value

	return &event
}
