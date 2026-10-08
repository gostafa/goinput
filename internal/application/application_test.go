// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/gostafa/goinput/internal/adapters/singleton"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	backendview "github.com/gostafa/goinput/internal/ports/backend"
	captureview "github.com/gostafa/goinput/internal/ports/capture"
)

func TestManagerLifetimeCancellationReachesOperation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	manager := newTestManager(ctx, t)

	cancel()

	result, err := withManagerOperation(t.Context(), manager, canceledOperation)
	if result || !errors.Is(err, domain.ErrClosed) {
		t.Fatalf("canceled operation = (%v, %v)", result, err)
	}
}

func TestReadRejectsNilContext(t *testing.T) {
	t.Parallel()

	var ctx context.Context

	event, err := streamRead(ctx, testStream(firstTestValue))

	var empty domain.Event

	if event != empty || !errors.Is(err, domain.ErrInvalidOptions) {
		t.Fatalf("nil read context = %v", err)
	}
}

func newTestManager(ctx context.Context, t *testing.T) *Manager {
	t.Helper()

	manager, err := NewManager(ctx, domain.Options{BufferSize: testEmptyBuffer}, nil)
	if err != nil {
		t.Fatal(err)
	}

	return manager
}

func canceledOperation(ctx context.Context) (bool, error) {
	<-ctx.Done()

	return false, errors.Join(context.Cause(ctx))
}

type (
	testBackendOperations = backendview.Operations[domain.DeviceInfo, domain.DeviceID, ports.EventSink, ports.Capture]

	testBackend struct {
		*testBackendOperations

		closes int
	}
)

const (
	backendClosedMessage  = "backend closed %d times, want once"
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

func newTestBackend() *testBackend {
	backend := &testBackend{
		testBackendOperations: new(testBackendOperations),
		closes:                testEmptyBuffer,
	}

	backend.Operations.Discover = func(context.Context) ([]domain.DeviceInfo, error) { return nil, nil }
	backend.Operations.Open = func(context.Context, domain.DeviceID, ports.EventSink) (ports.Capture, error) {
		return nil, domain.ErrUnsupported
	}

	return backend
}

func TestCoordinatorSharesAndReleasesSession(t *testing.T) {
	t.Parallel()

	backend := newTestBackend()
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
		t.Fatalf(backendClosedMessage, backend.closes)
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

	stream := testStream(firstTestValue)
	condition1 := !streamPublish(stream,
		testEvent(firstTestValue),
	) || streamPublish(stream,
		testEvent(testQueueCapacity),
	)

	if condition1 {
		t.Fatal("overflow did not terminate the stream")
	}

	err := resultError(streamRead(t.Context(), stream))

	if !errors.Is(err, domain.ErrEventLoss) || stream.size != emptyQueueSize {
		t.Fatalf("overflow retained events or lost its cause: %v", err)
	}
}

func TestStreamCancellationPreservesQueue(t *testing.T) {
	t.Parallel()

	stream := testStream(firstTestValue)
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(domain.ErrDisconnected)

	condition2 := !streamPublish(stream,
		testEvent(firstTestValue),
	)

	if condition2 {
		t.Fatal(publishFailureMessage)
	}

	err := resultError(streamRead(ctx, stream))

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

type (
	appFixture = struct {
		manager *Manager
		backend *testBackend
		capture *captureview.Operations[domain.DeviceInfo, domain.Capabilities]
		sink    ports.EventSink
	}
)

func lifecycleFixture(t *testing.T) *appFixture {
	t.Helper()

	fixture := new(appFixture)

	fixture.backend = newTestBackend()
	fixture.capture = testCapture()
	configureFixtureBackend(fixture)

	coordinator := NewCoordinator(func(context.Context, ports.Retrier) (ports.Backend, error) {
		return fixture.backend, nil
	}, nil)

	fixture.manager = managerWithCoordinator(t, coordinator)

	return fixture
}

func managerWithCoordinator(t *testing.T, coordinator *Coordinator) *Manager {
	t.Helper()

	provider := singleton.New(
		func(context.Context) (*Coordinator, error) { return coordinator, nil },
	)
	manager, err := NewManager(t.Context(), domain.Options{BufferSize: testQueueCapacity}, provider)
	appCause(t, err, nil)
	t.Cleanup(func() { appCause(t, managerClose(manager), nil) })

	return manager
}

func configureFixtureBackend(fixture *appFixture) {
	fixture.backend.Operations.Open = func(
		_ context.Context, _ domain.DeviceID, sink ports.EventSink,
	) (ports.Capture, error) {
		fixture.sink = sink

		return fixture.capture, nil
	}
	fixture.backend.Operations.Discover = func(context.Context) ([]domain.DeviceInfo, error) {
		return []domain.DeviceInfo{fixture.capture.Info()}, nil
	}
}

func testCapture() *captureview.Operations[domain.DeviceInfo, domain.Capabilities] {
	capture := new(captureview.Operations[domain.DeviceInfo, domain.Capabilities])
	configureCaptureDescriptors(capture)

	capture.Operations.Extension = func(any) bool { return true }
	capture.Operations.Close = func() error { return nil }

	return capture
}

func configureCaptureDescriptors(
	capture *captureview.Operations[domain.DeviceInfo, domain.Capabilities],
) {
	capture.Operations.Info = func() domain.DeviceInfo {
		var info domain.DeviceInfo

		info.ID = testDeviceID

		return info
	}
	capture.Operations.Capabilities = func() domain.Capabilities {
		return domain.Capabilities{Controls: nil, Repeat: domain.SupportUnsupported, Complete: true}
	}
}

func appCause(t *testing.T, actual, expected error) {
	t.Helper()

	if !errors.Is(actual, expected) {
		t.Fatalf("error = %v, want cause %v", actual, expected)
	}
}

func appEqual[Value comparable](t *testing.T, actual, expected Value) {
	t.Helper()

	if actual != expected {
		t.Fatalf("value = %v, want %v", actual, expected)
	}
}

func openedTestDevice(
	t *testing.T,
	fixture *appFixture,
) *deviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event] {
	t.Helper()

	device, err := ManagerView(fixture.manager).Open(t.Context(), testDeviceID)
	appCause(t, err, nil)

	concrete, ok := device.(*deviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event])
	appEqual(t, ok, true)

	return concrete
}

func TestManagerViewCaptureLifecycle(t *testing.T) {
	t.Parallel()

	fixture := lifecycleFixture(t)
	view := ManagerView(fixture.manager)
	checkFixtureDiscovery(t, fixture)

	device := openedTestDevice(t, fixture)
	checkDeviceSnapshots(t, device)
	checkDeviceDelivery(t, fixture, device)
	appCause(t, view.Close(), nil)
	appEqual(t, fixture.backend.closes, unitStep)
}

func checkFixtureDiscovery(t *testing.T, fixture *appFixture) {
	t.Helper()

	infos, err := ManagerView(fixture.manager).Devices(t.Context())
	appCause(t, err, nil)
	appEqual(t, len(infos), unitStep)
}

func checkDeviceSnapshots(t *testing.T, device Device) {
	t.Helper()
	appEqual(t, device.Info().ID, domain.DeviceID(testDeviceID))
	appEqual(t, device.Capabilities().Complete, true)

	extension, ok := device.(ExtensionProvider)
	appEqual(t, ok, true)
	appEqual(t, extension.Extension(new(bool)), true)
}

func checkDeviceDelivery(t *testing.T, fixture *appFixture, device Device) {
	t.Helper()
	appEqual(t, fixture.sink.Publish(testEvent(firstTestValue)), true)

	event, err := device.Read(t.Context())
	appCause(t, err, nil)
	appEqual(t, event.Value, float64(firstTestValue))
	appCause(t, device.Close(), nil)

	extension, ok := device.(ExtensionProvider)
	appEqual(t, ok, true)
	appEqual(t, extension.Extension(new(bool)), false)
	appEqual(t, fixture.sink.Publish(testEvent(firstTestValue)), false)
}

func TestFailedOpenReturnsNilDevice(t *testing.T) {
	t.Parallel()

	fixture := lifecycleFixture(t)

	fixture.backend.Operations.Open = func(context.Context, domain.DeviceID, ports.EventSink) (ports.Capture, error) {
		return nil, domain.ErrNotFound
	}

	device, err := ManagerView(fixture.manager).Open(t.Context(), testDeviceID)
	appCause(t, err, domain.ErrNotFound)
	appEqual(t, device == nil, true)
	appCause(t, resultError(managerOpen(t.Context(), fixture.manager, "")), domain.ErrNotFound)
}

func TestFailedCaptureIsReleased(t *testing.T) {
	t.Parallel()

	fixture := lifecycleFixture(t)
	closes := testEmptyBuffer

	fixture.capture.Operations.Close = func() error {
		closes++

		return domain.ErrDisconnected
	}
	fixture.backend.Operations.Open = captureOpenResult[ports.Capture](
		fixture.capture,
		domain.ErrNotFound,
	)
	appCause(
		t,
		resultError(managerOpen(t.Context(), fixture.manager, testDeviceID)),
		domain.ErrDisconnected,
	)
	appEqual(t, closes, unitStep)
}

func TestMissingCaptureIsUnsupported(t *testing.T) {
	t.Parallel()

	fixture := lifecycleFixture(t)

	fixture.backend.Operations.Open = captureOpenResult[ports.Capture](nil, nil)
	appCause(
		t,
		resultError(managerOpen(t.Context(), fixture.manager, testDeviceID)),
		domain.ErrUnsupported,
	)
}

func captureOpenResult[Capture ports.Capture](
	capture Capture,
	err error,
) func(context.Context, domain.DeviceID, ports.EventSink) (ports.Capture, error) {
	return func(context.Context, domain.DeviceID, ports.EventSink) (ports.Capture, error) {
		return capture, err
	}
}

func TestDiscoveryKeepsPartialResults(t *testing.T) {
	t.Parallel()

	fixture := lifecycleFixture(t)

	fixture.backend.Operations.Discover = func(context.Context) ([]domain.DeviceInfo, error) {
		return []domain.DeviceInfo{fixture.capture.Info()}, domain.ErrDisconnected
	}

	infos, err := managerDevices(t.Context(), fixture.manager)
	appCause(t, err, domain.ErrDisconnected)
	appEqual(t, len(infos), unitStep)
}

func TestManagerOperationsRejectClosedAndNilContext(t *testing.T) {
	t.Parallel()

	fixture := lifecycleFixture(t)

	var ctx context.Context

	appCause(t, resultError(managerDevices(ctx, fixture.manager)), domain.ErrInvalidOptions)
	appCause(t, managerClose(fixture.manager), nil)
	appCause(t, resultError(managerDevices(t.Context(), fixture.manager)), domain.ErrClosed)
	appCause(t, managerAttachStream(fixture.manager, testStream(unitStep)), domain.ErrClosed)
}

func TestDiscoveryPropagatesProviderFailure(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t.Context(), t)

	manager.provider = singleton.New(func(context.Context) (*Coordinator, error) {
		return nil, domain.ErrUnsupported
	})
	appCause(t, resultError(managerDevices(t.Context(), manager)), domain.ErrUnsupported)
	appCause(t, resultError(managerOpen(t.Context(), manager, testDeviceID)), domain.ErrUnsupported)
	appCause(t, managerClose(manager), nil)
}

func TestStaleLeaseCannotDeliverCallbacks(t *testing.T) {
	t.Parallel()

	fixture := lifecycleFixture(t)
	device := openedTestDevice(t, fixture)
	appCause(t, managerClose(fixture.manager), nil)
	appEqual(t, fixture.sink.Publish(testEvent(firstTestValue)), false)
	fixture.sink.Fail(domain.ErrDisconnected)
	appCause(t, resultError(device.Read(t.Context())), domain.ErrClosed)
	appCause(t, leaseRelease(fixture.manager.lease), nil)
}

func TestSinkFailureTerminatesCapture(t *testing.T) {
	t.Parallel()

	fixture := lifecycleFixture(t)
	device := openedTestDevice(t, fixture)
	fixture.sink.Fail(nil)
	appCause(t, resultError(device.Read(t.Context())), domain.ErrDisconnected)
	appCause(t, device.Close(), nil)
}

func TestStreamCleanupFailureRetainsTerminalCause(t *testing.T) {
	t.Parallel()

	fixture := lifecycleFixture(t)

	fixture.capture.Operations.Close = func() error { return domain.ErrDisconnected }

	stream := managerNewStream(fixture.manager, testDeviceID)
	streamAdoptCapture(stream, fixture.capture)
	streamFail(stream, domain.ErrEventLoss)
	streamCloseOnFailure(stream)
	appCause(t, stream.closeErr, domain.ErrDisconnected)
	appCause(t, resultError(streamRead(t.Context(), stream)), domain.ErrEventLoss)
}

func TestAttachFailureClosesCapture(t *testing.T) {
	t.Parallel()

	fixture := lifecycleFixture(t)
	stream := managerNewStream(fixture.manager, testDeviceID)
	streamAdoptCapture(stream, fixture.capture)
	appCause(t, managerClose(fixture.manager), nil)
	appCause(t, resultError(managerRegisterStream(fixture.manager, stream)), domain.ErrClosed)
}

func TestCanceledSessionCreationReleasesBackend(t *testing.T) {
	t.Parallel()

	backend := new(testBackend)
	coordinator := NewCoordinator(func(context.Context, ports.Retrier) (ports.Backend, error) {
		return backend, nil
	}, nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result, err := createSession(ctx, coordinator.factory, coordinator.retrier)
	checkCanceledSession(t, result, err)

	if backend.closes != unitStep {
		t.Fatalf(backendClosedMessage, backend.closes)
	}
}

func checkCanceledSession(t *testing.T, result ports.Backend, err error) {
	t.Helper()

	if result != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled session = (%v, %v), want (nil, context.Canceled)", result, err)
	}
}

func TestSessionFactoryFailureReturnsNoBackend(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator(func(context.Context, ports.Retrier) (ports.Backend, error) {
		return nil, domain.ErrUnsupported
	}, nil)
	result, err := createSession(t.Context(), coordinator.factory, coordinator.retrier)

	if result != nil || !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("missing session = (%v, %v), want (nil, ErrUnsupported)", result, err)
	}
}

func TestSessionStartupFailureLeavesCoordinatorIdle(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator(func(context.Context, ports.Retrier) (ports.Backend, error) {
		return nil, domain.ErrUnsupported
	}, nil)
	manager := managerWithCoordinator(t, coordinator)
	appCause(t, resultError(managerDevices(t.Context(), manager)), domain.ErrUnsupported)
	appEqual(t, coordinator.state, sessionIdle)
}

func TestMissingSessionBackendIsUnsupported(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator(emptyBackendResult[ports.Backend](nil), nil)
	appCause(t, resultError(coordinatorAcquire(t.Context(), coordinator)), domain.ErrUnsupported)
}

func emptyBackendResult[Backend ports.Backend](backend Backend) ports.Factory {
	return func(context.Context, ports.Retrier) (ports.Backend, error) {
		return backend, nil
	}
}

func TestCanceledAcquisitionDoesNotStartSession(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	coordinator := NewCoordinator(nil, nil)
	appCause(t, resultError(coordinatorAcquire(ctx, coordinator)), context.Canceled)
	appEqual(t, coordinator.state, sessionIdle)
}

func TestInvalidSessionStateUnlocksCoordinator(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator(nil, nil)

	coordinator.state = sessionState(testQueueCapacity + testQueueCapacity)
	appCause(t, resultError(coordinatorAcquire(t.Context(), coordinator)), domain.ErrUnsupported)
	appEqual(t, coordinator.mu.TryLock(), true)
	coordinator.mu.Unlock()
}

func TestSessionTransitionWaitsForSignal(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator(nil, nil)

	coordinator.state = sessionStarting
	close(coordinator.changed)

	lease, ready, err := coordinatorAcquireAttempt(t.Context(), coordinator)
	appCause(t, err, nil)
	appEqual(t, ready, false)
	appEqual(t, lease == nil, true)
	appEqual(t, coordinatorDropLease(coordinator, testEmptyBuffer), false)
}

func TestWaitForSessionChangeHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	appCause(t, waitForChange(ctx, make(chan struct{})), context.Canceled)
}

func TestLeaseGateHonorsCancellation(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t.Context(), t)
	manager.leaseGate <- struct{}{}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	appCause(t, resultError(managerGetLease(ctx, manager)), context.Canceled)
	appCause(t, resultError(managerCurrentOrNewLease(ctx, manager)), context.Canceled)
	appCause(t, managerClose(manager), nil)
}

func TestCloseDuringCaptureOpenRejectsRegistration(t *testing.T) {
	t.Parallel()

	fixture := lifecycleFixture(t)

	fixture.backend.Operations.Open = closingCaptureOpen(fixture)
	appCause(
		t,
		resultError(managerOpen(t.Context(), fixture.manager, testDeviceID)),
		domain.ErrClosed,
	)
}

func closingCaptureOpen(
	fixture *appFixture,
) func(context.Context, domain.DeviceID, ports.EventSink) (ports.Capture, error) {
	return func(context.Context, domain.DeviceID, ports.EventSink) (ports.Capture, error) {
		fixture.manager.mu.Lock()

		fixture.manager.closed = true
		fixture.manager.mu.Unlock()

		return fixture.capture, nil
	}
}

func TestEmptyStreamReadWaitsForCancellation(t *testing.T) {
	t.Parallel()

	stream := testStream(testQueueCapacity)
	stream.notify <- struct{}{}

	event, available, err := streamReadAttempt(t.Context(), stream)
	appCause(t, err, nil)
	appEqual(t, available, false)
	appEqual(t, event.Value, float64(testEmptyBuffer))
	checkCanceledStreamWait(t, stream)
}

func checkCanceledStreamWait(t *testing.T, stream *stream) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	appCause(t, streamWaitEvent(ctx, stream), context.Canceled)
	streamFail(stream, domain.ErrClosed)
	appCause(t, streamWaitEvent(t.Context(), stream), nil)
}

func TestQueuedReadPreservesOrderAndWakesNextReader(t *testing.T) {
	t.Parallel()

	stream := testStream(testQueueCapacity)
	appEqual(t, streamPublish(stream, testEvent(firstTestValue)), true)
	appEqual(t, streamPublish(stream, testEvent(thirdTestValue)), true)

	event, err := streamRead(t.Context(), stream)
	appCause(t, err, nil)
	appEqual(t, event.Value, float64(firstTestValue))

	event, err = streamRead(t.Context(), stream)
	appCause(t, err, nil)
	appEqual(t, event.Value, float64(thirdTestValue))
}
