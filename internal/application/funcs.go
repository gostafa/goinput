package application

import (
	"context"
	"errors"
	"time"

	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
)

func NewCoordinator(factory ports.Factory, retrier ports.Retrier) *Coordinator {
	return &Coordinator{factory: factory, retrier: retrier, changed: make(chan struct{})}
}

func (c *Coordinator) signal() {
	close(c.changed)
	c.changed = make(chan struct{})
}

func (c *Coordinator) acquire(ctx context.Context) (*lease, error) {
	for {
		if err := context.Cause(ctx); err != nil {
			return nil, err
		}
		c.mu.Lock()
		switch c.state {
		case sessionRunning:
			c.refs++
			l := &lease{coordinator: c, backend: c.backend, generation: c.generation}
			c.mu.Unlock()
			return l, nil
		case sessionIdle:
			c.state = sessionStarting
			c.signal()
			c.mu.Unlock()
			backend, err := c.factory(ctx, c.retrier)
			if err == nil {
				err = context.Cause(ctx)
			} else {
				err = errors.Join(err, context.Cause(ctx))
			}
			if backend == nil && err == nil {
				err = domain.ErrUnsupported
			}
			if err != nil && backend != nil {
				err = errors.Join(err, backend.Close())
			}
			c.mu.Lock()
			if err != nil {
				c.state = sessionIdle
				c.signal()
				c.mu.Unlock()
				return nil, err
			}
			c.backend, c.refs, c.state = backend, 1, sessionRunning
			c.generation++
			l := &lease{coordinator: c, backend: backend, generation: c.generation}
			c.signal()
			c.mu.Unlock()
			return l, nil
		default:
			changed := c.changed
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, context.Cause(ctx)
			case <-changed:
			}
		}
	}
}

func (l *lease) release() error {
	l.once.Do(func() {
		c := l.coordinator
		c.mu.Lock()
		if c.state != sessionRunning || c.generation != l.generation {
			c.mu.Unlock()
			return
		}
		c.refs--
		if c.refs != 0 {
			c.mu.Unlock()
			return
		}
		c.state = sessionStopping
		c.signal()
		c.mu.Unlock()
		l.err = l.backend.Close()
		c.mu.Lock()
		c.backend, c.state = nil, sessionIdle
		c.signal()
		c.mu.Unlock()
	})
	return l.err
}

func (s leasedSink) Publish(event domain.Event) bool {
	c := s.lease.coordinator
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != sessionRunning || c.generation != s.lease.generation {
		return false
	}
	return s.sink.Publish(event)
}

func (s leasedSink) Fail(err error) {
	c := s.lease.coordinator
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == sessionRunning && c.generation == s.lease.generation {
		s.sink.Fail(err)
	}
}

func NewManager(options domain.Options, provider ports.Provider[*Coordinator]) (*Manager, error) {
	if options.BufferSize < 0 {
		return nil, domain.ErrInvalidOptions
	}
	if options.BufferSize == 0 {
		options.BufferSize = domain.DefaultBufferSize
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	return &Manager{
		options: options, provider: provider, devices: make(map[*stream]struct{}),
		leaseGate: make(chan struct{}, 1), ctx: ctx, cancel: cancel, closeDone: make(chan struct{}),
	}, nil
}

func (m *Manager) begin(ctx context.Context) (context.Context, func(), error) {
	if ctx == nil {
		panic("goinput: nil context")
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, nil, domain.ErrClosed
	}
	m.ops.Add(1)
	m.mu.Unlock()
	opctx, cancel := context.WithCancelCause(ctx)
	stop := context.AfterFunc(m.ctx, func() { cancel(domain.ErrClosed) })
	return opctx, func() { stop(); cancel(nil); m.ops.Done() }, nil
}

func (m *Manager) getLease(ctx context.Context) (*lease, error) {
	select {
	case m.leaseGate <- struct{}{}:
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	}
	defer func() { <-m.leaseGate }()
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	if m.lease != nil {
		return m.lease, nil
	}
	coordinator, err := m.provider.Get(ctx)
	if err != nil {
		return nil, err
	}
	l, err := coordinator.acquire(ctx)
	if err != nil {
		return nil, err
	}
	m.lease = l
	return l, nil
}

// Devices returns accessible endpoints. It may return partial results alongside
// an error containing discovery/permission diagnostics. Context bounds discovery,
// but synchronous native calls may take longer than its deadline to return.
func (m *Manager) Devices(ctx context.Context) ([]domain.DeviceInfo, error) {
	opctx, done, err := m.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	budget, cancel := context.WithTimeout(opctx, discoveryBudget)
	defer cancel()
	l, err := m.getLease(budget)
	if err != nil {
		return nil, &domain.OpError{Op: "devices", Err: err}
	}
	infos, err := l.backend.Discover(budget)
	if cause := context.Cause(budget); cause != nil {
		err = errors.Join(err, cause)
	}
	for i := range infos {
		infos[i] = domain.CloneInfo(infos[i])
	}
	if err != nil {
		err = &domain.OpError{Op: "devices", Err: err}
	}
	return infos, err
}

// Open starts an independent event stream. Opening the same endpoint twice
// creates two subscriptions, each with its own buffer and lifecycle.
func (m *Manager) Open(ctx context.Context, id domain.DeviceID) (Device, error) {
	opctx, done, err := m.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	if id == "" {
		return nil, &domain.OpError{Op: "open", Err: domain.ErrNotFound}
	}
	l, err := m.getLease(opctx)
	if err != nil {
		return nil, &domain.OpError{Op: "open", DeviceID: id, Err: err}
	}
	d := &stream{owner: m, id: id, queue: make([]domain.Event, m.options.BufferSize),
		notify: make(chan struct{}, 1), done: make(chan struct{}), closeDone: make(chan struct{})}
	capture, err := l.backend.Open(opctx, id, leasedSink{lease: l, sink: d})
	if err == nil && capture == nil {
		err = domain.ErrUnsupported
	}
	if err == nil {
		err = context.Cause(opctx)
	}
	if err != nil {
		err = errors.Join(err, context.Cause(opctx))
		d.Fail(err)
		if capture != nil {
			err = errors.Join(err, capture.Close())
		}
		return nil, &domain.OpError{Op: "open", DeviceID: id, Err: err}
	}
	d.capture = capture
	d.info = domain.CloneInfo(capture.Info())
	d.caps = domain.CloneCapabilities(capture.Capabilities())
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		d.Fail(domain.ErrClosed)
		_ = d.Close()
		return nil, domain.ErrClosed
	}
	m.devices[d] = struct{}{}
	m.mu.Unlock()
	go func() { <-d.done; _ = d.Close() }()
	return d, nil
}

// Close cancels pending manager operations, closes all captures, and releases the
// shared session lease. Native resources stop after the last manager releases.
func (m *Manager) Close() error {
	m.closeOnce.Do(func() {
		m.mu.Lock()
		m.closed = true
		m.cancel(domain.ErrClosed)
		m.mu.Unlock()
		m.ops.Wait()
		m.mu.Lock()
		devices := make([]*stream, 0, len(m.devices))
		for d := range m.devices {
			devices = append(devices, d)
		}
		m.mu.Unlock()
		for _, d := range devices {
			m.closeErr = errors.Join(m.closeErr, d.Close())
		}
		if m.lease != nil {
			m.closeErr = errors.Join(m.closeErr, m.lease.release())
		}
		close(m.closeDone)
	})
	<-m.closeDone
	return m.closeErr
}

func (d *stream) Info() domain.DeviceInfo           { return domain.CloneInfo(d.info) }
func (d *stream) Capabilities() domain.Capabilities { return domain.CloneCapabilities(d.caps) }

func (d *stream) Extension(target any) bool {
	d.mu.Lock()
	closed := d.terminal != nil
	d.mu.Unlock()
	if closed {
		return false
	}
	return d.capture.Extension(target)
}

func (d *stream) Publish(event domain.Event) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.terminal != nil {
		return false
	}
	if d.size == len(d.queue) {
		d.failLocked(domain.ErrEventLoss)
		return false
	}
	if event.DeviceID == "" {
		event.DeviceID = d.id
	}
	if event.Timestamp.ReceivedAt.IsZero() {
		event.Timestamp.ReceivedAt = time.Now()
	}
	if event.Timestamp.Time.IsZero() {
		event.Timestamp.Time = event.Timestamp.ReceivedAt.UTC()
		event.Timestamp.Source = domain.TimestampReceipt
	}
	d.queue[(d.head+d.size)%len(d.queue)] = event
	d.size++
	select {
	case d.notify <- struct{}{}:
	default:
	}
	return true
}

func (d *stream) failLocked(err error) {
	if d.terminal != nil {
		return
	}
	if err == nil {
		err = domain.ErrDisconnected
	}
	d.terminal = &domain.OpError{Op: "read", DeviceID: d.id, Err: err}
	clear(d.queue)
	d.size = 0
	close(d.done)
}

func (d *stream) Fail(err error) {
	d.mu.Lock()
	d.failLocked(err)
	d.mu.Unlock()
}

func (d *stream) Read(ctx context.Context) (domain.Event, error) {
	if ctx == nil {
		panic("goinput: nil context")
	}
	for {
		if err := context.Cause(ctx); err != nil {
			return domain.Event{}, err
		}
		d.mu.Lock()
		if d.terminal != nil {
			err := d.terminal
			d.mu.Unlock()
			return domain.Event{}, err
		}
		if d.size != 0 {
			event := d.queue[d.head]
			d.queue[d.head] = domain.Event{}
			d.head = (d.head + 1) % len(d.queue)
			d.size--
			if d.size > 0 {
				select {
				case d.notify <- struct{}{}:
				default:
				}
			}
			d.mu.Unlock()
			return event, nil
		}
		d.mu.Unlock()
		select {
		case <-ctx.Done():
			return domain.Event{}, context.Cause(ctx)
		case <-d.done:
		case <-d.notify:
		}
	}
}

func (d *stream) Close() error {
	d.Fail(domain.ErrClosed)
	d.closeOnce.Do(func() {
		d.closeErr = d.capture.Close()
		d.owner.mu.Lock()
		delete(d.owner.devices, d)
		d.owner.mu.Unlock()
		close(d.closeDone)
	})
	<-d.closeDone
	return d.closeErr
}
