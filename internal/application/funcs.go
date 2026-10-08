// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
)

func streamCloseOnFailure(stream *stream) {
	<-stream.done

	err := streamClose(stream)
	if err != nil {
		streamFail(stream, err)
	}
}

// NewCoordinator creates shared state for lazily acquired native sessions.
func NewCoordinator(factory ports.Factory, retrier ports.Retrier) *Coordinator {
	return &Coordinator{
		backend:    nil,
		refs:       emptyQueueSize,
		generation: emptyQueueSize,
		mu:         sync.Mutex{},
		state:      sessionIdle,
		factory:    factory,
		retrier:    retrier,
		changed:    make(chan struct{}),
	}
}

func coordinatorSignal(coordinator *Coordinator) {
	close(coordinator.changed)

	coordinator.changed = make(chan struct{})
}

func coordinatorAcquire(ctx context.Context, coordinator *Coordinator) (*lease, error) {
	for {
		lease, ready, err := coordinatorAcquireAttempt(ctx, coordinator)
		if err != nil {
			return nil, errors.Join(err)
		}

		if ready {
			return lease, nil
		}
	}
}

func leaseRelease(lease *lease) error {
	lease.once.Do(func() { releaseSessionLease(lease) })

	return lease.err
}

func leasedSinkPublish(sink leasedSink, event *domain.Event) bool {
	c := sink.lease.coordinator
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != sessionRunning || c.generation != sink.lease.generation {
		return false
	}

	return sink.sink.Publish(event)
}

func leasedSinkFail(sink leasedSink, err error) {
	c := sink.lease.coordinator
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state == sessionRunning && c.generation == sink.lease.generation {
		sink.sink.Fail(err)
	}
}

// NewManager creates a manager with independent captures and cancellation.
func NewManager(
	ctx context.Context,
	options domain.Options,
	provider ports.Provider[*Coordinator],
) (*Manager, error) {
	if ctx == nil || options.BufferSize < emptyQueueSize {
		return nil, domain.ErrInvalidOptions
	}

	if options.BufferSize == emptyQueueSize {
		options.BufferSize = domain.DefaultBufferSize
	}

	manager := new(Manager)

	manager.options, manager.provider = options, provider
	initializeManagerResources(ctx, manager)

	return manager, nil
}

func withManagerOperation[Value any](
	ctx context.Context,
	manager *Manager,
	operation func(context.Context) (Value, error),
) (result Value, failure error) {
	err := beginManagerOperation(ctx, manager)
	if err != nil {
		return result, fmt.Errorf("begin: %w", err)
	}

	opctx, cancel := context.WithCancelCause(ctx)
	stop := manager.onClose(cancel)

	defer finishManagerOperation(manager, stop, cancel)

	result, failure = operation(opctx)

	return result, errors.Join(failure)
}

func finishManagerOperation(manager *Manager, stop func() bool, cancel context.CancelCauseFunc) {
	stop()
	cancel(nil)
	manager.ops.Done()
}

func managerGetLease(ctx context.Context, manager *Manager) (*lease, error) {
	err := managerEnterLeaseGate(ctx, manager)
	if err != nil {
		return nil, fmt.Errorf(leaseErrorFormat, err)
	}

	defer func() { <-manager.leaseGate }()

	result, err := managerCurrentOrNewLease(ctx, manager)
	if err != nil {
		return nil, errors.Join(err)
	}

	return result, nil
}

func managerCurrentOrNewLease(ctx context.Context, manager *Manager) (*lease, error) {
	err := context.Cause(ctx)
	if err != nil {
		return nil, fmt.Errorf(leaseErrorFormat, err)
	}

	if manager.lease != nil {
		return manager.lease, nil
	}

	result0, callErr := managerAcquireLease(ctx, manager)
	if callErr != nil {
		return nil, errors.Join(callErr)
	}

	return result0, nil
}

// Devices returns accessible endpoints. It may return partial results alongside
// an error containing discovery/permission diagnostics. Context bounds discovery,
// but synchronous native calls may take longer than its deadline to return.
func managerDevices(ctx context.Context, manager *Manager) ([]domain.DeviceInfo, error) {
	result, err := withManagerOperation(
		ctx,
		manager,
		func(ctx context.Context) ([]domain.DeviceInfo, error) {
			budget, cancel := context.WithTimeout(ctx, discoveryBudget)
			defer cancel()

			return managerDiscover(budget, manager)
		},
	)

	return result, errors.Join(err)
}

// Open starts an independent event stream. Opening the same endpoint twice
// creates two subscriptions, each with its own buffer and lifecycle.
func managerOpen(
	ctx context.Context,
	manager *Manager,
	id domain.DeviceID,
) (*deviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event], error) {
	result, err := withManagerOperation(
		ctx,
		manager,
		func(ctx context.Context) (*deviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event], error) {
			return managerOpenDevice(ctx, manager, id)
		},
	)
	if err != nil {
		return nil, errors.Join(err)
	}

	return result, nil
}

func managerOpenDevice(
	ctx context.Context,
	manager *Manager,
	id domain.DeviceID,
) (*deviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event], error) {
	if id == "" {
		return nil, &domain.OpError{DeviceID: id, Op: operationOpen, Err: domain.ErrNotFound}
	}

	result, failure := managerOpenStream(ctx, manager, id)
	if failure != nil {
		return nil, errors.Join(failure)
	}

	return result, nil
}

// Close cancels pending manager operations, closes all captures, and releases the
// shared session lease. Native resources stop after the last manager releases.
func managerClose(manager *Manager) error {
	manager.closeOnce.Do(func(receiver *Manager) func() {
		return func() {
			managerCloseResources(receiver)
		}
	}(manager))
	<-manager.closeDone

	return manager.closeErr
}

func streamInfo(stream *stream) domain.DeviceInfo { return domain.CloneInfo(&stream.info) }

func streamCapabilities(stream *stream) domain.Capabilities {
	return domain.CloneCapabilities(&stream.caps)
}

func streamExtension(stream *stream, target any) bool {
	stream.mu.Lock()

	closed := stream.terminal != nil
	stream.mu.Unlock()

	if closed {
		return false
	}

	return stream.capture.Extension(target)
}

func streamPublish(stream *stream, event *domain.Event) bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	if !streamCanPublish(stream) {
		return false
	}

	prepared := streamPrepareEvent(stream, event)

	stream.queue[(stream.head+stream.size)%len(stream.queue)] = prepared
	stream.size++
	streamSignalEvent(stream)

	return true
}

func streamFailLocked(stream *stream, err error) {
	if stream.terminal != nil {
		return
	}

	if err == nil {
		err = domain.ErrDisconnected
	}

	stream.terminal = &domain.OpError{Op: "read", DeviceID: stream.id, Err: err}
	clear(stream.queue)

	stream.size = emptyQueueSize
	close(stream.done)
}

func streamFail(stream *stream, err error) {
	stream.mu.Lock()
	streamFailLocked(stream, err)
	stream.mu.Unlock()
}

func streamRead(ctx context.Context, stream *stream) (domain.Event, error) {
	if ctx == nil {
		return domain.Event{}, domain.ErrInvalidOptions
	}

	result0, callErr := streamReadLoop(ctx, stream)
	if callErr != nil {
		return domain.Event{}, errors.Join(callErr)
	}

	return result0, nil
}

func streamClose(stream *stream) error {
	streamFail(stream, domain.ErrClosed)
	stream.closeOnce.Do(func() {
		stream.closeErr = stream.capture.Close()
		stream.release()
		close(stream.closeDone)
	})
	<-stream.closeDone

	return stream.closeErr
}

func coordinatorAcquireState(
	ctx context.Context,
	coordinator *Coordinator,
) (acquisition, error) {
	coordinator.mu.Lock()

	result, err := acquireLockedState(ctx, coordinator)

	return result, errors.Join(err)
}

func acquireLockedState(ctx context.Context, coordinator *Coordinator) (acquisition, error) {
	switch coordinator.state {
	case sessionRunning:
		return acquireRunningResult(coordinator), nil
	case sessionIdle:
		wrappedValue0, wrappedErr := coordinatorAcquireIdle(ctx, coordinator)

		return wrappedValue0, errors.Join(wrappedErr)

	case sessionStarting, sessionStopping:
		return waitingAcquisition(coordinator), nil
	default:
		return acquisition{}, errors.Join(invalidCoordinatorState(coordinator))
	}
}

func waitForChange(ctx context.Context, changed <-chan struct{}) error {
	select {
	case <-ctx.Done():
		return errors.Join(context.Cause(ctx))
	case <-changed:
		return nil
	}
}

func coordinatorCurrentLease(coordinator *Coordinator) *lease {
	return &lease{
		err:         nil,
		once:        sync.Once{},
		coordinator: coordinator,
		backend:     coordinator.backend,
		generation:  coordinator.generation,
	}
}

func coordinatorStartSession(ctx context.Context, coordinator *Coordinator) (*lease, error) {
	backend, err := createSession(ctx, coordinator.factory, coordinator.retrier)
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	defer coordinatorSignal(coordinator)

	if err != nil {
		coordinator.state = sessionIdle

		return nil, errors.Join(err)
	}

	coordinator.backend, coordinator.refs, coordinator.state = backend, unitStep, sessionRunning
	coordinator.generation++

	return coordinatorCurrentLease(coordinator), nil
}

func createSession[Backend ports.Backend](
	ctx context.Context,
	factory func(context.Context, ports.Retrier) (Backend, error),
	retrier ports.Retrier,
) (result Backend, failure error) {
	backend, err := factory(ctx, retrier)

	err = errors.Join(err, context.Cause(ctx))

	if any(backend) == nil && err == nil {
		err = domain.ErrUnsupported
	}

	err = closeFailedBackend(backend, err)
	if err != nil {
		return result, errors.Join(err)
	}

	return backend, nil
}

func managerStartOperation(manager *Manager) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if manager.closed {
		return domain.ErrClosed
	}

	manager.ops.Add(unitStep)

	return nil
}

func managerEnterLeaseGate(ctx context.Context, manager *Manager) error {
	select {
	case manager.leaseGate <- struct{}{}:
		return nil
	case <-ctx.Done():
		return errors.Join(context.Cause(ctx))
	}
}

func managerAcquireLease(ctx context.Context, manager *Manager) (*lease, error) {
	coordinator, err := manager.provider.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquireLease: %w", err)
	}

	lease, err := coordinatorAcquire(ctx, coordinator)
	if err == nil {
		manager.lease = lease
	}

	if err != nil {
		return nil, errors.Join(err)
	}

	return lease, nil
}

func managerDiscover(ctx context.Context, manager *Manager) ([]domain.DeviceInfo, error) {
	lease, err := managerGetLease(ctx, manager)
	if err != nil {
		return nil, &domain.OpError{DeviceID: "", Op: operationDevices, Err: err}
	}

	infos, err := lease.backend.Discover(ctx)

	err = errors.Join(err, context.Cause(ctx))

	for index := range infos {
		infos[index] = domain.CloneInfo(&infos[index])
	}

	if err != nil {
		err = &domain.OpError{DeviceID: "", Op: operationDevices, Err: err}
	}

	return infos, err
}

func managerOpenStream(
	ctx context.Context,
	manager *Manager,
	id domain.DeviceID,
) (*deviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event], error) {
	stream, err := openManagerCapture(ctx, manager, id)
	if err != nil {
		return nil, errors.Join(err)
	}

	result, failure := managerRegisterStream(manager, stream)
	if failure != nil {
		return nil, errors.Join(failure)
	}

	return result, nil
}

func openManagerCapture(
	ctx context.Context,
	manager *Manager,
	id domain.DeviceID,
) (*stream, error) {
	lease, err := managerGetLease(ctx, manager)
	if err != nil {
		return nil, &domain.OpError{Op: operationOpen, DeviceID: id, Err: err}
	}

	stream := managerNewStream(manager, id)

	err = streamOpenCapture(ctx, stream, lease)
	if err != nil {
		return nil, &domain.OpError{Op: operationOpen, DeviceID: id, Err: err}
	}

	return stream, nil
}

func managerNewStream(manager *Manager, id domain.DeviceID) *stream {
	stream := new(stream)

	stream.id = id
	stream.info.Transport = domain.TransportUnknown
	stream.caps.Repeat = domain.SupportUnknown
	stream.queue = makeEventQueue(manager.options.BufferSize)
	stream.notify = make(chan struct{}, unitStep)
	stream.done, stream.closeDone = make(chan struct{}), make(chan struct{})
	stream.release = func() { managerReleaseStream(manager, stream) }

	return stream
}

func streamOpenCapture(ctx context.Context, stream *stream, lease *lease) error {
	capture, err := lease.backend.Open(
		ctx,
		stream.id,
		leasedSinkView(leasedSink{lease: lease, sink: streamSinkView(stream)}),
	)
	if err == nil && capture == nil {
		err = domain.ErrUnsupported
	}

	err = errors.Join(err, context.Cause(ctx))
	if err != nil {
		return errors.Join(streamAbortCapture(stream, capture, err))
	}

	streamAdoptCapture(stream, capture)

	return nil
}

func streamAbortCapture(stream *stream, capture ports.Capture, err error) error {
	streamFail(stream, err)

	if capture != nil {
		return errors.Join(err, capture.Close())
	}

	return err
}

func managerRegisterStream(
	manager *Manager,
	stream *stream,
) (*deviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event], error) {
	err := managerAttachStream(manager, stream)
	if err != nil {
		streamFail(stream, err)

		return nil, errors.Join(err, streamClose(stream))
	}

	go streamCloseOnFailure(stream)

	return streamView(stream), nil
}

func managerCloseResources(manager *Manager) {
	manager.mu.Lock()

	manager.closed = true
	manager.cancel(domain.ErrClosed)
	manager.mu.Unlock()
	manager.ops.Wait()
	managerCloseStreams(manager)

	if manager.lease != nil {
		manager.closeErr = errors.Join(manager.closeErr, leaseRelease(manager.lease))
	}

	close(manager.closeDone)
}

func managerCloseStreams(manager *Manager) {
	manager.mu.Lock()

	devices := make([]*stream, emptyQueueSize, len(manager.devices))
	for device := range manager.devices {
		devices = append(devices, device)
	}

	manager.mu.Unlock()

	for index := range devices {
		manager.closeErr = errors.Join(manager.closeErr, streamClose(devices[index]))
	}
}

func streamCanPublish(stream *stream) bool {
	if stream.terminal != nil {
		return false
	}

	if stream.size == len(stream.queue) {
		streamFailLocked(stream, domain.ErrEventLoss)

		return false
	}

	return true
}

func streamPrepareEvent(stream *stream, source *domain.Event) domain.Event {
	event := *source
	if event.DeviceID == "" {
		event.DeviceID = stream.id
	}

	if event.Timestamp.ReceivedAt.IsZero() {
		event.Timestamp.ReceivedAt = time.Now()
	}

	if event.Timestamp.Time.IsZero() {
		event.Timestamp.Time = event.Timestamp.ReceivedAt.UTC()
		event.Timestamp.Source = domain.TimestampReceipt
	}

	return event
}

func streamSignalEvent(stream *stream) {
	select {
	case stream.notify <- struct{}{}:
	default:
	}
}

func streamReadAvailable(
	ctx context.Context,
	stream *stream,
) (result domain.Event, available bool, failure error) {
	err := context.Cause(ctx)
	if err != nil {
		return domain.Event{}, false, fmt.Errorf("readAvailable: %w", err)
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()

	if stream.terminal != nil {
		return domain.Event{}, false, stream.terminal
	}

	if stream.size != emptyQueueSize {
		return streamDequeue(stream), true, nil
	}

	return emptyStreamEvent(), false, nil
}

func streamDequeue(stream *stream) domain.Event {
	event := stream.queue[stream.head]

	stream.queue[stream.head] = emptyStreamEvent()
	stream.head = (stream.head + unitStep) % len(stream.queue)
	stream.size--

	if stream.size > emptyQueueSize {
		streamSignalEvent(stream)
	}

	return event
}

func streamWaitEvent(ctx context.Context, stream *stream) error {
	select {
	case <-ctx.Done():
		return errors.Join(context.Cause(ctx))
	case <-stream.done:
	case <-stream.notify:
	}

	return nil
}

func coordinatorAcquireAttempt(
	ctx context.Context,
	coordinator *Coordinator,
) (result *lease, ready bool, failure error) {
	err := context.Cause(ctx)
	if err != nil {
		return nil, false, errors.Join(err)
	}

	state, err := coordinatorAcquireState(ctx, coordinator)
	if err != nil {
		return nil, true, errors.Join(err)
	}

	if state.lease != nil {
		return state.lease, true, nil
	}

	return nil, false, errors.Join(waitForChange(ctx, state.changed))
}

func coordinatorAcquireIdle(
	ctx context.Context,
	coordinator *Coordinator,
) (acquisition, error) {
	coordinator.state = sessionStarting
	coordinatorSignal(coordinator)
	coordinator.mu.Unlock()

	lease, err := coordinatorStartSession(ctx, coordinator)
	if err != nil {
		return acquisition{}, errors.Join(err)
	}

	return acquisition{lease: lease, changed: nil}, nil
}

func streamReadLoop(ctx context.Context, stream *stream) (domain.Event, error) {
	for {
		event, ready, err := streamReadAttempt(ctx, stream)
		if ready || err != nil {
			return event, errors.Join(err)
		}
	}
}

func managerAttachStream(manager *Manager, stream *stream) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if manager.closed {
		return domain.ErrClosed
	}

	manager.devices[stream] = struct{}{}

	return nil
}

func streamReadAttempt(
	ctx context.Context,
	stream *stream,
) (result domain.Event, available bool, failure error) {
	event, ready, err := streamReadAvailable(ctx, stream)
	if ready || err != nil {
		return event, ready, errors.Join(err)
	}

	return domain.Event{}, false, errors.Join(streamWaitEvent(ctx, stream))
}

func streamStoreSnapshots(stream *stream, info *domain.DeviceInfo) {
	stream.info = domain.CloneInfo(info)

	caps := stream.capture.Capabilities()

	stream.caps = domain.CloneCapabilities(&caps)
}

func closeFailedBackend(backend ports.Backend, err error) error {
	if err != nil && backend != nil {
		return errors.Join(err, backend.Close())
	}

	return err
}

func releaseSessionLease(lease *lease) {
	coordinator := lease.coordinator
	if !coordinatorDropLease(coordinator, lease.generation) {
		return
	}

	lease.err = lease.backend.Close()

	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()

	coordinator.backend, coordinator.state = nil, sessionIdle
	coordinatorSignal(coordinator)
}

func coordinatorDropLease(coordinator *Coordinator, generation uint64) bool {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()

	if coordinator.state != sessionRunning || coordinator.generation != generation {
		return false
	}

	coordinator.refs--
	if coordinator.refs != emptyQueueSize {
		return false
	}

	coordinator.state = sessionStopping
	coordinatorSignal(coordinator)

	return true
}

func initializeManagerResources(ctx context.Context, manager *Manager) {
	lifetime, cancel := context.WithCancelCause(ctx)

	manager.onClose = func(cancel context.CancelCauseFunc) func() bool {
		return context.AfterFunc(lifetime, func() { cancel(domain.ErrClosed) })
	}
	manager.cancel = cancel
	manager.devices = make(map[*stream]struct{})
	manager.leaseGate = make(chan struct{}, unitStep)
	manager.closeDone = make(chan struct{})
}

func managerReleaseStream(manager *Manager, stream *stream) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	delete(manager.devices, stream)
}

func emptyStreamEvent() domain.Event {
	event := new(domain.Event)

	event.Action = domain.ActionUnknown
	event.Timestamp.Source = domain.TimestampUnknown

	return *event
}

func acquireRunningResult(coordinator *Coordinator) acquisition {
	defer coordinator.mu.Unlock()

	coordinator.refs++

	return acquisition{lease: coordinatorCurrentLease(coordinator), changed: nil}
}

func streamAdoptCapture(stream *stream, capture ports.Capture) {
	stream.capture = capture

	info := capture.Info()
	streamStoreSnapshots(stream, &info)
}

func beginManagerOperation(ctx context.Context, manager *Manager) error {
	if ctx == nil {
		return domain.ErrInvalidOptions
	}

	return errors.Join(managerStartOperation(manager))
}

func makeEventQueue(capacity int) []domain.Event {
	queue := make([]domain.Event, emptyQueueSize, capacity)

	return queue[:cap(queue)]
}

func waitingAcquisition(coordinator *Coordinator) acquisition {
	defer coordinator.mu.Unlock()

	return acquisition{lease: nil, changed: coordinator.changed}
}

func invalidCoordinatorState(coordinator *Coordinator) error {
	coordinator.mu.Unlock()

	return domain.ErrUnsupported
}

// resultError retains the error when an operation's value is irrelevant.
func resultError[T any](_ T, err error) error { return errors.Join(err) }

// ManagerView exposes operations while keeping mutable manager state private.
func ManagerView(
	state *Manager,
) *managerOperations[domain.DeviceInfo, domain.DeviceID, Device] {
	view := new(managerOperations[domain.DeviceInfo, domain.DeviceID, Device])

	view.Operations.Devices = func(ctx context.Context) ([]domain.DeviceInfo, error) {
		return managerDevices(ctx, state)
	}
	configureManagerOpen(view, state)

	view.Operations.Close = func() error { return managerClose(state) }

	return view
}

func configureManagerOpen(
	view *managerOperations[domain.DeviceInfo, domain.DeviceID, Device],
	state *Manager,
) {
	view.Operations.Open = func(ctx context.Context, id domain.DeviceID) (Device, error) {
		device, err := managerOpen(ctx, state, id)
		if err != nil {
			return nil, errors.Join(err)
		}

		return device, nil
	}
}

func streamView(
	state *stream,
) *deviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event] {
	view := new(deviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event])

	view.Operations.Info = func() domain.DeviceInfo { return streamInfo(state) }
	view.Operations.Capabilities = func() domain.Capabilities { return streamCapabilities(state) }
	view.Operations.Read = func(ctx context.Context) (domain.Event, error) { return streamRead(ctx, state) }
	configureDeviceClose(view, state)

	return view
}

func leasedSinkView(state leasedSink) *sinkOperations[domain.Event] {
	return sinkView(state, leasedSinkPublish, leasedSinkFail)
}

func streamSinkView(state *stream) *sinkOperations[domain.Event] {
	return sinkView(state, streamPublish, streamFail)
}

func configureDeviceClose(
	view *deviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event],
	state *stream,
) {
	view.Operations.Extension = func(target any) bool { return streamExtension(state, target) }
	view.Operations.Close = func() error { return streamClose(state) }
}

func sinkView[S any](
	state S,
	publish func(S, *domain.Event) bool,
	fail func(S, error),
) *sinkOperations[domain.Event] {
	view := new(sinkOperations[domain.Event])

	view.Operations.Publish = func(event *domain.Event) bool { return publish(state, event) }
	view.Operations.Fail = func(err error) { fail(state, err) }

	return view
}
